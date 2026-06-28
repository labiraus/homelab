package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"pkg/api"
	"pkg/base"
	"pkg/prometheusutil"
)

func TestAssistantProxyForwardsAuthenticatedIdentityToProcessor(t *testing.T) {
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())
	t.Setenv("PROCESSOR_BASE_URL", "http://processor")
	previous := proxyAssistantAPI
	t.Cleanup(func() {
		proxyAssistantAPI = previous
	})

	proxyAssistantAPI = func(ctx context.Context, path string, method string, body []byte) (assistantProxyResponse, error) {
		if path != "/assistant/chat" {
			t.Fatalf("expected processor assistant path, got %q", path)
		}
		if method != http.MethodPost {
			t.Fatalf("expected POST, got %q", method)
		}
		email, ok := proxiedEmail(ctx)
		if !ok || email != "oliver@labiraus.com" {
			t.Fatalf("expected authenticated email to be available, got %q", email)
		}
		return assistantProxyResponse{
			statusCode:  http.StatusOK,
			contentType: "application/json",
			body:        []byte(`{"ok":true}`),
		}, nil
	}

	request := httptest.NewRequest(http.MethodPost, "/api/assistant/chat", nil)
	request = request.WithContext(api.WithAuthStatus(request.Context(), api.AuthStatus{
		Mode:  api.AuthModeOIDC,
		Email: "Oliver@Labiraus.com",
		Valid: true,
	}))
	recorder := httptest.NewRecorder()

	assistantProxyHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	var response map[string]bool
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected JSON response: %v", err)
	}
	if !response["ok"] {
		t.Fatalf("expected ok response, got %#v", response)
	}
}
