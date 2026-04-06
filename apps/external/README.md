# external

`external` is the public Go HTTP API for the homelab app stack.

It is the browser-facing API for `ui` and should remain a stable surface even as the internal async pipeline evolves behind `orchestrator` and `processor`.

## Endpoints

- `/auth/status`
- `/auth/login/google`
- `/users/count`
- `/readiness`
- `/liveness`
- `/metrics`

The service is published behind the gateway at `/api/...`.

## Authentication

`external` now attaches auth middleware from `apps/pkg/api`.

- certificate auth is derived from trusted `X-Forwarded-Client-Cert` details from Istio
- OIDC auth is derived from a trusted upstream email header, currently `X-Auth-Request-Email`
- the authenticated email is validated against `auth.users` in Postgres
- `/auth/status` returns the resolved mode, email, validity, and invalid reason
- `/auth/login/google` redirects to `OIDC_LOGIN_URL`
