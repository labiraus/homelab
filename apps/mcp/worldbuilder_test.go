package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"pkg/api"
	"pkg/base"
	"strings"
	"testing"
	"time"
)

func TestWorldbuilderCatalogIsOptInAndDraftOnly(t *testing.T) {
	t.Setenv("WORLDBUILDER_MCP_ENABLED", "false")
	if _, ok := findToolOperationByName("worldbuilder.propose"); ok {
		t.Fatal("disabled tool exposed")
	}
	t.Setenv("WORLDBUILDER_MCP_ENABLED", "true")
	for _, name := range []string{"worlds", "canon", "search", "record", "propose"} {
		resolution, ok := findToolOperationByName("worldbuilder." + name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if name != "propose" && !operationIsReadOnly(resolution.operation) {
			t.Fatal("read not annotated")
		}
	}
	for _, name := range []string{"publish", "review", "submit", "administration", "sessions"} {
		if _, ok := findToolOperationByName("worldbuilder." + name); ok {
			t.Fatal("human capability exposed")
		}
	}
}

func TestWorldbuilderProxySignsValidatedIdentityAndExactBody(t *testing.T) {
	t.Setenv("WORLDBUILDER_MCP_ENABLED", "true")
	key := strings.Repeat("k", 32)
	t.Setenv("WORLDBUILDER_MCP_KEY", key)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		raw, _ := io.ReadAll(r.Body)
		actor, err := api.VerifyDelegation([]byte(key), r.Header.Get("X-Worldbuilder-Delegation"), "worldbuilder-mcp-v1", r.Method, r.URL.RequestURI(), raw, time.Now())
		if err != nil || actor != "reader@example.test" {
			t.Fatalf("delegation: %s %v", actor, err)
		}
		if r.Header.Get("X-Forwarded-Email") != "" {
			t.Fatal("raw identity header forwarded")
		}
		var body map[string]any
		json.Unmarshal(raw, &body)
		if body["operation"] != "canon" || body["worldId"] != "world" {
			t.Fatal("operation not bound")
		}
		w.Write([]byte(`{"revision":1}`))
	}))
	defer server.Close()
	t.Setenv("API_BASE_URL", server.URL)
	args := map[string]any{"worldId": "world", "timeline": "main"}
	rawIdentity := context.WithValue(context.Background(), base.UserID, "reader@example.test")
	if _, err := proxyWorldbuilder(rawIdentity, "canon", args); err == nil || calls != 0 {
		t.Fatal("unvalidated context accepted")
	}
	ctx := api.WithAuthStatus(rawIdentity, api.AuthStatus{Email: "reader@example.test", Mode: api.AuthModeOIDC, Valid: true})
	if _, err := proxyWorldbuilder(ctx, "canon", args); err != nil || calls != 1 {
		t.Fatalf("valid proxy: %+v calls %d", err, calls)
	}
	request := httptest.NewRequest("POST", "/mcp", nil).WithContext(ctx)
	statusCode, response := handleMCPRequest(ctx, request, jsonRPCRequest{JSONRPC: "2.0", ID: "world-read", Method: "tools/call", Params: mustMarshalParams(t, map[string]any{"name": "worldbuilder.canon", "arguments": args})})
	if statusCode != 200 || response.Error != nil || calls != 2 {
		t.Fatalf("tool dispatch: %+v calls %d", response, calls)
	}
}
