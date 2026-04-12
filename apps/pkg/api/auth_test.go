package api

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"pkg/base"
	"testing"
)

func TestResolveAuthStatusCertificate(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-Forwarded-Client-Cert", `By=spiffe://cluster.local/ns/ingress/sa/gateway-istio;Hash=abc;URI=spiffe://homelab/users/oliver@labiraus.com`)

	status := ResolveAuthStatus(context.Background(), headers, "X-Forwarded-Client-Cert", "X-Auth-Request-Email")

	if status.Mode != AuthModeCertificate {
		t.Fatalf("expected certificate mode, got %q", status.Mode)
	}
	if status.Email != "oliver@labiraus.com" {
		t.Fatalf("expected certificate email to be parsed, got %q", status.Email)
	}
}

func TestResolveAuthStatusOIDC(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-Auth-Request-Email", "oliver@labiraus.com")

	status := ResolveAuthStatus(context.Background(), headers, "X-Forwarded-Client-Cert", "X-Auth-Request-Email")

	if status.Mode != AuthModeOIDC {
		t.Fatalf("expected oidc mode, got %q", status.Mode)
	}
	if status.Email != "oliver@labiraus.com" {
		t.Fatalf("expected oidc email to be parsed, got %q", status.Email)
	}
}

func TestResolveAuthStatusOIDCFallsBackToForwardedUser(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-Forwarded-User", "oliver@labiraus.com")

	status := ResolveAuthStatus(context.Background(), headers, "X-Forwarded-Client-Cert", "X-Forwarded-Email")

	if status.Mode != AuthModeOIDC {
		t.Fatalf("expected oidc mode, got %q", status.Mode)
	}
	if status.Email != "oliver@labiraus.com" {
		t.Fatalf("expected forwarded user email to be parsed, got %q", status.Email)
	}
}

func TestResolveAuthStatusOIDCFallsBackToBasicAuthUsername(t *testing.T) {
	headers := http.Header{}
	credentials := base64.StdEncoding.EncodeToString([]byte("oliver@labiraus.com:unused"))
	headers.Set("Authorization", "Basic "+credentials)

	status := ResolveAuthStatus(context.Background(), headers, "X-Forwarded-Client-Cert", "X-Forwarded-Email")

	if status.Mode != AuthModeOIDC {
		t.Fatalf("expected oidc mode, got %q", status.Mode)
	}
	if status.Email != "oliver@labiraus.com" {
		t.Fatalf("expected basic auth username email to be parsed, got %q", status.Email)
	}
}

func TestResolveAuthStatusNone(t *testing.T) {
	status := ResolveAuthStatus(context.Background(), http.Header{}, "X-Forwarded-Client-Cert", "X-Auth-Request-Email")

	if status.Mode != AuthModeNone {
		t.Fatalf("expected none mode, got %q", status.Mode)
	}
	if status.Valid {
		t.Fatal("expected no-auth status to be invalid")
	}
	if status.InvalidReason == "" {
		t.Fatal("expected no-auth status to include a reason")
	}
}

func TestNewAuthMiddlewareValidatesIdentity(t *testing.T) {
	middleware := NewAuthMiddleware(AuthOptions{
		Validator: func(ctx context.Context, email string) (bool, string, error) {
			if email == "oliver@labiraus.com" {
				return true, "", nil
			}
			return false, "email is not recognized", nil
		},
	})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, ok := AuthStatusFromContext(r.Context())
		if !ok {
			t.Fatal("expected auth status on request context")
		}
		if !status.Valid {
			t.Fatalf("expected valid identity, got invalid status: %+v", status)
		}
		userID, ok := r.Context().Value(base.UserID).(string)
		if !ok {
			t.Fatal("expected user id on request context")
		}
		if userID != "oliver@labiraus.com" {
			t.Fatalf("expected user id to match email, got %q", userID)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Auth-Request-Email", "oliver@labiraus.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected success response, got %d", recorder.Code)
	}
}

func TestNewAuthMiddlewareMarksUnknownUserInvalid(t *testing.T) {
	middleware := NewAuthMiddleware(AuthOptions{
		Validator: func(ctx context.Context, email string) (bool, string, error) {
			return false, "email is not recognized", nil
		},
	})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, ok := AuthStatusFromContext(r.Context())
		if !ok {
			t.Fatal("expected auth status on request context")
		}
		if status.Valid {
			t.Fatal("expected unknown user to be invalid")
		}
		if status.InvalidReason != "email is not recognized" {
			t.Fatalf("expected invalid reason to be preserved, got %q", status.InvalidReason)
		}
		userID, ok := r.Context().Value(base.UserID).(string)
		if !ok {
			t.Fatal("expected user id on request context")
		}
		if userID != "someone@example.com" {
			t.Fatalf("expected user id to match email, got %q", userID)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Auth-Request-Email", "someone@example.com")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected success response, got %d", recorder.Code)
	}
}

func TestWithUserIDLeavesContextUnchangedForEmptyValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), base.TraceID, "trace-123")
	updated := WithUserID(ctx, "  ")

	if updated != ctx {
		t.Fatal("expected empty user id to leave context unchanged")
	}
}
