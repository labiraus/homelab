package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"pkg/api"
	"pkg/base"
	"pkg/natsutil"
	"pkg/prometheusutil"
)

func main() {
	ctx := base.Start("orchestrator")

	mux := http.NewServeMux()
	prometheusutil.Start(mux)
	mux.HandleFunc("/documents", documentsHandler)

	done := api.Start(ctx, mux, 8080)

	go func() {
		if err := startNATSWithRetry(ctx, 5*time.Second); err != nil {
			slog.ErrorContext(ctx, "nats bootstrap stopped", "error", err)
			return
		}
		close(base.Ready)
	}()

	<-done
}

func startNATSWithRetry(ctx context.Context, retryDelay time.Duration) error {
	for {
		if err := startNATS(ctx); err != nil {
			slog.ErrorContext(ctx, "nats bootstrap failed", "error", err, "retryDelay", retryDelay.String())
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay):
				continue
			}
		}

		return nil
	}
}

func startNATS(ctx context.Context) error {
	return natsutil.Start(ctx, natsutil.NATSConfig{
		Servers: strings.Split(strings.TrimSpace(os.Getenv("NATS_URLS")), ","),
		Streams: map[string]natsutil.Stream{
			"documents": {
				Name:     streamName(),
				Subject:  subjectName(),
				Replicas: 3,
			},
		},
	})
}
