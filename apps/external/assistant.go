package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"pkg/api"
	"pkg/prometheusutil"
)

const (
	assistantProxyLabel       = "assistantProxyHandler"
	maxAssistantProxyBodySize = 2 << 20
)

type assistantProxyResponse struct {
	statusCode  int
	contentType string
	body        []byte
}

var (
	assistantHTTPClient = &http.Client{Timeout: 120 * time.Second}
	proxyAssistantAPI   = callAssistantAPI
)

func assistantProxyHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	startTime := time.Now()
	prometheusutil.IncrementProcessed(assistantProxyLabel, "call")
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
			w.WriteHeader(http.StatusInternalServerError)
		}
		if err != nil {
			slog.ErrorContext(r.Context(), err.Error())
			prometheusutil.IncrementProcessed(assistantProxyLabel, "error")
		}
		prometheusutil.OpDuration(assistantProxyLabel, time.Since(startTime))
	}()

	if !assistantConfigured() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "assistant service is unavailable"})
		return
	}

	body, readErr := io.ReadAll(io.LimitReader(r.Body, maxAssistantProxyBodySize+1))
	if readErr != nil {
		err = readErr
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid request body"})
		return
	}
	if len(body) > maxAssistantProxyBodySize {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "request body is too large"})
		return
	}

	path := assistantPath(r.URL.Path)
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	response, proxyErr := proxyAssistantAPI(r.Context(), path, r.Method, body)
	if proxyErr != nil {
		err = proxyErr
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "assistant request failed"})
		return
	}

	contentType := strings.TrimSpace(response.contentType)
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(response.statusCode)
	if len(response.body) > 0 {
		_, err = w.Write(response.body)
	}
}

func callAssistantAPI(ctx context.Context, path string, method string, body []byte) (assistantProxyResponse, error) {
	endpoint := strings.TrimRight(assistantBaseURL(), "/") + path
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return assistantProxyResponse{}, err
	}
	request.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	if email, ok := proxiedEmail(ctx); ok {
		request.Header.Set("X-Forwarded-Email", email)
		request.Header.Set("UserID", email)
	}

	response, err := assistantHTTPClient.Do(request)
	if err != nil {
		return assistantProxyResponse{}, err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxAssistantProxyBodySize+1))
	if err != nil {
		return assistantProxyResponse{}, err
	}
	if len(responseBody) > maxAssistantProxyBodySize {
		return assistantProxyResponse{}, fmt.Errorf("assistant response exceeded %d bytes", maxAssistantProxyBodySize)
	}

	return assistantProxyResponse{
		statusCode:  response.StatusCode,
		contentType: response.Header.Get("Content-Type"),
		body:        responseBody,
	}, nil
}

func assistantPath(path string) string {
	return "/assistant/" + strings.TrimPrefix(path, "/api/assistant/")
}

func proxiedEmail(ctx context.Context) (string, bool) {
	if status, ok := api.AuthStatusFromContext(ctx); ok && status.Valid && strings.TrimSpace(status.Email) != "" {
		return strings.ToLower(strings.TrimSpace(status.Email)), true
	}
	if userID, ok := api.UserIDFromContext(ctx); ok && strings.TrimSpace(userID) != "" {
		return strings.ToLower(strings.TrimSpace(userID)), true
	}
	return "", false
}

func assistantConfigured() bool {
	return assistantBaseURL() != ""
}

func assistantBaseURL() string {
	return strings.TrimSpace(os.Getenv("ASSISTANT_BASE_URL"))
}
