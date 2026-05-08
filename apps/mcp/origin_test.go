package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"pkg/base"
	"pkg/prometheusutil"
)

func TestMCPPostRejectsDisallowedOrigin(t *testing.T) {
	base.ServiceName = "mcp_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":"1","method":"initialize","params":{"protocolVersion":"2025-11-25"}}`)
	request.Host = "mcp.labiraus.com"
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("Origin", "https://evil.example")

	mcpPostAPI(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}

	var response jsonRPCResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json-rpc error payload: %v", err)
	}
	if response.Error == nil || response.Error.Message != "Origin is not allowed" {
		t.Fatalf("expected origin validation error, got %#v", response.Error)
	}
}

func TestMCPGetRejectsDisallowedOrigin(t *testing.T) {
	base.ServiceName = "mcp_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	session := sessionRegistry.create(supportedProtocolVersions[0])
	request, recorder := httptestJSONRequest(t, http.MethodGet, "/mcp", "")
	request.Host = "mcp.labiraus.com"
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("MCP-Session-Id", session.ID)
	request.Header.Set("MCP-Protocol-Version", session.ProtocolVersion)
	request.Header.Set("Origin", "https://evil.example")

	mcpGetAPI(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}

	var response jsonRPCResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json-rpc error payload: %v", err)
	}
	if response.Error == nil || response.Error.Message != "Origin is not allowed" {
		t.Fatalf("expected origin validation error, got %#v", response.Error)
	}
}
