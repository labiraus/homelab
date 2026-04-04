# orchestrator

`orchestrator` is an internal Go service that validates document submissions and enqueues them to Kafka.

## Endpoints

- `POST /documents`
- `/readiness`
- `/liveness`

The health endpoints are provided by [pkg/api](/workspaces/homelab/apps/pkg/api).
