## Scope

This file applies to everything under `apps/mcp/`.

## MCP Compatibility Rules

- Treat `/mcp` Streamable HTTP as the primary transport, but keep the legacy `/sse` plus `/messages` HTTP+SSE fallback working unless repo guidance explicitly drops 2024-era client support.
- When changing transport behavior, test at least `initialize`, `notifications/initialized`, `id: null` notification variants, `resources/list`, and one-way response messages.
- Keep supported protocol-version negotiation broad for native clients: currently `2025-11-25`, `2025-06-18`, `2025-03-26`, and `2024-11-05`.
- Keep `/.well-known/mcp.json`, OAuth protected-resource metadata, app README, and Helm route values aligned whenever endpoints or advertised transports change.
- Preserve Origin validation and CORS preflight behavior together; browser compatibility should not weaken DNS-rebinding protection.
