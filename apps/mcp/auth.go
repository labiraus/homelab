package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"pkg/api"
	"pkg/base"
	"strings"
	"time"
)

// The gateway authenticates the caller; external remains the registered-user authority.
func requireTransportIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		protected := false
		for _, path := range []string{"/mcp", "/sse", "/message", "/messages"} {
			protected = protected || r.URL.Path == path || strings.HasPrefix(r.URL.Path, path+"/")
		}
		if !protected || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		status, ok := api.AuthStatusFromContext(r.Context())
		if !ok || !status.Valid || status.Email == "" || (status.Mode != api.AuthModeOIDC && status.Mode != api.AuthModeCertificate) {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="https://mcp.labiraus.com/.well-known/oauth-protected-resource"`)
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		endpoint := strings.TrimRight(base.GetEnv("API_BASE_URL", defaultAPIBaseURL), "/") + "/api/auth/status"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			http.Error(w, "Identity validation unavailable", http.StatusServiceUnavailable)
			return
		}
		// Forward only the normalized edge-authenticated identity, never caller headers.
		req.Header.Set("X-Forwarded-Email", status.Email)
		client := *mcpHTTPClient
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "Identity validation unavailable", http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()
		var verified api.AuthStatus
		if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&verified) != nil {
			http.Error(w, "Identity validation unavailable", http.StatusServiceUnavailable)
			return
		}
		if !verified.Valid || verified.Email != status.Email {
			http.Error(w, "Registered identity required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
