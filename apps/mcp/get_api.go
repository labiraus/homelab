package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"pkg/prometheusutil"
	"strings"
	"time"
)

const mcpGetHandlerName = "GET /mcp"

func mcpGetAPI(w http.ResponseWriter, r *http.Request) {
	var err error

	ctx := r.Context()
	startTime := time.Now()

	prometheusutil.IncrementProcessed(mcpGetHandlerName, "call")

	defer func() {
		p := recover()
		if p != nil {
			w.WriteHeader(http.StatusInternalServerError)

			err = fmt.Errorf("panic: %v", p)
		}

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			prometheusutil.IncrementProcessed(mcpGetHandlerName, "error")
		}

		prometheusutil.OpDuration(mcpGetHandlerName, time.Since(startTime))
	}()

	slog.DebugContext(ctx, fmt.Sprintf("%v called", mcpGetHandlerName))

	if status, response := validateOriginRequest(r); response != nil {
		writeJSONRPC(w, status, response)
		return
	}

	if !strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream") {
		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	sessionID, _, status, response := validateSessionRequest(r)
	if response != nil {
		writeJSONRPC(w, status, response)
		return
	}

	if err = serveSessionStream(w, r, sessionID); err != nil {
		writeJSONRPC(w, http.StatusInternalServerError, &jsonRPCResponse{
			JSONRPC: "2.0",
			Error: &jsonRPCError{
				Code:    -32000,
				Message: "Failed to open MCP event stream",
			},
		})
		return
	}

	slog.DebugContext(ctx, fmt.Sprintf("%v complete", mcpGetHandlerName))
}
