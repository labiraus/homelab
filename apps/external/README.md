# external

`external` is the public Go HTTP API for the homelab app stack.

It is the browser-facing API for `ui` and should remain a stable surface even as the internal async pipeline evolves behind `orchestrator` and `processor`.

## Endpoints

- `/auth/status`
- `/api/auth/status`
- `/auth/providers`
- `/api/auth/providers`
- `/users/count`
- `/api/users/count`
- `/readiness`
- `/liveness`
- `/metrics`

The browser-facing path is published through `oauth2-proxy` at `/api/...`, while the service continues to serve both root and `/api`-prefixed handlers for compatibility.

## Authentication

`external` now attaches auth middleware from `apps/pkg/api`.

- certificate auth is derived from trusted `X-Forwarded-Client-Cert` details from Istio
- OIDC auth is currently expected to arrive from `oauth2-proxy` through the trusted upstream email header `X-Forwarded-Email`
- the authenticated email is validated against `auth.users` in Postgres
- `/auth/status` returns the resolved mode, email, validity, and invalid reason
- `/auth/providers` returns the configured federated login providers for browser clients
- in the current repo choice, `OIDC_LOGIN_URL` should point at the local `oauth2-proxy` browser start URL, not directly at Google
