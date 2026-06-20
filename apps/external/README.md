# external

`external` is the public Go HTTP API for the homelab app stack.

It is the browser-facing API for `ui` and should remain a stable surface even as the internal async pipeline evolves behind `orchestrator` and `processor`.

Keep document and assistant workflows on the existing `/api/documents/*` and `/api/assistant/*` shapes. Retrieval, cited context, and assistant calls should not introduce a parallel public RAG API unless the repo plan explicitly changes.

## Endpoints

- `/api/auth/status`
- `/api/auth/providers`
- `/api/assistant/...`
- `/api/users/count`
- `/api/documents/tree`
- `/api/documents/object`
- `/api/documents/upload`
- `/api/documents/events`
- `/api/documents/inventory`
- `/api/documents/history`
- `/api/documents/search`
- `/api/documents/context`
- `/api/documents/create-text`
- `/api/documents/curation`
- `/api/documents/edit-text`
- `/api/documents/reprocess`
- `/api/documents/revert`
- `/api/documents/scan-bucket`
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
- `/api/documents/inventory` returns Postgres-backed document inventory rows, processing versions, latest lifecycle summary fields, and curated metadata
- `/api/documents/history` returns the durable Postgres-backed lifecycle history for a document, optionally narrowed to one processing version
- `/api/documents/search` embeds a natural-language query, runs pgvector similarity search against the current processed chunk version, and returns ranked matches with document metadata, chunk metadata, and citation objects for the source URI plus chunk identity
- `/api/documents/context` uses the same retrieval path and assembles a compact context block with `[1]`, `[2]` style references plus citation objects and any available chunk-level citation hints
- `/api/documents/create-text` proxies text-object creation to `orchestrator` `POST /documents/create-text`
- `/api/documents/curation` proxies metadata-only updates to `orchestrator` `POST /documents/curation`
- `/api/documents/edit-text` proxies guarded text-object replacement requests to `orchestrator` `POST /documents/edit-text`
- `/api/documents/reprocess` proxies explicit processing refresh requests to `orchestrator` `POST /documents/reprocess`
- `/api/documents/revert` proxies MinIO-version-backed raw text reverts to `orchestrator` `POST /documents/revert`
- `/api/documents/scan-bucket` proxies bucket or prefix reconciliation requests to `orchestrator` `POST /documents/scan-bucket`

Inventory, search, and context requests accept optional `documentId`, folder-like `prefix`, and `metadata` filters. Inventory also accepts `status`. The `metadata` filter is an exact-match JSON object applied to `rag.documents.metadata`, for example `{"metadata":{"tag":"runbook"}}`.

History requests require `documentId` and accept optional `processingVersion` and `limit` fields. Results are read from `rag.document_lifecycle_events`, while `rag.documents.last_event_*` remains the quick inventory summary.

When `processor` extracts HTML, retrieval hits can also include chunk metadata such as the source page title and heading path. Citation labels use that metadata when available, but older text chunks still fall back to the existing `path chunk N` label shape.

Control action requests require `ORCHESTRATOR_BASE_URL` and preserve the orchestrator response status and JSON body. `external` validates method, configuration, and JSON shape, but the orchestrator remains responsible for document existence checks, raw text writes, version decisions, metadata persistence, bucket reconciliation, and queueing.

The MinIO-backed browser routes expect the standard MinIO runtime configuration:

- `MINIO_ENDPOINT`
- `MINIO_ACCESS_KEY`
- `MINIO_SECRET_KEY`
- `MINIO_USE_SSL`
- `MINIO_REGION`
- `MINIO_BUCKET`

The UI treats these document routes as authenticated functionality for recognized users, and the API is expected to stay behind the same trusted auth middleware as the rest of the browser-facing surface.

Semantic search also uses:

- `EMBEDDING_MODEL`, defaulting to `local-embeddings`
- `EMBEDDING_ENDPOINT`, only when routing to an external OpenAI-compatible embeddings service

When `EMBEDDING_MODEL=local-embeddings` and `EMBEDDING_ENDPOINT` is empty, `external` uses the same built-in deterministic 384-dimensional local embedding function as the processor so query vectors and stored chunk vectors remain comparable.

Document control proxying also uses:

- `ORCHESTRATOR_BASE_URL`

## Assistant Proxy Surface

`/api/assistant/...` proxies the authenticated browser assistant API to the internal `assistant` service.
The proxy forwards the resolved authenticated email through `X-Forwarded-Email` and `UserID` so assistant state remains scoped by user.

The assistant proxy expects:

- `ASSISTANT_BASE_URL`

## Document Event Stream

`external` also exposes an authenticated server-sent events stream at `/api/documents/events`.

- authenticated browser clients connect with `Accept: text/event-stream`
- each event payload is the raw lifecycle JSON emitted on NATS under `documents.events.>`
- successful browser uploads publish `documents.events.minio.stored` after the raw object is written to MinIO
- the current lifecycle subjects are `documents.events.minio.stored`, `documents.events.processor.queued`, `documents.events.processor.started`, `documents.events.processor.completed`, and `documents.events.processor.failed`
- the handler sends keepalives so long-lived browser connections stay open through the edge

The document event bridge expects:

- `NATS_URLS`
- `NATS_EVENTS_STREAM` with a default of `document-events`
- `NATS_EVENTS_SUBJECT` with a default of `documents.events.>`

Lifecycle delivery to the browser is best-effort. The document processing path remains successful even if the notification fan-out path is temporarily unavailable.
