package main

import (
	"net/http"
	"testing"
)

func TestMCPHandler(t *testing.T) {
	request, recorder := httptestJSONRequest(t, http.MethodGet, "/mcp", "")
	mcpHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
}
