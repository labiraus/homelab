package api

import (
	"context"
	"net/http"
	"net/url"
	"pkg/base"
	"strings"
)

type AuthMode string

const (
	AuthModeCertificate AuthMode = "certificate"
	AuthModeOIDC        AuthMode = "oidc"
	AuthModeNone        AuthMode = "none"
)

type AuthStatus struct {
	Mode          AuthMode `json:"mode"`
	Email         string   `json:"email,omitempty"`
	Valid         bool     `json:"valid"`
	InvalidReason string   `json:"invalidReason,omitempty"`
}

type AuthValidator func(ctx context.Context, email string) (bool, string, error)

type AuthOptions struct {
	CertificateIdentityHeader string
	OIDCEmailHeader           string
	Validator                 AuthValidator
}

type authStatusContextKey struct{}

func NewAuthMiddleware(options AuthOptions) func(next http.Handler) http.Handler {
	certificateHeader := headerOrDefault(options.CertificateIdentityHeader, "X-Forwarded-Client-Cert")
	oidcHeader := headerOrDefault(options.OIDCEmailHeader, "X-Auth-Request-Email")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			status := ResolveAuthStatus(r.Context(), r.Header, certificateHeader, oidcHeader)
			status = validateAuthStatus(r.Context(), status, options.Validator)
			ctx := WithAuthStatus(r.Context(), status)
			ctx = WithUserID(ctx, status.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ResolveAuthStatus(ctx context.Context, headers http.Header, certificateHeader string, oidcHeader string) AuthStatus {
	if existing, ok := AuthStatusFromContext(ctx); ok {
		return existing
	}

	if email, ok := extractCertificateEmail(headers.Get(certificateHeader)); ok {
		return AuthStatus{
			Mode:  AuthModeCertificate,
			Email: normalizeEmail(email),
		}
	}

	if email := resolveOIDCEmail(headers, oidcHeader); email != "" {
		return AuthStatus{
			Mode:  AuthModeOIDC,
			Email: email,
		}
	}

	return AuthStatus{
		Mode:          AuthModeNone,
		InvalidReason: "no authenticated identity was provided",
	}
}

func resolveOIDCEmail(headers http.Header, oidcHeader string) string {
	candidates := []string{
		oidcHeader,
		"X-Forwarded-Email",
		"X-Auth-Request-Email",
		"X-Email",
		"X-Forwarded-User",
		"X-Auth-Request-User",
		"X-User",
	}

	for _, header := range candidates {
		if email := normalizeEmail(headers.Get(header)); email != "" {
			return email
		}
	}

	if username, _, ok := basicAuthUsername(headers.Get("Authorization")); ok {
		if email := normalizeEmail(username); email != "" {
			return email
		}
	}

	return ""
}

func WithAuthStatus(ctx context.Context, status AuthStatus) context.Context {
	return context.WithValue(ctx, authStatusContextKey{}, status)
}

func AuthStatusFromContext(ctx context.Context) (AuthStatus, bool) {
	status, ok := ctx.Value(authStatusContextKey{}).(AuthStatus)
	return status, ok
}

func WithUserID(ctx context.Context, userID string) context.Context {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ctx
	}

	return context.WithValue(ctx, base.UserID, userID)
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(base.UserID).(string)
	return userID, ok
}

func validateAuthStatus(ctx context.Context, status AuthStatus, validator AuthValidator) AuthStatus {
	if status.Email == "" {
		if status.InvalidReason == "" {
			status.InvalidReason = "authenticated identity did not include an email address"
		}
		return status
	}

	if validator == nil {
		status.Valid = true
		status.InvalidReason = ""
		return status
	}

	valid, reason, err := validator(ctx, status.Email)
	if err != nil {
		status.Valid = false
		status.InvalidReason = "identity validation failed"
		return status
	}

	status.Valid = valid
	status.InvalidReason = reason
	return status
}

func extractCertificateEmail(xfcc string) (string, bool) {
	if strings.TrimSpace(xfcc) == "" {
		return "", false
	}

	for _, segment := range strings.Split(xfcc, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(segment), "=")
		if !ok {
			continue
		}

		switch strings.ToLower(key) {
		case "uri":
			if email := emailFromURI(unquoteXFCCValue(value)); email != "" {
				return email, true
			}
		case "subject":
			if email := emailFromSubject(unquoteXFCCValue(value)); email != "" {
				return email, true
			}
		}
	}

	return "", false
}

func emailFromURI(raw string) string {
	if raw == "" {
		return ""
	}

	if strings.HasPrefix(strings.ToLower(raw), "mailto:") {
		return normalizeEmail(strings.TrimPrefix(raw, "mailto:"))
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) == 0 {
		return ""
	}

	return normalizeEmail(segments[len(segments)-1])
}

func emailFromSubject(subject string) string {
	for _, part := range strings.Split(subject, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}

		switch strings.ToLower(key) {
		case "emailaddress", "e":
			return normalizeEmail(value)
		}
	}

	return ""
}

func normalizeEmail(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	if !strings.Contains(email, "@") {
		return ""
	}
	return email
}

func headerOrDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return http.CanonicalHeaderKey(value)
}

func unquoteXFCCValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "\"")
	value = strings.TrimSuffix(value, "\"")
	return value
}

func basicAuthUsername(authorization string) (string, string, bool) {
	if strings.TrimSpace(authorization) == "" {
		return "", "", false
	}

	request, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		return "", "", false
	}

	request.Header.Set("Authorization", authorization)
	username, password, ok := request.BasicAuth()
	return username, password, ok
}
