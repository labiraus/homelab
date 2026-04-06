package main

import (
	"context"
	"encoding/json"
	"net/http"
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

	if manifest.Authorization.Meta.CertificateDiscoveryURL != "https://mcp.labiraus.com/.well-known/auth/certificate.json" {
		t.Fatalf("unexpected certificate discovery URL: %q", manifest.Authorization.Meta.CertificateDiscoveryURL)
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
}

func TestHandleToolsCallUsesMinIOAdapter(t *testing.T) {
	previous := listBucketObjects
	t.Cleanup(func() {
		listBucketObjects = previous
	})

	listBucketObjects = func(ctx context.Context, bucket string, arguments map[string]any) (operationResponse, *jsonRPCError) {
		if bucket != "documents" {
			t.Fatalf("expected documents bucket, got %q", bucket)
		}
		if arguments["prefix"] != "inbox/" {
			t.Fatalf("expected prefix inbox/, got %#v", arguments["prefix"])
		}

		return operationResponse{
			ContentType: "application/json",
			Body:        `{"bucket":"documents","objects":[{"key":"inbox/a.txt"}]}`,
		}, nil
	}

	request, _ := httptestJSONRequest(t, http.MethodPost, "/mcp", "")
	responseStatus, responseBody := handleMCPRequest(context.Background(), request, jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "tools/call",
		Params: mustMarshalParams(t, map[string]any{
			"name": "minio.documents.listObjects",
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
