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

Current browser product surfaces:

- chat persists conversation trails, user messages, assistant replies, citations, and read-only MCP tool-call records
- memories are explicit user-approved rows scoped to the authenticated email and can be archived by that same identity
- file proposals are create/edit requests that remain pending until the authenticated browser user approves or rejects them
- approvals call `orchestrator`, which writes raw text through the control plane, queues reingestion, and records document-change audit metadata
- rejections persist an audit-oriented reason in the proposal decision response and do not call the orchestrator write path
- audit rows are read back from `rag.document_change_audits` by authenticated email and can stage a MinIO-version-backed revert request

Safety expectations:

- assistant model tool use stays allowlisted and read-only; the current model-readable tool surface is RAG context lookup only
- memory remains explicit and user-approved rather than inferred silently from chats
- create, edit, and revert operations require browser-side approval or action and must continue to flow through `orchestrator`
- proposal and audit queries must always be scoped by the authenticated email forwarded by `external`
- future write-like assistant behavior needs a durable proposal or equivalent browser approval path before it can reach raw documents

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
