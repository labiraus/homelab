# oauth2-proxy + Google

This document describes the current browser authentication choice for the homelab public surface.

The chosen near-term pattern is:

- `oauth2-proxy` handles browser sign-in and session cookies
- Google is the upstream identity provider
- `oauth2-proxy` is the browser-facing reverse proxy for both `ui` and `external`
- `mcp` continues to publish OAuth protected-resource metadata for bearer-capable clients while also documenting the current client-certificate access path for certificate-authenticated MCP clients

This keeps browser authentication simple now without locking the repo into Google-only logic inside the application code.

## Current URLs

Shared public hostname:

- `https://mcp.labiraus.com`

oauth2-proxy browser endpoints on that host:

- login start: `https://mcp.labiraus.com/oauth2/start?rd=https%3A%2F%2Fmcp.labiraus.com%2F`
- callback: `https://mcp.labiraus.com/oauth2/callback`

Public legal pages used by Google Auth Platform:

- privacy policy: `https://mcp.labiraus.com/privacy-policy.html`
- terms of service: `https://mcp.labiraus.com/terms-of-service.html`

## How The Flow Works

1. The browser reaches `ui` or `external` through the shared host.
2. The public Gateway sends `/`, `/api`, and `/oauth2/*` to `oauth2-proxy`.
3. If no valid session cookie is present, `oauth2-proxy` redirects the browser to Google.
4. Google redirects back to `/oauth2/callback`.
5. `oauth2-proxy` sets the browser session cookie and then proxies:
   - `/` to `ui`
   - `/api/...` to `external`
6. `oauth2-proxy` forwards trusted identity headers to `external`. The API prefers `X-Forwarded-Email` and also tolerates related proxy user/email header variants so browser auth remains stable across proxy versions and modes.
7. `external` validates the resulting email against `auth.users`.

When `oauth2-proxy` is fronting mesh-internal services behind Istio, it should not preserve the public browser `Host` header on upstream requests. In this repo, `OAUTH2_PROXY_PASS_HOST_HEADER=false` is important so Istio's outbound HTTP listener matches the request to `homelab-ui` or `homelab-external` instead of falling back to `PassthroughCluster` for `mcp.labiraus.com`.

## Google Auth Platform Changes

For the Google OAuth client, use a `Web application` client.

Set these values:

- Authorized redirect URIs:
  - `https://mcp.labiraus.com/oauth2/callback`
- Authorized JavaScript origins:
  - `https://mcp.labiraus.com`

Set these branding/support URLs:

- Home page: `https://mcp.labiraus.com/`
- Privacy policy: `https://mcp.labiraus.com/privacy-policy.html`
- Terms of service: `https://mcp.labiraus.com/terms-of-service.html`

For a rough PoC, keep the Google app in testing mode and add your own Google account as a test user.

## DNS Changes

If `mcp.labiraus.com` already points at the public Istio gateway, no additional DNS records are required for `oauth2-proxy`.

Why:

- `/oauth2/start`
- `/oauth2/callback`
- `/privacy-policy.html`
- `/terms-of-service.html`

are path-based routes on the same hostname, not separate DNS names.

If `mcp.labiraus.com` does not already exist in DNS, create the normal record that points the hostname at the public gateway load balancer:

- `A` record if you are targeting a fixed IP
- `CNAME` record if you are targeting a managed DNS name from the load balancer layer

No new hostname is required for the current browser login choice.

## Kubernetes And Helm Changes In This Repo

The repo now carries:

- `helm/infra/oauth2-proxy/`
  - deploys `oauth2-proxy` on port `4180`
  - exposes `/`, `/api`, and `/oauth2` on `mcp.labiraus.com`
  - proxies `/` to `ui`
  - proxies `/api/...` to `external`
  - redirects browser hits on `/oauth2` and `/oauth2/auth` back to `/` so users do not get stranded on the raw auth-check endpoint
- `helm/bootstrap/istio/values.yaml`
  - defines the Istio `oauth2-proxy` extension provider
  - points that provider at `homelab-oauth2-proxy.homelab.svc.cluster.local:80`
  - sends external-auth checks to `/oauth2/auth`
  - adds an explicit `X-Auth-Request-Redirect` back to `https://mcp.labiraus.com/` so successful browser sign-in returns to the UI instead of leaving the browser on `/oauth2/auth`
- `helm/apps/ui/values.yaml`
  - no longer publishes `/` directly on the shared host
- `helm/apps/external/values.yaml`
  - no longer publishes `/api` directly on the shared host
  - publishes `OIDC_LOGIN_URL` as the local `/oauth2/start` URL
- `helm/apps/mcp/values.yaml`
  - sets `OIDC_ISSUER_URL` to Google for protected-resource discovery
  - exposes both `/.well-known/mcp.json` and `/.well-known/oauth-protected-resource` on the shared host

## Required Cluster Secret

Before this flow works in-cluster, create the `oauth2-proxy-google` secret in the `homelab` namespace with:

- `OAUTH2_PROXY_CLIENT_ID`
- `OAUTH2_PROXY_CLIENT_SECRET`
- `OAUTH2_PROXY_COOKIE_SECRET`

`OAUTH2_PROXY_COOKIE_SECRET` should be a strong random secret suitable for cookie encryption.

The current chart expects that Secret to already exist in-cluster. It is intentionally not generated from tracked Helm values so real Google credentials do not need to live in Git.

## Current Boundaries

This choice applies to browser login for `ui` and `external`, both of which now sit behind `oauth2-proxy` on the shared hostname instead of using Istio browser ext-auth directly.

It does not replace the separate certificate-auth path for `mcp`. The current Labiraus MCP access story is all-or-nothing: clients authenticate either through the Google-backed bearer path or through trusted client certificates. A dedicated hostname or listener is still the cleaner long-term shape for strict certificate-only access, but the shared host can already consume trusted `X-Forwarded-Client-Cert` details when that identity is forwarded by the edge.

## Related Docs

- [Authentication](/workspaces/homelab/docs/Auth.md)
- [Google OIDC Setup](/workspaces/homelab/docs/GoogleOIDCSetup.md)
- [external README](/workspaces/homelab/apps/external/README.md)
- [mcp README](/workspaces/homelab/apps/mcp/README.md)
