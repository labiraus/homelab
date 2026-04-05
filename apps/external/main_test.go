package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"pkg/api"
	"pkg/base"
	"pkg/prometheusutil"
)

func TestUserCountHandler(t *testing.T) {
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	originalFetchUserCount := fetchUserCount
	t.Cleanup(func() {
		fetchUserCount = originalFetchUserCount
	})

	fetchUserCount = func(ctx context.Context) (int, error) {
		return 7, nil
	}

	request := httptest.NewRequest(http.MethodGet, "/users/count", nil)
	recorder := httptest.NewRecorder()

	userCountHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response HelloResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json response: %v", err)
	}

	expected := "There are 7 users in the database."
	if response.Data != expected {
		t.Fatalf("expected %q, got %q", expected, response.Data)
	}
}

func TestUserCountHandlerReturnsErrorPayload(t *testing.T) {
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	originalFetchUserCount := fetchUserCount
	t.Cleanup(func() {
		fetchUserCount = originalFetchUserCount
	})

	fetchUserCount = func(ctx context.Context) (int, error) {
		return 0, fmt.Errorf("boom")
	}

	request := httptest.NewRequest(http.MethodGet, "/users/count", nil)
	recorder := httptest.NewRecorder()

	userCountHandler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json response: %v", err)
	}

	if response.Error != "could not fetch user count" {
		t.Fatalf("expected error payload to be set, got %q", response.Error)
	}
}

func TestAuthStatusHandlerReturnsMiddlewareStatus(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/auth/status", nil)
	request = request.WithContext(api.WithAuthStatus(request.Context(), api.AuthStatus{
		Mode:  api.AuthModeOIDC,
		Email: "oliver@labiraus.com",
		Valid: true,
	}))
	recorder := httptest.NewRecorder()

	authStatusHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response AuthStatusResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json response: %v", err)
	}

	if response.Mode != string(api.AuthModeOIDC) {
		t.Fatalf("expected auth mode %q, got %q", api.AuthModeOIDC, response.Mode)
	}
	if response.Email != "oliver@labiraus.com" {
		t.Fatalf("expected email to be returned, got %q", response.Email)
	}
	if !response.Valid {
		t.Fatal("expected auth status to be valid")
	}
}

func TestGoogleLoginHandlerRedirectsWhenConfigured(t *testing.T) {
	t.Setenv("OIDC_LOGIN_URL", "https://accounts.google.com/o/oauth2/v2/auth")

	request := httptest.NewRequest(http.MethodGet, "/auth/login/google", nil)
	recorder := httptest.NewRecorder()

	googleLoginHandler(recorder, request)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected redirect status, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "https://accounts.google.com/o/oauth2/v2/auth" {
		t.Fatalf("expected redirect location to be set, got %q", location)
	}
}
