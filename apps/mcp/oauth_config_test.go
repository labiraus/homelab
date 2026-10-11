package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"pkg/api"
	"pkg/base"
	"pkg/prometheusutil"
	"testing"
)

func TestNativeOAuthDiscoveryMatchesCanonicalResource(t *testing.T) {
	base.ServiceName = "mcp_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())
	t.Setenv("MCP_OAUTH_ISSUER_URL", "https://mcp.example.test")
	request := httptest.NewRequest("GET", "https://untrusted.example/.well-known/oauth-protected-resource", nil)
	response := httptest.NewRecorder()
	oauthProtectedResourceAPI(response, request)
	var document oauthProtectedResourceDocument
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.Resource != "https://mcp.example.test/mcp" || len(document.AuthorizationServers) != 1 || document.AuthorizationServers[0] != "https://mcp.example.test" {
		t.Fatal(document)
	}
	if len(document.ScopesSupported) != 1 || document.ScopesSupported[0] != "mcp" || len(document.AccessModes) != 1 || document.AccessModes[0].Name != "labiraus-oauth" {
		t.Fatal(document)
	}
	if transportChallenge() != `Bearer resource_metadata="https://mcp.example.test/.well-known/oauth-protected-resource", scope="mcp"` {
		t.Fatal(transportChallenge())
	}
	manifest := newManifestDocument(request)
	if len(manifest.Authorization.Meta.AccessModes) != 1 || manifest.Authorization.Meta.ResourceMetadataURL != "https://mcp.example.test/.well-known/oauth-protected-resource" {
		t.Fatal(manifest.Authorization)
	}
}

func TestNativeTransportRevalidatesCredentialAndPreservesStreamContext(t *testing.T) {
	t.Setenv("MCP_OAUTH_ISSUER_URL", "https://mcp.example.test")
	authority := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/mcp-oauth/check" {
			t.Error("wrong authority path", r.URL.Path)
		}
		if r.Header.Get("X-Auth-Request-Email") != "" || r.Header.Get("X-Forwarded-Email") != "" {
			t.Error("untrusted identity forwarded")
		}
		if r.Header.Get("Authorization") != "Bearer owned-native-fixture" {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("X-Auth-Request-Email", "real@example.test")
		w.WriteHeader(200)
	}))
	defer authority.Close()
	t.Setenv("API_BASE_URL", authority.URL)
	called := false
	handler := requireTransportIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		status, ok := api.AuthStatusFromContext(r.Context())
		if !ok || !status.Valid || status.Email != "real@example.test" {
			t.Error("forged identity retained", status)
		}
		if user, ok := api.UserIDFromContext(r.Context()); !ok || user != "real@example.test" {
			t.Error("wrong tool identity", user)
		}
		if _, bounded := r.Context().Deadline(); bounded {
			t.Error("short authority timeout applied to long-lived MCP stream")
		}
		w.WriteHeader(200)
	}))
	for _, credential := range []string{"", "Bearer bad-fixture", "Bearer owned-native-fixture"} {
		called = false
		request := httptest.NewRequest("GET", "/mcp", nil)
		request.Header.Set("Authorization", credential)
		request.Header.Set("X-Auth-Request-Email", "forged@example.test")
		request = request.WithContext(api.WithAuthStatus(request.Context(), api.AuthStatus{Mode: api.AuthModeOIDC, Email: "forged@example.test", Valid: true}))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if credential == "Bearer owned-native-fixture" {
			if response.Code != 200 || !called {
				t.Fatal("valid credential refused")
			}
		} else if response.Code != 401 || called {
			t.Fatal("forged edge identity accepted")
		}
	}
}
