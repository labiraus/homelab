import importlib.util
import json
from pathlib import Path
import ssl
import unittest
from unittest.mock import patch

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
        data = {"resource": smoke.BASE + "/mcp", "authorization_servers": ["https://accounts.google.com"]}
        self.assertTrue(self.inspect("/.well-known/oauth-protected-resource", data)["ok"])
        data["resource"] = "https://unrelated.example/mcp"
        self.assertFalse(self.inspect("/.well-known/oauth-protected-resource", data)["ok"])

    def test_anonymous_identity_must_not_be_valid(self):
        self.assertTrue(self.inspect("/api/auth/status", {"mode": "none", "valid": False})["ok"])
        self.assertFalse(self.inspect("/api/auth/status", {"mode": "oidc", "valid": True})["ok"])
        self.assertFalse(self.inspect("/api/auth/status", {}, 403)["ok"])

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


if __name__ == "__main__":
    unittest.main()
