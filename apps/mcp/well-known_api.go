package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"pkg/prometheusutil"
	"time"
)

const wellKnownHandlerName = "GET /.well-known/mcp.json"

func wellKnownAPI(w http.ResponseWriter, r *http.Request) {
	var err error

	ctx := r.Context()
	startTime := time.Now()

	prometheusutil.IncrementProcessed(wellKnownHandlerName, "call")

	defer func() {
		p := recover()
		if p != nil {
			w.WriteHeader(http.StatusInternalServerError)

			err = fmt.Errorf("panic: %v", p)
		}

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			prometheusutil.IncrementProcessed(wellKnownHandlerName, "error")
		}

		prometheusutil.OpDuration(wellKnownHandlerName, time.Since(startTime))
	}()

	slog.DebugContext(ctx, fmt.Sprintf("%v called", wellKnownHandlerName))

	manifest := buildManifest(r)

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(manifest)
	if err != nil {
		err = fmt.Errorf("failed to write response: %w", err)

		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	slog.DebugContext(ctx, fmt.Sprintf("%v complete", wellKnownHandlerName))
}
