# external

`external` is the public Go HTTP API for the homelab app stack.

It is the browser-facing API for `ui` and should remain a stable surface even as the internal async pipeline evolves behind `orchestrator` and `processor`.

## Endpoints

- `/users/count`
- `/readiness`
- `/liveness`
- `/metrics`

The service is published behind the gateway at `/api/...`.
