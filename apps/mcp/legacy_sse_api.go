package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"pkg/prometheusutil"
	"strings"
	"time"
)

const legacySSEHandlerName = "GET /sse"
const legacyMessagesHandlerName = "POST /messages"
const legacyMessageHandlerName = "POST /message"
const legacyMessagePath = "/messages"

func legacyMCPSSEAPI(w http.ResponseWriter, r *http.Request) {
	var err error

	ctx := r.Context()
	startTime := time.Now()

	prometheusutil.IncrementProcessed(legacySSEHandlerName, "call")

	defer func() {
		p := recover()
		if p != nil {
			w.WriteHeader(http.StatusInternalServerError)
			err = fmt.Errorf("panic: %v", p)
		}

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			prometheusutil.IncrementProcessed(legacySSEHandlerName, "error")
		}

		prometheusutil.OpDuration(legacySSEHandlerName, time.Since(startTime))
	}()

	if status, response := prepareOriginResponse(w, r, []string{http.MethodPost, http.MethodGet, http.MethodOptions}); response != nil {
		writeJSONRPC(w, status, response)
		return
	}

	if !strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream") {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	session := sessionRegistry.create("2024-11-05")
	defer sessionRegistry.delete(session.ID)

	endpoint := legacyMessagePath + "?sessionId=" + url.QueryEscape(session.ID)

	if err = serveSessionStreamWithOptions(w, r, session.ID, sessionStreamOptions{
		Endpoint:        endpoint,
		UseMessageEvent: true,
	}); err != nil {
		writeJSONRPC(w, http.StatusInternalServerError, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &jsonRPCError{
				Code:    -32000,
				Message: "Failed to open legacy MCP event stream",
			},
		})
		return
	}
}

func legacyMCPMessageAPI(w http.ResponseWriter, r *http.Request) {
	var err error

	ctx := r.Context()
	startTime := time.Now()

	prometheusutil.IncrementProcessed(legacyMessagesHandlerName, "call")

	defer func() {
		p := recover()
		if p != nil {
			w.WriteHeader(http.StatusInternalServerError)
			err = fmt.Errorf("panic: %v", p)
		}

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			prometheusutil.IncrementProcessed(legacyMessagesHandlerName, "error")
		}

		prometheusutil.OpDuration(legacyMessagesHandlerName, time.Since(startTime))
	}()

	if status, response := prepareOriginResponse(w, r, []string{http.MethodPost, http.MethodGet, http.MethodOptions}); response != nil {
		writeJSONRPC(w, status, response)
		return
	}

	sessionID := legacySessionIDFromRequest(r)
	if sessionID == "" {
		writeJSONRPC(w, http.StatusBadRequest, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &jsonRPCError{
				Code:    -32600,
				Message: "sessionId query parameter is required",
			},
		})
		return
	}

	session, ok := sessionRegistry.get(sessionID)
	if !ok {
		writeJSONRPC(w, http.StatusNotFound, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &jsonRPCError{
				Code:    -32600,
				Message: "Unknown MCP SSE session",
			},
		})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		err = fmt.Errorf("body unreadable: %w", err)
		writeJSONRPC(w, http.StatusBadRequest, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &jsonRPCError{
				Code:    -32700,
				Message: "Failed to read request body",
			},
		})
		return
	}

	trimmedBody := bytes.TrimSpace(body)
	if len(trimmedBody) == 0 {
		writeJSONRPC(w, http.StatusBadRequest, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &jsonRPCError{
				Code:    -32700,
				Message: "Request body must not be empty",
			},
		})
		return
	}

	if trimmedBody[0] == '[' {
		accepted, status, response := validateLegacyOneWayBatchRequest(trimmedBody)
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
				Message: "Legacy JSON-RPC batch requests with callable methods are not supported",
			},
		})
		return
	}

	var req jsonRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		err = fmt.Errorf("failed to unmarshal request: %w", err)
		writeJSONRPC(w, http.StatusBadRequest, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &jsonRPCError{
				Code:    -32700,
				Message: "Invalid JSON-RPC payload",
			},
		})
		return
	}

	hasID := jsonRPCMessageHasID(body)
	if req.Method == "" || !hasID || isJSONRPCNotificationMethod(req.Method) {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	if req.Method != "initialize" {
		if protocolVersion := protocolVersionFromHeader(r); protocolVersion != "" && negotiateProtocolVersion(protocolVersion) != protocolVersion {
			writeJSONRPC(w, http.StatusBadRequest, &jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &jsonRPCError{
					Code:    -32600,
					Message: "Unsupported MCP protocol version",
				},
			})
			return
		}
	}

	request := legacyRequestWithSession(r, session)
	_, responseBody := handleMCPRequest(ctx, request, req)
	if responseBody == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	if req.Method == "initialize" && responseBody.Error == nil {
		var params initializeParams
		if err := json.Unmarshal(req.Params, &params); err == nil {
			sessionRegistry.setProtocolVersion(sessionID, negotiateProtocolVersion(params.ProtocolVersion))
		}
	}

	responseBytes, err := json.Marshal(responseBody)
	if err != nil {
		err = fmt.Errorf("failed to encode legacy SSE response: %w", err)
		writeJSONRPC(w, http.StatusInternalServerError, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &jsonRPCError{
				Code:    -32000,
				Message: "Failed to encode MCP response",
			},
		})
		return
	}

	if !sessionRegistry.send(sessionID, responseBytes) {
		writeJSONRPC(w, http.StatusServiceUnavailable, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &jsonRPCError{
				Code:    -32000,
				Message: "MCP SSE session stream is not available",
			},
		})
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func legacySessionIDFromRequest(r *http.Request) string {
	if sessionID := strings.TrimSpace(r.URL.Query().Get("sessionId")); sessionID != "" {
		return sessionID
	}
	if sessionID := strings.TrimSpace(r.URL.Query().Get("session_id")); sessionID != "" {
		return sessionID
	}
	return sessionIDFromRequest(r)
}

func legacyRequestWithSession(r *http.Request, session *mcpSession) *http.Request {
	request := r.Clone(r.Context())
	request.Header = r.Header.Clone()
	request.Header.Set("MCP-Session-Id", session.ID)
	if request.Header.Get("MCP-Protocol-Version") == "" {
		request.Header.Set("MCP-Protocol-Version", session.ProtocolVersion)
	}
	return request
}

func validateLegacyOneWayBatchRequest(body []byte) (bool, int, *jsonRPCResponse) {
	var messages []json.RawMessage
	if err := json.Unmarshal(body, &messages); err != nil || len(messages) == 0 {
		return false, http.StatusBadRequest, &jsonRPCResponse{
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
			return false, http.StatusBadRequest, &jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &jsonRPCError{
					Code:    -32600,
					Message: "Invalid JSON-RPC batch item",
				},
			}
		}
		if !oneWay {
			return false, 0, nil
		}
	}

	return true, 0, nil
}
