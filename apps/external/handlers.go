package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"pkg/api"
	"pkg/prometheusutil"
	"strings"
	"time"
)

func userCountHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	startTime := time.Now()
	prometheusutil.IncrementProcessed(userCountLabel, "call")
	defer func() {
		p := recover()
		if p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
		if err != nil {
			slog.ErrorContext(r.Context(), err.Error())
			prometheusutil.IncrementProcessed(userCountLabel, "error")
		}
		prometheusutil.OpDuration(userCountLabel, time.Since(startTime))
	}()

	count, err := fetchUserCount(r.Context())
	if err != nil {
		err = fmt.Errorf("error fetching user count: %w", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "could not fetch user count"})
		return
	}

	response := HelloResponse{
		Data: fmt.Sprintf("There are %d users in the database.", count),
	}

	w.Header().Set("Content-Type", "application/json")
	if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
		err = fmt.Errorf("error marshalling json response: %w", encodeErr)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func authStatusHandler(w http.ResponseWriter, r *http.Request) {
	status, ok := api.AuthStatusFromContext(r.Context())
	if !ok {
		status = api.AuthStatus{
			Mode:          api.AuthModeNone,
			InvalidReason: "authentication middleware did not populate request context",
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(AuthStatusResponse{
		Mode:          string(status.Mode),
		Email:         status.Email,
		Valid:         status.Valid,
		InvalidReason: status.InvalidReason,
	})
}

func authProvidersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(AuthProvidersResponse{
		Providers: []AuthProvider{
			buildGoogleAuthProvider(),
		},
	})
}

func authUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	email := strings.TrimPrefix(r.URL.Path, "/api/auth/users/")
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "email is required"})
		return
	}

	user, found, err := fetchIdentityByEmail(r.Context(), email)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "could not fetch auth user"})
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "auth user not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
}

func buildGoogleAuthProvider() AuthProvider {
	loginURL := strings.TrimSpace(os.Getenv("OIDC_LOGIN_URL"))
	issuerURL := strings.TrimSpace(os.Getenv("OIDC_ISSUER_URL"))
	if issuerURL == "" {
		issuerURL = "https://accounts.google.com"
	}

	return AuthProvider{
		ID:               "google",
		Name:             "Google",
		Issuer:           issuerURL,
		AuthorizationURL: loginURL,
		Configured:       loginURL != "",
	}
}
