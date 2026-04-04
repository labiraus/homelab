# mcp

`mcp` is a small public Go service that exposes the cluster MCP endpoint.

## Endpoints

- `/mcp`
- `/readiness`
- `/liveness`

The health endpoints are provided by [pkg/api](/workspaces/homelab/apps/pkg/api).
