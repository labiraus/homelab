# external

`external` is the public Go HTTP API for the homelab app stack.

It is the browser-facing API for `ui` and should remain a stable surface even as the internal async pipeline evolves behind `orchestrator` and `processor`.

## Endpoints

- `/api/auth/status`
- `/api/auth/providers`
- `/api/users/count`
- `/api/documents/tree`
- `/api/documents/object`
- `/api/documents/upload`
- `/api/documents/events`
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

## Document Browser Surface

`external` now also fronts the browser-facing MinIO document browser.

- `/api/documents/tree` returns the immediate folders and files for a prefix in the documents bucket
- `/api/documents/object` streams a document back for inline preview or download
- `/api/documents/upload` accepts multipart uploads for the current folder view

These routes expect the standard MinIO runtime configuration:

- `MINIO_ENDPOINT`
- `MINIO_ACCESS_KEY`
- `MINIO_SECRET_KEY`
- `MINIO_USE_SSL`
- `MINIO_REGION`
- `MINIO_BUCKET`

The UI treats these document routes as authenticated functionality for recognized users, and the API is expected to stay behind the same trusted auth middleware as the rest of the browser-facing surface.

## Document Event Stream

`external` also exposes an authenticated server-sent events stream at `/api/documents/events`.

- authenticated browser clients connect with `Accept: text/event-stream`
- each event payload is the raw lifecycle JSON emitted on NATS under `documents.events.>`
- the current lifecycle subjects are `documents.events.processor.queued`, `documents.events.processor.started`, `documents.events.processor.completed`, and `documents.events.processor.failed`
- the handler sends keepalives so long-lived browser connections stay open through the edge

The document event bridge expects:

- `NATS_URLS`
- `NATS_EVENTS_STREAM` with a default of `document-events`
- `NATS_EVENTS_SUBJECT` with a default of `documents.events.>`

Lifecycle delivery to the browser is best-effort. The document processing path remains successful even if the notification fan-out path is temporarily unavailable.
