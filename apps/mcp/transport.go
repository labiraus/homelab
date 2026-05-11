package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"pkg/base"
	"pkg/minioutil"
	"pkg/postgresutil"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

const defaultTimeout = 30 * time.Second
const defaultAPIBaseURL = "http://homelab-orchestrator.homelab.svc.cluster.local"

var supportedProtocolVersions = []string{
	"2025-11-25",
	"2025-06-18",
	"2025-03-26",
	"2024-11-05",
}

var (
	mcpHTTPClient = &http.Client{
		Timeout: defaultTimeout,
	}

	proxyOperationRequest   = proxyAPIRequest
	readPostgresUserCount   = postgresUserCount
	readPostgresUserByEmail = postgresUserByEmail
	listFolderEntries       = minioListFolderEntries
	listBucketObjects       = minioListBucketObjects
	readBucketObject        = minioReadBucketObject
	writeBucketObject       = minioPutBucketObject
	writeBucketTextObject   = minioWriteBucketTextObject
	deleteBucketObject      = minioDeleteBucketObject
	moveBucketObject        = minioMoveBucketObject
	listDocumentInventory   = postgresDocumentInventory
	listDocumentHistory     = postgresDocumentHistory
	assembleDocumentContext = postgresDocumentContext
	searchDocumentChunks    = postgresDocumentSearch
)

type resourceResolution struct {
	capability manifestCapabilitySource
	operation  manifestOperationSource
	uri        string
	params     map[string]string
}

type promptResolution struct {
	capability manifestCapabilitySource
	operation  manifestOperationSource
}

type toolResolution struct {
	capability manifestCapabilitySource
	operation  manifestOperationSource
}

type operationResponse struct {
	ContentType string
	Body        string
}

func handleMCPRequest(ctx context.Context, r *http.Request, req jsonRPCRequest) (int, *jsonRPCResponse) {
	switch req.Method {
	case "initialize":
		return handleInitialize(req)
	case "ping":
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{},
		}
	case "notifications/initialized":
		return http.StatusAccepted, nil
	case "prompts/list":
		return handlePromptsList(ctx, r, req)
	case "prompts/get":
		return handlePromptsGet(ctx, r, req)
	case "resources/list":
		return handleResourcesList(ctx, r, req)
	case "resources/templates/list":
		return handleResourceTemplatesList(ctx, r, req)
	case "resources/read":
		return handleResourcesRead(ctx, r, req)
	case "resources/subscribe":
		return handleResourcesSubscribe(r, req)
	case "resources/unsubscribe":
		return handleResourcesUnsubscribe(r, req)
	case "tools/list":
		return handleToolsList(ctx, r, req)
	case "tools/call":
		return handleToolsCall(ctx, r, req)
	default:
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

func handleInitialize(req jsonRPCRequest) (int, *jsonRPCResponse) {
	var params initializeParams

	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return http.StatusOK, &jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &jsonRPCError{
					Code:    -32602,
					Message: "Invalid initialize params",
				},
			}
		}
	}

	protocolVersion := negotiateProtocolVersion(params.ProtocolVersion)

	return http.StatusOK, &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]any{
				"prompts": map[string]any{},
				"resources": map[string]any{
					"subscribe": true,
				},
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "labiraus",
				"version": base.BuildVersion,
			},
			"instructions": "Use the Labiraus MCP server as the single entrypoint for orchestrator actions plus direct Postgres and MinIO-backed read and write capabilities. Access currently requires either Google-backed bearer authentication or trusted client-certificate authentication at the edge.",
		},
	}
}

func handlePromptsList(ctx context.Context, r *http.Request, req jsonRPCRequest) (int, *jsonRPCResponse) {
	manifest, rpcErr := manifestForMCPRequest(ctx, r)
	if rpcErr != nil {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   rpcErr,
		}
	}

	return http.StatusOK, &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"prompts": manifest.Prompts,
		},
	}
}

func handlePromptsGet(ctx context.Context, r *http.Request, req jsonRPCRequest) (int, *jsonRPCResponse) {
	var params getPromptParams

	if err := json.Unmarshal(req.Params, &params); err != nil || strings.TrimSpace(params.Name) == "" {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32602,
				Message: "Invalid prompts/get params",
			},
		}
	}

	resolution, ok := findPromptOperationByName(params.Name)
	if !ok {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32602,
				Message: "Unknown prompt",
			},
		}
	}

	argumentNames := promptArgumentNames(resolution.operation.PromptArguments)
	if err := validatePromptArgumentNames(params.Arguments, argumentNames); err != nil {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32602,
				Message: err.Error(),
			},
		}
	}

	arguments := extractArgumentStrings(params.Arguments, argumentNames)
	if err := validatePromptArguments(resolution.operation.PromptArguments, arguments); err != nil {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32602,
				Message: err.Error(),
			},
		}
	}

	return http.StatusOK, &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"description": operationDescription(resolution.capability, resolution.operation),
			"messages":    renderPromptMessages(resolution.operation.PromptMessages, resolution.operation.PromptArguments, arguments),
		},
	}
}

func handleResourcesList(ctx context.Context, r *http.Request, req jsonRPCRequest) (int, *jsonRPCResponse) {
	manifest, rpcErr := manifestForMCPRequest(ctx, r)
	if rpcErr != nil {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   rpcErr,
		}
	}

	return http.StatusOK, &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"resources": manifest.Resources,
		},
	}
}

func handleResourceTemplatesList(ctx context.Context, r *http.Request, req jsonRPCRequest) (int, *jsonRPCResponse) {
	manifest, rpcErr := manifestForMCPRequest(ctx, r)
	if rpcErr != nil {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   rpcErr,
		}
	}

	return http.StatusOK, &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"resourceTemplates": manifest.ResourceTemplates,
		},
	}
}

func handleToolsList(ctx context.Context, r *http.Request, req jsonRPCRequest) (int, *jsonRPCResponse) {
	manifest, rpcErr := manifestForMCPRequest(ctx, r)
	if rpcErr != nil {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   rpcErr,
		}
	}

	return http.StatusOK, &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"tools": manifest.Tools,
		},
	}
}

func handleResourcesRead(ctx context.Context, r *http.Request, req jsonRPCRequest) (int, *jsonRPCResponse) {
	var params readResourceParams

	err := json.Unmarshal(req.Params, &params)
	if err != nil || strings.TrimSpace(params.URI) == "" {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32602,
				Message: "Invalid resources/read params",
			},
		}
	}

	resolution, ok := findResourceOperationByURI(params.URI)
	if !ok {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32602,
				Message: "Unknown resource URI",
			},
		}
	}

	if resolution.operation.Lifecycle != manifestLifecycleLive || resolution.operation.Binding == nil {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32004,
				Message: "Requested resource is planned but not yet implemented",
				Data:    resolution.operation.ID,
			},
		}
	}

	response, rpcErr := executeResourceOperation(ctx, r, resolution)
	if rpcErr != nil {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   rpcErr,
		}
	}

	return http.StatusOK, &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"contents": []map[string]any{resourceContentItem(resolution.uri, response)},
		},
	}
}

func handleResourcesSubscribe(r *http.Request, req jsonRPCRequest) (int, *jsonRPCResponse) {
	var params subscribeResourceParams

	if err := json.Unmarshal(req.Params, &params); err != nil || strings.TrimSpace(params.URI) == "" {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32602,
				Message: "Invalid resources/subscribe params",
			},
		}
	}

	resolution, ok := findResourceOperationByURI(params.URI)
	if !ok || resolution.operation.Binding == nil || resolution.operation.Binding.ExecutionMode != manifestExecutionModeNATSSubscription {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32602,
				Message: "Unknown subscribable resource URI",
			},
		}
	}

	if !sessionRegistry.subscribe(sessionIDFromRequest(r), params.URI) {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32600,
				Message: "MCP session is not available",
			},
		}
	}

	return http.StatusOK, &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{},
	}
}

func handleResourcesUnsubscribe(r *http.Request, req jsonRPCRequest) (int, *jsonRPCResponse) {
	var params unsubscribeResourceParams

	if err := json.Unmarshal(req.Params, &params); err != nil || strings.TrimSpace(params.URI) == "" {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32602,
				Message: "Invalid resources/unsubscribe params",
			},
		}
	}

	if !sessionRegistry.unsubscribe(sessionIDFromRequest(r), params.URI) {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32600,
				Message: "MCP session is not available",
			},
		}
	}

	return http.StatusOK, &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{},
	}
}

func handleToolsCall(ctx context.Context, r *http.Request, req jsonRPCRequest) (int, *jsonRPCResponse) {
	var params callToolParams

	err := json.Unmarshal(req.Params, &params)
	if err != nil || strings.TrimSpace(params.Name) == "" {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32602,
				Message: "Invalid tools/call params",
			},
		}
	}

	resolution, ok := findToolOperationByName(params.Name)
	if !ok {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32602,
				Message: "Unknown tool",
			},
		}
	}

	if resolution.operation.Lifecycle != manifestLifecycleLive || resolution.operation.Binding == nil {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32004,
				Message: "Requested tool is planned but not yet implemented",
				Data:    resolution.operation.ID,
			},
		}
	}

	inputSchema := toToolInputSchema(resolution.operation)
	if err := validateToolArgumentNames(params.Arguments, inputSchema); err != nil {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &jsonRPCError{
				Code:    -32602,
				Message: err.Error(),
			},
		}
	}

	response, upstreamErr := executeToolOperation(ctx, r, resolution, params.Arguments)
	result := map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": response.Body,
			},
		},
		"isError": upstreamErr != nil,
	}

	if structured, ok := decodeStructuredContent(response.Body, response.ContentType); ok {
		result["structuredContent"] = structured
	}

	if upstreamErr != nil {
		return http.StatusOK, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}
	}

	return http.StatusOK, &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func manifestForMCPRequest(ctx context.Context, r *http.Request) (manifestDocument, *jsonRPCError) {
	return buildManifest(r), nil
}

func executeResourceOperation(ctx context.Context, r *http.Request, resolution resourceResolution) (operationResponse, *jsonRPCError) {
	binding := resolution.operation.Binding

	switch binding.ExecutionMode {
	case manifestExecutionModePostgresQuery:
		return readPostgresUserCount(ctx, binding.Query)
	case manifestExecutionModePostgresUserByEmail:
		email, ok := resolution.params["email"]
		if !ok || strings.TrimSpace(email) == "" {
			return operationResponse{}, &jsonRPCError{
				Code:    -32602,
				Message: "Resource URI is missing email",
			}
		}
		return readPostgresUserByEmail(ctx, email)
	case manifestExecutionModeNATSSubscription:
		documentID, ok := resolution.params["documentId"]
		if !ok || strings.TrimSpace(documentID) == "" {
			return operationResponse{}, &jsonRPCError{
				Code:    -32602,
				Message: "Resource URI is missing documentId",
			}
		}
		return readDocumentNotificationResource(ctx, documentID)
	case manifestExecutionModeMinIOGetObject:
		objectKey, ok := resolution.params["objectKey"]
		if !ok || strings.TrimSpace(objectKey) == "" {
			return operationResponse{}, &jsonRPCError{
				Code:    -32602,
				Message: "Resource URI is missing objectKey",
			}
		}
		return readBucketObject(ctx, binding.Bucket, objectKey)
	case manifestExecutionModeHTTPProxy:
		contentType, body, rpcErr := proxyOperationRequest(ctx, r, resolution.operation.Method, binding.Path, nil)
		return operationResponse{ContentType: defaultContentType(contentType), Body: body}, rpcErr
	default:
		return operationResponse{}, &jsonRPCError{
			Code:    -32601,
			Message: "Resource execution mode is not supported",
			Data:    binding.ExecutionMode,
		}
	}
}

func executeToolOperation(ctx context.Context, r *http.Request, resolution toolResolution, arguments map[string]any) (operationResponse, *jsonRPCError) {
	binding := resolution.operation.Binding
	if arguments == nil {
		arguments = map[string]any{}
	}

	switch binding.ExecutionMode {
	case manifestExecutionModeHTTPProxy:
		path, rpcErr := buildOperationPath(binding.Path, extractArgumentStrings(arguments, extractPathParameters(resolution.operation.Path)))
		if rpcErr != nil {
			return operationResponse{}, rpcErr
		}

		contentType, body, upstreamErr := proxyOperationRequest(ctx, r, resolution.operation.Method, path, bodyForToolCall(resolution.operation, arguments))
		return operationResponse{ContentType: defaultContentType(contentType), Body: body}, upstreamErr
	case manifestExecutionModeMinIOListFolder:
		return listFolderEntries(ctx, binding.Bucket, arguments)
	case manifestExecutionModeMinIOList:
		return listBucketObjects(ctx, binding.Bucket, arguments)
	case manifestExecutionModeMinIOPutObject:
		objectKey, rpcErr := requiredStringArgument(arguments, "objectKey")
		if rpcErr != nil {
			return operationResponse{}, rpcErr
		}

		bodyMap, rpcErr := requiredObjectArgument(arguments, "body")
		if rpcErr != nil {
			return operationResponse{}, rpcErr
		}

		base64Value, rpcErr := requiredStringArgument(bodyMap, "base64")
		if rpcErr != nil {
			return operationResponse{}, rpcErr
		}

		payload, err := base64.StdEncoding.DecodeString(base64Value)
		if err != nil {
			return operationResponse{}, &jsonRPCError{
				Code:    -32602,
				Message: "body.base64 must be valid base64",
				Data:    err.Error(),
			}
		}

		contentType := optionalStringArgument(bodyMap, "contentType", "application/octet-stream")
		return writeBucketObject(ctx, binding.Bucket, objectKey, payload, contentType)
	case manifestExecutionModeMinIOPutText:
		objectKey, rpcErr := requiredStringArgument(arguments, "objectKey")
		if rpcErr != nil {
			return operationResponse{}, rpcErr
		}

		bodyMap, rpcErr := requiredObjectArgument(arguments, "body")
		if rpcErr != nil {
			return operationResponse{}, rpcErr
		}

		text, rpcErr := requiredStringArgument(bodyMap, "text")
		if rpcErr != nil {
			return operationResponse{}, rpcErr
		}

		contentType := optionalStringArgument(bodyMap, "contentType", "text/plain; charset=utf-8")
		return writeBucketTextObject(ctx, binding.Bucket, objectKey, text, contentType)
	case manifestExecutionModeMinIODelete:
		objectKey, rpcErr := requiredStringArgument(arguments, "objectKey")
		if rpcErr != nil {
			return operationResponse{}, rpcErr
		}
		return deleteBucketObject(ctx, binding.Bucket, objectKey)
	case manifestExecutionModeMinIOMove:
		sourceObjectKey, rpcErr := requiredStringArgument(arguments, "sourceObjectKey")
		if rpcErr != nil {
			return operationResponse{}, rpcErr
		}
		destinationObjectKey, rpcErr := requiredStringArgument(arguments, "destinationObjectKey")
		if rpcErr != nil {
			return operationResponse{}, rpcErr
		}
		return moveBucketObject(ctx, binding.Bucket, sourceObjectKey, destinationObjectKey)
	case manifestExecutionModeDocumentInventory:
		return listDocumentInventory(ctx, arguments)
	case manifestExecutionModeDocumentHistory:
		return listDocumentHistory(ctx, arguments)
	case manifestExecutionModeDocumentContext:
		return assembleDocumentContext(ctx, arguments)
	case manifestExecutionModeDocumentSearch:
		return searchDocumentChunks(ctx, arguments)
	default:
		return operationResponse{}, &jsonRPCError{
			Code:    -32601,
			Message: "Tool execution mode is not supported",
			Data:    binding.ExecutionMode,
		}
	}
}

func proxyAPIRequest(ctx context.Context, r *http.Request, method string, path string, body any) (string, string, *jsonRPCError) {
	apiBaseURL := strings.TrimSpace(base.GetEnv("API_BASE_URL", ""))
	if apiBaseURL == "" {
		apiBaseURL = defaultAPIBaseURL
	}

	url := strings.TrimRight(apiBaseURL, "/") + path

	var requestBody io.Reader

	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return "", "", &jsonRPCError{
				Code:    -32602,
				Message: "Failed to encode tool request body",
				Data:    err.Error(),
			}
		}

		requestBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, requestBody)
	if err != nil {
		return "", "", &jsonRPCError{
			Code:    -32000,
			Message: "Failed to build upstream request",
			Data:    err.Error(),
		}
	}

	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if authorization := r.Header.Get("Authorization"); authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	if traceID, ok := ctx.Value(base.TraceID).(string); ok && traceID != "" {
		req.Header.Set(string(base.TraceID), traceID)
	}

	if userID, ok := ctx.Value(base.UserID).(string); ok && userID != "" {
		req.Header.Set(string(base.UserID), userID)
	}

	resp, err := mcpHTTPClient.Do(req)
	if err != nil {
		return "", "", &jsonRPCError{
			Code:    -32000,
			Message: "Failed to call upstream API",
			Data:    err.Error(),
		}
	}
	defer resp.Body.Close()

	responseBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", &jsonRPCError{
			Code:    -32000,
			Message: "Failed to read upstream response",
			Data:    err.Error(),
		}
	}

	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	responseBody := strings.TrimSpace(string(responseBytes))

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if responseBody == "" {
			responseBody = fmt.Sprintf("upstream API returned %d", resp.StatusCode)
		}

		return contentType, responseBody, &jsonRPCError{
			Code:    -32000,
			Message: "Upstream API request failed",
			Data: map[string]any{
				"status": resp.StatusCode,
				"path":   path,
			},
		}
	}

	return contentType, responseBody, nil
}

func postgresUserCount(ctx context.Context, query string) (operationResponse, *jsonRPCError) {
	if postgresutil.QueryRow == nil {
		return operationResponse{
				ContentType: "application/json",
				Body:        `{"error":"postgres is not initialized"}`,
			}, &jsonRPCError{
				Code:    -32000,
				Message: "Postgres backend is unavailable",
			}
	}

	var count int
	if err := postgresutil.QueryRow(ctx, query).Scan(&count); err != nil {
		return operationResponse{
				ContentType: "application/json",
				Body:        fmt.Sprintf(`{"error":%q}`, err.Error()),
			}, &jsonRPCError{
				Code:    -32000,
				Message: "Postgres query failed",
				Data:    err.Error(),
			}
	}

	body, err := json.Marshal(map[string]any{
		"userCount": count,
	})
	if err != nil {
		return operationResponse{}, &jsonRPCError{
			Code:    -32000,
			Message: "Failed to encode Postgres response",
			Data:    err.Error(),
		}
	}

	return operationResponse{
		ContentType: "application/json",
		Body:        string(body),
	}, nil
}

func readDocumentNotificationResource(ctx context.Context, documentID string) (operationResponse, *jsonRPCError) {
	if postgresutil.QueryRow == nil {
		return operationResponse{
				ContentType: "application/json",
				Body:        `{"error":"postgres is not initialized"}`,
			}, &jsonRPCError{
				Code:    -32000,
				Message: "Postgres backend is unavailable",
			}
	}

	var payload string
	if err := postgresutil.QueryRow(
		ctx,
		`SELECT json_build_object(
			'documentId', document_id,
			'bucket', bucket_name,
			'objectKey', object_key,
			'contentType', content_type,
			'status', status,
			'desiredProcessingVersion', desired_processing_version,
			'currentProcessingVersion', current_processing_version,
			'lastProcessedAt', last_processed_at,
			'lastEventSubject', last_event_subject,
			'lastEventAt', last_event_at,
			'lastError', last_error
		)::text
		FROM rag.documents
		WHERE document_id = $1`,
		documentID,
	).Scan(&payload); err != nil {
		return operationResponse{
				ContentType: "application/json",
				Body:        fmt.Sprintf(`{"error":%q}`, err.Error()),
			}, &jsonRPCError{
				Code:    -32000,
				Message: "Document notification resource read failed",
				Data:    documentID,
			}
	}

	return operationResponse{
		ContentType: "application/json",
		Body:        payload,
	}, nil
}

func minioListBucketObjects(ctx context.Context, bucket string, arguments map[string]any) (operationResponse, *jsonRPCError) {
	objects, err := minioutil.ListObjectInfoInBucket(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    optionalStringArgument(arguments, "prefix", ""),
		Recursive: optionalBoolArgument(arguments, "recursive", false),
	}, optionalIntArgument(arguments, "maxKeys", 0))
	if err != nil {
		return operationResponse{
				ContentType: "application/json",
				Body:        fmt.Sprintf(`{"error":%q}`, err.Error()),
			}, &jsonRPCError{
				Code:    -32000,
				Message: "MinIO list operation failed",
				Data:    err.Error(),
			}
	}

	body, err := json.Marshal(map[string]any{
		"bucket":  bucket,
		"objects": objects,
	})
	if err != nil {
		return operationResponse{}, &jsonRPCError{
			Code:    -32000,
			Message: "Failed to encode MinIO list response",
			Data:    err.Error(),
		}
	}

	return operationResponse{
		ContentType: "application/json",
		Body:        string(body),
	}, nil
}

func minioListFolderEntries(ctx context.Context, bucket string, arguments map[string]any) (operationResponse, *jsonRPCError) {
	entries, err := minioutil.ListFolderEntriesInBucket(ctx, bucket, optionalStringArgument(arguments, "prefix", ""), optionalIntArgument(arguments, "maxKeys", 0))
	if err != nil {
		return operationResponse{
				ContentType: "application/json",
				Body:        fmt.Sprintf(`{"error":%q}`, err.Error()),
			}, &jsonRPCError{
				Code:    -32000,
				Message: "MinIO folder list operation failed",
				Data:    err.Error(),
			}
	}

	body, err := json.Marshal(map[string]any{
		"bucket":  bucket,
		"prefix":  optionalStringArgument(arguments, "prefix", ""),
		"entries": entries,
	})
	if err != nil {
		return operationResponse{}, &jsonRPCError{
			Code:    -32000,
			Message: "Failed to encode MinIO folder list response",
			Data:    err.Error(),
		}
	}

	return operationResponse{
		ContentType: "application/json",
		Body:        string(body),
	}, nil
}

func minioReadBucketObject(ctx context.Context, bucket string, objectKey string) (operationResponse, *jsonRPCError) {
	object, err := minioutil.ReadObjectFromBucket(ctx, bucket, objectKey)
	if err != nil {
		return operationResponse{
				ContentType: "application/json",
				Body:        fmt.Sprintf(`{"error":%q}`, err.Error()),
			}, &jsonRPCError{
				Code:    -32000,
				Message: "MinIO object read failed",
				Data: map[string]any{
					"bucket":    bucket,
					"objectKey": objectKey,
				},
			}
	}

	return operationResponse{
		ContentType: defaultContentType(object.Info.ContentType),
		Body:        string(object.Body),
	}, nil
}

func minioPutBucketObject(ctx context.Context, bucket string, objectKey string, payload []byte, contentType string) (operationResponse, *jsonRPCError) {
	object, err := minioutil.PutObjectBytesToBucket(ctx, bucket, objectKey, payload, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return operationResponse{
				ContentType: "application/json",
				Body:        fmt.Sprintf(`{"error":%q}`, err.Error()),
			}, &jsonRPCError{
				Code:    -32000,
				Message: "MinIO write operation failed",
				Data: map[string]any{
					"bucket":    bucket,
					"objectKey": objectKey,
				},
			}
	}

	body, err := json.Marshal(object)
	if err != nil {
		return operationResponse{}, &jsonRPCError{
			Code:    -32000,
			Message: "Failed to encode MinIO write response",
			Data:    err.Error(),
		}
	}

	return operationResponse{
		ContentType: "application/json",
		Body:        string(body),
	}, nil
}

func minioWriteBucketTextObject(ctx context.Context, bucket string, objectKey string, text string, contentType string) (operationResponse, *jsonRPCError) {
	object, err := minioutil.PutTextObjectToBucket(ctx, bucket, objectKey, text, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return operationResponse{
				ContentType: "application/json",
				Body:        fmt.Sprintf(`{"error":%q}`, err.Error()),
			}, &jsonRPCError{
				Code:    -32000,
				Message: "MinIO write operation failed",
				Data: map[string]any{
					"bucket":    bucket,
					"objectKey": objectKey,
				},
			}
	}

	body, err := json.Marshal(object)
	if err != nil {
		return operationResponse{}, &jsonRPCError{
			Code:    -32000,
			Message: "Failed to encode MinIO write response",
			Data:    err.Error(),
		}
	}

	return operationResponse{
		ContentType: "application/json",
		Body:        string(body),
	}, nil
}

func minioDeleteBucketObject(ctx context.Context, bucket string, objectKey string) (operationResponse, *jsonRPCError) {
	if err := minioutil.DeleteObjectFromBucket(ctx, bucket, objectKey); err != nil {
		return operationResponse{
				ContentType: "application/json",
				Body:        fmt.Sprintf(`{"error":%q}`, err.Error()),
			}, &jsonRPCError{
				Code:    -32000,
				Message: "MinIO delete operation failed",
				Data: map[string]any{
					"bucket":    bucket,
					"objectKey": objectKey,
				},
			}
	}

	body, err := json.Marshal(map[string]any{
		"bucket":    bucket,
		"objectKey": objectKey,
		"deleted":   true,
	})
	if err != nil {
		return operationResponse{}, &jsonRPCError{
			Code:    -32000,
			Message: "Failed to encode MinIO delete response",
			Data:    err.Error(),
		}
	}

	return operationResponse{
		ContentType: "application/json",
		Body:        string(body),
	}, nil
}

func findResourceOperationByURI(uri string) (resourceResolution, bool) {
	for _, capability := range capabilityCatalog {
		for _, operation := range capability.Operations {
			if operation.Primitive != manifestPrimitiveResource && operation.Primitive != manifestPrimitiveResourceTemplate {
				continue
			}

			uriTemplate := toOperationURI(operation.Path)
			params, ok := matchOperationURI(uriTemplate, uri)
			if !ok {
				continue
			}

			return resourceResolution{
				capability: capability,
				operation:  operation,
				uri:        uri,
				params:     params,
			}, true
		}
	}

	return resourceResolution{}, false
}

func findPromptOperationByName(name string) (promptResolution, bool) {
	for _, capability := range capabilityCatalog {
		for _, operation := range capability.Operations {
			if operation.Primitive == manifestPrimitivePrompt && operation.ID == name {
				return promptResolution{
					capability: capability,
					operation:  operation,
				}, true
			}
		}
	}

	return promptResolution{}, false
}

func findToolOperationByName(name string) (toolResolution, bool) {
	for _, capability := range capabilityCatalog {
		for _, operation := range capability.Operations {
			if operation.Primitive == manifestPrimitiveTool && operation.ID == name {
				return toolResolution{
					capability: capability,
					operation:  operation,
				}, true
			}
		}
	}

	return toolResolution{}, false
}

func buildOperationPath(pathTemplate string, parameters map[string]string) (string, *jsonRPCError) {
	path := pathTemplate

	for _, parameter := range extractPathParameters(pathTemplate) {
		value, ok := parameters[parameter]
		if !ok || strings.TrimSpace(value) == "" {
			return "", &jsonRPCError{
				Code:    -32602,
				Message: fmt.Sprintf("Missing path parameter %q", parameter),
			}
		}

		path = strings.ReplaceAll(path, "{"+parameter+"}", url.PathEscape(value))
	}

	return path, nil
}

func bodyForToolCall(operation manifestOperationSource, arguments map[string]any) any {
	if operation.Method != http.MethodPost && operation.Method != http.MethodPut {
		return nil
	}

	if body, ok := arguments["body"]; ok {
		return body
	}

	return map[string]any{}
}

func matchOperationURI(templateURI string, uri string) (map[string]string, bool) {
	templatePath := strings.TrimPrefix(templateURI, "homelab://mcp")
	resourcePath := strings.TrimPrefix(uri, "homelab://mcp")

	templateParts := splitNonEmpty(templatePath)
	resourceParts := splitNonEmpty(resourcePath)
	params := map[string]string{}

	for index := 0; index < len(templateParts); index++ {
		if index >= len(resourceParts) {
			return nil, false
		}

		templatePart := templateParts[index]
		if !isPathParameter(templatePart) {
			if templatePart != resourceParts[index] {
				return nil, false
			}
			continue
		}

		name := strings.TrimSuffix(strings.TrimPrefix(templatePart, "{"), "}")
		if index == len(templateParts)-1 {
			value, err := url.PathUnescape(strings.Join(resourceParts[index:], "/"))
			if err != nil {
				return nil, false
			}
			params[name] = value
			return params, true
		}

		value, err := url.PathUnescape(resourceParts[index])
		if err != nil {
			return nil, false
		}
		params[name] = value
	}

	if len(resourceParts) != len(templateParts) {
		return nil, false
	}

	return params, true
}

func splitNonEmpty(path string) []string {
	rawParts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	parts := []string{}

	for _, part := range rawParts {
		if part != "" {
			parts = append(parts, part)
		}
	}

	return parts
}

func isPathParameter(part string) bool {
	return strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}")
}

func extractArgumentStrings(arguments map[string]any, keys []string) map[string]string {
	parameters := map[string]string{}

	for _, key := range keys {
		if value, ok := arguments[key]; ok {
			parameters[key] = fmt.Sprint(value)
		}
	}

	return parameters
}

func promptArgumentNames(arguments []manifestPromptArgument) []string {
	names := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		names = append(names, argument.Name)
	}
	return names
}

func validatePromptArgumentNames(arguments map[string]any, knownNames []string) error {
	return validateKnownArgumentNames("prompt", arguments, knownNames)
}

func validateToolArgumentNames(arguments map[string]any, schema manifestSchema) error {
	if schema.AdditionalProperties == true {
		return nil
	}

	knownNames := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		knownNames = append(knownNames, name)
	}

	return validateKnownArgumentNames("tool", arguments, knownNames)
}

func validateKnownArgumentNames(label string, arguments map[string]any, knownNames []string) error {
	if len(arguments) == 0 {
		return nil
	}

	known := make(map[string]struct{}, len(knownNames))
	for _, name := range knownNames {
		known[name] = struct{}{}
	}

	unknown := make([]string, 0)
	for name := range arguments {
		if _, ok := known[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) == 0 {
		return nil
	}

	sort.Strings(unknown)
	if len(unknown) == 1 {
		return fmt.Errorf("unknown %s argument %q", label, unknown[0])
	}

	return fmt.Errorf("unknown %s arguments %q", label, strings.Join(unknown, ", "))
}

func validatePromptArguments(definitions []manifestPromptArgument, arguments map[string]string) error {
	for _, definition := range definitions {
		if definition.Required && strings.TrimSpace(arguments[definition.Name]) == "" {
			return fmt.Errorf("missing required prompt argument %q", definition.Name)
		}
	}

	return nil
}

func renderPromptMessages(templates []manifestPromptMessage, definitions []manifestPromptArgument, arguments map[string]string) []manifestPromptMessage {
	renderArguments := promptRenderArguments(definitions, arguments)
	rendered := make([]manifestPromptMessage, 0, len(templates))
	for _, template := range templates {
		message := template
		message.Content.Text = renderPromptText(template.Content.Text, renderArguments)
		rendered = append(rendered, message)
	}
	return rendered
}

func promptRenderArguments(definitions []manifestPromptArgument, arguments map[string]string) map[string]string {
	renderArguments := make(map[string]string, len(definitions))
	for _, definition := range definitions {
		value := arguments[definition.Name]
		if strings.TrimSpace(value) == "" && !definition.Required {
			value = "not supplied"
		}
		renderArguments[definition.Name] = value
	}
	return renderArguments
}

func renderPromptText(template string, arguments map[string]string) string {
	rendered := template
	for key, value := range arguments {
		rendered = strings.ReplaceAll(rendered, "{{"+key+"}}", value)
	}
	return rendered
}

func negotiateProtocolVersion(requested string) string {
	for _, version := range supportedProtocolVersions {
		if version == requested {
			return version
		}
	}

	return supportedProtocolVersions[0]
}

func decodeStructuredContent(body string, contentType string) (any, bool) {
	if contentType != "application/json" {
		return nil, false
	}

	var decoded any
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		return nil, false
	}

	return decoded, true
}

func resourceContentItem(uri string, response operationResponse) map[string]any {
	item := map[string]any{
		"uri":      uri,
		"mimeType": response.ContentType,
	}
	if isTextContentType(response.ContentType) {
		item["text"] = response.Body
		return item
	}
	item["blob"] = base64.StdEncoding.EncodeToString([]byte(response.Body))
	return item
}

func isTextContentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return strings.HasPrefix(contentType, "text/") ||
		contentType == "application/json" ||
		contentType == "application/xml" ||
		contentType == "application/javascript"
}

func defaultContentType(contentType string) string {
	if strings.TrimSpace(contentType) == "" {
		return "application/json"
	}
	return contentType
}

func requiredStringArgument(arguments map[string]any, key string) (string, *jsonRPCError) {
	value := strings.TrimSpace(optionalStringArgument(arguments, key, ""))
	if value == "" {
		return "", &jsonRPCError{
			Code:    -32602,
			Message: fmt.Sprintf("Missing required argument %q", key),
		}
	}
	return value, nil
}

func optionalStringArgument(arguments map[string]any, key string, fallback string) string {
	value, ok := arguments[key]
	if !ok || value == nil {
		return fallback
	}

	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return fallback
		}
		return typed
	default:
		return fmt.Sprint(value)
	}
}

func optionalJSONMapArgument(arguments map[string]any, key string) map[string]any {
	value, ok := arguments[key]
	if !ok || value == nil {
		return nil
	}

	typed, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	result := map[string]any{}
	for rawKey, rawValue := range typed {
		filterKey := strings.TrimSpace(rawKey)
		if filterKey == "" || rawValue == nil {
			continue
		}
		if stringValue, ok := rawValue.(string); ok {
			rawValue = strings.TrimSpace(stringValue)
		}
		if rawValue == "" {
			continue
		}
		result[filterKey] = rawValue
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func optionalBoolArgument(arguments map[string]any, key string, fallback bool) bool {
	value, ok := arguments[key]
	if !ok || value == nil {
		return fallback
	}

	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(typed)
		if err != nil {
			return fallback
		}
		return parsed
	default:
		return fallback
	}
}

func optionalIntArgument(arguments map[string]any, key string, fallback int) int {
	value, ok := arguments[key]
	if !ok || value == nil {
		return fallback
	}

	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return fallback
		}
		return int(parsed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return fallback
		}
		return parsed
	default:
		return fallback
	}
}

func requiredObjectArgument(arguments map[string]any, key string) (map[string]any, *jsonRPCError) {
	value, ok := arguments[key]
	if !ok || value == nil {
		return nil, &jsonRPCError{
			Code:    -32602,
			Message: fmt.Sprintf("Missing required argument %q", key),
		}
	}

	typed, ok := value.(map[string]any)
	if !ok {
		return nil, &jsonRPCError{
			Code:    -32602,
			Message: fmt.Sprintf("Argument %q must be an object", key),
		}
	}

	return typed, nil
}
