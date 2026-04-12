# mcp

`mcp` is the AI-native public front door for agents and MCP-compatible clients.

Externally, this service should be treated as the Labiraus MCP server even though the repo and Kubernetes objects still use the `mcp` app name internally.

It exposes one MCP entrypoint that fronts:

- orchestrator actions for document workflows
- direct Postgres-backed read capabilities
- direct MinIO-backed document-bucket capabilities
- prompt examples for current and planned Labiraus capabilities
- a planned NATS-backed document notification subscription surface

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
- `API_BASE_URL` for orchestrator-backed HTTP proxy operations
- `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DATABASE`, `POSTGRES_SSLMODE` for Postgres-backed capabilities
- `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_USE_SSL`, `MINIO_REGION`, `MINIO_BUCKET` for MinIO-backed capabilities

If Postgres or MinIO configuration is omitted, the service still starts and advertises the relevant capabilities, but live calls against those backends return backend-unavailable errors until configuration is provided.

## Auth Surface

The current all-or-nothing access model is:

- Google-backed bearer authentication for browser and bearer-token capable clients
- trusted client-certificate authentication for certificate-authenticated MCP clients

The current browser-login choice in this repo is `oauth2-proxy + Google` for `ui` and `external`. That browser path does not add a local login endpoint to `mcp`; instead, `mcp` publishes protected-resource metadata for bearer-capable clients and separately documents the certificate-auth path in its metadata. When `X-Forwarded-Client-Cert` or `X-Auth-Request-Email` is forwarded by the edge, `mcp` consumes the shared auth middleware so proxied upstream calls can carry a normalized user identity in context.

## Prompt And Notification Roadmap

The MCP manifest now lists example prompts for both current and planned capabilities. The durable prompt catalog is documented in [docs/MCPPrompts.md](/workspaces/homelab/docs/MCPPrompts.md).

Document lifecycle notifications are still planned. The intended pattern is:

- `orchestrator` or the MinIO-ingest boundary emits `documents.events.minio.stored`
- `orchestrator` emits `documents.events.processor.queued`
- `processor` emits `documents.events.processor.started`
- `processor` emits `documents.events.processor.completed`
- `mcp` subscribes to those NATS JetStream events and forwards matching updates to MCP subscribers for a document-specific notification resource
