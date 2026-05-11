# mcp

`mcp` is the AI-native public front door for agents and MCP-compatible clients.

Externally, this service should be treated as the Labiraus MCP server even though the repo and Kubernetes objects still use the `mcp` app name internally.

It exposes one MCP entrypoint that fronts:

- orchestrator actions for document workflows
- direct Postgres-backed read capabilities
- direct MinIO-backed document-bucket capabilities, including folder-aware browsing and binary uploads
- prompt examples for the current Labiraus capability surface
- a live NATS-backed document notification subscription surface

The capability catalog in [manifest.go](/workspaces/homelab/apps/mcp/manifest.go) is the source of truth for both the published MCP manifest and runtime dispatch behavior.

## Endpoints

- `/mcp`
- `/sse`
- `/messages`
- `/message`
- `/.well-known/mcp.json`
- `/.well-known/oauth-protected-resource`
- `/.well-known/oauth-protected-resource/mcp`
- `/readiness`
- `/liveness`

The health endpoints are provided by [pkg/api](/workspaces/homelab/apps/pkg/api).
The shared host route also needs to publish the `/.well-known/*` discovery endpoints and legacy SSE paths, not only `/mcp`, or MCP clients cannot discover the advertised OAuth protected-resource metadata and older 2024 clients cannot fall back to HTTP+SSE.
Prompt discovery and retrieval are exposed through the MCP transport with `prompts/list` and `prompts/get`.

## Runtime Configuration

Live direct-backend capabilities currently expect:

- `OIDC_ISSUER_URL` for federated identity discovery. Defaults to `https://accounts.google.com`, so bearer-capable MCP clients can use standard Google/OIDC authorization discovery instead of a service-local login path.
- `API_BASE_URL` for orchestrator-backed HTTP proxy operations. Defaults to `http://homelab-orchestrator.homelab.svc.cluster.local`, and the Helm chart sets that in-cluster service URL explicitly.
- `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DATABASE`, `POSTGRES_SSLMODE` for Postgres-backed capabilities
- `EMBEDDING_MODEL` and optional `EMBEDDING_ENDPOINT` for semantic retrieval over processed document chunks. With `EMBEDDING_MODEL=local-embeddings` and no endpoint, `mcp` uses the built-in deterministic 384-dimensional local embedding function.
- `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_USE_SSL`, `MINIO_REGION`, `MINIO_BUCKET` for MinIO-backed capabilities
- `NATS_URLS`, `NATS_EVENTS_STREAM`, and `NATS_EVENTS_SUBJECT` for document lifecycle subscriptions and resource notifications

The Helm chart populates the live Postgres settings from the CNPG `data/app-db-bootstrap` secret and points `POSTGRES_HOST` at `app-db-rw.data.svc.cluster.local`. Its NetworkPolicy also allows egress to the CNPG `app-db` pods on port `5432` and to the orchestrator pods on port `8080` for HTTP proxy tools.

For Streamable HTTP hardening, `mcp` validates the `Origin` header on incoming MCP requests against the request origin by default. Set `MCP_ALLOWED_ORIGINS` to a comma-separated list only if the deployment intentionally needs additional browser origins.

If Postgres or MinIO configuration is omitted, the service still starts and advertises the relevant capabilities, but live calls against those backends return backend-unavailable errors until configuration is provided. Postgres initialization is retried in the background so orchestrator-proxied tools such as `documents.scanBucket` can stay available during a transient database connection failure.

The live MinIO shape now supports:

- `documents.scanBucket` for orchestrator-backed bucket inventory reconciliation and queueing of new or changed text objects
- `documents.curation.update` for orchestrator-backed curation of document inventory metadata
- `documents.editText` for orchestrator-backed text-object edits that queue a newer processing version
- `documents.reprocess` for orchestrator-backed requeueing of an existing inventory document at a newer processing version
- `documents.inventory.list` for Postgres-backed document inventory, curated metadata, and processing-state reads
- `documents.history.list` for durable Postgres-backed lifecycle history, optionally narrowed to one processing version
- `documents.search` for pgvector semantic search over the current processed chunk version, including document metadata and citation objects with source URI and chunk identity
- `documents.context` for pgvector-backed context assembly with stable citation references and citation objects
- `minio.documents.listFolder` for folder-and-file views of a prefix
- `minio.documents.listObjects` for flat object inventory
- `homelab://mcp/minio/documents/objects/{objectKey}` for object reads, including binary-safe blob responses
- `minio.documents.putObject` for base64-backed binary uploads
- `minio.documents.putTextObject` for text-first writes
- `minio.documents.deleteObject` for deletes
- `minio.documents.moveObject` for object rename and move workflows within the documents bucket

The live Postgres shape also supports `homelab://mcp/postgres/auth/users/{email}` for auth-user lookups.

`documents.inventory.list`, `documents.search`, and `documents.context` accept optional `documentId`, folder-like `prefix`, and exact-match `metadata` filters. Metadata filters are applied to `rag.documents.metadata`, so curated fields from `documents.curation.update` can narrow inventory and retrieval without introducing a separate graph or index service.

`tools/call` validates argument names, required fields, and primitive/object types against the selected tool's advertised input schema before execution. Unknown top-level arguments and malformed advertised fields return invalid-params errors so misspelled filters, malformed limits, missing request bodies, missing document IDs, or missing object keys do not silently fall back to broader defaults. Unknown nested body or metadata fields remain upstream-owned so service-specific payload evolution is not blocked by the MCP transport.

## Auth Surface

The current all-or-nothing access model is:

- Google-backed bearer authentication for browser and bearer-token capable clients
- trusted client-certificate authentication for certificate-authenticated MCP clients

The current browser-login choice in this repo is `oauth2-proxy + Google` for `ui` and `external`. That browser path does not add a local login endpoint to `mcp`; instead, `mcp` publishes protected-resource metadata for bearer-capable clients and separately documents the certificate-auth path in its metadata. When `X-Forwarded-Client-Cert` or `X-Auth-Request-Email` is forwarded by the edge, `mcp` consumes the shared auth middleware so proxied upstream calls can carry a normalized user identity in context.

## Prompt And Notification Surface

The MCP manifest now lists example prompts for the current capability surface. Prompt coverage includes ingestion, scan planning, inventory reads, metadata curation, text edits, reprocessing, retrieval, context assembly, lifecycle history, auth health, MinIO browsing, and document notification subscriptions. Prompt argument rendering follows the live tool schemas for scan bounds, edit metadata and versioning, retrieval filters and limits, and history filters; omitted optional arguments render as `not supplied` instead of leaking template placeholders, and unknown argument names are rejected so misspelled filters do not silently disappear. The durable prompt catalog is documented in [docs/MCPPrompts.md](/workspaces/homelab/docs/MCPPrompts.md).

Document lifecycle notifications are live through the Streamable HTTP transport:

- `initialize` advertises `resources.subscribe`
- successful initialization returns `MCP-Session-Id`
- subsequent session-bound `POST /mcp` requests require `MCP-Session-Id`, but the transport tolerates sessionless one-way messages and stateless request methods such as `resources/list` for native clients that drop the session header during startup
- JSON-RPC request responses sent through `POST /mcp`, including `initialize`, are returned as finite SSE `event: message` envelopes with `Content-Type: text/event-stream` for Codex/RMCP compatibility
- `DELETE /mcp` terminates a session and closes any attached stream
- UUID-shaped session IDs are restored on demand after pod restarts so native clients with cached stream sessions can recover without manual intervention
- `MCP-Protocol-Version` is honored when present; clients that omit it use the version negotiated during `initialize`, and supported-version header drift is tolerated for native client compatibility
- legacy `2024-11-05` negotiation is accepted for Codex/RMCP client compatibility
- JSON-RPC notifications, `id: null` notification variants, response-only messages, and response-only batches receive `202 Accepted` with an empty body so Streamable HTTP clients do not interpret one-way messages as JSON-RPC responses
- `GET /mcp` opens the server-to-client SSE stream for that session with an initial SSE comment and only emits real JSON-RPC messages or keepalive comments after that, not synthetic empty `data:` events
- `resources/subscribe` and `resources/unsubscribe` control subscriptions for `homelab://mcp/documents/notifications/{documentId}`
- matching lifecycle events emit `notifications/resources/updated`

For clients that still use the deprecated 2024-11-05 HTTP+SSE transport, `GET /sse` creates a legacy session and emits an `endpoint` event pointing at `/messages?sessionId=...`. Client JSON-RPC requests sent to that message endpoint are accepted with HTTP 202 and their JSON-RPC responses are delivered back as SSE `message` events. `/message` is kept as a compatibility alias for clients that use the singular endpoint name.

Browser-based MCP clients can preflight `/mcp`, `/sse`, `/messages`, and `/message`; CORS responses use the same `MCP_ALLOWED_ORIGINS` allowlist as Origin validation and expose `MCP-Session-Id` for Streamable HTTP clients.

The current event sequence is:

- `documents.events.minio.stored`
- `documents.events.processor.queued`
- `documents.events.processor.started`
- `documents.events.processor.completed`
- `documents.events.processor.failed`

`documents.events.minio.stored` is emitted by the browser-facing upload path after the raw object is written to MinIO. Notification fan-out is best-effort and does not block the underlying document upload, queue, or processing path.

Lifecycle events are also appended to `rag.document_lifecycle_events` when the orchestrator queues work and when the processor starts, completes, or fails a processing attempt. `documents.inventory.list` still exposes the latest event summary from `rag.documents.last_event_*`, while `documents.history.list` exposes the full recorded timeline.
