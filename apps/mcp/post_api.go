package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"pkg/prometheusutil"
	"strings"
	"time"
)

const mcpPostHandlerName = "POST /mcp"

func mcpPostAPI(w http.ResponseWriter, r *http.Request) {
	var err error

	ctx := r.Context()
	startTime := time.Now()

	prometheusutil.IncrementProcessed(mcpPostHandlerName, "call")

	defer func() {
		p := recover()
		if p != nil {
			w.WriteHeader(http.StatusInternalServerError)

			err = fmt.Errorf("panic: %v", p)
		}

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			prometheusutil.IncrementProcessed(mcpPostHandlerName, "error")
		}

		prometheusutil.OpDuration(mcpPostHandlerName, time.Since(startTime))
	}()

	slog.DebugContext(ctx, fmt.Sprintf("%v called", mcpPostHandlerName))

	if status, response := prepareOriginResponse(w, r, []string{http.MethodPost, http.MethodGet, http.MethodDelete, http.MethodOptions}); response != nil {
		writeJSONRPC(w, status, response)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		err = fmt.Errorf("body unreadable: %w", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		_ = json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			Error: &jsonRPCError{
				Code:    -32700,
				Message: "Failed to read request body",
			},
		})

		return
	}

	if len(bytes.TrimSpace(body)) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		_ = json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			Error: &jsonRPCError{
				Code:    -32700,
				Message: "Request body must not be empty",
			},
		})

		return
	}

	trimmedBody := bytes.TrimSpace(body)
	if len(trimmedBody) > 0 && trimmedBody[0] == '[' {
		accepted, _, status, response := validateOneWayBatchRequest(r, trimmedBody)
		if response != nil {
			writeJSONRPC(w, status, response)
			return
		}
		if accepted {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		writeJSONRPC(w, http.StatusBadRequest, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &jsonRPCError{
				Code:    -32600,
				Message: "JSON-RPC batch requests with callable methods are not supported",
			},
		})

		return
	}

	var req jsonRPCRequest

	err = json.Unmarshal(body, &req)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal request: %w", err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		_ = json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			Error: &jsonRPCError{
				Code:    -32700,
				Message: "Invalid JSON-RPC payload",
			},
		})

		return
	}

	hasID := jsonRPCMessageHasID(body)
	if req.Method == "" {
		_, _, status, response := validateSessionRequest(r)
		if response != nil {
			writeJSONRPC(w, status, response)
			return
		}

		w.WriteHeader(http.StatusAccepted)
		return
	}

	if !hasID || isJSONRPCNotificationMethod(req.Method) {
		if req.Method != "initialize" {
			_, _, status, response := validateSessionRequest(r)
			if response != nil {
				writeJSONRPC(w, status, response)
				return
			}
		}

		w.WriteHeader(http.StatusAccepted)
		return
	}

	if req.Method != "initialize" {
		_, _, status, response := validateSessionRequest(r)
		if response != nil {
			writeJSONRPC(w, status, response)
			return
		}
	}

	responseStatus, responseBody := handleMCPRequest(ctx, r, req)
	if responseBody == nil {
		w.WriteHeader(responseStatus)
		return
	}

	if req.Method == "initialize" && responseBody.Error == nil {
		var params initializeParams
		if err := json.Unmarshal(req.Params, &params); err == nil {
			protocolVersion := negotiateProtocolVersion(params.ProtocolVersion)
			session := sessionRegistry.create(protocolVersion)
			w.Header().Set(mcpSessionHeader, session.ID)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(responseStatus)

	err = json.NewEncoder(w).Encode(responseBody)
	if err != nil {
		err = fmt.Errorf("failed to write response: %w", err)

		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	slog.DebugContext(ctx, fmt.Sprintf("%v complete", mcpPostHandlerName))
}

type jsonRPCMessageShape struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
}

func validateOneWayBatchRequest(r *http.Request, body []byte) (bool, *mcpSession, int, *jsonRPCResponse) {
	var messages []json.RawMessage
	if err := json.Unmarshal(body, &messages); err != nil || len(messages) == 0 {
		return false, nil, http.StatusBadRequest, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &jsonRPCError{
				Code:    -32600,
				Message: "Invalid JSON-RPC batch payload",
			},
		}
	}

	for _, message := range messages {
		oneWay, err := isOneWayJSONRPCMessage(message)
		if err != nil {
			return false, nil, http.StatusBadRequest, &jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &jsonRPCError{
					Code:    -32600,
					Message: "Invalid JSON-RPC batch item",
				},
			}
		}
		if !oneWay {
			return false, nil, 0, nil
		}
	}

	_, session, status, response := validateSessionRequest(r)
	if response != nil {
		return false, nil, status, response
	}

	return true, session, 0, nil
}

func isOneWayJSONRPCMessage(message []byte) (bool, error) {
	trimmed := bytes.TrimSpace(message)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false, fmt.Errorf("json-rpc batch item must be an object")
	}

	var shape jsonRPCMessageShape
	if err := json.Unmarshal(message, &shape); err != nil {
		return false, err
	}

	if shape.Method == "" {
		return true, nil
	}

	if isJSONRPCNotificationMethod(shape.Method) {
		return true, nil
	}

	return shape.ID == nil, nil
}

func jsonRPCMessageHasID(body []byte) bool {
	var shape jsonRPCMessageShape
	if err := json.Unmarshal(body, &shape); err != nil {
		return false
	}
	return shape.ID != nil
}

func isJSONRPCNotificationMethod(method string) bool {
	return strings.HasPrefix(method, "notifications/")
}
