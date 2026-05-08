package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBuildWellKnownManifestIncludesLiveAndPlannedCapabilities(t *testing.T) {
	request, _ := httptestJSONRequest(t, http.MethodGet, "/.well-known/mcp.json", "")
	request.Host = "mcp.labiraus.com"
	request.Header.Set("X-Forwarded-Proto", "https")

	manifest := buildManifest(request)

	if manifest.Authorization == nil || manifest.Authorization.Meta == nil {
		t.Fatalf("expected authorization metadata to be present")
	}

	if manifest.Authorization.Meta.ResourceMetadataURL != "https://mcp.labiraus.com/.well-known/oauth-protected-resource" {
		t.Fatalf("unexpected resource metadata URL: %q", manifest.Authorization.Meta.ResourceMetadataURL)
	}
	if manifest.Authorization.Meta.Requirement != "one-of" {
		t.Fatalf("expected one-of access requirement, got %q", manifest.Authorization.Meta.Requirement)
	}
	if len(manifest.Prompts) == 0 {
		t.Fatalf("expected prompts to be published in the manifest")
	}
	if !manifestHasTransport(manifest, "streamable-http", "https://mcp.labiraus.com/mcp") {
		t.Fatalf("expected streamable-http transport in manifest, got %#v", manifest.Transports)
	}
	if !manifestHasTransport(manifest, "sse", "https://mcp.labiraus.com/sse") {
		t.Fatalf("expected legacy sse transport in manifest, got %#v", manifest.Transports)
	}

	liveTool := findToolInManifest(t, manifest, "documents.submit")
	if liveTool.Meta.Lifecycle != manifestLifecycleLive {
		t.Fatalf("expected live tool lifecycle, got %q", liveTool.Meta.Lifecycle)
	}
	if liveTool.Meta.Backend != manifestBackendOrchestrator {
		t.Fatalf("expected orchestrator backend, got %q", liveTool.Meta.Backend)
	}

	listTool := findToolInManifest(t, manifest, "minio.documents.listFolder")
	if listTool.Annotations == nil || !listTool.Annotations.ReadOnlyHint {
		t.Fatalf("expected folder listing tool to be marked read-only, got %#v", listTool.Annotations)
	}

	scanTool := findToolInManifest(t, manifest, "documents.scanBucket")
	if scanTool.Meta.Lifecycle != manifestLifecycleLive {
		t.Fatalf("expected live scan tool lifecycle, got %q", scanTool.Meta.Lifecycle)
	}
	if scanTool.Meta.ExecutionMode != manifestExecutionModeHTTPProxy {
		t.Fatalf("expected scan tool to proxy to orchestrator, got %q", scanTool.Meta.ExecutionMode)
	}

	curationTool := findToolInManifest(t, manifest, "documents.curation.update")
	if curationTool.Meta.Lifecycle != manifestLifecycleLive {
		t.Fatalf("expected live curation tool lifecycle, got %q", curationTool.Meta.Lifecycle)
	}
	if curationTool.Meta.ExecutionMode != manifestExecutionModeHTTPProxy {
		t.Fatalf("expected curation tool to proxy to orchestrator, got %q", curationTool.Meta.ExecutionMode)
	}

	editTool := findToolInManifest(t, manifest, "documents.editText")
	if editTool.Meta.Lifecycle != manifestLifecycleLive {
		t.Fatalf("expected live edit tool lifecycle, got %q", editTool.Meta.Lifecycle)
	}
	if editTool.Meta.ExecutionMode != manifestExecutionModeHTTPProxy {
		t.Fatalf("expected edit tool to proxy to orchestrator, got %q", editTool.Meta.ExecutionMode)
	}
	if editTool.Annotations == nil || !editTool.Annotations.DestructiveHint {
		t.Fatalf("expected edit tool to be marked destructive, got %#v", editTool.Annotations)
	}

	reprocessTool := findToolInManifest(t, manifest, "documents.reprocess")
	if reprocessTool.Meta.Lifecycle != manifestLifecycleLive {
		t.Fatalf("expected live reprocess tool lifecycle, got %q", reprocessTool.Meta.Lifecycle)
	}
	if reprocessTool.Meta.ExecutionMode != manifestExecutionModeHTTPProxy {
		t.Fatalf("expected reprocess tool to proxy to orchestrator, got %q", reprocessTool.Meta.ExecutionMode)
	}

	searchTool := findToolInManifest(t, manifest, "documents.search")
	if searchTool.Meta.ExecutionMode != manifestExecutionModeDocumentSearch {
		t.Fatalf("expected document search execution mode, got %q", searchTool.Meta.ExecutionMode)
	}

	moveTool := findToolInManifest(t, manifest, "minio.documents.moveObject")
	if moveTool.Meta.Lifecycle != manifestLifecycleLive {
		t.Fatalf("expected live move tool lifecycle, got %q", moveTool.Meta.Lifecycle)
	}

	template := findResourceTemplateInManifest(t, manifest, "minio.documents.object")
	if template.Meta.ExecutionMode != manifestExecutionModeMinIOGetObject {
		t.Fatalf("expected MinIO get execution mode, got %q", template.Meta.ExecutionMode)
	}

	notificationTemplate := findResourceTemplateInManifest(t, manifest, "documents.notifications.stream")
	if notificationTemplate.Meta.ExecutionMode != manifestExecutionModeNATSSubscription {
		t.Fatalf("expected NATS subscription execution mode, got %q", notificationTemplate.Meta.ExecutionMode)
	}

	authTemplate := findResourceTemplateInManifest(t, manifest, "postgres.auth.userByEmail")
	if authTemplate.Meta.ExecutionMode != manifestExecutionModePostgresUserByEmail {
		t.Fatalf("expected auth user lookup execution mode, got %q", authTemplate.Meta.ExecutionMode)
	}
}

func TestInitializeUsesLabirausServerInfo(t *testing.T) {
	request, _ := httptestJSONRequest(t, http.MethodPost, "/mcp", "")
	responseStatus, responseBody := handleMCPRequest(context.Background(), request, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "initialize",
		Params: mustMarshalParams(t, map[string]any{
			"protocolVersion": supportedProtocolVersions[0],
		}),
	})

	if responseStatus != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseStatus)
	}
	if responseBody == nil || responseBody.Error != nil {
		t.Fatalf("expected successful initialize response, got %#v", responseBody)
	}

	result, ok := responseBody.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected initialize result map, got %#v", responseBody.Result)
	}

	serverInfo, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("expected serverInfo map, got %#v", result["serverInfo"])
	}
	if serverInfo["name"] != "labiraus" {
		t.Fatalf("expected labiraus server name, got %#v", serverInfo["name"])
	}
}

func TestInitializeNegotiatesLegacyCodexProtocolVersion(t *testing.T) {
	request, _ := httptestJSONRequest(t, http.MethodPost, "/mcp", "")
	responseStatus, responseBody := handleMCPRequest(context.Background(), request, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "initialize",
		Params: mustMarshalParams(t, map[string]any{
			"protocolVersion": "2024-11-05",
		}),
	})

	if responseStatus != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseStatus)
	}
	if responseBody == nil || responseBody.Error != nil {
		t.Fatalf("expected successful initialize response, got %#v", responseBody)
	}

	result, ok := responseBody.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected initialize result map, got %#v", responseBody.Result)
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Fatalf("expected legacy protocol negotiation, got %#v", result["protocolVersion"])
	}
}

func TestMCPPostInitializeReturnsSSEMessageEventAndSession(t *testing.T) {
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":"1","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"codex-test","version":"0"}}}`)

	mcpPostAPI(recorder, request)

	response := assertRecorderJSONRPCEventResponse(t, recorder, http.StatusOK)
	if response.ID != "1" || response.Error != nil {
		t.Fatalf("expected initialize response with id 1, got %#v", response)
	}
	if recorder.Header().Get("MCP-Session-Id") == "" {
		t.Fatalf("expected initialize response to include %s", "MCP-Session-Id")
	}
}

func TestMCPPostAcceptsInitializedNotificationWithoutProtocolHeader(t *testing.T) {
	session := sessionRegistry.create(supportedProtocolVersions[0])
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	request.Header.Set("MCP-Session-Id", session.ID)

	mcpPostAPI(recorder, request)

	assertRecorderAcceptedNoBody(t, recorder)
}

func TestMCPPostAcceptsInitializedNotificationWithoutSessionHeader(t *testing.T) {
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	request.Header.Set("MCP-Protocol-Version", supportedProtocolVersions[0])

	mcpPostAPI(recorder, request)

	assertRecorderAcceptedNoBody(t, recorder)
}

func assertRecorderAcceptedNoBody(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusAccepted, recorder.Code, recorder.Body.String())
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("expected empty response body, got %q", recorder.Body.String())
	}
}

func assertRecorderJSONRPCEventResponse(t *testing.T, recorder *httptest.ResponseRecorder, status int) jsonRPCResponse {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("expected status %d, got %d with body %q", status, recorder.Code, recorder.Body.String())
	}
	if contentType := strings.TrimSpace(strings.Split(recorder.Header().Get("Content-Type"), ";")[0]); contentType != "text/event-stream" {
		t.Fatalf("expected text/event-stream content type, got %q", recorder.Header().Get("Content-Type"))
	}

	body := recorder.Body.String()
	const prefix = "event: message\ndata: "
	if !strings.HasPrefix(body, prefix) || !strings.HasSuffix(body, "\n\n") {
		t.Fatalf("expected JSON-RPC SSE message event, got body %q", body)
	}

	payload := strings.TrimSuffix(strings.TrimPrefix(body, prefix), "\n\n")
	var response jsonRPCResponse
	if err := json.Unmarshal([]byte(payload), &response); err != nil {
		t.Fatalf("expected valid json-rpc response in SSE data: %v", err)
	}
	return response
}

type cancelOnFlushRecorder struct {
	*httptest.ResponseRecorder
	cancel context.CancelFunc
}

func (recorder *cancelOnFlushRecorder) Flush() {
	recorder.ResponseRecorder.Flush()
	if recorder.cancel != nil {
		recorder.cancel()
		recorder.cancel = nil
	}
}

func TestStreamableHTTPGetEmitsInitialCommentOnly(t *testing.T) {
	session := sessionRegistry.create(supportedProtocolVersions[0])
	ctx, cancel := context.WithCancel(context.Background())
	request, _ := httptestJSONRequest(t, http.MethodGet, "/mcp", "")
	request = request.WithContext(ctx)
	recorder := &cancelOnFlushRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		cancel:           cancel,
	}

	if err := serveSessionStream(recorder, request, session.ID); err != nil {
		t.Fatalf("expected stream to close cleanly after context cancellation: %v", err)
	}

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if body := recorder.Body.String(); body != ": connected\n\n" {
		t.Fatalf("expected stream to start with an SSE comment only, got body %q", body)
	}
}

func TestLegacySSEStillEmitsEndpointEvent(t *testing.T) {
	session := sessionRegistry.create("2024-11-05")
	ctx, cancel := context.WithCancel(context.Background())
	request, _ := httptestJSONRequest(t, http.MethodGet, "/sse", "")
	request = request.WithContext(ctx)
	recorder := &cancelOnFlushRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		cancel:           cancel,
	}

	if err := serveSessionStreamWithOptions(recorder, request, session.ID, sessionStreamOptions{
		Endpoint:        "/messages?sessionId=" + session.ID,
		UseMessageEvent: true,
	}); err != nil {
		t.Fatalf("expected legacy stream to close cleanly after context cancellation: %v", err)
	}

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "event: endpoint\n") || !strings.Contains(body, "/messages?sessionId="+session.ID) {
		t.Fatalf("expected legacy endpoint event, got body %q", body)
	}
}

func TestMCPPostAcceptsLegacyInitializedNotification(t *testing.T) {
	session := sessionRegistry.create("2024-11-05")
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	request.Header.Set("MCP-Session-Id", session.ID)
	request.Header.Set("MCP-Protocol-Version", session.ProtocolVersion)

	mcpPostAPI(recorder, request)

	assertRecorderAcceptedNoBody(t, recorder)
}

func TestMCPPostAcceptsUnknownNotification(t *testing.T) {
	session := sessionRegistry.create(supportedProtocolVersions[0])
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":"1"}}`)
	request.Header.Set("MCP-Session-Id", session.ID)
	request.Header.Set("MCP-Protocol-Version", session.ProtocolVersion)

	mcpPostAPI(recorder, request)

	assertRecorderAcceptedNoBody(t, recorder)
}

func TestMCPPostAcceptsNotificationWithNullID(t *testing.T) {
	session := sessionRegistry.create(supportedProtocolVersions[0])
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":null,"method":"notifications/initialized"}`)
	request.Header.Set("MCP-Session-Id", session.ID)
	request.Header.Set("MCP-Protocol-Version", session.ProtocolVersion)

	mcpPostAPI(recorder, request)

	assertRecorderAcceptedNoBody(t, recorder)
}

func TestMCPPostAcceptsNotificationWithNonNullID(t *testing.T) {
	session := sessionRegistry.create(supportedProtocolVersions[0])
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":"init-note","method":"notifications/initialized"}`)
	request.Header.Set("MCP-Session-Id", session.ID)
	request.Header.Set("MCP-Protocol-Version", session.ProtocolVersion)

	mcpPostAPI(recorder, request)

	assertRecorderAcceptedNoBody(t, recorder)
}

func TestMCPPostAcceptsResponseOnlyMessage(t *testing.T) {
	session := sessionRegistry.create(supportedProtocolVersions[0])
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":"server-request-1","result":{}}`)
	request.Header.Set("MCP-Session-Id", session.ID)
	request.Header.Set("MCP-Protocol-Version", session.ProtocolVersion)

	mcpPostAPI(recorder, request)

	assertRecorderAcceptedNoBody(t, recorder)
}

func TestMCPPostAcceptsResponseOnlyMessageWithoutSessionHeader(t *testing.T) {
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":"server-request-1","result":{}}`)
	request.Header.Set("MCP-Protocol-Version", supportedProtocolVersions[0])

	mcpPostAPI(recorder, request)

	assertRecorderAcceptedNoBody(t, recorder)
}

func TestMCPPostAcceptsLegacyUnknownNotification(t *testing.T) {
	session := sessionRegistry.create("2024-11-05")
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":"1"}}`)
	request.Header.Set("MCP-Session-Id", session.ID)
	request.Header.Set("MCP-Protocol-Version", session.ProtocolVersion)

	mcpPostAPI(recorder, request)

	assertRecorderAcceptedNoBody(t, recorder)
}

func TestMCPPostAcceptsOneWayBatch(t *testing.T) {
	session := sessionRegistry.create(supportedProtocolVersions[0])
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `[
		{"jsonrpc":"2.0","method":"notifications/initialized"},
		{"jsonrpc":"2.0","id":"server-request-1","result":{}}
	]`)
	request.Header.Set("MCP-Session-Id", session.ID)
	request.Header.Set("MCP-Protocol-Version", session.ProtocolVersion)

	mcpPostAPI(recorder, request)

	assertRecorderAcceptedNoBody(t, recorder)
}

func TestMCPPostAcceptsOneWayBatchWithoutSessionHeader(t *testing.T) {
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `[
		{"jsonrpc":"2.0","method":"notifications/initialized"},
		{"jsonrpc":"2.0","id":"server-request-1","result":{}}
	]`)
	request.Header.Set("MCP-Protocol-Version", supportedProtocolVersions[0])

	mcpPostAPI(recorder, request)

	assertRecorderAcceptedNoBody(t, recorder)
}

func TestMCPPostAcceptsOneWayBatchWithNotificationID(t *testing.T) {
	session := sessionRegistry.create(supportedProtocolVersions[0])
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `[
		{"jsonrpc":"2.0","id":null,"method":"notifications/initialized"},
		{"jsonrpc":"2.0","id":"server-request-1","result":{}}
	]`)
	request.Header.Set("MCP-Session-Id", session.ID)
	request.Header.Set("MCP-Protocol-Version", session.ProtocolVersion)

	mcpPostAPI(recorder, request)

	assertRecorderAcceptedNoBody(t, recorder)
}

func TestMCPPostAcceptsLegacyOneWayBatch(t *testing.T) {
	session := sessionRegistry.create("2024-11-05")
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `[
		{"jsonrpc":"2.0","method":"notifications/initialized"},
		{"jsonrpc":"2.0","id":"server-request-1","result":{}}
	]`)
	request.Header.Set("MCP-Session-Id", session.ID)
	request.Header.Set("MCP-Protocol-Version", session.ProtocolVersion)

	mcpPostAPI(recorder, request)

	assertRecorderAcceptedNoBody(t, recorder)
}

func TestMCPPostRejectsBatchContainingRequest(t *testing.T) {
	session := sessionRegistry.create(supportedProtocolVersions[0])
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `[{"jsonrpc":"2.0","id":"1","method":"resources/list"}]`)
	request.Header.Set("MCP-Session-Id", session.ID)
	request.Header.Set("MCP-Protocol-Version", session.ProtocolVersion)

	mcpPostAPI(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}

	var response jsonRPCResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json-rpc error response: %v", err)
	}
	if response.Error == nil || response.Error.Message != "JSON-RPC batch requests with callable methods are not supported" {
		t.Fatalf("expected batch request error, got %#v", response.Error)
	}
}

func TestMCPPostAcceptsStatelessResourcesListWithoutSessionHeader(t *testing.T) {
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":"1","method":"resources/list"}`)
	request.Header.Set("MCP-Protocol-Version", supportedProtocolVersions[0])

	mcpPostAPI(recorder, request)

	response := assertRecorderJSONRPCEventResponse(t, recorder, http.StatusOK)
	if response.Error != nil {
		t.Fatalf("expected resources/list to succeed, got %#v", response.Error)
	}
}

func TestMCPPostReturnsJSONRPCMethodErrorAsSSEMessageEvent(t *testing.T) {
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":"1","method":"unknown/method"}`)
	request.Header.Set("MCP-Protocol-Version", supportedProtocolVersions[0])

	mcpPostAPI(recorder, request)

	response := assertRecorderJSONRPCEventResponse(t, recorder, http.StatusOK)
	if response.Error == nil || response.Error.Message != "Method not found" {
		t.Fatalf("expected method-not-found response in SSE event, got %#v", response.Error)
	}
}

func TestMCPPostRejectsSessionBoundSubscribeWithoutSessionHeader(t *testing.T) {
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":"1","method":"resources/subscribe","params":{"uri":"`+documentNotificationResourceURI("doc-1")+`"}}`)
	request.Header.Set("MCP-Protocol-Version", supportedProtocolVersions[0])

	mcpPostAPI(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}

	var response jsonRPCResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json-rpc error response: %v", err)
	}
	if response.Error == nil || response.Error.Message != "MCP-Session-Id header is required" {
		t.Fatalf("expected missing session error, got %#v", response.Error)
	}
}

func TestMCPPostAcceptsSessionRequestWithoutProtocolHeader(t *testing.T) {
	session := sessionRegistry.create(supportedProtocolVersions[0])
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":"1","method":"resources/list"}`)
	request.Header.Set("MCP-Session-Id", session.ID)

	mcpPostAPI(recorder, request)

	response := assertRecorderJSONRPCEventResponse(t, recorder, http.StatusOK)
	if response.Error != nil {
		t.Fatalf("expected resources/list to succeed, got %#v", response.Error)
	}
}

func TestMCPPostAcceptsSupportedProtocolHeaderDrift(t *testing.T) {
	session := sessionRegistry.create("2024-11-05")
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":"1","method":"resources/list"}`)
	request.Header.Set("MCP-Session-Id", session.ID)
	request.Header.Set("MCP-Protocol-Version", supportedProtocolVersions[0])

	mcpPostAPI(recorder, request)

	response := assertRecorderJSONRPCEventResponse(t, recorder, http.StatusOK)
	if response.Error != nil {
		t.Fatalf("expected resources/list to succeed, got %#v", response.Error)
	}
}

func TestMCPPostRejectsUnsupportedProtocolHeaderWithoutSessionHeader(t *testing.T) {
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":"1","method":"resources/list"}`)
	request.Header.Set("MCP-Protocol-Version", "1999-01-01")

	mcpPostAPI(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}

	var response jsonRPCResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json-rpc error response: %v", err)
	}
	if response.Error == nil || response.Error.Message != "Unsupported MCP protocol version" {
		t.Fatalf("expected unsupported protocol error, got %#v", response.Error)
	}
}

func TestMCPPostRestoresUnknownUUIDSession(t *testing.T) {
	sessionID := "11111111-1111-4111-8111-111111111111"
	sessionRegistry.delete(sessionID)

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":"1","method":"resources/list"}`)
	request.Header.Set("MCP-Session-Id", sessionID)
	request.Header.Set("MCP-Protocol-Version", "2025-03-26")

	mcpPostAPI(recorder, request)

	assertRecorderJSONRPCEventResponse(t, recorder, http.StatusOK)

	session, ok := sessionRegistry.get(sessionID)
	if !ok {
		t.Fatalf("expected unknown UUID session to be restored")
	}
	if session.ProtocolVersion != "2025-03-26" {
		t.Fatalf("expected restored protocol version, got %q", session.ProtocolVersion)
	}
}

func TestValidateSessionRequestRestoresUnknownUUIDForStream(t *testing.T) {
	sessionID := "22222222-2222-4222-8222-222222222222"
	sessionRegistry.delete(sessionID)

	request, _ := httptestJSONRequest(t, http.MethodGet, "/mcp", "")
	request.Header.Set("MCP-Session-Id", sessionID)
	request.Header.Set("MCP-Protocol-Version", "2025-06-18")

	restoredSessionID, session, status, response := validateSessionRequest(request)

	if response != nil {
		t.Fatalf("expected session validation to succeed, got status %d response %#v", status, response)
	}
	if restoredSessionID != sessionID {
		t.Fatalf("expected restored session id %q, got %q", sessionID, restoredSessionID)
	}
	if session == nil || session.ProtocolVersion != "2025-06-18" {
		t.Fatalf("expected restored stream session with protocol version, got %#v", session)
	}
}

func TestMCPPostRejectsMalformedUnknownSessionID(t *testing.T) {
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":"1","method":"resources/list"}`)
	request.Header.Set("MCP-Session-Id", "not-a-session-id")
	request.Header.Set("MCP-Protocol-Version", supportedProtocolVersions[0])

	mcpPostAPI(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusNotFound, recorder.Code, recorder.Body.String())
	}

	var response jsonRPCResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json-rpc error response: %v", err)
	}
	if response.Error == nil || response.Error.Message != "Unknown MCP session" {
		t.Fatalf("expected unknown session error, got %#v", response.Error)
	}
}

func TestMCPPostRejectsUnsupportedProtocolHeader(t *testing.T) {
	session := sessionRegistry.create(supportedProtocolVersions[0])
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":"1","method":"resources/list"}`)
	request.Header.Set("MCP-Session-Id", session.ID)
	request.Header.Set("MCP-Protocol-Version", "1999-01-01")

	mcpPostAPI(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}

	var response jsonRPCResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json-rpc error response: %v", err)
	}
	if response.Error == nil || response.Error.Message != "Unsupported MCP protocol version" {
		t.Fatalf("expected unsupported protocol error, got %#v", response.Error)
	}
}

func TestMCPDeleteTerminatesSession(t *testing.T) {
	session := sessionRegistry.create(supportedProtocolVersions[0])
	request, recorder := httptestJSONRequest(t, http.MethodDelete, "/mcp", "")
	request.Header.Set("MCP-Session-Id", session.ID)

	mcpDeleteAPI(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusAccepted, recorder.Code, recorder.Body.String())
	}

	if _, ok := sessionRegistry.get(session.ID); ok {
		t.Fatalf("expected session to be deleted")
	}
}

func TestMCPOptionsAllowsConfiguredOrigin(t *testing.T) {
	t.Setenv(mcpAllowedOriginsEnv, "https://client.example")
	request, recorder := httptestJSONRequest(t, http.MethodOptions, "/mcp", "")
	request.Header.Set("Origin", "https://client.example")

	mcpOptionsAPI(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") != "https://client.example" {
		t.Fatalf("expected CORS origin header, got %q", recorder.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestLegacyMCPMessageSendsInitializeResponseOverSSE(t *testing.T) {
	session := sessionRegistry.create("2024-11-05")
	stream, ok := sessionRegistry.registerStream(session.ID)
	if !ok {
		t.Fatalf("expected stream registration to succeed")
	}
	t.Cleanup(func() {
		sessionRegistry.unregisterStream(session.ID, stream)
	})

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/messages?sessionId="+session.ID, `{"jsonrpc":"2.0","id":"1","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"legacy-client","version":"test"}}}`)

	legacyMCPMessageAPI(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusAccepted, recorder.Code, recorder.Body.String())
	}

	select {
	case message := <-stream.messages:
		var response jsonRPCResponse
		if err := json.Unmarshal(message, &response); err != nil {
			t.Fatalf("expected JSON-RPC response on SSE stream: %v", err)
		}
		if response.ID != "1" || response.Error != nil {
			t.Fatalf("expected initialize response with id 1, got %#v", response)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected initialize response to be queued on SSE stream")
	}
}

func TestLegacyMCPMessageAcceptsOneWayNotificationBatch(t *testing.T) {
	session := sessionRegistry.create("2024-11-05")
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/messages?sessionId="+session.ID, `[
		{"jsonrpc":"2.0","id":null,"method":"notifications/initialized"},
		{"jsonrpc":"2.0","id":"server-request-1","result":{}}
	]`)

	legacyMCPMessageAPI(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusAccepted, recorder.Code, recorder.Body.String())
	}
}

func TestHandlePromptsListReturnsPromptCatalog(t *testing.T) {
	request, _ := httptestJSONRequest(t, http.MethodPost, "/mcp", "")
	responseStatus, responseBody := handleMCPRequest(context.Background(), request, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "prompts/list",
	})

	if responseStatus != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseStatus)
	}
	if responseBody == nil || responseBody.Error != nil {
		t.Fatalf("expected successful prompt list response, got %#v", responseBody)
	}

	result, ok := responseBody.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected prompt list result map, got %#v", responseBody.Result)
	}

	prompts, ok := result["prompts"].([]manifestPrompt)
	if ok && len(prompts) > 0 {
		return
	}

	decodedPrompts, ok := result["prompts"].([]any)
	if !ok || len(decodedPrompts) == 0 {
		t.Fatalf("expected prompt entries, got %#v", result["prompts"])
	}
}

func TestHandlePromptsGetRendersArguments(t *testing.T) {
	request, _ := httptestJSONRequest(t, http.MethodPost, "/mcp", "")
	responseStatus, responseBody := handleMCPRequest(context.Background(), request, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "prompts/get",
		Params: mustMarshalParams(t, map[string]any{
			"name": "documents.notifications.subscribe.prompt",
			"arguments": map[string]any{
				"documentId": "doc-123",
			},
		}),
	})

	if responseStatus != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseStatus)
	}
	if responseBody == nil || responseBody.Error != nil {
		t.Fatalf("expected successful prompt get response, got %#v", responseBody)
	}

	result, ok := responseBody.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected prompt result map, got %#v", responseBody.Result)
	}

	messages, ok := result["messages"].([]manifestPromptMessage)
	if ok && len(messages) == 1 {
		if !strings.Contains(messages[0].Content.Text, "doc-123") {
			t.Fatalf("expected rendered document id in prompt message, got %#v", messages[0].Content.Text)
		}
		return
	}

	decodedMessages, ok := result["messages"].([]any)
	if !ok || len(decodedMessages) != 1 {
		t.Fatalf("expected one prompt message, got %#v", result["messages"])
	}

	message, ok := decodedMessages[0].(map[string]any)
	if !ok {
		t.Fatalf("expected prompt message map, got %#v", decodedMessages[0])
	}

	content, ok := message["content"].(map[string]any)
	if !ok {
		t.Fatalf("expected prompt content map, got %#v", message["content"])
	}
	if !strings.Contains(content["text"].(string), "doc-123") {
		t.Fatalf("expected rendered document id in prompt message, got %#v", content["text"])
	}
}

func TestHandleToolsCallUsesMinIOAdapter(t *testing.T) {
	previous := listFolderEntries
	t.Cleanup(func() {
		listFolderEntries = previous
	})

	listFolderEntries = func(ctx context.Context, bucket string, arguments map[string]any) (operationResponse, *jsonRPCError) {
		if bucket != "documents" {
			t.Fatalf("expected documents bucket, got %q", bucket)
		}
		if arguments["prefix"] != "inbox/" {
			t.Fatalf("expected prefix inbox/, got %#v", arguments["prefix"])
		}

		return operationResponse{
			ContentType: "application/json",
			Body:        `{"bucket":"documents","entries":[{"name":"inbox","type":"folder","prefix":"inbox/"}]}`,
		}, nil
	}

	request, _ := httptestJSONRequest(t, http.MethodPost, "/mcp", "")
	responseStatus, responseBody := handleMCPRequest(context.Background(), request, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "tools/call",
		Params: mustMarshalParams(t, map[string]any{
			"name": "minio.documents.listFolder",
			"arguments": map[string]any{
				"prefix": "inbox/",
			},
		}),
	})

	if responseStatus != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseStatus)
	}
	if responseBody == nil || responseBody.Error != nil {
		t.Fatalf("expected successful tool response, got %#v", responseBody)
	}

	result, ok := responseBody.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected tool result map, got %#v", responseBody.Result)
	}

	if result["isError"] != false {
		t.Fatalf("expected non-error result, got %#v", result["isError"])
	}

	structured, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("expected structured content, got %#v", result["structuredContent"])
	}
	if structured["bucket"] != "documents" {
		t.Fatalf("expected documents bucket, got %#v", structured["bucket"])
	}
}

func TestProxyAPIRequestDefaultsToOrchestratorService(t *testing.T) {
	previous := mcpHTTPClient
	t.Cleanup(func() {
		mcpHTTPClient = previous
	})

	mcpHTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "http://homelab-orchestrator.homelab.svc.cluster.local/documents" {
				t.Fatalf("expected orchestrator default upstream, got %q", req.URL.String())
			}

			return &http.Response{
				StatusCode: http.StatusAccepted,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"status":"queued"}`)),
			}, nil
		}),
	}

	t.Setenv("API_BASE_URL", "")
	request, _ := httptestJSONRequest(t, http.MethodPost, "/mcp", "")

	contentType, body, rpcErr := proxyAPIRequest(context.Background(), request, http.MethodPost, "/documents", map[string]any{
		"documentId": "doc-1",
	})

	if rpcErr != nil {
		t.Fatalf("expected successful proxy request, got %#v", rpcErr)
	}
	if contentType != "application/json" {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	if body != `{"status":"queued"}` {
		t.Fatalf("expected queued response body, got %q", body)
	}
}

func TestHandleToolsCallProxiesDocumentEditText(t *testing.T) {
	previous := proxyOperationRequest
	t.Cleanup(func() {
		proxyOperationRequest = previous
	})

	proxyOperationRequest = func(ctx context.Context, r *http.Request, method string, path string, body any) (string, string, *jsonRPCError) {
		if method != http.MethodPost {
			t.Fatalf("expected POST, got %q", method)
		}
		if path != "/documents/edit-text" {
			t.Fatalf("expected edit-text path, got %q", path)
		}
		bodyMap, ok := body.(map[string]any)
		if !ok {
			t.Fatalf("expected proxy body map, got %#v", body)
		}
		if bodyMap["documentId"] != "doc-1" || bodyMap["text"] != "edited notes" {
			t.Fatalf("unexpected proxy body: %#v", bodyMap)
		}
		return "application/json", `{"status":"queued","documentId":"doc-1","processingVersion":3}`, nil
	}

	request, _ := httptestJSONRequest(t, http.MethodPost, "/mcp", "")
	responseStatus, responseBody := handleMCPRequest(context.Background(), request, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "tools/call",
		Params: mustMarshalParams(t, map[string]any{
			"name": "documents.editText",
			"arguments": map[string]any{
				"body": map[string]any{
					"documentId": "doc-1",
					"text":       "edited notes",
				},
			},
		}),
	})

	if responseStatus != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseStatus)
	}
	if responseBody == nil || responseBody.Error != nil {
		t.Fatalf("expected successful tool response, got %#v", responseBody)
	}
}

func TestHandleToolsCallUploadsBinaryObject(t *testing.T) {
	previous := writeBucketObject
	t.Cleanup(func() {
		writeBucketObject = previous
	})

	writeBucketObject = func(ctx context.Context, bucket string, objectKey string, payload []byte, contentType string) (operationResponse, *jsonRPCError) {
		if bucket != "documents" {
			t.Fatalf("expected documents bucket, got %q", bucket)
		}
		if objectKey != "inbox/demo.bin" {
			t.Fatalf("expected inbox/demo.bin object key, got %q", objectKey)
		}
		if string(payload) != "hello" {
			t.Fatalf("expected decoded payload, got %q", string(payload))
		}
		if contentType != "application/octet-stream" {
			t.Fatalf("expected octet-stream content type, got %q", contentType)
		}
		return operationResponse{
			ContentType: "application/json",
			Body:        `{"key":"inbox/demo.bin","size":5}`,
		}, nil
	}

	request, _ := httptestJSONRequest(t, http.MethodPost, "/mcp", "")
	responseStatus, responseBody := handleMCPRequest(context.Background(), request, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "tools/call",
		Params: mustMarshalParams(t, map[string]any{
			"name": "minio.documents.putObject",
			"arguments": map[string]any{
				"objectKey": "inbox/demo.bin",
				"body": map[string]any{
					"base64": "aGVsbG8=",
				},
			},
		}),
	})

	if responseStatus != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseStatus)
	}
	if responseBody == nil || responseBody.Error != nil {
		t.Fatalf("expected successful tool response, got %#v", responseBody)
	}
}

func TestHandleToolsCallMovesObject(t *testing.T) {
	previous := moveBucketObject
	t.Cleanup(func() {
		moveBucketObject = previous
	})

	moveBucketObject = func(ctx context.Context, bucket string, sourceObjectKey string, destinationObjectKey string) (operationResponse, *jsonRPCError) {
		if bucket != "documents" {
			t.Fatalf("expected documents bucket, got %q", bucket)
		}
		if sourceObjectKey != "inbox/demo.txt" {
			t.Fatalf("expected source object key, got %q", sourceObjectKey)
		}
		if destinationObjectKey != "archive/demo.txt" {
			t.Fatalf("expected destination object key, got %q", destinationObjectKey)
		}
		return operationResponse{
			ContentType: "application/json",
			Body:        `{"moved":true}`,
		}, nil
	}

	request, _ := httptestJSONRequest(t, http.MethodPost, "/mcp", "")
	responseStatus, responseBody := handleMCPRequest(context.Background(), request, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "tools/call",
		Params: mustMarshalParams(t, map[string]any{
			"name": "minio.documents.moveObject",
			"arguments": map[string]any{
				"sourceObjectKey":      "inbox/demo.txt",
				"destinationObjectKey": "archive/demo.txt",
			},
		}),
	})

	if responseStatus != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseStatus)
	}
	if responseBody == nil || responseBody.Error != nil {
		t.Fatalf("expected successful tool response, got %#v", responseBody)
	}
}

func TestHandleToolsCallListsDocumentInventory(t *testing.T) {
	previous := listDocumentInventory
	t.Cleanup(func() {
		listDocumentInventory = previous
	})

	listDocumentInventory = func(ctx context.Context, arguments map[string]any) (operationResponse, *jsonRPCError) {
		if arguments["status"] != "processed" {
			t.Fatalf("expected status argument, got %#v", arguments["status"])
		}
		return operationResponse{
			ContentType: "application/json",
			Body:        `{"count":1,"documents":[{"documentId":"doc-1"}]}`,
		}, nil
	}

	request, _ := httptestJSONRequest(t, http.MethodPost, "/mcp", "")
	responseStatus, responseBody := handleMCPRequest(context.Background(), request, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "tools/call",
		Params: mustMarshalParams(t, map[string]any{
			"name": "documents.inventory.list",
			"arguments": map[string]any{
				"status": "processed",
			},
		}),
	})

	if responseStatus != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseStatus)
	}
	if responseBody == nil || responseBody.Error != nil {
		t.Fatalf("expected successful tool response, got %#v", responseBody)
	}
}

func TestDecodeDocumentMetadata(t *testing.T) {
	metadata, rpcErr := decodeDocumentMetadata(`{"summary":"Curated summary","tags":["campaign","npc"]}`)
	if rpcErr != nil {
		t.Fatalf("expected metadata decode to succeed: %#v", rpcErr)
	}
	if metadata["summary"] != "Curated summary" {
		t.Fatalf("expected decoded summary, got %+v", metadata)
	}
}

func TestHandleToolsCallSearchesDocuments(t *testing.T) {
	previous := searchDocumentChunks
	t.Cleanup(func() {
		searchDocumentChunks = previous
	})

	searchDocumentChunks = func(ctx context.Context, arguments map[string]any) (operationResponse, *jsonRPCError) {
		if arguments["query"] != "ancient tower" {
			t.Fatalf("expected query argument, got %#v", arguments["query"])
		}
		return operationResponse{
			ContentType: "application/json",
			Body:        `{"query":"ancient tower","hits":[]}`,
		}, nil
	}

	request, _ := httptestJSONRequest(t, http.MethodPost, "/mcp", "")
	responseStatus, responseBody := handleMCPRequest(context.Background(), request, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "tools/call",
		Params: mustMarshalParams(t, map[string]any{
			"name": "documents.search",
			"arguments": map[string]any{
				"query": "ancient tower",
			},
		}),
	})

	if responseStatus != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseStatus)
	}
	if responseBody == nil || responseBody.Error != nil {
		t.Fatalf("expected successful tool response, got %#v", responseBody)
	}
}

func TestDocumentChunkSearchBaseQueryUsesCurrentProcessingVersion(t *testing.T) {
	if !strings.Contains(documentChunkSearchBaseQuery(), "c.processing_version = d.current_processing_version") {
		t.Fatalf("expected search query to filter chunks to the document current processing version")
	}
}

func TestLocalSearchEmbeddingFallback(t *testing.T) {
	t.Setenv("EMBEDDING_ENDPOINT", "")
	t.Setenv("EMBEDDING_MODEL", "local-embeddings")

	embedding, model, err := getSearchEmbedding(context.Background(), "ancient tower")
	if err != nil {
		t.Fatalf("expected local embedding to succeed: %v", err)
	}
	if model != "local-embeddings" {
		t.Fatalf("expected local model, got %q", model)
	}
	if len(embedding) != 384 {
		t.Fatalf("expected 384-dimensional embedding, got %d", len(embedding))
	}
	if !embeddingsConfigured() {
		t.Fatal("expected local embeddings to count as configured")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestHandleResourcesReadReadsAuthUserByEmail(t *testing.T) {
	previous := readPostgresUserByEmail
	t.Cleanup(func() {
		readPostgresUserByEmail = previous
	})

	readPostgresUserByEmail = func(ctx context.Context, email string) (operationResponse, *jsonRPCError) {
		if email != "alice@example.com" {
			t.Fatalf("expected email path parameter, got %q", email)
		}
		return operationResponse{
			ContentType: "application/json",
			Body:        `{"email":"alice@example.com","displayName":"Alice"}`,
		}, nil
	}

	request, _ := httptestJSONRequest(t, http.MethodPost, "/mcp", "")
	responseStatus, responseBody := handleMCPRequest(context.Background(), request, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "resources/read",
		Params: mustMarshalParams(t, map[string]any{
			"uri": "homelab://mcp/postgres/auth/users/alice@example.com",
		}),
	})

	if responseStatus != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseStatus)
	}
	if responseBody == nil || responseBody.Error != nil {
		t.Fatalf("expected successful resource read, got %#v", responseBody)
	}

	result, ok := responseBody.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected resource result map, got %#v", responseBody.Result)
	}
	if result["contents"] == nil {
		t.Fatalf("expected resource contents, got %#v", result)
	}
}

func TestHandleResourcesReadMatchesNestedMinIOObjectKey(t *testing.T) {
	previous := readBucketObject
	t.Cleanup(func() {
		readBucketObject = previous
	})

	readBucketObject = func(ctx context.Context, bucket string, objectKey string) (operationResponse, *jsonRPCError) {
		if bucket != "documents" {
			t.Fatalf("expected documents bucket, got %q", bucket)
		}
		if objectKey != "nested/folder/file.txt" {
			t.Fatalf("expected nested object key, got %q", objectKey)
		}

		return operationResponse{
			ContentType: "text/plain",
			Body:        "hello",
		}, nil
	}

	request, _ := httptestJSONRequest(t, http.MethodPost, "/mcp", "")
	responseStatus, responseBody := handleMCPRequest(context.Background(), request, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "resources/read",
		Params: mustMarshalParams(t, map[string]any{
			"uri": "homelab://mcp/minio/documents/objects/nested/folder/file.txt",
		}),
	})

	if responseStatus != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseStatus)
	}
	if responseBody == nil || responseBody.Error != nil {
		t.Fatalf("expected successful resource read, got %#v", responseBody)
	}

	result, ok := responseBody.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected resource result map, got %#v", responseBody.Result)
	}

	contents, ok := result["contents"].([]map[string]any)
	if ok && len(contents) == 1 {
		if contents[0]["text"] != "hello" {
			t.Fatalf("expected hello body, got %#v", contents[0]["text"])
		}
		return
	}

	decodedContents, ok := result["contents"].([]any)
	if !ok || len(decodedContents) != 1 {
		t.Fatalf("expected one content item, got %#v", result["contents"])
	}

	content, ok := decodedContents[0].(map[string]any)
	if !ok {
		t.Fatalf("expected content map, got %#v", decodedContents[0])
	}
	if content["text"] != "hello" {
		t.Fatalf("expected hello body, got %#v", content["text"])
	}
}

func TestHandleResourcesReadReturnsBlobForBinaryObject(t *testing.T) {
	previous := readBucketObject
	t.Cleanup(func() {
		readBucketObject = previous
	})

	readBucketObject = func(ctx context.Context, bucket string, objectKey string) (operationResponse, *jsonRPCError) {
		return operationResponse{
			ContentType: "application/pdf",
			Body:        "%PDF",
		}, nil
	}

	request, _ := httptestJSONRequest(t, http.MethodPost, "/mcp", "")
	responseStatus, responseBody := handleMCPRequest(context.Background(), request, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "resources/read",
		Params: mustMarshalParams(t, map[string]any{
			"uri": "homelab://mcp/minio/documents/objects/report.pdf",
		}),
	})

	if responseStatus != http.StatusOK {
		t.Fatalf("expected status 200, got %d", responseStatus)
	}
	if responseBody == nil || responseBody.Error != nil {
		t.Fatalf("expected successful resource read, got %#v", responseBody)
	}

	result, ok := responseBody.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected resource result map, got %#v", responseBody.Result)
	}

	contents, ok := result["contents"].([]map[string]any)
	if ok && len(contents) == 1 {
		if contents[0]["blob"] != "JVBERg==" {
			t.Fatalf("expected base64 blob content, got %#v", contents[0]["blob"])
		}
		return
	}

	decodedContents, ok := result["contents"].([]any)
	if !ok || len(decodedContents) != 1 {
		t.Fatalf("expected one content item, got %#v", result["contents"])
	}

	content, ok := decodedContents[0].(map[string]any)
	if !ok {
		t.Fatalf("expected content map, got %#v", decodedContents[0])
	}
	if content["blob"] != "JVBERg==" {
		t.Fatalf("expected base64 blob content, got %#v", content["blob"])
	}
}

func findToolInManifest(t *testing.T, manifest manifestDocument, name string) manifestTool {
	t.Helper()

	for _, tool := range manifest.Tools {
		if tool.Name == name {
			return tool
		}
	}

	t.Fatalf("tool %q not found", name)
	return manifestTool{}
}

func findResourceTemplateInManifest(t *testing.T, manifest manifestDocument, name string) manifestResourceTemplate {
	t.Helper()

	for _, resourceTemplate := range manifest.ResourceTemplates {
		if resourceTemplate.Name == name {
			return resourceTemplate
		}
	}

	t.Fatalf("resource template %q not found", name)
	return manifestResourceTemplate{}
}

func manifestHasTransport(manifest manifestDocument, transportType string, url string) bool {
	for _, transport := range manifest.Transports {
		if transport.Type == transportType && transport.URL == url {
			return true
		}
	}
	return false
}

func mustMarshalParams(t *testing.T, value any) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("failed to marshal params: %v", err)
	}

	return data
}
