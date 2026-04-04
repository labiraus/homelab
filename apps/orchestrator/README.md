# orchestrator

`orchestrator` is the internal Go control-plane service.

It owns reconciliation and workflow decisions, including when documents should be queued for asynchronous processing.

The current implementation is intentionally small and starts with manual document submission while the MinIO reconciliation flow is being built out.

## Endpoints

- `POST /documents`
- `/readiness`
- `/liveness`

The health endpoints are provided by [pkg/api](/workspaces/homelab/apps/pkg/api).
