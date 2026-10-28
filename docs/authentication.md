# Authentication

Google authenticates browser users through oauth2-proxy. External authorizes the
normalized email against `auth.users`, then checks current world authority.
Native MCP clients use the separate local authorization server described below.
Metadata discovery, Google login and authenticated tool execution are distinct gates.

## Browser identity and routing

The shared Go middleware reports `mode`, `email`, `valid` and `invalidReason` through
`/api/auth/status`. Trusted certificate identity uses `mode: certificate`; trusted
proxy identity uses `oidc`; absent identity uses `none`. Unknown registered-user
identity fails application authorization. Never rerun the truncating legacy
`sql/auth.pgsql` seed against a populated registry.

`invalidReason: "identity validation failed"` means an email identity reached the
API, but the registered-user lookup returned an error. Check PostgreSQL readiness,
the `app-db-rw` ready endpoints and application-to-database transport before changing
Google settings or registering the account again. The current UI labels all invalid
statuses "Signed out", including this backend failure. `email is not recognized`
is the separate response for a completed lookup that found no registered account.

The browser route is `mcp.labiraus.com` → oauth2-proxy → UI or external. The UI reads
`/api/auth/providers`; external publishes the start URL in `OIDC_LOGIN_URL`.
The proxy forwards sanitized identity, with `X-Forwarded-Email` preferred by the
existing API. The API's supported header fallbacks are safe only behind the exact
trusted proxy/mesh boundary. Caller-supplied headers are not authentication.

Keep `OAUTH2_PROXY_PASS_HOST_HEADER=true` and the proxy-scoped mesh VirtualService
in `helm/infra/oauth2-proxy/templates/upstream-routing.yaml`. Preserving the public
Host lets external enforce same-origin writes; the explicit route avoids
PassthroughCluster against STRICT mTLS. Do not weaken Origin checks to compensate
for broken routing. Keep parent and child Istio provider arrays aligned: parent
values replace lists. Adding the first workload ALLOW policy must preserve all
existing intended routes.

## Google configuration

Use the existing project's Web application OAuth client, not a separate callback
inside each app. Repository-required settings are:

| Setting | Value |
| --- | --- |
| Browser start | `https://mcp.labiraus.com/oauth2/start?rd=https%3A%2F%2Fmcp.labiraus.com%2F` |
| Exact redirect URI | `https://mcp.labiraus.com/oauth2/callback` |
| JavaScript origin / home | `https://mcp.labiraus.com` / `https://mcp.labiraus.com/` |
| Privacy / terms | `/privacy-policy.html` / `/terms-of-service.html` on that host |
| Scopes | `openid email profile` |
| Proxy email domains | `OAUTH2_PROXY_EMAIL_DOMAINS="*"` |
| Intended audience | External, with production publishing for deployed access |
| Secret | `homelab/oauth2-proxy-google` (see [secrets](secrets.md)) |

An Internal Google project restricts sign-in to its Workspace organization.
Changing the audience does not register accounts or grant world membership.
Configure the existing client/project through Google Auth Platform; do not rotate
credentials just to change audience. Provider UI/policy may evolve; confirm the
current project settings before making changes.

Use the Administrator account-management UI to register intended normalized Google
emails. Administrators can manage global Administrator access through the audited
API with final-Administrator protection. Grant Author/Reader world access separately
where appropriate. The legacy `superuser` field/table is a compatibility name,
not a PostgreSQL superuser role. See [governance](worldbuilder/api.md).

Acceptance requires a fresh session for a registered outside-domain account,
`mode: oidc`, `valid: true` and permitted world actions; an unregistered account must
remain invalid and unable to read worlds. The proxy wildcard, lack of an `hd` filter
or a successful Google callback alone does not establish these behaviors. The
latest retained audience/device limitations are in [status](status.md).

## Client certificates

The optional certificate path uses gateway-validated email identity and
`SANITIZE_SET` forwarding of `X-Forwarded-Client-Cert`. It prefers URI SAN
`spiffe://homelab/users/<email>`, with email in the subject also supported.
The application client-auth CA is separate from the SSH user CA.

Use `bash scripts/generate-client-cert.sh init-ca ./.local/auth-certs "Homelab Client CA"`
only when creating a new CA. Issue a client with
`bash scripts/generate-client-cert.sh issue-client ./.local/auth-certs <registered-email>`;
export a password-protected PKCS#12 bundle with the script's `export-p12` command
when browser import is needed. Keep all keys/bundles local-only and protect the
export password. Use the system's verified server trust for HTTPS; a client-auth
CA is not automatically the server certificate issuer.

The gateway's `clientCertificateValidation.enabled` and `secretName` reference
`ingress/homelab-client-ca` containing public `ca.crt`. The Gateway renders
`spec.tls.frontend.default.validation.caCertificateRefs`. Enabling strict client
validation on the shared HTTPS listener makes certificates mandatory before browser
OIDC. Keep normal HTTPS for the shared browser hostname; use a dedicated listener
or hostname for certificate-only access.

## Native MCP authorization

Native authorization reuses website sign-in for explicit client consent, then
issues resource-bound opaque credentials. It does not mint Google tokens.
The normal source paths support both legacy SSE and Streamable HTTP. Preserve
public discovery and authenticate every transport path.

## Protocol and isolation

Discovery advertises the local issuer, `mcp` scope, S256 PKCE and public-client registration. Registration accepts HTTPS redirects or numerical loopback HTTP callbacks. HTTPS addresses match exactly; loopback callbacks may vary only their port. Registration is bounded to thirty clients per minute and 1,000 retained clients across replicas; unapproved registrations expire after one hour. Approved client IDs remain stable. Client names and return addresses are displayed before consent, with escaped HTML and framing disabled.

Consent runs at `/api/mcp-oauth/authorize` behind the existing browser proxy. Only that exact mesh principal may reach it. Approval requires a registered user, same-origin POST and a separate secure, HTTP-only CSRF cookie bound to the pending request and user. Codes expire in two minutes and redeem atomically against client, redirect, resource and PKCE challenge. Access tokens last fifteen minutes; rotating refresh tokens retain the family's original seven-day expiry. Reuse of a spent refresh token revokes all descendants. Only hashes are stored in PostgreSQL. Removing a registered user cascades its pending grants and tokens; adding the same email again does not resurrect old credentials.

The consent document uses `Referrer-Policy: same-origin`: `no-referrer` would turn a browser form POST's Origin into `null`, which must remain refused. Off-site callbacks receive no referrer. Its CSP permits the form's own origin and the already validated callback origin so Chromium can follow the consent redirect; codes remain bound to the complete registered URI. External CI exercises both decisions and callback delivery using real Chromium on owned loopback endpoints, alongside null/foreign Origin refusal.

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

Observe parent and child Flux values together during a provider switch; prevent an
older in-flight parent build from restoring an old child pin, and resume normal
reconciliation when published values agree. Temporary recovery patches must converge
to committed inputs. Dated native-login acceptance is in [status](status.md).

A repeated consent POST after a successful redirect may return 403. Check client
completion and timestamped status-only logs before treating a stale browser error
as a failed grant. Never expose callback codes or tokens. A CLI login and an existing
conversation's MCP connection are separate clients and need independent checks.
Public validators must expect the selected native issuer/endpoints and S256 PKCE,
not the legacy Google discovery contract.

Ordinary service replacement retains valid credentials in the shared store. Database restoration is different: a backup can precede a revocation. Before exposing a restored authorization store, apply `sql/mcp/revoke-after-restore.sql` to clear pending requests/codes and revoke all token families, then require fresh browser consent. Preserve approved client identities. Do not automatically perform this reset on ordinary startup.

The standard revocation endpoint accepts a token bound to its client and revokes its family. An operator may revoke families by registered email through the dedicated schema when a client or device is lost. No raw token is needed for an operator-side revocation. Retain the website's existing Google session and registered-user controls.

The protocol follows the [MCP authorization specification](https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization), [OpenAI client authentication contract](https://developers.openai.com/plugins/build/auth) and [Codex configuration](https://learn.chatgpt.com/docs/extend/mcp?surface=cli). Gateway failure behavior uses the explicit [Istio external authorization settings](https://istio.io/latest/docs/reference/config/istio.mesh.v1alpha1/#MeshConfig-ExtensionProvider-EnvoyExternalAuthorizationHttpProvider).

## Legacy edge compatibility

When the selected edge provider checks the OAuth proxy directly, its
`pathPrefix: /oauth2/auth?` deliberately prevents Envoy's appended transport path
from changing the auth-only endpoint into `/oauth2/auth/mcp`. Native OAuth instead
uses the configured local check endpoint. Inspect the effective provider before
changing either path. Anonymous, forged-header, expired-token and unavailable-
authority refusal must hold on every exposed transport.
