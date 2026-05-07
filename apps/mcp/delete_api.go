package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"pkg/prometheusutil"
	"time"
)

const mcpDeleteHandlerName = "DELETE /mcp"

func mcpDeleteAPI(w http.ResponseWriter, r *http.Request) {
	var err error

	ctx := r.Context()
	startTime := time.Now()

	prometheusutil.IncrementProcessed(mcpDeleteHandlerName, "call")

	defer func() {
		p := recover()
		if p != nil {
			w.WriteHeader(http.StatusInternalServerError)
			err = fmt.Errorf("panic: %v", p)
		}

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			prometheusutil.IncrementProcessed(mcpDeleteHandlerName, "error")
		}

		prometheusutil.OpDuration(mcpDeleteHandlerName, time.Since(startTime))
	}()

	if status, response := prepareOriginResponse(w, r, []string{http.MethodPost, http.MethodGet, http.MethodDelete, http.MethodOptions}); response != nil {
		writeJSONRPC(w, status, response)
		return
	}

	sessionID := sessionIDFromRequest(r)
	if sessionID == "" {
		writeJSONRPC(w, http.StatusBadRequest, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &jsonRPCError{
				Code:    -32600,
				Message: "MCP-Session-Id header is required",
			},
		})
		return
	}

	if !sessionRegistry.delete(sessionID) {
		writeJSONRPC(w, http.StatusNotFound, &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &jsonRPCError{
				Code:    -32600,
				Message: "Unknown MCP session",
			},
		})
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
