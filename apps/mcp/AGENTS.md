## Scope

This file applies to everything under `apps/mcp/`.

## MCP Compatibility Rules

- Treat `/mcp` Streamable HTTP as the primary transport, but keep the legacy `/sse` plus `/messages` HTTP+SSE fallback working unless repo guidance explicitly drops 2024-era client support.
- When changing transport behavior, test at least `initialize`, `notifications/initialized`, `id: null` notification variants, `resources/list`, and one-way response messages.
- For Streamable HTTP, JSON-RPC request responses sent through `POST /mcp` should be finite SSE `event: message` responses rather than bare `application/json`; this matches OpenAI Docs MCP and current Codex/RMCP compatibility behavior.
- For Streamable HTTP, JSON-RPC notifications, response-only messages, and one-way batches should return `202 Accepted` with an empty response body. Do not add JSON-RPC ack bodies for those one-way messages unless a new client compatibility finding is documented in the same change.
- For Streamable HTTP, `GET /mcp` should open with an SSE comment so proxies release the stream headers, but it must not send a synthetic empty SSE `data:` event. Only send real JSON-RPC messages or keepalive comments on that stream; the legacy `/sse` transport still sends its endpoint event.
- Preserve the stateless startup fallback for native clients that drop `MCP-Session-Id`: sessionless one-way messages and stateless request methods such as `resources/list` should work, while `resources/subscribe`, `resources/unsubscribe`, `GET /mcp`, and `DELETE /mcp` remain session-bound.
- Keep supported protocol-version negotiation broad for native clients: currently `2025-11-25`, `2025-06-18`, `2025-03-26`, and `2024-11-05`.
- Preserve UUID-shaped session restoration for Streamable HTTP clients; native clients can cache `MCP-Session-Id` values across pod restarts and may not recover cleanly from an immediate `404`.
- Keep `/.well-known/mcp.json`, OAuth protected-resource metadata, app README, and Helm route values aligned whenever endpoints or advertised transports change.
- Preserve CI stamping of `pkg/base.BuildVersion` with the immutable MCP image tag. Post-publication checks must observe that exact discovery version; a healthy older release or the default `local` version is not confirmation of the new deployment.
- Preserve Origin validation and CORS preflight behavior together; browser compatibility should not weaken DNS-rebinding protection.
- Public discovery is not proof of authentication. Require a Gateway CUSTOM login check on every Streamable HTTP and legacy transport, plus external's registered-user validation before serving those transports. Probe anonymous and forged identity headers against a real read operation; metadata and `/api/auth/status` alone missed an unauthenticated user-count read during LAN recovery. Never trust the gateway's mesh principal as evidence that its caller logged in.

## Worldbuilder delegation

World tools require validated AuthStatus and a signed exact-request assertion to external’s dedicated operation allowlist. Never reuse raw UserID/header forwarding for canon, or add review/publication/administration/private-story operations to the MCP route. Keep the human browser route mesh restriction and both MCP transport edge restrictions when changing delegation.

- Current MCP operation bindings proxy storage/database/search through external; unused legacy helpers and leftover live Secret keys do not establish direct backend consumption. Trace bindings and startup before proposing direct credentials or broader egress.
