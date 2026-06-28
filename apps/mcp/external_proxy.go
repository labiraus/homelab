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
	"strings"
)

func proxyUserCount(ctx context.Context, query string) (operationResponse, *jsonRPCError) {
	return proxyExternal(ctx, http.MethodGet, "/api/users/count", nil)
}

func proxyUserByEmail(ctx context.Context, email string) (operationResponse, *jsonRPCError) {
	return proxyExternal(ctx, http.MethodGet, "/api/auth/users/"+url.PathEscape(email), nil)
}

func proxyDocumentInventory(ctx context.Context, arguments map[string]any) (operationResponse, *jsonRPCError) {
	return proxyExternal(ctx, http.MethodPost, "/api/documents/inventory", arguments)
}

func proxyDocumentHistory(ctx context.Context, arguments map[string]any) (operationResponse, *jsonRPCError) {
	return proxyExternal(ctx, http.MethodPost, "/api/documents/history", arguments)
}

func proxyDocumentContext(ctx context.Context, arguments map[string]any) (operationResponse, *jsonRPCError) {
	return proxyExternal(ctx, http.MethodPost, "/api/documents/context", arguments)
}

func proxyDocumentSearch(ctx context.Context, arguments map[string]any) (operationResponse, *jsonRPCError) {
	return proxyExternal(ctx, http.MethodPost, "/api/documents/search", arguments)
}

func proxyListFolderEntries(ctx context.Context, bucket string, arguments map[string]any) (operationResponse, *jsonRPCError) {
	query := url.Values{}
	if prefix := optionalStringArgument(arguments, "prefix", ""); prefix != "" {
		query.Set("prefix", prefix)
	}
	if maxKeys := optionalIntArgument(arguments, "maxKeys", 0); maxKeys > 0 {
		query.Set("maxKeys", fmt.Sprintf("%d", maxKeys))
	}
	path := "/api/documents/tree"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return proxyExternal(ctx, http.MethodGet, path, nil)
}

func proxyListBucketObjects(ctx context.Context, bucket string, arguments map[string]any) (operationResponse, *jsonRPCError) {
	return proxyListFolderEntries(ctx, bucket, arguments)
}

func proxyReadBucketObject(ctx context.Context, bucket string, objectKey string) (operationResponse, *jsonRPCError) {
	query := url.Values{}
	query.Set("objectKey", objectKey)
	return proxyExternal(ctx, http.MethodGet, "/api/documents/object?"+query.Encode(), nil)
}

func proxyPutBucketObject(ctx context.Context, bucket string, objectKey string, payload []byte, contentType string) (operationResponse, *jsonRPCError) {
	return proxyExternal(ctx, http.MethodPut, "/api/documents/object", map[string]any{
		"objectKey":   objectKey,
		"base64":      base64.StdEncoding.EncodeToString(payload),
		"contentType": contentType,
	})
}

func proxyWriteBucketTextObject(ctx context.Context, bucket string, objectKey string, text string, contentType string) (operationResponse, *jsonRPCError) {
	return proxyExternal(ctx, http.MethodPut, "/api/documents/object", map[string]any{
		"objectKey":   objectKey,
		"text":        text,
		"contentType": contentType,
	})
}

func proxyDeleteBucketObject(ctx context.Context, bucket string, objectKey string) (operationResponse, *jsonRPCError) {
	return proxyExternal(ctx, http.MethodDelete, "/api/documents/object", map[string]any{"objectKey": objectKey})
}

func proxyMoveBucketObject(ctx context.Context, bucket string, sourceObjectKey string, destinationObjectKey string) (operationResponse, *jsonRPCError) {
	return proxyExternal(ctx, http.MethodPost, "/api/documents/move", map[string]any{
		"sourceObjectKey":      sourceObjectKey,
		"destinationObjectKey": destinationObjectKey,
	})
}

func proxyExternal(ctx context.Context, method string, path string, body any) (operationResponse, *jsonRPCError) {
	apiBaseURL := strings.TrimSpace(base.GetEnv("API_BASE_URL", ""))
	if apiBaseURL == "" {
		apiBaseURL = defaultAPIBaseURL
	}

	var requestBody io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return operationResponse{}, &jsonRPCError{Code: -32602, Message: "Failed to encode upstream request body", Data: err.Error()}
		}
		requestBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(apiBaseURL, "/")+path, requestBody)
	if err != nil {
		return operationResponse{}, &jsonRPCError{Code: -32000, Message: "Failed to build upstream request", Data: err.Error()}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if userID, ok := ctx.Value(base.UserID).(string); ok && strings.TrimSpace(userID) != "" {
		req.Header.Set(string(base.UserID), userID)
		req.Header.Set("X-Forwarded-Email", userID)
	}
	if traceID, ok := ctx.Value(base.TraceID).(string); ok && strings.TrimSpace(traceID) != "" {
		req.Header.Set(string(base.TraceID), traceID)
	}

	resp, err := mcpHTTPClient.Do(req)
	if err != nil {
		return operationResponse{}, &jsonRPCError{Code: -32000, Message: "Failed to call external API", Data: err.Error()}
	}
	defer resp.Body.Close()

	responseBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return operationResponse{}, &jsonRPCError{Code: -32000, Message: "Failed to read external API response", Data: err.Error()}
	}

	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	response := operationResponse{ContentType: defaultContentType(contentType), Body: strings.TrimSpace(string(responseBytes))}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if response.Body == "" {
			response.Body = fmt.Sprintf(`{"error":"external API returned %d"}`, resp.StatusCode)
		}
		return response, &jsonRPCError{
			Code:    -32000,
			Message: "External API request failed",
			Data: map[string]any{
				"status": resp.StatusCode,
				"path":   path,
			},
		}
	}
	return response, nil
}
