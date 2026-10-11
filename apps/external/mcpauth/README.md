# Native MCP authorization

Opt-in implementation, disabled until protocol/database tests and live mesh isolation pass. No production schema is installed by startup. See [operator activation and restore handling](../../../docs/MCPAuthentication.md).

The native client needs an MCP authorization server; Google-backed website cookies alone do not supply client registration or resource-bound access tokens. This package will reuse the authenticated browser identity for explicit consent, then issue opaque MCP-only tokens through authorization code + S256 PKCE. PostgreSQL stores only hashes of authorization handles, codes, CSRF values and tokens. Code redemption is atomic; refresh rotation retains replay evidence and revokes its family on reuse.

Public registration accepts bounded exact HTTPS or loopback redirect addresses. Browser authorization is confined to the verified OAuth proxy principal and requires same-origin POST plus a separate secure CSRF cookie. Both the gateway and MCP validate credentials against the private authority; native token checks never trust caller identity headers. Existing registered-user validation remains authoritative. Separate SQL and a non-owner runtime role are required; no startup migration, Google token minting or permissive fallback is allowed.

The implementation follows [MCP authorization](https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization) and the [OpenAI client contract](https://developers.openai.com/plugins/build/auth). Publication and actual Codex login/document-read acceptance remain required.
