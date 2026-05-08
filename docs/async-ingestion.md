# Async Ingestion Slice

This document describes the first coherent ingestion slice for the current architecture.

## Goal

Start with reconciliation of MinIO objects into Postgres, then layer asynchronous processing on top.

This keeps:

- MinIO as canonical raw storage
- Postgres as the source of truth
- NATS JetStream as execution transport
- `orchestrator` as control plane
- `processor` as stateless worker

## Initial Flow

### Phase 1

`orchestrator` scans a MinIO bucket or prefix and upserts document inventory rows into `rag.documents`.

The initial inventory record should capture:

- bucket
- object key
- source URI
- etag or version marker when available
- size
- last modified timestamp
- current workflow status
- processing-version intent

At this stage, the important outcome is durable document state in Postgres, not immediate extraction.

The current live scan endpoint is `POST /documents/scan-bucket`, also exposed through MCP as the `documents.scanBucket` tool. It accepts optional `bucket`, `prefix`, `maxKeys`, and `processingVersion` fields. Non-text objects are inventoried with status `unsupported`; unchanged known objects keep their current lifecycle status while refreshing reconciliation metadata.

### Phase 2

When `orchestrator` determines a document is new, changed, or needs reprocessing, it emits a NATS JetStream job.

NATS JetStream is used to trigger work and scale workers, but job completion and system state are still reflected back into Postgres.

The current request-driven slice is:

- `POST /documents` receives a MinIO-backed document reference
- `orchestrator` upserts the row in `rag.documents` as `pending`
- the same request flow publishes a JetStream job before committing
- if publish fails, the pending-row write is rolled back with the request

The current scan-driven slice uses the same queue path for new or changed `text/*` objects discovered during bucket reconciliation.

The current reprocess-driven slice also uses that queue path. `POST /documents/reprocess`, exposed through MCP as the `documents.reprocess` tool, accepts an existing `documentId` and optional `processingVersion`; when the version is omitted, `orchestrator` queues the next processing version after the current desired or completed version.

The current notification pattern on top of that job flow is:

- emit `documents.events.processor.queued` when the document is queued for processor execution
- emit `documents.events.processor.started` when the processor claims the work and begins execution
- emit `documents.events.processor.completed` when processing is finished and derived state has been written back
- emit `documents.events.processor.failed` when processing fails and the document is returned to a retryable state

`documents.events.minio.stored` remains reserved for a future ingest-boundary emitter rather than part of the current browser upload path.

The Labiraus MCP server now subscribes to that NATS event stream and forwards document-specific updates to MCP subscribers as resource notifications. The browser-facing `external` service also fans the same lifecycle events out to authenticated UI clients over SSE at `/api/documents/events`.

### Phase 3

`processor` consumes the job, performs extraction, chunking, and embedding, and writes derived results back to Postgres.

This includes:

- chunk rows
- embedding rows
- processing timestamps
- updated processing state on the owning document row

The current processor lifecycle is:

- claim `pending -> processing`
- fetch the referenced MinIO object
- decode UTF-8 text for supported `text/*` documents
- create chunks and embeddings
- mark the row `processed`

When `EMBEDDING_MODEL=local-embeddings` and no embedding endpoint is configured, the processor uses the built-in deterministic 384-dimensional embedding function. External embedding services should only be configured when a real OpenAI-compatible endpoint exists.

If the processor sees a message before the orchestrator commit is visible, or if the job has been superseded by a newer processing version, it retries or no-ops instead of duplicating derived data.

Lifecycle notifications are intentionally best-effort. A failure to publish or fan out a notification must not roll back the durable document state or block processing completion.

## Boundary Rules

### `orchestrator`

- owns reconciliation and lifecycle decisions
- owns when work should be enqueued
- should not do chunking or embedding

### `processor`

- owns execution of document-processing work
- should not own global document lifecycle state
- should not become the source of truth

## Why This Shape

This design keeps ingestion idempotent and explainable:

- raw truth stays in MinIO
- control-plane truth stays in Postgres
- workers stay disposable
- public APIs remain stable while internals evolve

## Future Follow-Up

Later phases can add:

- richer retrieval APIs beyond the current `external` search and MCP inventory/search tools
- an ingest-boundary emitter for `documents.events.minio.stored`
- richer versioned-derivation history beyond the current explicit reprocess queue path
- richer citation UX beyond the current search-result citation labels and source links
- richer context assembly and graph-style capabilities on top of the same document foundation
