# external

`external` is the public Go HTTP API for the homelab app stack.

It is the browser-facing API for `ui` and should remain a stable surface even as the internal async pipeline evolves behind `orchestrator` and `processor`.

## Endpoints

- `/api/auth/status`
- `/api/auth/providers`
- `/api/users/count`
- `/readiness`
- `/liveness`
- `/metrics`

The browser-facing path is published through `oauth2-proxy` at `/api/...`, and the service now treats that `/api` prefix as its only public route shape.

## Authentication

`external` now attaches auth middleware from `apps/pkg/api`.

- certificate auth is derived from trusted `X-Forwarded-Client-Cert` details from Istio
- OIDC auth is expected to arrive from `oauth2-proxy` through trusted upstream identity headers, preferring `X-Forwarded-Email` and falling back to related proxy user/email headers or Basic Auth username when needed
- the authenticated email is validated against `auth.users` in Postgres
- `/api/auth/status` returns the resolved mode, email, validity, and invalid reason
- `/api/auth/providers` returns the configured federated login providers for browser clients
- in the current repo choice, `OIDC_LOGIN_URL` should point at the local `oauth2-proxy` browser start URL, not directly at Google
