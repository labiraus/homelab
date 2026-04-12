package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"pkg/base"
	"pkg/prometheusutil"
)

const oauthProtectedResourceHandlerName = "GET /.well-known/oauth-protected-resource"

type oauthProtectedResourceDocument struct {
	Resource             string                            `json:"resource"`
	AuthorizationServers []string                          `json:"authorization_servers"`
	ScopesSupported      []string                          `json:"scopes_supported,omitempty"`
	BearerMethods        []string                          `json:"bearer_methods_supported,omitempty"`
	AccessModes          []manifestAuthorizationAccessMode `json:"access_modes,omitempty"`
	AccessRequirement    string                            `json:"access_requirement,omitempty"`
}

func oauthProtectedResourceAPI(w http.ResponseWriter, r *http.Request) {
	var err error

	ctx := r.Context()
	startTime := time.Now()

	prometheusutil.IncrementProcessed(oauthProtectedResourceHandlerName, "call")

	defer func() {
		p := recover()
		if p != nil {
			w.WriteHeader(http.StatusInternalServerError)
			err = fmt.Errorf("panic: %v", p)
		}

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			prometheusutil.IncrementProcessed(oauthProtectedResourceHandlerName, "error")
		}

		prometheusutil.OpDuration(oauthProtectedResourceHandlerName, time.Since(startTime))
	}()

	issuerURL := strings.TrimSpace(base.GetEnv("OIDC_ISSUER_URL", "https://accounts.google.com"))
	baseURL := requestBaseURL(r)
	document := oauthProtectedResourceDocument{
		Resource:             baseURL + "/mcp",
		AuthorizationServers: []string{issuerURL},
		ScopesSupported:      []string{"openid", "email", "profile"},
		BearerMethods:        []string{"header"},
		AccessModes: []manifestAuthorizationAccessMode{
			{
				Type:        "bearer",
				Name:        "google-oidc",
				Description: "Authenticate through the shared Google-backed OIDC flow and present the resulting bearer token to the MCP endpoint.",
			},
			{
				Type:        "client-certificate",
				Name:        "mtls-client-cert",
				Description: "Authenticate by presenting a trusted client certificate at the edge so Labiraus receives forwarded certificate identity.",
			},
		},
		AccessRequirement: "one-of",
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(document)
	if err != nil {
		err = fmt.Errorf("failed to write response: %w", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
