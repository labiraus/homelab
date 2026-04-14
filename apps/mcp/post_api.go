package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"pkg/prometheusutil"
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

	if status, response := validateOriginRequest(r); response != nil {
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		_ = json.NewEncoder(w).Encode(jsonRPCResponse{
			JSONRPC: "2.0",
			Error: &jsonRPCError{
				Code:    -32600,
				Message: "JSON-RPC batch requests are not supported",
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

	if req.Method == "" {
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
