#!/usr/bin/env python3
"""Credential-free public smoke check; never treats publication as Flux readiness."""
import argparse
import datetime
import json
import os
from pathlib import Path
import socket
import ssl
import time
import urllib.error
import urllib.parse
import urllib.request

BASE = "https://mcp.labiraus.com"
LIMIT = 512 * 1024


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def inspect_response(path, status, headers, body):
    """Return public metadata only, never cookies, response bodies or redirect queries."""
    result = {"path": path, "status": status, "ok": False}
    content_type = headers.get("Content-Type", "").split(";")[0].strip()
    result["content_type"] = content_type
    if path == "/api/auth/status":
        if status in (302, 303, 307):
            location = urllib.parse.urlsplit(headers.get("Location", ""))
            target = urllib.parse.urljoin(BASE + path, headers.get("Location", ""))
            parsed = urllib.parse.urlsplit(target)
            result["ok"] = (parsed.scheme == "https" and parsed.hostname == "mcp.labiraus.com"
                            and parsed.path in ("/oauth2/start", "/oauth2/sign_in")) or (
                parsed.scheme == "https" and parsed.hostname == "accounts.google.com"
                and parsed.path.startswith("/o/oauth2/"))
            result["redirect_host"] = location.hostname
            result["redirect_path"] = location.path
        elif status == 200 and content_type == "application/json":
            try:
                data = json.loads(body)
                result["ok"] = isinstance(data, dict) and data.get("valid") is False and data.get("mode") == "none"
            except (ValueError, TypeError):
                pass
        elif status == 401:
            result["ok"] = True
        if not result["ok"]:
            result["error"] = "unexpected_anonymous_auth_response"
        return result
    if status != 200 or content_type != "application/json":
        result["error"] = "discovery_requires_200_json"
        return result
    try:
        data = json.loads(body)
        if not isinstance(data, dict):
            raise ValueError("object required")
        if path == "/.well-known/mcp.json":
            server = data.get("server", {})
            result["ok"] = isinstance(server, dict) and server.get("name") == "labiraus" and any(
                isinstance(t, dict) and t.get("type") == "streamable-http" and t.get("url") == BASE + "/mcp"
                for t in data.get("transports", [])
            )
            version = server.get("version") if isinstance(server, dict) else None
            if isinstance(version, str) and len(version) <= 100:
                result["mcp_version"] = version
        else:
            result["ok"] = data.get("resource") == BASE + "/mcp" and (
                "https://accounts.google.com" in data.get("authorization_servers", []))
        if not result["ok"]:
            result["error"] = "unexpected_discovery_contract"
    except (ValueError, TypeError):
        result["error"] = "invalid_discovery_json"
    return result


def error_category(error):
    if isinstance(error, urllib.error.URLError):
        error = error.reason
    if isinstance(error, ssl.SSLCertVerificationError):
        return "tls_certificate_verification_failed"
    if isinstance(error, socket.gaierror):
        return "dns_lookup_failed"
    if isinstance(error, (TimeoutError, socket.timeout)):
        return "connection_timeout"
    if isinstance(error, ConnectionResetError):
        return "connection_reset"
    return "connection_failed"


def error_details(error):
    if isinstance(error, urllib.error.URLError):
        error = error.reason
    details = {"error": error_category(error), "exception": type(error).__name__}
    if isinstance(error, ssl.SSLError):
        details["tls_reason"] = getattr(error, "reason", None)
    if isinstance(error, ssl.SSLCertVerificationError):
        details["verify_code"] = getattr(error, "verify_code", None)
        details["verify_message"] = getattr(error, "verify_message", None)
    return details


def probe():
    host = urllib.parse.urlsplit(BASE).hostname
    report = {"time": datetime.datetime.now(datetime.timezone.utc).isoformat(), "ok": False, "checks": []}
    stage = "dns"
    try:
        report["addresses"] = sorted({x[4][0] for x in socket.getaddrinfo(host, 443, type=socket.SOCK_STREAM)})
        context = ssl.create_default_context()
        stage = "tcp"
        with socket.create_connection((host, 443), timeout=10) as raw:
            stage = "tls"
            with context.wrap_socket(raw, server_hostname=host) as connection:
                cert = connection.getpeercert()
                report["tls"] = {"version": connection.version(), "expires": cert.get("notAfter"),
                                 "dns_names": [v for k, v in cert.get("subjectAltName", []) if k == "DNS"]}
    except OSError as error:
        report.update(error_details(error))
        report["stage"] = stage
        return report
    opener = urllib.request.build_opener(NoRedirect, urllib.request.HTTPSHandler(context=context))
    for path in ("/.well-known/mcp.json", "/.well-known/oauth-protected-resource", "/api/auth/status"):
        try:
            req = urllib.request.Request(BASE + path, headers={"Accept": "application/json", "User-Agent": "homelab-public-smoke/1.0"})
            try:
                response = opener.open(req, timeout=10)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                body = response.read(LIMIT + 1)
                if len(body) > LIMIT:
                    result = {"path": path, "ok": False, "error": "response_too_large"}
                else:
                    result = inspect_response(path, response.status, response.headers, body)
        except (OSError, urllib.error.URLError) as error:
            result = {"path": path, "ok": False, **error_details(error)}
        report["checks"].append(result)
    report["ok"] = all(c["ok"] for c in report["checks"])
    return report


def require_version(report, expected):
    if not expected:
        return report
    report["expected_mcp_version"] = expected
    manifest = next((c for c in report["checks"] if c["path"] == "/.well-known/mcp.json"), {})
    if report["ok"] and manifest.get("mcp_version") != expected:
        report["ok"] = False
        report["error"] = "published_mcp_version_not_observed"
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--attempts", type=int, default=1)
    parser.add_argument("--interval", type=int, default=30)
    parser.add_argument("--output", default="public-endpoints.json")
    parser.add_argument("--expected-mcp-version", default="")
    args = parser.parse_args()
    if not 1 <= args.attempts <= 12 or not 0 <= args.interval <= 60:
        parser.error("attempts must be 1..12 and interval 0..60 seconds")
    reports = []
    for attempt in range(args.attempts):
        report = require_version(probe(), args.expected_mcp_version)
        reports.append(report)
        print(json.dumps(report), flush=True)
        Path(args.output).write_text(json.dumps({"target": BASE, "attempts": reports,
            "cluster_reconciliation": "unverified", "authenticated_functionality": "not_tested"}, indent=2) + "\n")
        if report["ok"]:
            break
        if attempt + 1 < args.attempts:
            time.sleep(args.interval)
    ok = reports[-1]["ok"]
    message = ("Public TLS, discovery and anonymous auth checks passed. " if ok else "Public endpoint checks failed: " +
               reports[-1].get("error", ", ".join(c.get("error", "") for c in reports[-1]["checks"] if not c["ok"])) + ". ")
    message += "Flux reconciliation and authenticated functionality are unverified."
    print(message)
    if os.environ.get("GITHUB_ACTIONS") == "true" and not ok:
        print("::error::" + message)
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as summary:
            summary.write("### Public endpoint validation\n\n" + message + "\n\nSee the sanitized JSON artifact for DNS/TLS and per-path results. No credentials or cluster mutations are used.\n")
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
