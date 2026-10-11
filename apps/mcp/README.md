# mcp

`mcp` is the AI-native public front door for agents and MCP-compatible clients.

Worldbuilder delegation preserves exact signed date tokens in arguments and structured replies. Its `at` filter accepts an integer or decimal string; JavaScript clients should send strings outside the safe integer range and preserve tokens when reading numeric replies. Proposal `from`/`until` fields follow the [date wire contract](../../docs/worldbuilder/API.md). Request-bound signing and the human review boundary remain unchanged.

Externally, this service should be treated as the Labiraus MCP server even though the repo and Kubernetes objects still use the `mcp` app name internally.
It is a primary product surface of this repo alongside the browser UI, and should stay aligned with the same Postgres-backed document state and orchestrator-owned control flows.

CI stamps the published image tag into `pkg/base.BuildVersion` using the Docker `BUILD_VERSION` argument (local builds default to `local`). Discovery and MCP initialization expose this version. After MCP image/chart publication, Public Endpoint Validation waits for that exact version through verified public HTTPS; an older healthy deployment does not satisfy the check. Public availability/version evidence still does not establish authenticated tool behavior or the status of every Flux release.

It exposes one MCP entrypoint that fronts:

- external-proxied document workflow actions
- external-proxied Postgres/OpenSearch/MinIO-backed capabilities
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

Live proxy-backed capabilities currently expect:

- `OIDC_ISSUER_URL` for federated identity discovery. Defaults to `https://accounts.google.com`, so bearer-capable MCP clients can use standard Google/OIDC authorization discovery instead of a service-local login path.
- `API_BASE_URL` for external-backed HTTP proxy operations. Defaults to `http://homelab-external.homelab.svc.cluster.local`, and the Helm chart sets that in-cluster service URL explicitly.
- `NATS_URLS`, `NATS_EVENTS_STREAM`, and `NATS_EVENTS_SUBJECT` for document lifecycle subscriptions and resource notifications

The Helm chart no longer injects direct Postgres, OpenSearch, or MinIO credentials into `mcp`; its NetworkPolicy allows egress to NATS for notifications and to `external` for tool/resource execution.

For Streamable HTTP hardening, `mcp` validates the `Origin` header on incoming MCP requests against the request origin by default. Set `MCP_ALLOWED_ORIGINS` to a comma-separated list only if the deployment intentionally needs additional browser origins.

If `external` is unavailable, the service still starts and advertises the relevant capabilities, but live tool/resource calls return upstream errors until the external API recovers.

The live external-proxied document shape now supports:

- `documents.scanBucket` for orchestrator-backed bucket inventory reconciliation and queueing of new or changed text objects
- `documents.curation.update` for orchestrator-backed curation of document inventory metadata
- `documents.editText` for orchestrator-backed text-object edits that queue a newer processing version
- `documents.reprocess` for orchestrator-backed requeueing of an existing inventory document at a newer processing version
- `documents.inventory.list` for Postgres-backed document inventory, curated metadata, and processing-state reads
- `documents.history.list` for durable Postgres-backed lifecycle history, optionally narrowed to one processing version
- `documents.search` for OpenSearch neural search over the current processed chunk version, including document metadata, chunk metadata, and citation objects with source URI and chunk identity
- `documents.context` for OpenSearch-backed context assembly with stable citation references, citation objects, and richer chunk-aware labels when available
- `minio.documents.listFolder` for folder-and-file views of a prefix
- `minio.documents.listObjects` for flat object inventory
- `homelab://mcp/minio/documents/objects/{objectKey}` for object reads, including binary-safe blob responses
- `minio.documents.putObject` for base64-backed binary uploads
- `minio.documents.putTextObject` for text-first writes
- `minio.documents.deleteObject` for deletes
- `minio.documents.moveObject` for object rename and move workflows within the documents bucket

The live Postgres shape also supports `homelab://mcp/postgres/auth/users/{email}` for auth-user lookups.

`documents.inventory.list`, `documents.search`, and `documents.context` accept optional `documentId`, folder-like `prefix`, and exact-match `metadata` filters. Metadata filters are applied to `rag.documents.metadata`, so curated fields from `documents.curation.update` can narrow inventory and retrieval without introducing a separate graph or index service.

When the processor has chunk-level citation hints, such as HTML titles or heading paths, the search and context payloads surface them under `chunkMetadata` and use them to render richer citation labels without changing the stable citation ID format.

`tools/call` validates argument names, required fields, and primitive/object types against the selected tool's advertised input schema before execution. Tool schemas advertise strict top-level arguments with `additionalProperties: false`; unknown top-level arguments and malformed advertised fields return invalid-params errors so misspelled filters, malformed limits, missing request bodies, missing document IDs, or missing object keys do not silently fall back to broader defaults. Unknown nested body or metadata fields remain upstream-owned so service-specific payload evolution is not blocked by the MCP transport.

## Auth Surface

Every `/mcp`, `/sse`, `/messages` and `/message` request requires edge authentication and a registered identity. The chart attaches an Istio CUSTOM policy to the public Gateway using the existing Google/oauth2-proxy provider. MCP checks the normalized identity against external's `/api/auth/status`; unknown users, malformed responses and an unavailable identity authority are refused before transport/session handling. Discovery and CORS preflight remain public. The backend never forwards caller cookies or bearer credentials to the identity lookup.

The permanent policy and registered-user check are deployed. The verified artifact is pinned in the Flux apps chart, its OCI source is resumed, and temporary containment has been removed after public anonymous/forged-header refusal checks. Browser Google sign-in works. Current bindings proxy document/storage/database operations through external; unused direct-backend credentials were removed from the live MCP Secret.

Native login remains an explicit recovery gap. On 2026-10-11 the configured Codex connection at `https://mcp.labiraus.com/mcp` was tested directly with `codex mcp login homelab-mcp`; it failed before sign-in with `Dynamic client registration not supported`. Protected-resource discovery points at Google, while the client has no pre-registered OAuth client. OAuth2 Proxy's browser session does not implement an MCP authorization server. Simply enabling JWT validation would not provide registration, the native callback or resource-bound access tokens. Follow the [client configuration](https://learn.chatgpt.com/docs/extend/mcp?surface=cli) and [MCP authorization contract](https://developers.openai.com/plugins/build/auth) when completing this flow. Do not remove edge authentication to make discovery or login appear successful.

The discovery documents currently advertise bearer and certificate access modes, but neither native flow has end-to-end acceptance. The shared HTTPS listener does not request client certificates, and possession of a local certificate is not authentication. Treat those advertised modes as unfinished until issuer/client registration, callback, token audience, identity validation and a real document read are verified. Forwarded identity headers are only meaningful behind the authenticated and sanitising edge.

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

## Worldbuilder tools (opt-in)

`worldbuilder.worlds`, `.canon`, `.search`, `.record` and `.propose` are advertised only with `WORLDBUILDER_MCP_ENABLED=true`. A dedicated `WORLDBUILDER_MCP_KEY` (at least 32 cryptographically random bytes) signs the current authenticated actor and exact request to external’s narrow delegation endpoint. Raw user-ID context is insufficient. External rechecks active identity and membership; proposal tools create drafts only, never submit/review/publish or read private stories. See [contracts](../../docs/worldbuilder/API.md) and [LAN activation gates](../../docs/worldbuilder/LAN-RUNBOOK.md). The chart requires `worldbuilder.delegationSecret` and nonempty exact `worldbuilder.trustedEdgePrincipals`; both transports must be verified before activation. No secrets or canon database credentials belong in MCP source.
