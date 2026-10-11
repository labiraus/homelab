package main

import (
	"bytes"
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

func TestWorldbuilderExactTicksThroughToolDelegation(t *testing.T) {
	t.Setenv("WORLDBUILDER_MCP_ENABLED", "true")
	key := strings.Repeat("k", 32)
	t.Setenv("WORLDBUILDER_MCP_KEY", key)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if _, err := api.VerifyDelegation([]byte(key), r.Header.Get("X-Worldbuilder-Delegation"), "worldbuilder-mcp-v1", r.Method, r.URL.RequestURI(), raw, time.Now()); err != nil {
			t.Error(err)
		}
		if !bytes.Contains(raw, []byte("9223372036854775807")) || bytes.Contains(raw, []byte("9223372036854776000")) {
			t.Errorf("rounded signed request: %s", raw)
		}
		w.Write([]byte(`{"records":[{"id":"epoch","kind":"event","from":-9223372036854775808,"until":9223372036854775807}]}`))
	}))
	defer server.Close()
	t.Setenv("API_BASE_URL", server.URL)
	ctx := api.WithAuthStatus(context.Background(), api.AuthStatus{Email: "reader@example.test", Mode: api.AuthModeOIDC, Valid: true})
	for _, params := range []string{
		`{"name":"worldbuilder.canon","arguments":{"worldId":"world","timeline":"main","at":9223372036854775807}}`,
		`{"name":"worldbuilder.canon","arguments":{"worldId":"world","timeline":"main","at":"9223372036854775807"}}`,
		`{"name":"worldbuilder.propose","arguments":{"worldId":"world","id":"test","baseRevision":1,"reason":"exact","operations":[{"action":"put","id":"epoch","record":{"id":"epoch","kind":"event","from":-9223372036854775808,"until":9223372036854775807}}]}}`,
	} {
		request := httptest.NewRequest("POST", "/mcp", nil).WithContext(ctx)
		_, response := handleMCPRequest(ctx, request, jsonRPCRequest{JSONRPC: "2.0", ID: "exact", Method: "tools/call", Params: json.RawMessage(params)})
		raw, err := json.Marshal(response)
		if err != nil || response.Error != nil || !bytes.Contains(raw, []byte(`"structuredContent":{"records":[{"from":-9223372036854775808`)) || !bytes.Contains(raw, []byte(`"until":9223372036854775807`)) {
			t.Fatalf("structured evidence lost precision: %s %v", raw, err)
		}
	}
	resolution, _ := findToolOperationByName("worldbuilder.canon")
	for _, invalid := range []any{true, 1.5, []any{1}, map[string]any{}} {
		if err := validateToolArguments(map[string]any{"worldId": "world", "timeline": "main", "at": invalid}, toToolInputSchema(resolution.operation)); err == nil {
			t.Fatalf("invalid date type accepted: %v", invalid)
		}
	}
}

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
