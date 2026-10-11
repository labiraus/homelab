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
		if nativeOAuthIssuer() != "" {
			serveNativeIdentity(next, w, r)
			return
		}
		status, ok := api.AuthStatusFromContext(r.Context())
		if !ok || !status.Valid || status.Email == "" || (status.Mode != api.AuthModeOIDC && status.Mode != api.AuthModeCertificate) {
			w.Header().Set("WWW-Authenticate", transportChallenge())
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

// Validate the credential at its authority independently of edge path matching
// and forwarded identity headers. Only this fixed private authority receives it.
func serveNativeIdentity(next http.Handler, w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	endpoint := strings.TrimRight(base.GetEnv("API_BASE_URL", defaultAPIBaseURL), "/") + "/api/mcp-oauth/check"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		http.Error(w, "Identity validation unavailable", 503)
		return
	}
	for _, name := range []string{"Authorization", "Cookie"} {
		if value := r.Header.Get(name); value != "" {
			request.Header.Set(name, value)
		}
	}
	client := *mcpHTTPClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(request)
	if err != nil {
		http.Error(w, "Identity validation unavailable", 503)
		return
	}
	defer response.Body.Close()
	if response.StatusCode == 401 || response.StatusCode == 403 {
		w.Header().Set("WWW-Authenticate", transportChallenge())
		http.Error(w, "Authentication required", 401)
		return
	}
	email := strings.ToLower(strings.TrimSpace(response.Header.Get("X-Auth-Request-Email")))
	if response.StatusCode != 200 || email == "" {
		http.Error(w, "Identity validation unavailable", 503)
		return
	}
	ctx = api.WithAuthStatus(r.Context(), api.AuthStatus{Mode: api.AuthModeOIDC, Email: email, Valid: true})
	ctx = api.WithUserID(ctx, email)
	next.ServeHTTP(w, r.WithContext(ctx))
}
