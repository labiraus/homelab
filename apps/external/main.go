package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"pkg/api"
	"pkg/base"
	"pkg/kubernetesutil"
	"pkg/minioutil"
	"pkg/postgresutil"
	"pkg/prometheusutil"

	"github.com/patrickmn/go-cache"
)

const (
	helloHandlerLabel = "helloHandler"
	userCountLabel    = "userCountHandler"
)

var (
	c          = cache.New(5*time.Minute, 10*time.Minute)
	kubeAccess = false
)

var configValue string

func main() {
	var err error
	ctx := base.Start("external")
	defer func() {
		p := recover()
		if p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			log.Fatal("Code terminated unexpectedly")
		}
	}()
	configValue = base.GetEnv("configValue", "unknown")

	if postgresConfigured() {
		postgresConfig := map[string]postgresutil.PostgresConfig{
			"default": {
				Host:     base.GetEnv("POSTGRES_HOST", ""),
				Port:     base.GetEnv("POSTGRES_PORT", "5432"),
				User:     base.GetEnv("POSTGRES_USER", ""),
				Password: base.GetEnv("POSTGRES_PASSWORD", ""),
				Database: base.GetEnv("POSTGRES_DATABASE", ""),
				SSLMode:  base.GetEnv("POSTGRES_SSLMODE", "disable"),
			},
		}
		if err = postgresutil.Init(ctx, postgresConfig); err != nil {
			return
		}
	} else {
		slog.InfoContext(ctx, "postgres config not provided; user count endpoint will be unavailable")
	}

	if minioConfigured() {
		if err = minioutil.Init(ctx, map[string]minioutil.Config{
			"default": {
				Endpoint:  base.GetEnv("MINIO_ENDPOINT", ""),
				AccessKey: base.GetEnv("MINIO_ACCESS_KEY", ""),
				SecretKey: base.GetEnv("MINIO_SECRET_KEY", ""),
				UseSSL:    strings.EqualFold(base.GetEnv("MINIO_USE_SSL", "false"), "true"),
				Region:    base.GetEnv("MINIO_REGION", ""),
				Bucket:    base.GetEnv("MINIO_BUCKET", "documents"),
			},
		}); err != nil {
			return
		}
	} else {
		slog.InfoContext(ctx, "minio config not provided; document browser endpoints will be unavailable")
	}

	mux := http.NewServeMux()
	prometheusutil.Start(mux)
	mux.HandleFunc("/api/auth/status", authStatusHandler)
	mux.HandleFunc("/api/auth/providers", authProvidersHandler)
	mux.HandleFunc("/api/users/count", userCountHandler)
	mux.HandleFunc("/api/documents/tree", documentsTreeHandler)
	mux.HandleFunc("/api/documents/object", documentObjectHandler)
	mux.HandleFunc("/api/documents/upload", documentUploadHandler)
	mux.HandleFunc("/api/documents/events", documentEventsHandler)
	mux.HandleFunc("/api/documents/search", documentSearchHandler)

	done := api.Start(ctx, mux, 8080, api.NewAuthMiddleware(api.AuthOptions{
		OIDCEmailHeader: "X-Forwarded-Email",
		Validator:       validateIdentityEmail,
	}))

	kubeAccess, err = kubernetesutil.Start()
	if err != nil {
		return
	}
	if !kubeAccess {
		slog.InfoContext(ctx, "kubernetes access not available")
	}

	if err = startDocumentEvents(ctx); err != nil {
		return
	}

	close(base.Ready)
	<-done
	slog.InfoContext(ctx, "finishing")
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
