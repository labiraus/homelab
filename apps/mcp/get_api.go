package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"pkg/prometheusutil"
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

	w.Header().Set("Allow", http.MethodPost)
	w.WriteHeader(http.StatusMethodNotAllowed)

	slog.DebugContext(ctx, fmt.Sprintf("%v complete", mcpGetHandlerName))
}
