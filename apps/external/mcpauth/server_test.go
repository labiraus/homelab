package mcpauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRedirectBoundary(t *testing.T) {
	for _, v := range []string{"https://client.example/callback", "http://127.0.0.1:1234/callback", "http://[::1]:4321/callback"} {
		if !validRedirect(v) {
			t.Fatal(v)
		}
	}
	for _, v := range []string{"http://evil.example/callback", "http://localhost/callback", "https://user:pass@client.example/callback", "https://client.example/callback#fragment", "javascript:alert(1)", "//client.example/callback"} {
		if validRedirect(v) {
			t.Fatal(v)
		}
	}
	if !matchesRedirect("http://127.0.0.1/callback", "http://127.0.0.1:3456/callback") {
		t.Fatal("loopback port substitution refused")
	}
	for _, v := range []string{"http://127.0.0.2/callback", "http://127.0.0.1/callback/evil", "http://127.0.0.1/callback?x=1", "http://127.0.0.1/%63allback"} {
		if matchesRedirect("http://127.0.0.1/callback", v) {
			t.Fatal(v)
		}
	}
}
func TestNativeOAuthPostgres(t *testing.T) {
	dsn := os.Getenv("MCP_OAUTH_TEST_DATABASE_URL")
	adminDSN := os.Getenv("MCP_OAUTH_TEST_ADMIN_DATABASE_URL")
	if dsn == "" || adminDSN == "" {
		t.Skip("owned disposable OAuth PostgreSQL fixture not configured")
	}
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	admin, err := pgxpool.New(ctx, adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	var role string
	var elevated, owner, canon, authWrite bool
	err = pool.QueryRow(ctx, `SELECT current_user,(SELECT rolsuper OR rolcreatedb OR rolcreaterole OR rolbypassrls FROM pg_roles WHERE rolname=current_user),(SELECT tableowner=current_user FROM pg_tables WHERE schemaname='mcp_oauth' AND tablename='token'),has_schema_privilege(current_user,'worldbuilder','USAGE'),has_table_privilege(current_user,'auth.users','INSERT')`).Scan(&role, &elevated, &owner, &canon, &authWrite)
	if err != nil || role != "mcp_oauth_runtime" || elevated || owner || canon || authWrite {
		t.Fatalf("runtime isolation: %s %v %v %v %v %v", role, elevated, owner, canon, authWrite, err)
	}
	store := &Postgres{Pool: pool}
	s, err := New("https://mcp.example.test", store, func(r *http.Request) (string, bool) {
		return "owner@example.test", r.Header.Get("Fixture-Browser") == "authenticated"
	})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	s.Register(mux)
	call := func(method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://mcp.example.test"+path, strings.NewReader(body))
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	decode := func(w *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		var m map[string]any
		if json.Unmarshal(w.Body.Bytes(), &m) != nil {
			t.Fatalf("invalid JSON: %s", w.Body.String())
		}
		return m
	}
	formHeaders := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	metadata := decode(call("GET", "/.well-known/oauth-authorization-server", "", nil))
	if metadata["issuer"] != s.Issuer || metadata["authorization_response_iss_parameter_supported"] != true {
		t.Fatal(metadata)
	}
	registration := call("POST", "/mcp-oauth/register", `{"client_name":"Fixture <script>alert(1)</script>","redirect_uris":["http://127.0.0.1/callback"],"token_endpoint_auth_method":"none","grant_types":["authorization_code","refresh_token"]}`, nil)
	if registration.Code != 201 {
		t.Fatal(registration.Code, registration.Body.String())
	}
	client := decode(registration)["client_id"].(string)
	verifier := strings.Repeat("a", 43)
	authorization := func() url.Values {
		return url.Values{"response_type": {"code"}, "scope": {"mcp"}, "resource": {s.Resource}, "client_id": {client}, "redirect_uri": {"http://127.0.0.1:45123/callback"}, "state": {"test-state"}, "code_challenge_method": {"S256"}, "code_challenge": {pkce(verifier)}}
	}
	issueCode := func() string {
		t.Helper()
		q := authorization()
		w := call("GET", AuthorizePath+"?"+q.Encode(), "", map[string]string{"Fixture-Browser": "authenticated"})
		if w.Code != 200 {
			t.Fatalf("consent page %d %s", w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "<script>") {
			t.Fatal("client name not escaped")
		}
		matches := regexp.MustCompile(`name="request" value="([^"]+)"`).FindStringSubmatch(w.Body.String())
		if len(matches) != 2 {
			t.Fatal("missing consent handle")
		}
		cookie := w.Result().Cookies()[0]
		if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
			t.Fatal("unsafe cookie")
		}
		body := url.Values{"request": {matches[1]}, "decision": {"allow"}}.Encode()
		headers := map[string]string{"Fixture-Browser": "authenticated", "Content-Type": "application/x-www-form-urlencoded", "Origin": s.Issuer, "Cookie": cookie.Name + "=" + cookie.Value}
		badHeaders := map[string]string{}
		for k, v := range headers {
			badHeaders[k] = v
		}
		badHeaders["Origin"] = "https://evil.example"
		if call("POST", AuthorizePath, body, badHeaders).Code != 403 {
			t.Fatal("cross-origin consent accepted")
		}
		delete(badHeaders, "Origin")
		if call("POST", AuthorizePath, body, badHeaders).Code != 403 {
			t.Fatal("missing origin accepted")
		}
		w = call("POST", AuthorizePath, body, headers)
		if w.Code != 303 {
			t.Fatal(w.Code, w.Body.String())
		}
		dest, _ := url.Parse(w.Header().Get("Location"))
		if dest.Query().Get("iss") != s.Issuer || dest.Query().Get("state") != "test-state" {
			t.Fatal("issuer/state lost")
		}
		if call("POST", AuthorizePath, body, headers).Code != 403 {
			t.Fatal("consent handle replayed")
		}
		return dest.Query().Get("code")
	}
	t.Run("anonymous and malformed authorization refused", func(t *testing.T) {
		q := authorization()
		if call("GET", AuthorizePath+"?"+q.Encode(), "", map[string]string{"X-Forwarded-Email": "owner@example.test"}).Code != 401 {
			t.Fatal("forged browser identity accepted")
		}
		for key, value := range map[string]string{"redirect_uri": "https://evil.example/callback", "resource": "https://evil.example/mcp", "code_challenge_method": "plain", "scope": "admin"} {
			q = authorization()
			q.Set(key, value)
			if call("GET", AuthorizePath+"?"+q.Encode(), "", map[string]string{"Fixture-Browser": "authenticated"}).Code != 400 {
				t.Fatal(key)
			}
		}
		q = authorization()
		q.Add("redirect_uri", "https://evil.example")
		if call("GET", AuthorizePath+"?"+q.Encode(), "", map[string]string{"Fixture-Browser": "authenticated"}).Code != 400 {
			t.Fatal("duplicate parameter accepted")
		}
	})
	code := issueCode()
	tokenForm := func(code string) url.Values {
		return url.Values{"grant_type": {"authorization_code"}, "client_id": {client}, "resource": {s.Resource}, "code": {code}, "redirect_uri": {"http://127.0.0.1:45123/callback"}, "code_verifier": {verifier}}
	}
	for key, value := range map[string]string{"code_verifier": strings.Repeat("b", 43), "client_id": "other-client", "redirect_uri": "http://127.0.0.1:45124/callback", "resource": "https://other.example/mcp"} {
		f := tokenForm(code)
		f.Set(key, value)
		if call("POST", "/mcp-oauth/token", f.Encode(), formHeaders).Code != 400 {
			t.Fatal("bad token binding", key)
		}
	}
	w := call("POST", "/mcp-oauth/token", tokenForm(code).Encode(), formHeaders)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	tokens := decode(w)
	access, refresh := tokens["access_token"].(string), tokens["refresh_token"].(string)
	if call("POST", "/mcp-oauth/token", tokenForm(code).Encode(), formHeaders).Code != 400 {
		t.Fatal("code replay accepted")
	}
	check := func(access string) *httptest.ResponseRecorder {
		return call("POST", CheckPath, "", map[string]string{"Authorization": "Bearer " + access, "X-Forwarded-Email": "forged@example.test"})
	}
	if got := check(access); got.Code != 200 || got.Header().Get("X-Auth-Request-Email") != "owner@example.test" {
		t.Fatal("token identity not independently checked", got.Code)
	}
	if call("POST", CheckPath, "", map[string]string{"X-Forwarded-Email": "owner@example.test"}).Code != 401 {
		t.Fatal("forged transport accepted")
	}
	if _, err = store.Check(ctx, digest(access), "https://other.example/mcp"); !errors.Is(err, ErrDenied) {
		t.Fatal("wrong audience accepted")
	}
	var stored string
	if err = admin.QueryRow(ctx, `SELECT access_hash FROM mcp_oauth.token WHERE refresh_hash=$1`, digest(refresh)).Scan(&stored); err != nil || stored == access || stored != digest(access) {
		t.Fatal("raw credential stored", err)
	}
	t.Run("refresh rotation and family replay revocation", func(t *testing.T) {
		f := url.Values{"grant_type": {"refresh_token"}, "client_id": {client}, "refresh_token": {refresh}, "resource": {s.Resource}}
		w = call("POST", "/mcp-oauth/token", f.Encode(), formHeaders)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		next := decode(w)["access_token"].(string)
		if check(access).Code != 401 || check(next).Code != 200 {
			t.Fatal("rotation did not replace access")
		}
		if call("POST", "/mcp-oauth/token", f.Encode(), formHeaders).Code != 400 || check(next).Code != 401 {
			t.Fatal("replay did not revoke descendants")
		}
	})
	t.Run("concurrent code redemption is single use", func(t *testing.T) {
		code := issueCode()
		var successes atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				err := store.Exchange(ctx, CodeInput{Hash: digest(code), ClientID: client, Redirect: "http://127.0.0.1:45123/callback", Resource: s.Resource, Challenge: pkce(verifier)}, Credentials{AccessHash: digest(fmt.Sprint("race-access", i)), RefreshHash: digest(fmt.Sprint("race-refresh", i)), Family: "race-family"})
				if err == nil {
					successes.Add(1)
				} else if !errors.Is(err, ErrDenied) {
					t.Error(err)
				}
			}(i)
		}
		wg.Wait()
		if successes.Load() != 1 {
			t.Fatal(successes.Load())
		}
	})
	t.Run("refresh and revoke serialize across service instances", func(t *testing.T) {
		w := call("POST", "/mcp-oauth/token", tokenForm(issueCode()).Encode(), formHeaders)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		raw := decode(w)
		oldRefresh := raw["refresh_token"].(string)
		oldAccess := raw["access_token"].(string)
		other := &Postgres{Pool: pool}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			err := other.Rotate(ctx, digest(oldRefresh), client, s.Resource, Credentials{AccessHash: digest("concurrent-new-access"), RefreshHash: digest("concurrent-new-refresh")})
			if err != nil && !errors.Is(err, ErrDenied) {
				t.Error(err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := store.Revoke(ctx, digest(oldAccess), client); err != nil {
				t.Error(err)
			}
		}()
		wg.Wait()
		if _, err := store.Check(ctx, digest("concurrent-new-access"), s.Resource); !errors.Is(err, ErrDenied) {
			t.Fatal("descendant survived concurrent revocation", err)
		}
	})
	t.Run("new server instance retains authorized token", func(t *testing.T) {
		w := call("POST", "/mcp-oauth/token", tokenForm(issueCode()).Encode(), formHeaders)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		raw := decode(w)
		replacement, err := New(s.Issuer, &Postgres{Pool: pool}, s.BrowserIdentity)
		if err != nil {
			t.Fatal(err)
		}
		nextMux := http.NewServeMux()
		replacement.Register(nextMux)
		r := httptest.NewRequest("GET", CheckPath, nil)
		r.Header.Set("Authorization", "Bearer "+raw["access_token"].(string))
		response := httptest.NewRecorder()
		nextMux.ServeHTTP(response, r)
		if response.Code != 200 || response.Header().Get("X-Auth-Request-Email") != "owner@example.test" {
			t.Fatal("durable grant lost on replacement", response.Code)
		}
	})
	t.Run("expiry and explicit revocation", func(t *testing.T) {
		w = call("POST", "/mcp-oauth/token", tokenForm(issueCode()).Encode(), formHeaders)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		token := decode(w)["access_token"].(string)
		if _, err = admin.Exec(ctx, `UPDATE mcp_oauth.token SET expires_at=now()-interval '1 second' WHERE access_hash=$1`, digest(token)); err != nil {
			t.Fatal(err)
		}
		if check(token).Code != 401 {
			t.Fatal("expired access accepted")
		}
		w = call("POST", "/mcp-oauth/token", tokenForm(issueCode()).Encode(), formHeaders)
		token = decode(w)["access_token"].(string)
		if err = store.Revoke(ctx, digest(token), "other-client"); err != nil || check(token).Code != 200 {
			t.Fatal("other client revoked token")
		}
		if err = store.Revoke(ctx, digest(token), client); err != nil || check(token).Code != 401 {
			t.Fatal("revocation failed", err)
		}
	})
	t.Run("registry removal revokes tokens without resurrection", func(t *testing.T) {
		w := call("POST", "/mcp-oauth/token", tokenForm(issueCode()).Encode(), formHeaders)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		old := decode(w)["access_token"].(string)
		if _, err := admin.Exec(ctx, `DELETE FROM auth.users WHERE email='owner@example.test'`); err != nil {
			t.Fatal(err)
		}
		if check(old).Code != 401 {
			t.Fatal("removed user retained access")
		}
		if _, err := admin.Exec(ctx, `INSERT INTO auth.users(email,display_name) VALUES('owner@example.test','Fixture owner')`); err != nil {
			t.Fatal(err)
		}
		if check(old).Code != 401 {
			t.Fatal("re-adding user resurrected credentials")
		}
		// Fresh consent is required, and leaves an actual valid grant for the
		// separate database-restore/revocation drill in the fixture harness.
		w = call("POST", "/mcp-oauth/token", tokenForm(issueCode()).Encode(), formHeaders)
		if w.Code != 200 || check(decode(w)["access_token"].(string)).Code != 200 {
			t.Fatal("fresh consent failed")
		}
	})
	// Database outage must fail closed without pretending the token is invalid.
	pool.Close()
	if check(access).Code != 503 {
		t.Fatal("store outage did not report unavailable")
	}
}
