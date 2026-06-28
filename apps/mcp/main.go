package main

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"pkg/api"
	"pkg/base"
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
	slog.InfoContext(ctx, "MCP data and control operations proxy through external API")

	mux := http.NewServeMux()
	prometheusutil.Start(mux)
	mux.HandleFunc(wellKnownHandlerName, wellKnownAPI)
	mux.HandleFunc(oauthProtectedResourceHandlerName, oauthProtectedResourceAPI)
	mux.HandleFunc(oauthProtectedResourceForMCPHandlerName, oauthProtectedResourceAPI)
	mux.HandleFunc(mcpGetHandlerName, mcpGetAPI)
	mux.HandleFunc(mcpPostHandlerName, mcpPostAPI)
	mux.HandleFunc(mcpDeleteHandlerName, mcpDeleteAPI)
	mux.HandleFunc(mcpOptionsHandlerName, mcpOptionsAPI)
	mux.HandleFunc(legacySSEHandlerName, legacyMCPSSEAPI)
	mux.HandleFunc(legacyMessagesHandlerName, legacyMCPMessageAPI)
	mux.HandleFunc(legacyMessageHandlerName, legacyMCPMessageAPI)
	mux.HandleFunc(legacySSEOptionsHandlerName, legacyMCPOptionsAPI)
	mux.HandleFunc(legacyMessagesOptionsHandlerName, legacyMCPOptionsAPI)
	mux.HandleFunc(legacyMessageOptionsHandlerName, legacyMCPOptionsAPI)

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
