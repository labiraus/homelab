# mcp

`mcp` is the AI-native public front door for agents and MCP-compatible clients.

It should expose stable capabilities while hiding internal pipeline details behind the MCP surface.

## Endpoints

- `/mcp`
- `/readiness`
- `/liveness`

The health endpoints are provided by [pkg/api](/workspaces/homelab/apps/pkg/api).
