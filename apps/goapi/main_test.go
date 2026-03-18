package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserCountHandler(t *testing.T) {
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
