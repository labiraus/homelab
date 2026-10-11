package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"external/mcpauth"
	"github.com/jackc/pgx/v5/pgxpool"
	"pkg/api"
)

func registerMCPOAuth(ctx context.Context, mux *http.ServeMux) error {
	if !strings.EqualFold(os.Getenv("MCP_OAUTH_ENABLED"), "true") {
		return nil
	}
	dsn := strings.TrimSpace(os.Getenv("MCP_OAUTH_DATABASE_URL"))
	if dsn == "" {
		return fmt.Errorf("native MCP OAuth requires its dedicated database login")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("invalid native MCP OAuth database configuration")
	}
	cfg.MaxConns = 4
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("native MCP OAuth database unavailable")
	}
	configured := false
	defer func() {
		if !configured {
			pool.Close()
		}
	}()
	var user string
	var version int
	var elevated, owner, canon, userWrite bool
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err = pool.QueryRow(checkCtx, `SELECT current_user,(SELECT rolsuper OR rolcreatedb OR rolcreaterole OR rolbypassrls OR rolinherit FROM pg_roles WHERE rolname=current_user),(SELECT tableowner=current_user FROM pg_tables WHERE schemaname='mcp_oauth' AND tablename='token'),CASE WHEN to_regnamespace('worldbuilder') IS NULL THEN false ELSE has_schema_privilege(current_user,'worldbuilder','USAGE') END,has_table_privilege(current_user,'auth.users','INSERT'),(SELECT max(version) FROM mcp_oauth.schema_version)`).Scan(&user, &elevated, &owner, &canon, &userWrite, &version)
	if err != nil || user != "mcp_oauth_runtime" || elevated || owner || canon || userWrite || version != 1 {
		return fmt.Errorf("native MCP OAuth requires the isolated non-owner mcp_oauth_runtime role and explicit schema")
	}
	server, err := mcpauth.New(os.Getenv("MCP_OAUTH_ISSUER_URL"), &mcpauth.Postgres{Pool: pool}, func(r *http.Request) (string, bool) {
		identity, ok := api.AuthStatusFromContext(r.Context())
		return identity.Email, ok && identity.Valid && identity.Mode == api.AuthModeOIDC
	})
	if err != nil {
		return err
	}
	// Cookie checks go only to the configured private proxy endpoint; caller URLs,
	// forwarded identities and redirects cannot change the authentication target.
	proxy, err := url.Parse(os.Getenv("MCP_OAUTH_COOKIE_AUTH_URL"))
	if err != nil || proxy.Scheme != "http" || !strings.HasSuffix(proxy.Hostname(), ".svc.cluster.local") || proxy.User != nil || proxy.RawQuery != "" || proxy.Fragment != "" || proxy.Path != "/oauth2/auth" {
		return fmt.Errorf("native MCP OAuth requires its private cookie validation endpoint")
	}
	client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	issuer, _ := url.Parse(server.Issuer)
	server.CheckCookie = func(ctx context.Context, cookie string) (string, error) {
		request, err := http.NewRequestWithContext(ctx, "GET", proxy.String(), nil)
		if err != nil {
			return "", err
		}
		request.Header.Set("Cookie", cookie)
		request.Header.Set("X-Forwarded-Proto", "https")
		request.Header.Set("X-Forwarded-Host", issuer.Host)
		response, err := client.Do(request)
		if err != nil {
			return "", errors.New("cookie authority unavailable")
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusAccepted && response.StatusCode != http.StatusOK {
			return "", mcpauth.ErrDenied
		}
		email := strings.ToLower(strings.TrimSpace(response.Header.Get("X-Auth-Request-Email")))
		if email == "" {
			return "", mcpauth.ErrDenied
		}
		valid, _, err := validateIdentityEmail(ctx, email)
		if err != nil {
			return "", err
		}
		if !valid {
			return "", mcpauth.ErrDenied
		}
		return email, nil
	}
	server.Register(mux)
	configured = true
	go func() { <-ctx.Done(); pool.Close() }()
	return nil
}
