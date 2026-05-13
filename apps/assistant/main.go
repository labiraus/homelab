package main

import (
	"log/slog"
	"net/http"
	"os"
	"strings"

	"pkg/api"
	"pkg/base"
	"pkg/postgresutil"
	"pkg/prometheusutil"
)

func main() {
	ctx := base.Start("assistant")

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
		slog.InfoContext(ctx, "postgres config not provided; assistant endpoints will be unavailable")
	}

	mux := http.NewServeMux()
	prometheusutil.Start(mux)
	mux.HandleFunc("/assistant/conversations", conversationsHandler)
	mux.HandleFunc("/assistant/conversations/", conversationDetailHandler)
	mux.HandleFunc("/assistant/chat", chatHandler)
	mux.HandleFunc("/assistant/memories", memoriesHandler)
	mux.HandleFunc("/assistant/memories/", memoryActionHandler)
	mux.HandleFunc("/assistant/proposals", proposalsHandler)
	mux.HandleFunc("/assistant/proposals/", proposalActionHandler)
	mux.HandleFunc("/assistant/audits", auditsHandler)

	done := api.Start(ctx, mux, 8080, api.NewAuthMiddleware(api.AuthOptions{
		OIDCEmailHeader: "X-Forwarded-Email",
	}))
	close(base.Ready)
	<-done
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
