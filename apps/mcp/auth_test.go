package main

import (
	"net/http"
	"net/http/httptest"
	"pkg/api"
	"testing"
)

func TestTransportRequiresRegisteredIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, email, response string
		upstream, want        int
	}{
		{"anonymous", "", "", 200, 401},
		{"unknown", "unknown@example.com", `{"valid":false,"email":"unknown@example.com"}`, 200, 403},
		{"wrong-identity", "alice@example.com", `{"valid":true,"email":"bob@example.com"}`, 200, 403},
		{"unavailable", "alice@example.com", `{}`, 503, 503},
		{"malformed", "alice@example.com", `not-json`, 200, 503},
		{"redirect", "alice@example.com", `{}`, 302, 503},
		{"registered", "alice@example.com", `{"valid":true,"email":"alice@example.com","mode":"oidc"}`, 200, 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/api/auth/status" || r.Header.Get("X-Forwarded-Email") != tc.email || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
					t.Error("identity lookup forwarded unexpected path or credentials")
				}
				w.WriteHeader(tc.upstream)
				_, _ = w.Write([]byte(tc.response))
			}))
			defer upstream.Close()
			t.Setenv("API_BASE_URL", upstream.URL)
			for _, path := range []string{"/mcp", "/sse", "/messages", "/message"} {
				r := httptest.NewRequest(http.MethodPost, path, nil)
				r.Header.Set("Cookie", "must-not-forward")
				r.Header.Set("Authorization", "must-not-forward")
				if tc.email != "" {
					r = r.WithContext(api.WithAuthStatus(r.Context(), api.AuthStatus{Mode: api.AuthModeOIDC, Email: tc.email, Valid: true}))
				}
				w := httptest.NewRecorder()
				requireTransportIdentity(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })).ServeHTTP(w, r)
				if w.Code != tc.want {
					t.Fatalf("%s: got %d, want %d", path, w.Code, tc.want)
				}
			}
			if tc.email == "" && calls != 0 {
				t.Fatal("anonymous request reached identity authority")
			}
		})
	}
}

func TestTransportGatePreservesDiscoveryHealthAndPreflight(t *testing.T) {
	for _, path := range []string{"/.well-known/mcp.json", "/.well-known/oauth-protected-resource", "/readiness", "/liveness", "/metrics"} {
		w := httptest.NewRecorder()
		requireTransportIdentity(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })).ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 204 {
			t.Fatalf("public endpoint %s blocked", path)
		}
	}
	w := httptest.NewRecorder()
	requireTransportIdentity(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })).ServeHTTP(w, httptest.NewRequest("OPTIONS", "/mcp", nil))
	if w.Code != 204 {
		t.Fatal("CORS handler was not reached")
	}
}
