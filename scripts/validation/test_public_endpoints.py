import importlib.util
import json
from pathlib import Path
import ssl
import subprocess
import unittest
from unittest.mock import patch, MagicMock

spec = importlib.util.spec_from_file_location("public_endpoints", Path(__file__).with_name("public-endpoints.py"))
smoke = importlib.util.module_from_spec(spec)
spec.loader.exec_module(smoke)


class PublicEndpointTests(unittest.TestCase):
    def inspect(self, path, data, status=200, headers=None):
        return smoke.inspect_response(path, status, headers or {"Content-Type": "application/json"}, json.dumps(data).encode())

    def test_discovery_requires_actual_labiraus_contract(self):
        body = {"server": {"name": "labiraus", "version": "0.0.0-uat.123"},
                "transports": [{"type": "streamable-http", "url": smoke.BASE + "/mcp"}]}
        self.assertTrue(self.inspect("/.well-known/mcp.json", body)["ok"])
        body["transports"][0]["url"] = "https://unrelated.example/mcp"
        self.assertFalse(self.inspect("/.well-known/mcp.json", body)["ok"])
        self.assertFalse(self.inspect("/.well-known/mcp.json", {"server": None})["ok"])

    def test_filter_page_and_redirect_are_not_discovery_success(self):
        for status in (200, 302, 403, 503):
            self.assertFalse(self.inspect("/.well-known/mcp.json", "block page", status, {"Content-Type": "text/html"})["ok"])

    def test_oauth_resource_is_bound_to_our_host(self):
        data = {"resource": smoke.BASE + "/mcp", "authorization_servers": [smoke.BASE], "scopes_supported": ["mcp"]}
        self.assertTrue(self.inspect("/.well-known/oauth-protected-resource", data)["ok"])
        for servers in (["https://accounts.google.com"], [smoke.BASE, "https://unrelated.example"], []):
            self.assertFalse(self.inspect("/.well-known/oauth-protected-resource", {**data, "authorization_servers": servers})["ok"])
        data["resource"] = "https://unrelated.example/mcp"
        self.assertFalse(self.inspect("/.well-known/oauth-protected-resource", data)["ok"])

    def test_native_issuer_requires_pinned_endpoints_and_pkce(self):
        data = {"issuer": smoke.BASE, "authorization_endpoint": smoke.BASE + "/api/mcp-oauth/authorize",
                "token_endpoint": smoke.BASE + "/mcp-oauth/token", "registration_endpoint": smoke.BASE + "/mcp-oauth/register",
                "revocation_endpoint": smoke.BASE + "/mcp-oauth/revoke", "code_challenge_methods_supported": ["S256"],
                "response_types_supported": ["code"], "token_endpoint_auth_methods_supported": ["none"],
                "scopes_supported": ["mcp"], "grant_types_supported": ["authorization_code", "refresh_token"]}
        path = "/.well-known/oauth-authorization-server"
        self.assertTrue(self.inspect(path, data)["ok"])
        for key, value in (("issuer", "https://unrelated.example"), ("token_endpoint", "http://mcp.labiraus.com/mcp-oauth/token"),
                           ("registration_endpoint", "https://unrelated.example/register"), ("code_challenge_methods_supported", ["plain"]),
                           ("response_types_supported", ["token"]), ("grant_types_supported", ["authorization_code"]),
                           ("scopes_supported", [])):
            self.assertFalse(self.inspect(path, {**data, key: value})["ok"], key)

    def test_anonymous_identity_must_not_be_valid(self):
        self.assertTrue(self.inspect("/api/auth/status", {"mode": "none", "valid": False})["ok"])
        self.assertFalse(self.inspect("/api/auth/status", {"mode": "oidc", "valid": True})["ok"])
        self.assertFalse(self.inspect("/api/auth/status", {}, 403)["ok"])

    def test_transport_must_refuse_data_even_when_discovery_is_public(self):
        for path in smoke.TRANSPORT_PATHS:
            for status in (401, 403):
                self.assertTrue(self.inspect(path, {}, status)["ok"])
            for status in (200, 202, 404, 500, 503):
                self.assertFalse(self.inspect(path, {}, status)["ok"])
            self.assertFalse(self.inspect(path, {}, 200, {"Content-Type": "text/event-stream"})["ok"])
            self.assertTrue(self.inspect(path, {}, 302, {"Location": "/oauth2/start?state=private"})["ok"])

    def test_redirect_does_not_leak_query_or_accept_other_origins(self):
        result = self.inspect("/api/auth/status", {}, 302, {"Location": "/oauth2/start?state=private"})
        self.assertTrue(result["ok"])
        self.assertNotIn("private", json.dumps(result))
        for url in ("https://unrelated.example/oauth2/start", "http://mcp.labiraus.com/oauth2/start"):
            self.assertFalse(self.inspect("/api/auth/status", {}, 302, {"Location": url})["ok"])

    def test_tls_failure_never_sends_http(self):
        error = ssl.SSLCertVerificationError(1, "mismatch")
        # Real OpenSSL errors carry a string reason; only URLError.reason wraps an exception.
        error.reason = "CERTIFICATE_VERIFY_FAILED"
        with patch.object(smoke.socket, "getaddrinfo", return_value=[(0, 0, 0, "", ("192.0.2.1", 443))]), \
             patch.object(smoke.socket, "create_connection", side_effect=error), \
             patch.object(smoke.urllib.request, "build_opener") as opener:
            result = smoke.probe()
        self.assertFalse(result["ok"])
        self.assertEqual(result["error"], "tls_certificate_verification_failed")
        self.assertEqual(result["tls_reason"], "CERTIFICATE_VERIFY_FAILED")
        opener.assert_not_called()

    def test_healthy_old_release_does_not_confirm_new_deployment(self):
        report = {"ok": True, "checks": [{"path": "/.well-known/mcp.json", "mcp_version": "local"}]}
        self.assertFalse(smoke.require_version(report, "0.0.0-uat.123")["ok"])
        self.assertEqual(report["error"], "published_mcp_version_not_observed")
        report = {"ok": True, "checks": [{"path": "/.well-known/mcp.json", "mcp_version": "0.0.0-uat.123"}]}
        self.assertTrue(smoke.require_version(report, "0.0.0-uat.123")["ok"])

    def test_diagnostics_keep_verification_and_redact_http_redirect(self):
        response = MagicMock()
        response.__enter__.return_value = response
        response.status = 302
        response.headers = {"Location": "https://unrelated.example/path?private=value", "Set-Cookie": "secret=value"}
        opener = MagicMock()
        opener.open.return_value = response
        with patch.object(smoke.subprocess, "run", return_value=subprocess.CompletedProcess([], 1, b"", b"")) as run, \
             patch.object(smoke.urllib.request, "build_opener", return_value=opener):
            report = smoke.connection_diagnostics()
        self.assertIn("-verify_return_error", run.call_args.args[0])
        self.assertIn("-verify_hostname", run.call_args.args[0])
        request = opener.open.call_args.args[0]
        self.assertEqual(request.full_url, "http://mcp.labiraus.com/.well-known/acme-challenge/homelab-readonly-probe")
        self.assertNotIn("Authorization", request.headers)
        self.assertNotIn("private", json.dumps(report))
        self.assertNotIn("secret", json.dumps(report))
        self.assertEqual(report["tls_verification_exit"], 1)
        response.read.assert_not_called()


if __name__ == "__main__":
    unittest.main()
