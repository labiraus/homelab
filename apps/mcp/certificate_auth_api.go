package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"pkg/prometheusutil"
	"time"
)

const certificateDiscoveryHandlerName = "GET /.well-known/auth/certificate.json"

type certificateAuthDiscoveryDocument struct {
	Type                   string   `json:"type"`
	StatusEndpoint         string   `json:"statusEndpoint"`
	TrustedHeaders         []string `json:"trustedHeaders"`
	IdentityFields         []string `json:"identityFields"`
	RecommendedGatewayMode string   `json:"recommendedGatewayMode"`
}

func certificateDiscoveryAPI(w http.ResponseWriter, r *http.Request) {
	var err error

	ctx := r.Context()
	startTime := time.Now()

	prometheusutil.IncrementProcessed(certificateDiscoveryHandlerName, "call")

	defer func() {
		p := recover()
		if p != nil {
			w.WriteHeader(http.StatusInternalServerError)
			err = fmt.Errorf("panic: %v", p)
		}

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			prometheusutil.IncrementProcessed(certificateDiscoveryHandlerName, "error")
		}

		prometheusutil.OpDuration(certificateDiscoveryHandlerName, time.Since(startTime))
	}()

	baseURL := requestBaseURL(r)
	document := certificateAuthDiscoveryDocument{
		Type:                   "x509-client-certificate",
		StatusEndpoint:         baseURL + "/api/auth/status",
		TrustedHeaders:         []string{"X-Forwarded-Client-Cert"},
		IdentityFields:         []string{"uri", "subject.emailAddress"},
		RecommendedGatewayMode: "SANITIZE_SET",
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(document)
	if err != nil {
		err = fmt.Errorf("failed to write response: %w", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
