# Labiraus MCP Prompts

This document tracks the prompt catalog that the Labiraus MCP server publishes through `prompts/list` and `prompts/get`.

Use it alongside [.codex/REPO_PLAN.md](/workspaces/homelab/.codex/REPO_PLAN.md) when shaping future MCP capabilities so the prompt surface grows in step with the actual repo plan.

## Current Prompt Catalog

The manifest currently lists these prompt names:

- `documents.submit.example`
- `documents.scanBucket.plan`
- `documents.reprocess.prompt`
- `documents.curation.update.prompt`
- `documents.editText.prompt`
- `documents.context.prompt`
- `documents.search.prompt`
- `documents.inventory.prompt`
- `documents.history.prompt`
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

### `documents.reprocess.prompt`

- lifecycle: `live`
- purpose: show how to queue a newer processing version for an existing inventory document
- arguments:
  - `documentId` required
  - `processingVersion` optional

Use this when:

- an agent needs to refresh derived chunks and embeddings for an existing text document
- the orchestration result should be followed with a version-specific history lookup

The live `documents.reprocess` tool sends the request through the orchestrator control plane. By default, the orchestrator chooses the next processing version; callers should pass `processingVersion` only when they intentionally need an explicit version. The returned processing version can be inspected with `documents.history.list`.

### `documents.curation.update.prompt`

- lifecycle: `live`
- purpose: show how to update curated metadata for an existing inventory document
- arguments:
  - `documentId` required
  - `metadata` optional

Use this when:

- an agent needs to tag or correct metadata on an inventory row
- curated metadata should later narrow inventory, search, or context calls through exact-match filters

The live `documents.curation.update` tool writes through the orchestrator control plane to `rag.documents.metadata`. Omitting `replace` or setting it to false performs a targeted merge; setting `replace` should be reserved for intentional full metadata replacement.

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
  - `metadata` optional

Use this when:

- an agent needs ready-to-use context rather than raw search hits
- cited context should stay tied to the current processed chunk version

The live `documents.context` tool uses the same pgvector retrieval path as `documents.search`, then emits a compact context string with `[1]`, `[2]` style references. Each reference has a corresponding citation object that identifies the source URI, object key, chunk ID, chunk index, and processing version. The optional `metadata` object applies exact-match filters against curated `rag.documents.metadata` fields.

### `documents.search.prompt`

- lifecycle: `live`
- purpose: show how to search processed document chunks through the MCP retrieval surface
- arguments:
  - `query` required
  - `prefix` optional
  - `metadata` optional

Use this when:

- an agent needs semantic retrieval over processed documents
- narrowing search to a folder-like object-key prefix would improve the answer

The live `documents.search` tool embeds the query with the configured embedding path and searches the current processed chunk version across `rag.embeddings`, `rag.chunks`, and `rag.documents` in Postgres. Results include document metadata plus citation objects with source URI and chunk identity. Optional `metadata` filters use exact JSON containment, for example `{"tag":"runbook"}`. With `EMBEDDING_MODEL=local-embeddings` and no `EMBEDDING_ENDPOINT`, that embedding path is the built-in deterministic 384-dimensional local embedding function.

### `documents.inventory.prompt`

- lifecycle: `live`
- purpose: show how to inspect Postgres-backed document inventory and processing state
- arguments:
  - `status` optional
  - `prefix` optional
  - `documentId` optional
  - `metadata` optional

Use this when:

- an agent needs document state without relying on a matching retrieval chunk
- a prefix, status, document ID, or curated metadata filter can narrow the inventory read

The live `documents.inventory.list` tool reads `rag.documents` and returns source identity, metadata, current and desired processing versions, latest lifecycle summary fields, reconciliation timestamps, and errors. If a prefix looks stale, call `documents.scanBucket` first and then list inventory again.

### `documents.history.prompt`

- lifecycle: `live`
- purpose: show how to inspect durable document lifecycle history for a processing attempt
- arguments:
  - `documentId` required
  - `processingVersion` optional

Use this when:

- an agent needs to audit whether a document was queued, started, completed, or failed
- a reprocess attempt needs a timeline separate from the current inventory summary

The live `documents.history.list` tool reads `rag.document_lifecycle_events` and returns lifecycle events with their original event payloads. The current inventory row still exposes only the latest event summary through `lastEventSubject` and `lastEventAt`.

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

1. `documents.events.minio.stored`
2. `documents.events.processor.queued`
3. `documents.events.processor.started`
4. `documents.events.processor.completed`
5. `documents.events.processor.failed`

The live shape is:

- `external`, `orchestrator`, and `processor` emit lifecycle events onto NATS JetStream under `documents.events.>`
- `mcp` exposes `homelab://mcp/documents/notifications/{documentId}` as a live resource template
- MCP clients call `resources/subscribe` and keep a `GET /mcp` SSE stream open for the session identified by `MCP-Session-Id`
- `mcp` forwards matching updates as `notifications/resources/updated`

`documents.events.minio.stored` is emitted by the browser-facing upload path after the raw object is written to MinIO. The prompt documents the live subscription pattern rather than a future-only design.
