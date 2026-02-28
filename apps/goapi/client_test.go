package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetUser(t *testing.T) {
	// Create a mock server to handle the HTTP request
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set the response status code
		w.WriteHeader(http.StatusOK)

		// Set the response body
		response := UserResponse{
			UserID:   1,
			Username: "John Doe",
			Email:    "john.doe@gmail.com",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Set the mock server URL as the userPath
	userPath = mockServer.URL + "/user"

	// Call the GetUser function
	user, err := GetUser(1)

	// Check if an error occurred
	if err != nil {
		t.Errorf("GetUser returned an error: %v", err)
	}

	// Check if the user ID is correct
	if user.UserID != 1 {
		t.Errorf("GetUser returned an incorrect user ID. Expected: 1, Got: %d", user.UserID)
	}

	// Check if the user name is correct
	expectedName := "John Doe"
	if user.Username != expectedName {
		t.Errorf("GetUser returned an incorrect user name. Expected: %s, Got: %s", expectedName, user.Username)
	}

	// Check if the user email is correct
	expectedEmail := "john.doe@gmail.com"
	if user.Email != expectedEmail {
		t.Errorf("GetUser returned an incorrect user email. Expected: %s, Got: %s", expectedEmail, user.Email)
	}
}
