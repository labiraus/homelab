# Native MCP authentication

The website uses Google through OAuth2 Proxy. Native MCP clients need a separate authorization-code flow: the observed `codex mcp login homelab-mcp` failed at Google's unsupported dynamic client registration. The opt-in authorization server in `apps/external/mcpauth/` reuses website sign-in for explicit client consent, then issues resource-bound opaque tokens. It does not mint Google credentials. Disposable protocol/restore tests and production-clone migration rehearsals pass; production activation and actual client login remain separate gates.

## Protocol and isolation

Discovery advertises the local issuer, `mcp` scope, S256 PKCE and public-client registration. Registration accepts HTTPS redirects or numerical loopback HTTP callbacks. HTTPS addresses match exactly; loopback callbacks may vary only their port. Registration is bounded to thirty clients per minute and 1,000 retained clients across replicas; unapproved registrations expire after one hour. Approved client IDs remain stable. Client names and return addresses are displayed before consent, with escaped HTML and framing disabled.

Consent runs at `/api/mcp-oauth/authorize` behind the existing browser proxy. Only that exact mesh principal may reach it. Approval requires a registered user, same-origin POST and a separate secure, HTTP-only CSRF cookie bound to the pending request and user. Codes expire in two minutes and redeem atomically against client, redirect, resource and PKCE challenge. Access tokens last fifteen minutes; rotating refresh tokens retain the family's original seven-day expiry. Reuse of a spent refresh token revokes all descendants. Only hashes are stored in PostgreSQL. Removing a registered user cascades its pending grants and tokens; adding the same email again does not resurrect old credentials.

The gateway's `mcp-oauth` provider checks tokens through `/api/mcp-oauth/check`. Only the exact gateway and MCP service principals may reach that endpoint. MCP independently validates the original credential there too, so a forwarded identity or an edge path-matching discrepancy cannot authenticate a caller. It fails closed on errors and forwards only the validated email/user headers; application registered-user validation remains in place. Existing browser cookies are independently checked with the private OAuth2 Proxy endpoint. Caller identity headers never substitute for either validation. Public routing exposes only authorization metadata, registration, token exchange and revocation; consent remains on the authenticated `/api` route.

`mcp_oauth_runtime` is a separate non-owner login with no inheritance, elevated database privileges, canon access or user-registry writes. It can read registered users, operate its own credential tables and read its schema version. External refuses startup with the wrong role or schema. Neither startup nor Helm installs SQL.

The new external ALLOW policy preserves existing non-OAuth paths, whose existing NetworkPolicies and Worldbuilder DENY boundaries still apply. Consent has an explicit browser-proxy allowance. Without these rules, adding the first ALLOW policy would silently deny the website and retained service APIs.

## Activation order

1. Pass `WORLDBUILDER_TEST_TRANSPORT=exec bash scripts/worldbuilder/test-postgres.sh` when local Docker forwarding requires the existing exec bridge. The owned fixture covers protocol bindings, consent refusal, replay/concurrency, expiry, revocation, replacement and role isolation. Run MCP/app tests and the composed manifest/API-server gate too.
2. Rehearse `sql/mcp/001_up.sql` and `sql/mcp/grants.sql` on an isolated production restore. Compare existing table, role and function fingerprints; only the intended registry SELECT/USAGE grants should change. Take a fresh production recovery point before applying the additive migration.
3. Provision a distinct CNPG basic-auth Secret and enable `nativeOAuthRole` in the PostgreSQL chart. Its `passwordSecret` supplies the `mcp_oauth_runtime` login. Preserve the closed Worldbuilder owner and existing runtime identities. Apply schema/grants explicitly as the database operator.
4. Create an indirect external Secret containing only `MCP_OAUTH_DATABASE_URL`, with the dedicated role and TLS required. Publish external's `nativeOAuth.enabled`, `databaseSecret`, exact `browserPrincipal` and observed gateway/proxy settings. Native MCP remains on its existing provider during this stage.
5. Verify public metadata/registration, browser-proxy-only consent, gateway/MCP-only token checks and private cookie validation. Check effective Istio providers after both child and parent charts reconcile; the parent replaces provider lists. There is only one selected CUSTOM provider on the observed gateway, so switch the existing MCP policy rather than adding competing providers.
6. Publish MCP's `nativeOAuth.enabled` with the same canonical host and a verified immutable app artifact. All Streamable HTTP and legacy transport paths retain edge checks. Observe anonymous, forged-header, invalid/expired-token and unavailable-authority refusals.
7. Run `codex mcp login homelab-mcp` in the user's actual configured client. Complete the displayed browser consent, then verify initialize, tool listing and a real document read. Keep Worldbuilder delegation separately gated. Native access does not grant human review, publication or administration.

Never write tokens or connection passwords into Git, logs, command arguments or chat. CLI authentication may need its ordinary local credential store; use the user's existing connection rather than replacing their client configuration with a fixture.

## Restart, restore and revocation

Production staging on 2026-10-11: backup `20261011T203021` completed before native authorization changes. The dedicated `data/mcp-oauth-credentials` Secret is provisioned separately and `flux-data` enables its managed role while preserving the closed canon migration owner. Schema installation and application activation are subsequent explicit steps.

Ordinary service replacement retains valid credentials in the shared store. Database restoration is different: a backup can precede a revocation. Before exposing a restored authorization store, apply `sql/mcp/revoke-after-restore.sql` to clear pending requests/codes and revoke all token families, then require fresh browser consent. Preserve approved client identities. Do not automatically perform this reset on ordinary startup.

The standard revocation endpoint accepts a token bound to its client and revokes its family. An operator may revoke families by registered email through the dedicated schema when a client or device is lost. No raw token is needed for an operator-side revocation. Retain the website's existing Google session and registered-user controls.

The protocol follows the [MCP authorization specification](https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization), [OpenAI client authentication contract](https://developers.openai.com/plugins/build/auth) and [Codex configuration](https://learn.chatgpt.com/docs/extend/mcp?surface=cli). Gateway failure behavior uses the explicit [Istio external authorization settings](https://istio.io/latest/docs/reference/config/istio.mesh.v1alpha1/#MeshConfig-ExtensionProvider-EnvoyExternalAuthorizationHttpProvider).
