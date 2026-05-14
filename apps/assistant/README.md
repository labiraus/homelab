# assistant

`assistant` is the browser-first LLM service for Labiraus.

It owns:

- saved conversations and messages
- explicit, user-approved memories scoped by authenticated email
- read-only RAG tool calls through the Labiraus MCP `documents.context` tool
- file create/edit proposals that require browser approval
- proposal decisions and audit views backed by Postgres

The service exposes `/assistant/...` internally. The public browser path is `/api/assistant/...` through `external`, which forwards the authenticated email as the user identity.

Approved file proposals call `orchestrator` create/edit endpoints. The model does not receive write tools directly; write-like outcomes are persisted as proposals until the user approves them.

Important environment variables:

- `POSTGRES_*`: assistant state storage
- `MCP_BASE_URL`: Labiraus MCP endpoint, usually `http://homelab-mcp.homelab.svc.cluster.local/mcp`
- `ORCHESTRATOR_BASE_URL`: internal orchestrator endpoint
- `LLM_BASE_URL`: OpenAI-compatible base URL, including `/v1`
- `LLM_MODEL`: served model name, defaulting to `Qwen/Qwen2.5-0.5B-Instruct`

Run tests:

```bash
go test ./...
```
