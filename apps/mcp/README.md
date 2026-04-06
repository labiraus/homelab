# mcp

`mcp` is the AI-native public front door for agents and MCP-compatible clients.

It exposes one MCP entrypoint that fronts:

- orchestrator actions for document workflows
- direct Postgres-backed read capabilities
- direct MinIO-backed document-bucket capabilities

The capability catalog in [manifest.go](/workspaces/homelab/apps/mcp/manifest.go) is the source of truth for both the published MCP manifest and runtime dispatch behavior. Live and planned capabilities share the same registry, with planned entries surfaced in the manifest through `_meta.lifecycle` without pretending they are executable yet.

## Endpoints

- `/mcp`
- `/.well-known/mcp.json`
- `/.well-known/auth/certificate.json`
- `/readiness`
- `/liveness`

The health endpoints are provided by [pkg/api](/workspaces/homelab/apps/pkg/api).

## Runtime Configuration

Live direct-backend capabilities currently expect:

- `API_BASE_URL` for orchestrator-backed HTTP proxy operations
- `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DATABASE`, `POSTGRES_SSLMODE` for Postgres-backed capabilities
- `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_USE_SSL`, `MINIO_REGION`, `MINIO_BUCKET` for MinIO-backed capabilities

If Postgres or MinIO configuration is omitted, the service still starts and advertises the relevant capabilities, but live calls against those backends return backend-unavailable errors until configuration is provided.
