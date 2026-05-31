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

The near-term direction is quality and safety rather than new write surfaces: improve grounded answers, memory review, proposal approval/rejection, audit inspection, and revert staging while keeping tool calls allowlisted and writes browser-approved.

Local generation currently targets the OpenAI-compatible vLLM route fronted by Envoy AI Gateway. Keep the small default model until the `helheim` GPU path is validated for startup, memory pressure, answer quality, and tool-call behavior.

Validate the deployed model route with:

```bash
make vllm-gateway-smoke
```

That target sends an OpenAI-compatible chat request through the same internal Gateway URL configured as `LLM_BASE_URL`.

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
