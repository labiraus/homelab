# Labiraus MCP Prompts

This document tracks the prompt catalog that the Labiraus MCP server publishes through `prompts/list` and `prompts/get`.

Use it alongside [.codex/REPO_PLAN.md](/workspaces/homelab/.codex/REPO_PLAN.md) when shaping future MCP capabilities so the prompt surface grows in step with the actual repo plan.

## Current Prompt Catalog

The manifest currently lists these prompt names:

- `documents.submit.example`
- `documents.scanBucket.plan`
- `documents.editText.prompt`
- `documents.context.prompt`
- `documents.search.prompt`
- `postgres.auth.userCount.prompt`
- `minio.documents.browse.prompt`
- `documents.notifications.subscribe.prompt`

## Prompt Intent

### `documents.submit.example`

- lifecycle: `live`
- purpose: show a ready-to-run example for the current `documents.submit` tool
- arguments:
  - `documentId` required
  - `sourceUri` required
  - `contentType` required

Use this when:

- an agent needs a concrete ingestion payload shape
- you want prompt-first guidance before calling the live tool

The current example should point at a MinIO-backed reference payload with:

- `bucket`
- `objectKey`
- `sourceUri`
- `contentType`

This live ingestion path currently supports `text/*` documents only.

### `documents.scanBucket.plan`

- lifecycle: `live`
- purpose: describe the MinIO scan and Postgres reconciliation flow
- arguments:
  - `prefix` optional

Use this when:

- preparing a live `documents.scanBucket` call
- reviewing how bucket reconciliation decides what should be queued

The live scan upserts document inventory rows, marks non-`text/*` objects as `unsupported`, preserves unchanged known object status, and queues new or changed text objects through the orchestrator/processor path.

### `documents.editText.prompt`

- lifecycle: `live`
- purpose: show how to overwrite an existing text object and queue refreshed derived state
- arguments:
  - `documentId` required

Use this when:

- an agent needs to edit a known `text/*` inventory document
- the edit should trigger a newer processing version without a separate scan or reprocess call

The live `documents.editText` tool writes replacement text to the existing MinIO bucket/object key, merges optional metadata into `rag.documents.metadata`, marks the edit with `editedBy: orchestrator.editText`, and queues processor work for the next processing version by default.

### `documents.context.prompt`

- lifecycle: `live`
- purpose: show how to assemble a citation-backed context block from processed document chunks
- arguments:
  - `query` required
  - `prefix` optional

Use this when:

- an agent needs ready-to-use context rather than raw search hits
- cited context should stay tied to the current processed chunk version

The live `documents.context` tool uses the same pgvector retrieval path as `documents.search`, then emits a compact context string with `[1]`, `[2]` style references. Each reference has a corresponding citation object that identifies the source URI, object key, chunk ID, chunk index, and processing version.

### `documents.search.prompt`

- lifecycle: `live`
- purpose: show how to search processed document chunks through the MCP retrieval surface
- arguments:
  - `query` required
  - `prefix` optional

Use this when:

- an agent needs semantic retrieval over processed documents
- narrowing search to a folder-like object-key prefix would improve the answer

The live `documents.search` tool embeds the query with the configured embedding path and searches the current processed chunk version across `rag.embeddings`, `rag.chunks`, and `rag.documents` in Postgres. Results include citation objects with source URI and chunk identity. With `EMBEDDING_MODEL=local-embeddings` and no `EMBEDDING_ENDPOINT`, that embedding path is the built-in deterministic 384-dimensional local embedding function.

### `postgres.auth.userCount.prompt`

- lifecycle: `live`
- purpose: explain the quickest Postgres-backed auth-health read available today
- arguments: none

Use this when:

- verifying the Labiraus Postgres capability surface
- checking whether the deployed MCP server can execute a simple auth query

### `minio.documents.browse.prompt`

- lifecycle: `live`
- purpose: show how to browse the documents bucket with the current folder-aware MinIO tools and resources
- arguments:
  - `prefix` optional

Use this when:

- exploring MinIO-backed document storage through MCP
- narrowing reads before fetching specific objects

### `documents.notifications.subscribe.prompt`

- lifecycle: `live`
- purpose: explain the current document-lifecycle subscription flow driven by NATS JetStream and Streamable HTTP sessions
- arguments:
  - `documentId` required

Use this when:

- subscribing to the notification resource for a specific document
- validating that event naming stays aligned across `orchestrator`, `processor`, and `mcp`

## Notification Pattern

The current document notification flow is:

1. `documents.events.processor.queued`
2. `documents.events.processor.started`
3. `documents.events.processor.completed`
4. `documents.events.processor.failed`

The live shape is:

- `orchestrator` and `processor` emit lifecycle events onto NATS JetStream under `documents.events.>`
- `mcp` exposes `homelab://mcp/documents/notifications/{documentId}` as a live resource template
- MCP clients call `resources/subscribe` and keep a `GET /mcp` SSE stream open for the session identified by `MCP-Session-Id`
- `mcp` forwards matching updates as `notifications/resources/updated`

`documents.events.minio.stored` remains reserved for a future ingest-boundary emitter. The prompt now documents the live subscription pattern rather than a future-only design.
