# external

`external` is the public Go HTTP API for the homelab app stack.

## Endpoints

- `/users/count`
- `/readiness`
- `/liveness`
- `/metrics`

The service is published behind the gateway at `/api/...`.
