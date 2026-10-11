package main

import (
	"pkg/base"
	"strings"
)

func nativeOAuthIssuer() string {
	return strings.TrimRight(strings.TrimSpace(base.GetEnv("MCP_OAUTH_ISSUER_URL", "")), "/")
}
func authorizationAccessModes() []manifestAuthorizationAccessMode {
	if nativeOAuthIssuer() != "" {
		return []manifestAuthorizationAccessMode{{Type: "bearer", Name: "labiraus-oauth", Description: "Use the native MCP authorization-code and S256 PKCE flow, with explicit consent through the existing Google-backed website sign-in."}}
	}
	return []manifestAuthorizationAccessMode{
		{Type: "bearer", Name: "google-oidc", Description: "Google-backed bearer integration requires separate native-client acceptance."},
		{Type: "client-certificate", Name: "mtls-client-cert", Description: "Certificate integration requires a verified client-certificate listener and separate native-client acceptance."},
	}
}

func transportChallenge() string {
	if issuer := nativeOAuthIssuer(); issuer != "" {
		return `Bearer resource_metadata="` + issuer + `/.well-known/oauth-protected-resource", scope="mcp"`
	}
	return `Bearer resource_metadata="https://mcp.labiraus.com/.well-known/oauth-protected-resource"`
}
