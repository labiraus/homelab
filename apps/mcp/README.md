# mcp

`mcp` is the AI-native public front door for agents and MCP-compatible clients.

Externally, this service should be treated as the Labiraus MCP server even though the repo and Kubernetes objects still use the `mcp` app name internally.

It exposes one MCP entrypoint that fronts:

- orchestrator actions for document workflows
- direct Postgres-backed read capabilities
- direct MinIO-backed document-bucket capabilities, including folder-aware browsing and binary uploads
- prompt examples for current and planned Labiraus capabilities
- a live NATS-backed document notification subscription surface

The capability catalog in [manifest.go](/workspaces/homelab/apps/mcp/manifest.go) is the source of truth for both the published MCP manifest and runtime dispatch behavior. Live and planned capabilities share the same registry, with planned entries surfaced in the manifest through `_meta.lifecycle` without pretending they are executable yet.

## Endpoints

- `/mcp`
- `/.well-known/mcp.json`
- `/.well-known/oauth-protected-resource`
- `/readiness`
- `/liveness`

The health endpoints are provided by [pkg/api](/workspaces/homelab/apps/pkg/api).
The shared host route also needs to publish the two `/.well-known/*` discovery endpoints, not only `/mcp`, or MCP clients cannot discover the advertised OAuth protected-resource metadata.
Prompt discovery and retrieval are exposed through the MCP transport with `prompts/list` and `prompts/get`.

## Runtime Configuration

Live direct-backend capabilities currently expect:

- `OIDC_ISSUER_URL` for federated identity discovery. Defaults to `https://accounts.google.com`, so bearer-capable MCP clients can use standard Google/OIDC authorization discovery instead of a service-local login path.
- `API_BASE_URL` for orchestrator-backed HTTP proxy operations. Defaults to `http://homelab-orchestrator.homelab.svc.cluster.local`, and the Helm chart sets that in-cluster service URL explicitly.
- `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DATABASE`, `POSTGRES_SSLMODE` for Postgres-backed capabilities
- `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_USE_SSL`, `MINIO_REGION`, `MINIO_BUCKET` for MinIO-backed capabilities
- `NATS_URLS`, `NATS_EVENTS_STREAM`, and `NATS_EVENTS_SUBJECT` for document lifecycle subscriptions and resource notifications

For Streamable HTTP hardening, `mcp` validates the `Origin` header on incoming MCP requests against the request origin by default. Set `MCP_ALLOWED_ORIGINS` to a comma-separated list only if the deployment intentionally needs additional browser origins.

If Postgres or MinIO configuration is omitted, the service still starts and advertises the relevant capabilities, but live calls against those backends return backend-unavailable errors until configuration is provided.

The live MinIO shape now supports:

- `minio.documents.listFolder` for folder-and-file views of a prefix
- `minio.documents.listObjects` for flat object inventory
- `homelab://mcp/minio/documents/objects/{objectKey}` for object reads, including binary-safe blob responses
- `minio.documents.putObject` for base64-backed binary uploads
- `minio.documents.putTextObject` for text-first writes
- `minio.documents.deleteObject` for deletes

## Auth Surface

The current all-or-nothing access model is:

- Google-backed bearer authentication for browser and bearer-token capable clients
- trusted client-certificate authentication for certificate-authenticated MCP clients

The current browser-login choice in this repo is `oauth2-proxy + Google` for `ui` and `external`. That browser path does not add a local login endpoint to `mcp`; instead, `mcp` publishes protected-resource metadata for bearer-capable clients and separately documents the certificate-auth path in its metadata. When `X-Forwarded-Client-Cert` or `X-Auth-Request-Email` is forwarded by the edge, `mcp` consumes the shared auth middleware so proxied upstream calls can carry a normalized user identity in context.

## Prompt And Notification Surface

The MCP manifest now lists example prompts for both current and planned capabilities. The durable prompt catalog is documented in [docs/MCPPrompts.md](/workspaces/homelab/docs/MCPPrompts.md).

Document lifecycle notifications are live through the Streamable HTTP transport:

- `initialize` advertises `resources.subscribe`
- successful initialization returns `MCP-Session-Id`
- subsequent `POST /mcp` requests require `MCP-Session-Id`
- `MCP-Protocol-Version` is honored when present; clients that omit it use the version negotiated during `initialize`, and supported-version header drift is tolerated for native client compatibility
- legacy `2024-11-05` negotiation is accepted for Codex/RMCP client compatibility
- JSON-RPC notifications and response-only batches return `202 Accepted` with no body, including unknown notification methods, so strict clients do not receive invalid notification responses
- `GET /mcp` opens the server-to-client SSE stream for that session
- `resources/subscribe` and `resources/unsubscribe` control subscriptions for `homelab://mcp/documents/notifications/{documentId}`
- matching lifecycle events emit `notifications/resources/updated`

The current event sequence is:

- `documents.events.processor.queued`
- `documents.events.processor.started`
- `documents.events.processor.completed`
- `documents.events.processor.failed`

`documents.events.minio.stored` remains reserved for a future ingest-boundary emitter. Notification fan-out is best-effort and does not block the underlying document queue or processing path.
