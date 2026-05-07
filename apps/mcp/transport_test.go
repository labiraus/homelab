package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
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

	liveTool := findToolInManifest(t, manifest, "documents.submit")
	if liveTool.Meta.Lifecycle != manifestLifecycleLive {
		t.Fatalf("expected live tool lifecycle, got %q", liveTool.Meta.Lifecycle)
	}
	if liveTool.Meta.Backend != manifestBackendOrchestrator {
		t.Fatalf("expected orchestrator backend, got %q", liveTool.Meta.Backend)
	}

	plannedTool := findToolInManifest(t, manifest, "documents.scanBucket")
	if plannedTool.Meta.Lifecycle != manifestLifecyclePlanned {
		t.Fatalf("expected planned tool lifecycle, got %q", plannedTool.Meta.Lifecycle)
	}

	template := findResourceTemplateInManifest(t, manifest, "minio.documents.object")
	if template.Meta.ExecutionMode != manifestExecutionModeMinIOGetObject {
		t.Fatalf("expected MinIO get execution mode, got %q", template.Meta.ExecutionMode)
	}

	notificationTemplate := findResourceTemplateInManifest(t, manifest, "documents.notifications.stream")
	if notificationTemplate.Meta.ExecutionMode != manifestExecutionModeNATSSubscription {
		t.Fatalf("expected NATS subscription execution mode, got %q", notificationTemplate.Meta.ExecutionMode)
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

func TestMCPPostAcceptsInitializedNotificationWithoutProtocolHeader(t *testing.T) {
	session := sessionRegistry.create(supportedProtocolVersions[0])
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	request.Header.Set(mcpSessionHeader, session.ID)

	mcpPostAPI(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusAccepted, recorder.Code, recorder.Body.String())
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("expected empty notification response body, got %q", recorder.Body.String())
	}
}

func TestMCPPostAcceptsSessionRequestWithoutProtocolHeader(t *testing.T) {
	session := sessionRegistry.create(supportedProtocolVersions[0])
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":"1","method":"resources/list"}`)
	request.Header.Set(mcpSessionHeader, session.ID)

	mcpPostAPI(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response jsonRPCResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json-rpc response: %v", err)
	}
	if response.Error != nil {
		t.Fatalf("expected resources/list to succeed, got %#v", response.Error)
	}
}

func TestMCPPostRejectsMismatchedProtocolHeader(t *testing.T) {
	session := sessionRegistry.create(supportedProtocolVersions[0])
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":"1","method":"resources/list"}`)
	request.Header.Set(mcpSessionHeader, session.ID)
	request.Header.Set(mcpProtocolVersionHeader, "2024-11-05")

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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestHandleResourcesReadRejectsPlannedCapability(t *testing.T) {
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
	if responseBody == nil || responseBody.Error == nil {
		t.Fatalf("expected JSON-RPC error, got %#v", responseBody)
	}
	if responseBody.Error.Code != -32004 {
		t.Fatalf("expected planned-capability error code, got %d", responseBody.Error.Code)
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

func mustMarshalParams(t *testing.T, value any) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("failed to marshal params: %v", err)
	}

	return data
}
