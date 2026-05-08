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
	"pkg/documentevents"
	"pkg/minioutil"
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

	if minioConfigured() {
		if err := minioutil.Init(ctx, map[string]minioutil.Config{
			"documents": {
				Endpoint:  base.GetEnv("MINIO_ENDPOINT", ""),
				AccessKey: base.GetEnv("MINIO_ACCESS_KEY", ""),
				SecretKey: base.GetEnv("MINIO_SECRET_KEY", ""),
				UseSSL:    strings.EqualFold(base.GetEnv("MINIO_USE_SSL", "false"), "true"),
				Region:    base.GetEnv("MINIO_REGION", ""),
				Bucket:    base.GetEnv("MINIO_BUCKET", "documents"),
			},
		}); err != nil {
			slog.ErrorContext(ctx, "minio bootstrap failed", "error", err)
			return
		}
	} else {
		slog.InfoContext(ctx, "minio config not provided; bucket scan endpoint will be unavailable")
	}

	mux := http.NewServeMux()
	prometheusutil.Start(mux)
	mux.HandleFunc("/documents", documentsHandler)
	mux.HandleFunc("/documents/curation", documentCurationHandler)
	mux.HandleFunc("/documents/edit-text", editTextDocumentHandler)
	mux.HandleFunc("/documents/scan-bucket", scanBucketHandler)
	mux.HandleFunc("/documents/reprocess", reprocessDocumentHandler)

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
				Name:      streamName(),
				Subject:   subjectName(),
				Replicas:  3,
				Retention: natsutil.RetentionWorkQueue,
			},
			documentevents.StreamID: {
				Name:      documentEventsStreamName(),
				Subject:   documentEventsSubject(),
				Replicas:  3,
				Retention: natsutil.RetentionLimits,
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

func minioConfigured() bool {
	requiredVars := []string{
		"MINIO_ENDPOINT",
		"MINIO_ACCESS_KEY",
		"MINIO_SECRET_KEY",
	}

	for _, key := range requiredVars {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			return false
		}
	}

	return true
}
