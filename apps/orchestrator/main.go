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
	"pkg/postgresutil"
	"pkg/prometheusutil"
)

func main() {
	ctx := base.Start("orchestrator")

	if postgresConfigured() {
		if err := postgresutil.Init(ctx, map[string]postgresutil.PostgresConfig{
			"default": {
				Host:     base.GetEnv("POSTGRES_HOST", ""),
				Port:     base.GetEnv("POSTGRES_PORT", "5432"),
				User:     base.GetEnv("POSTGRES_USER", ""),
				Password: base.GetEnv("POSTGRES_PASSWORD", ""),
				Database: base.GetEnv("POSTGRES_DATABASE", ""),
				SSLMode:  base.GetEnv("POSTGRES_SSLMODE", "disable"),
			},
		}); err != nil {
			slog.ErrorContext(ctx, "postgres bootstrap failed", "error", err)
			return
		}
	} else {
		slog.InfoContext(ctx, "postgres config not provided; document queue endpoint will be unavailable")
	}

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

func postgresConfigured() bool {
	requiredVars := []string{
		"POSTGRES_HOST",
		"POSTGRES_USER",
		"POSTGRES_PASSWORD",
		"POSTGRES_DATABASE",
	}

	for _, key := range requiredVars {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			return false
		}
	}

	return true
}
