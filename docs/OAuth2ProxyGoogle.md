# oauth2-proxy + Google

This document describes the current browser authentication choice for the homelab public surface.

The chosen near-term pattern is:

- `oauth2-proxy` handles browser sign-in and session cookies
- Google is the upstream identity provider
- `ui` and `external` are protected by Istio external authorization using `oauth2-proxy`
- `mcp` continues to publish OAuth protected-resource metadata for bearer-capable clients and remains compatible with a future dedicated certificate-auth hostname

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
2. Istio calls the `oauth2-proxy` external authorizer.
3. If no valid session cookie is present, `oauth2-proxy` redirects the browser to Google.
4. Google redirects back to `/oauth2/callback`.
5. `oauth2-proxy` sets the browser session cookie.
6. Istio retries the request and forwards trusted headers such as `X-Auth-Request-Email` to `external`.
7. `external` validates the resulting email against `auth.users`.

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

- `helm/apps/oauth2-proxy/`
  - deploys `oauth2-proxy` on port `4180`
  - exposes `/oauth2` on `mcp.labiraus.com`
- `helm/bootstrap/istio/values.yaml`
  - defines the Istio `oauth2-proxy` extension provider
- `helm/apps/ui/values.yaml`
  - applies Istio `CUSTOM` auth through `oauth2-proxy`
- `helm/apps/external/values.yaml`
  - applies Istio `CUSTOM` auth through `oauth2-proxy`
  - publishes `OIDC_LOGIN_URL` as the local `/oauth2/start` URL
- `helm/apps/mcp/values.yaml`
  - sets `OIDC_ISSUER_URL` to Google for protected-resource discovery

## Required Cluster Secret

Before this flow works in-cluster, create the `oauth2-proxy-google` secret in the `homelab` namespace with:

- `OAUTH2_PROXY_CLIENT_ID`
- `OAUTH2_PROXY_CLIENT_SECRET`
- `OAUTH2_PROXY_COOKIE_SECRET`

`OAUTH2_PROXY_COOKIE_SECRET` should be a strong random secret suitable for cookie encryption.

The current chart expects that Secret to already exist in-cluster. It is intentionally not generated from tracked Helm values so real Google credentials do not need to live in Git.

## Current Boundaries

This choice applies to browser login for `ui` and `external`.

It does not replace the separate certificate-auth direction for `mcp`. For strict certificate authentication, the recommended future shape is still a dedicated hostname or dedicated listener instead of putting strict mTLS on the shared browser host.

## Related Docs

- [Authentication](/workspaces/homelab/docs/Auth.md)
- [Google OIDC Setup](/workspaces/homelab/docs/GoogleOIDCSetup.md)
- [external README](/workspaces/homelab/apps/external/README.md)
- [mcp README](/workspaces/homelab/apps/mcp/README.md)
