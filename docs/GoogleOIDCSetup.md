# Google OIDC Setup

This document covers the Google-side setup required before the homelab `ui`, `external`, and `mcp` services can participate in a real Google-backed sign-in flow.

This repo does not complete the OAuth callback itself. Instead:

- `ui` reads `/api/auth/providers` and sends the browser to the published authorization URL
- `external` publishes provider metadata from `OIDC_LOGIN_URL`
- `mcp` publishes OAuth protected-resource metadata and points clients at `OIDC_ISSUER_URL`
- an upstream gateway or auth proxy completes the Google OAuth/OIDC flow and forwards the authenticated email header to `external`

Because of that split, most of the Google configuration belongs to the upstream auth layer rather than these app services directly.

The current chosen upstream auth layer is `oauth2-proxy` on the shared host `mcp.labiraus.com`.

## What You Need From Google

Minimum Google-side setup:

- a Google Cloud project
- an OAuth consent screen or equivalent Google Auth Platform branding setup
- an OAuth 2.0 client credential, usually a `Web application`
- at least one authorized redirect URI
- test users if the app is not ready for broader publication

For this repo, you usually do not register `ui`, `external`, or `mcp` themselves as the Google callback target. You register the callback handled by your upstream auth component.

## Recommended Architecture For This Repo

The practical flow is:

1. The browser reaches `ui`.
2. `ui` fetches `/api/auth/providers` from `external`.
3. `external` returns a Google provider record using `OIDC_LOGIN_URL`.
4. The browser navigates to that authorization URL.
5. Google authenticates the user and redirects back to the upstream auth proxy or gateway callback.
6. The upstream auth layer validates the response, creates its local session or token state, and forwards a trusted email header such as `X-Auth-Request-Email`.
7. `external` validates that email against `auth.users`.

The important implication is:

- Google must trust the redirect URI used by your upstream auth component
- this repo must trust the identity header emitted by that upstream auth component

## Google Cloud Checklist

Use this checklist when wiring up the PoC:

1. Create or choose a Google Cloud project.
2. Configure the OAuth consent screen / Google Auth branding.
3. Choose `External` audience if you are authenticating personal Google accounts outside a single Google Workspace organization.
4. Add your own Google account as a test user if the app is still in testing mode.
5. Create OAuth credentials for a `Web application`.
6. Add the exact authorized redirect URI used by your upstream auth proxy.
7. Record the client ID and client secret.

For a PoC, the most common Google error is a redirect mismatch. Google requires the redirect URI to match exactly, including:

- scheme such as `https`
- host name
- path
- trailing slash behavior

If the upstream auth layer sends a different callback URI than the one registered in Google Cloud, Google rejects the flow.

## Redirect URI Guidance

Register the redirect URI for the system that receives Google’s callback.

Examples:

- If you use `oauth2-proxy`, the redirect URI is usually the proxy callback endpoint, not `/api/auth/providers`.
- If you use a gateway-native OIDC integration, the redirect URI is whatever callback path that gateway defines.

For this repo, the following are not normally Google redirect URIs:

- `https://mcp.labiraus.com/api/auth/providers`
- `https://mcp.labiraus.com/api/auth/status`
- `https://mcp.labiraus.com/mcp`

Those are discovery, status, or protected-resource endpoints. They are not the OAuth callback target.

For the current chosen setup, the Google redirect URI is:

- `https://mcp.labiraus.com/oauth2/callback`

## Scopes

For basic sign-in, the usual OpenID Connect scopes are:

- `openid`
- `email`
- `profile`

That is enough for the repo’s current identity model, which keys users by email address in `auth.users`.

## Settings You Need In The Homelab Repo

After the Google client is created, map the values into the auth stack like this:

- `OIDC_ISSUER_URL`
  - set to `https://accounts.google.com` when Google is the issuer
- `OIDC_LOGIN_URL`
  - set to the upstream auth layer’s Google-start URL
  - this is what `external` publishes to the UI
- trusted identity header
  - the upstream auth layer must inject the email header expected by `apps/pkg/api`
  - today that default header is `X-Auth-Request-Email`

Important distinction:

- `OIDC_ISSUER_URL` identifies who issued the identity
- `OIDC_LOGIN_URL` identifies where the browser should begin the login flow

## What Google Client Type To Use

In most cases for this repo, use a `Web application` OAuth client.

That is the right fit when:

- the user signs in through a browser
- an upstream web gateway or auth proxy handles redirects
- the callback lands on an HTTPS endpoint you control

You usually do not need a JavaScript-origin-only Google client for this repo because the browser is not talking directly to Google from React code.

## Consent Screen And Test Mode

For a rough PoC:

- keep the app in testing mode
- add explicit test users
- avoid broad publication until redirect URIs, branding, and data usage are settled

If your Google app is unverified or still restricted to test users, sign-in works only for those allowed users.

## Common Failure Modes

Common causes of login failure:

- redirect URI mismatch in Google Cloud
- wrong client ID or secret in the upstream auth layer
- `OIDC_LOGIN_URL` points to the wrong start endpoint
- the upstream auth layer does not forward `X-Auth-Request-Email`
- the authenticated email is not present in `auth.users`
- `OIDC_ISSUER_URL` does not match the real issuer used by the auth stack

Symptoms you may see:

- Google shows a redirect URI mismatch error before returning to your site
- the UI login button exists but sends the browser to the wrong place
- `/api/auth/status` shows `mode: none`
- `/api/auth/status` shows `mode: oidc` but `valid: false`

## PoC Example

For a typical PoC using Google plus an upstream auth proxy:

1. Create a Google OAuth web client.
2. Register the proxy callback URI with Google.
3. Put the Google client ID and secret into that proxy.
4. Expose a start-login URL from that proxy.
5. Set `OIDC_LOGIN_URL` in the homelab deployment to that start-login URL.
6. Set `OIDC_ISSUER_URL=https://accounts.google.com`.
7. Ensure the proxy forwards `X-Auth-Request-Email`.
8. Add the signed-in email to `auth.users`.

For the current repo choice, the browser start URL published by `external` should be:

- `https://mcp.labiraus.com/oauth2/start?rd=https%3A%2F%2Fmcp.labiraus.com%2F`

## Related Repo Docs

- [Authentication](/workspaces/homelab/docs/Auth.md)
- [oauth2-proxy + Google](/workspaces/homelab/docs/OAuth2ProxyGoogle.md)
- [external README](/workspaces/homelab/apps/external/README.md)
- [ui README](/workspaces/homelab/apps/ui/README.md)
- [mcp README](/workspaces/homelab/apps/mcp/README.md)

## References

Official Google references used for this document:

- https://developers.google.com/identity/openid-connect/openid-connect
- https://developers.google.com/identity/protocols/oauth2/web-server
- https://support.google.com/googleapi/answer/6158849
- https://support.google.com/cloud/answer/15544987
