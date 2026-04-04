package main

import (
	"encoding/json"
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

	mux := http.NewServeMux()
	prometheusutil.Start(mux)
	mux.HandleFunc("/mcp", mcpHandler)

	done := api.Start(ctx, mux, 8080)

	close(base.Ready)
	<-done
}

func mcpHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response{
		Name:         "mcp",
		Version:      "v1",
		Capabilities: []string{"documents", "health"},
		Method:       r.Method,
	})
}
