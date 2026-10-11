package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestMCPOAuthStartup(t *testing.T) {
	dsn := os.Getenv("MCP_OAUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("owned OAuth PostgreSQL fixture not configured")
	}
	t.Setenv("MCP_OAUTH_ENABLED", "true")
	t.Setenv("MCP_OAUTH_DATABASE_URL", dsn)
	t.Setenv("MCP_OAUTH_ISSUER_URL", "https://mcp.example.test")
	t.Setenv("MCP_OAUTH_COOKIE_AUTH_URL", "http://proxy.homelab.svc.cluster.local/oauth2/auth")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mux := http.NewServeMux()
	if err := registerMCPOAuth(ctx, mux); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil))
	if response.Code != 200 {
		t.Fatal(response.Code)
	}
	t.Setenv("MCP_OAUTH_DATABASE_URL", os.Getenv("MCP_OAUTH_TEST_ADMIN_DATABASE_URL"))
	if err := registerMCPOAuth(ctx, http.NewServeMux()); err == nil {
		t.Fatal("privileged owner accepted as OAuth runtime")
	}
	t.Setenv("MCP_OAUTH_DATABASE_URL", dsn)
	t.Setenv("MCP_OAUTH_COOKIE_AUTH_URL", "https://evil.example/oauth2/auth")
	if err := registerMCPOAuth(ctx, http.NewServeMux()); err == nil {
		t.Fatal("external cookie validation target accepted")
	}
}
