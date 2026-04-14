package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"pkg/api"
	"pkg/base"
	"pkg/minioutil"
	"pkg/postgresutil"
	"pkg/prometheusutil"
)

type response struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
	Method       string   `json:"method"`
}

func main() {
	ctx := base.Start("mcp")

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
			slog.ErrorContext(ctx, err.Error())
			return
		}
	} else {
		slog.InfoContext(ctx, "postgres config not provided; direct postgres MCP capabilities will return unavailable errors")
	}

	if minioConfigured() {
		if err := minioutil.Init(ctx, map[string]minioutil.Config{
			"default": {
				Endpoint:  base.GetEnv("MINIO_ENDPOINT", ""),
				AccessKey: base.GetEnv("MINIO_ACCESS_KEY", ""),
				SecretKey: base.GetEnv("MINIO_SECRET_KEY", ""),
				UseSSL:    strings.EqualFold(base.GetEnv("MINIO_USE_SSL", "false"), "true"),
				Region:    base.GetEnv("MINIO_REGION", ""),
				Bucket:    base.GetEnv("MINIO_BUCKET", "documents"),
			},
		}); err != nil {
			slog.ErrorContext(ctx, err.Error())
			return
		}
	} else {
		slog.InfoContext(ctx, "minio config not provided; direct MinIO MCP capabilities will return unavailable errors")
	}

	mux := http.NewServeMux()
	prometheusutil.Start(mux)
	mux.HandleFunc(wellKnownHandlerName, wellKnownAPI)
	mux.HandleFunc(oauthProtectedResourceHandlerName, oauthProtectedResourceAPI)
	mux.HandleFunc(mcpGetHandlerName, mcpGetAPI)
	mux.HandleFunc(mcpPostHandlerName, mcpPostAPI)

	if err := startDocumentNotifications(ctx); err != nil {
		slog.ErrorContext(ctx, err.Error())
		return
	}

	done := api.Start(ctx, mux, 8080, api.NewAuthMiddleware(api.AuthOptions{}))

	close(base.Ready)
	<-done
}

func mcpHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response{
		Name:         "labiraus",
		Version:      "v1",
		Capabilities: []string{"documents", "health"},
		Method:       r.Method,
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
		if strings.TrimSpace(base.GetEnv(key, "")) == "" {
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
		if strings.TrimSpace(base.GetEnv(key, "")) == "" {
			return false
		}
	}

	return true
}
