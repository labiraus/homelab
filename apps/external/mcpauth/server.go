package mcpauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const AuthorizePath = "/api/mcp-oauth/authorize"
const CheckPath = "/api/mcp-oauth/check"
const cookieName = "__Host-mcp_oauth_csrf"

var verifierPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
var challengePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
var consentPage = template.Must(template.New("consent").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Connect to Labiraus</title><main><h1>Connect to Labiraus</h1><p>Signed in as {{.Email}}.</p><p>Client: <strong>{{.Name}}</strong></p><p>This client can read documents, account information and worlds, and make permitted document and draft changes using your existing access. It cannot administer users or approve and publish Worldbuilder changes.</p><p>Return address: <code>{{.Redirect}}</code></p><form method="post" action="/api/mcp-oauth/authorize"><input type="hidden" name="request" value="{{.Request}}"><button type="submit" name="decision" value="allow">Allow connection</button> <button type="submit" name="decision" value="deny">Cancel</button></form></main></html>`))

type Server struct {
	Issuer, Resource string
	Store            Store
	// BrowserIdentity is used only behind the exact authenticated proxy mesh
	// boundary. CheckCookie independently verifies a cookie with that proxy.
	BrowserIdentity func(*http.Request) (string, bool)
	CheckCookie     func(context.Context, string) (string, error)
}

func New(issuer string, store Store, identity func(*http.Request) (string, bool)) (*Server, error) {
	u, err := url.Parse(issuer)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || store == nil || identity == nil {
		return nil, errors.New("native MCP OAuth requires a canonical HTTPS origin, store and browser identity")
	}
	issuer = strings.TrimSuffix(issuer, "/")
	return &Server{Issuer: issuer, Resource: issuer + "/mcp", Store: store, BrowserIdentity: identity}, nil
}
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("/.well-known/oauth-authorization-server", s.wrap(s.metadata))
	mux.HandleFunc("/mcp-oauth/register", s.wrap(s.register))
	mux.HandleFunc("/mcp-oauth/token", s.wrap(s.token))
	mux.HandleFunc("/mcp-oauth/revoke", s.wrap(s.revoke))
	mux.HandleFunc(AuthorizePath, s.wrap(s.authorize))
	mux.HandleFunc(CheckPath, s.wrap(s.check))
}
func (s *Server) wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		r.Body = http.MaxBytesReader(w, r.Body, 16384)
		next(w, r.WithContext(ctx))
	}
}
func jsonReply(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func failure(w http.ResponseWriter, status int, code string) {
	jsonReply(w, status, map[string]string{"error": code})
}
func method(w http.ResponseWriter, r *http.Request, wanted string) bool {
	if r.Method == wanted {
		return true
	}
	w.Header().Set("Allow", wanted)
	failure(w, 405, "invalid_request")
	return false
}
func opaque(prefix string) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(b)
}
func digest(value string) string { h := sha256.Sum256([]byte(value)); return hex.EncodeToString(h[:]) }
func pkce(value string) string {
	h := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
func validRedirect(raw string) bool {
	if len(raw) > 2048 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return u.Scheme == "http" && ip != nil && ip.IsLoopback()
}
func matchesRedirect(registered, actual string) bool {
	if !validRedirect(actual) {
		return false
	}
	if registered == actual {
		return true
	}
	a, err := url.Parse(registered)
	if err != nil {
		return false
	}
	b, _ := url.Parse(actual)
	return a.Scheme == "http" && b.Scheme == "http" && a.Hostname() == b.Hostname() && a.EscapedPath() == b.EscapedPath() && a.RawQuery == b.RawQuery
}
func (s *Server) metadata(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	jsonReply(w, 200, map[string]any{"issuer": s.Issuer, "authorization_endpoint": s.Issuer + AuthorizePath, "token_endpoint": s.Issuer + "/mcp-oauth/token", "registration_endpoint": s.Issuer + "/mcp-oauth/register", "revocation_endpoint": s.Issuer + "/mcp-oauth/revoke", "authorization_response_iss_parameter_supported": true, "response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code", "refresh_token"}, "token_endpoint_auth_methods_supported": []string{"none"}, "code_challenge_methods_supported": []string{"S256"}, "scopes_supported": []string{"mcp"}})
}
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	var input struct {
		Name      string   `json:"client_name"`
		Redirects []string `json:"redirect_uris"`
		Auth      string   `json:"token_endpoint_auth_method"`
		Grants    []string `json:"grant_types"`
		Responses []string `json:"response_types"`
		Scope     string   `json:"scope"`
	}
	d := json.NewDecoder(r.Body)
	if d.Decode(&input) != nil || d.Decode(new(any)) != io.EOF {
		failure(w, 400, "invalid_client_metadata")
		return
	}
	if input.Auth != "none" || len(input.Redirects) < 1 || len(input.Redirects) > 8 || len(input.Name) > 100 || (input.Scope != "" && input.Scope != "mcp") {
		failure(w, 400, "invalid_client_metadata")
		return
	}
	for _, g := range input.Grants {
		if g != "authorization_code" && g != "refresh_token" {
			failure(w, 400, "invalid_client_metadata")
			return
		}
	}
	for _, v := range input.Responses {
		if v != "code" {
			failure(w, 400, "invalid_client_metadata")
			return
		}
	}
	for _, uri := range input.Redirects {
		if !validRedirect(uri) {
			failure(w, 400, "invalid_redirect_uri")
			return
		}
	}
	if strings.TrimSpace(input.Name) == "" {
		input.Name = "MCP client"
	}
	client := Client{ID: opaque("mcp_client_"), Name: input.Name, Redirects: input.Redirects}
	if err := s.Store.Register(r.Context(), client); err != nil {
		if errors.Is(err, ErrLimited) {
			failure(w, 429, "temporarily_unavailable")
		} else {
			failure(w, 503, "temporarily_unavailable")
		}
		return
	}
	jsonReply(w, 201, map[string]any{"client_id": client.ID, "client_name": client.Name, "redirect_uris": client.Redirects, "token_endpoint_auth_method": "none", "grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"}, "scope": "mcp"})
}
func single(values url.Values) bool {
	for _, v := range values {
		if len(v) != 1 {
			return false
		}
	}
	return true
}
func readForm(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") && r.ParseForm() == nil && single(r.PostForm) && len(r.URL.RawQuery) == 0
}
func (s *Server) authorize(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy", "default-src 'none'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	w.Header().Set("X-Frame-Options", "DENY")
	email, ok := s.BrowserIdentity(r)
	if !ok || email == "" {
		failure(w, 401, "login_required")
		return
	}
	if r.Method == "POST" {
		s.consent(w, r, email)
		return
	}
	if !method(w, r, "GET") {
		return
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || !single(q) || q.Get("response_type") != "code" || q.Get("scope") != "mcp" || q.Get("resource") != s.Resource || q.Get("code_challenge_method") != "S256" || !challengePattern.MatchString(q.Get("code_challenge")) || len(q.Get("state")) < 1 || len(q.Get("state")) > 4096 || len(q.Get("client_id")) > 200 {
		failure(w, 400, "invalid_request")
		return
	}
	client, err := s.Store.Client(r.Context(), q.Get("client_id"))
	if err != nil {
		failure(w, 400, "invalid_client")
		return
	}
	matched := false
	for _, uri := range client.Redirects {
		matched = matched || matchesRedirect(uri, q.Get("redirect_uri"))
	}
	if !matched {
		failure(w, 400, "invalid_request")
		return
	}
	handle, csrf := opaque(""), opaque("")
	auth := Authorization{IDHash: digest(handle), ClientID: client.ID, Email: email, Redirect: q.Get("redirect_uri"), Resource: s.Resource, Challenge: q.Get("code_challenge"), State: q.Get("state"), CSRFHash: digest(csrf)}
	if err = s.Store.SaveAuthorization(r.Context(), auth); err != nil {
		failure(w, 503, "temporarily_unavailable")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: csrf, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 600})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = consentPage.Execute(w, struct{ Email, Name, Redirect, Request string }{email, client.Name, auth.Redirect, handle})
}
func (s *Server) consent(w http.ResponseWriter, r *http.Request, email string) {
	if r.Header.Get("Origin") != s.Issuer || !readForm(r) {
		failure(w, 403, "invalid_request")
		return
	}
	cookie, err := r.Cookie(cookieName)
	if err != nil || !challengePattern.MatchString(cookie.Value) || !challengePattern.MatchString(r.PostForm.Get("request")) {
		failure(w, 403, "invalid_request")
		return
	}
	decision := r.PostForm.Get("decision")
	if decision != "allow" && decision != "deny" {
		failure(w, 400, "invalid_request")
		return
	}
	code := opaque("mcp_code_")
	a, err := s.Store.Consent(r.Context(), digest(r.PostForm.Get("request")), digest(cookie.Value), email, digest(code), decision == "allow")
	if err != nil {
		failure(w, 403, "invalid_request")
		return
	}
	dest, err := url.Parse(a.Redirect)
	if err != nil {
		failure(w, 500, "server_error")
		return
	}
	q := dest.Query()
	q.Set("state", a.State)
	q.Set("iss", s.Issuer)
	if decision == "allow" {
		q.Set("code", code)
	} else {
		q.Set("error", "access_denied")
	}
	dest.RawQuery = q.Encode()
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	http.Redirect(w, r, dest.String(), http.StatusSeeOther)
}
func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	if !readForm(r) || r.Header.Get("Authorization") != "" {
		failure(w, 400, "invalid_request")
		return
	}
	f := r.PostForm
	client := f.Get("client_id")
	if len(client) < 1 || len(client) > 200 {
		failure(w, 400, "invalid_client")
		return
	}
	access, refresh := opaque("mcp_a_"), opaque("mcp_r_")
	t := Credentials{AccessHash: digest(access), RefreshHash: digest(refresh), Family: opaque("")}
	var err error
	switch f.Get("grant_type") {
	case "authorization_code":
		if f.Get("resource") != s.Resource || !verifierPattern.MatchString(f.Get("code_verifier")) || len(f.Get("code")) > 200 || !validRedirect(f.Get("redirect_uri")) {
			failure(w, 400, "invalid_grant")
			return
		}
		err = s.Store.Exchange(r.Context(), CodeInput{Hash: digest(f.Get("code")), ClientID: client, Redirect: f.Get("redirect_uri"), Resource: s.Resource, Challenge: pkce(f.Get("code_verifier"))}, t)
	case "refresh_token":
		if (f.Get("resource") != "" && f.Get("resource") != s.Resource) || len(f.Get("refresh_token")) < 1 || len(f.Get("refresh_token")) > 200 || (f.Get("scope") != "" && f.Get("scope") != "mcp") {
			failure(w, 400, "invalid_grant")
			return
		}
		err = s.Store.Rotate(r.Context(), digest(f.Get("refresh_token")), client, s.Resource, t)
	default:
		failure(w, 400, "unsupported_grant_type")
		return
	}
	if err != nil {
		if errors.Is(err, ErrDenied) {
			failure(w, 400, "invalid_grant")
		} else {
			failure(w, 503, "temporarily_unavailable")
		}
		return
	}
	jsonReply(w, 200, map[string]any{"access_token": access, "token_type": "Bearer", "expires_in": 900, "refresh_token": refresh, "scope": "mcp"})
}
func (s *Server) revoke(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	if !readForm(r) || len(r.PostForm.Get("token")) > 200 || len(r.PostForm.Get("client_id")) > 200 {
		failure(w, 400, "invalid_request")
		return
	}
	if err := s.Store.Revoke(r.Context(), digest(r.PostForm.Get("token")), r.PostForm.Get("client_id")); err != nil {
		failure(w, 503, "temporarily_unavailable")
		return
	}
	w.WriteHeader(200)
}
func (s *Server) check(w http.ResponseWriter, r *http.Request) {
	// Only the exact mesh gateway and MCP identities can reach this route.
	var email string
	var err error
	auth := r.Header.Get("Authorization")
	parts := strings.Fields(auth)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && strings.HasPrefix(parts[1], "mcp_a_") && len(auth) < 200 {
		email, err = s.Store.Check(r.Context(), digest(parts[1]), s.Resource)
	} else if auth == "" && r.Header.Get("Cookie") != "" && s.CheckCookie != nil {
		email, err = s.CheckCookie(r.Context(), r.Header.Get("Cookie"))
	} else {
		err = ErrDenied
	}
	if err != nil && !errors.Is(err, ErrDenied) {
		failure(w, 503, "temporarily_unavailable")
		return
	}
	if err != nil || email == "" {
		w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+s.Issuer+`/.well-known/oauth-protected-resource", scope="mcp"`)
		failure(w, 401, "invalid_token")
		return
	}
	w.Header().Set("X-Auth-Request-Email", email)
	w.Header().Set("X-Auth-Request-User", email)
	w.Header().Set("X-Forwarded-Email", email)
	w.WriteHeader(200)
}
