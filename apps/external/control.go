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

	"pkg/prometheusutil"
)

const (
	documentCurationLabel      = "documentCurationHandler"
	documentEditTextLabel      = "documentEditTextHandler"
	documentReprocessLabel     = "documentReprocessHandler"
	maxDocumentControlBodySize = 1 << 20
)

type orchestratorDocumentActionResponse struct {
	statusCode  int
	contentType string
	body        []byte
}

var (
	orchestratorHTTPClient          = &http.Client{Timeout: 30 * time.Second}
	proxyOrchestratorDocumentAction = callOrchestratorDocumentAction
)

func documentCurationHandler(w http.ResponseWriter, r *http.Request) {
	handleOrchestratorDocumentAction(documentCurationLabel, "/documents/curation", w, r)
}

func documentEditTextHandler(w http.ResponseWriter, r *http.Request) {
	handleOrchestratorDocumentAction(documentEditTextLabel, "/documents/edit-text", w, r)
}

func documentReprocessHandler(w http.ResponseWriter, r *http.Request) {
	handleOrchestratorDocumentAction(documentReprocessLabel, "/documents/reprocess", w, r)
}

func handleOrchestratorDocumentAction(label string, path string, w http.ResponseWriter, r *http.Request) {
	var err error
	startTime := time.Now()
	prometheusutil.IncrementProcessed(label, "call")

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
			w.WriteHeader(http.StatusInternalServerError)
		}
		if err != nil {
			slog.ErrorContext(r.Context(), err.Error())
			prometheusutil.IncrementProcessed(label, "error")
		}
		prometheusutil.OpDuration(label, time.Since(startTime))
	}()

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !orchestratorConfigured() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "document control plane is unavailable"})
		return
	}

	body, readErr := io.ReadAll(io.LimitReader(r.Body, maxDocumentControlBodySize+1))
	if readErr != nil {
		err = readErr
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid request body"})
		return
	}
	if len(body) > maxDocumentControlBodySize {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "request body is too large"})
		return
	}

	var request map[string]any
	if err = json.Unmarshal(body, &request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid request body"})
		return
	}

	response, proxyErr := proxyOrchestratorDocumentAction(r.Context(), path, body)
	if proxyErr != nil {
		err = proxyErr
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "document control plane request failed"})
		return
	}

	contentType := strings.TrimSpace(response.contentType)
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(response.statusCode)
	if len(response.body) > 0 {
		if _, err = w.Write(response.body); err != nil {
			return
		}
	}
}

func callOrchestratorDocumentAction(ctx context.Context, path string, body []byte) (orchestratorDocumentActionResponse, error) {
	endpoint := strings.TrimRight(orchestratorBaseURL(), "/") + path
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return orchestratorDocumentActionResponse{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")

	response, err := orchestratorHTTPClient.Do(request)
	if err != nil {
		return orchestratorDocumentActionResponse{}, err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxDocumentControlBodySize+1))
	if err != nil {
		return orchestratorDocumentActionResponse{}, err
	}
	if len(responseBody) > maxDocumentControlBodySize {
		return orchestratorDocumentActionResponse{}, fmt.Errorf("orchestrator response exceeded %d bytes", maxDocumentControlBodySize)
	}

	return orchestratorDocumentActionResponse{
		statusCode:  response.StatusCode,
		contentType: response.Header.Get("Content-Type"),
		body:        responseBody,
	}, nil
}

func orchestratorConfigured() bool {
	return orchestratorBaseURL() != ""
}

func orchestratorBaseURL() string {
	return strings.TrimSpace(os.Getenv("ORCHESTRATOR_BASE_URL"))
}
