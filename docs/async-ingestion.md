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

The current architecture keeps ingestion inside `orchestrator` plus `processor`; it does not add a separate ingestion application.

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

The current live scan endpoint is `POST /documents/scan-bucket`, exposed through MCP as the `documents.scanBucket` tool and through `external` as `POST /api/documents/scan-bucket`. It accepts optional `bucket`, `prefix`, `maxKeys`, and `processingVersion` fields. Non-text objects are inventoried with status `unsupported`; unchanged known objects keep their current lifecycle status while refreshing reconciliation metadata.

### Phase 2

When `orchestrator` determines a document is new, changed, or needs reprocessing, it emits a NATS JetStream job.

NATS JetStream is used to trigger work and scale workers, but job completion and system state are still reflected back into Postgres.

The current request-driven slice is:

- `POST /documents` receives a MinIO-backed document reference
- `orchestrator` upserts the row in `rag.documents` as `pending`
- the same request flow publishes a JetStream job before committing
- if publish fails, the pending-row write is rolled back with the request

The current scan-driven slice uses the same queue path for new or changed `text/*` objects discovered during bucket reconciliation.

The current curation slice is metadata-only. `POST /documents/curation`, exposed through MCP as the `documents.curation.update` tool and through `external` as `POST /api/documents/curation`, updates `rag.documents.metadata` for an existing inventory row without changing raw MinIO objects, chunks, embeddings, or lifecycle status.

The current edit slice is text-only. `POST /documents/edit-text`, exposed through MCP as the `documents.editText` tool and through `external` as `POST /api/documents/edit-text`, overwrites the existing MinIO object for an inventory row, merges edit metadata, and queues a newer processing version through the same control-plane path. It intentionally edits only existing `text/*` inventory rows so object identity, access assumptions, and derived-state refreshes stay explicit.

The current reprocess-driven slice also uses that queue path. `POST /documents/reprocess`, exposed through MCP as the `documents.reprocess` tool and through `external` as `POST /api/documents/reprocess`, accepts an existing `documentId` and optional `processingVersion`; when the version is omitted, `orchestrator` queues the next processing version after the current desired or completed version.

The current notification pattern on top of that job flow is:

- emit `documents.events.minio.stored` when the browser-facing upload path writes the raw object to MinIO
- emit `documents.events.processor.queued` when the document is queued for processor execution
- emit `documents.events.processor.started` when the processor claims the work and begins execution
- emit `documents.events.processor.completed` when processing is finished and derived state has been written back
- emit `documents.events.processor.failed` when processing fails and the document is returned to a retryable state

The Labiraus MCP server now subscribes to that NATS event stream and forwards document-specific updates to MCP subscribers as resource notifications. The browser-facing `external` service also fans the same lifecycle events out to authenticated UI clients over SSE at `/api/documents/events`.

The current durable history slice records processor queue/start/complete/fail lifecycle events in `rag.document_lifecycle_events`. The event stream remains best-effort delivery for subscribers, while Postgres provides the audit trail that can be queried later through `documents.history.list` in MCP or `POST /api/documents/history` in `external`. The UI Search tab consumes the external history route to show a lifecycle panel for retrieved documents.

The current browser inventory slice exposes `POST /api/documents/inventory` through `external`. It reads `rag.documents` and returns document IDs, object keys, processing status, current and desired processing versions, latest lifecycle summary fields, errors, timestamps, and curated metadata. The UI Inventory tab uses that route with optional status, exact document ID, prefix, and exact metadata filters so browser operators can inspect reconciliation and processing state without needing a matching retrieval chunk.

The Inventory tab can also trigger reconciliation through `POST /api/documents/scan-bucket`, sending the current prefix filter with a bounded scan limit. `external` proxies the request to `orchestrator`, and the UI refreshes the current inventory filters after a successful scan response.

Inventory rows can also load durable lifecycle history through the existing `POST /api/documents/history` route. This keeps inventory drilldown read-only and backed by `rag.document_lifecycle_events`, using the same lifecycle panel shape as Search.

Inventory rows can update curated metadata through the existing `POST /api/documents/curation` route. The browser still sends the request through `external`, while `orchestrator` owns validation and metadata persistence on the inventory row.

Text inventory rows with source object keys can also load raw text through `GET /api/documents/object` and save guarded text edits through the existing `POST /api/documents/edit-text` route. The browser requires an explicit overwrite confirmation, while `orchestrator` remains responsible for writing the raw MinIO object, selecting the processing version, updating inventory state, and queueing processor work.

Text inventory rows with source object keys can queue reprocessing through the existing `POST /api/documents/reprocess` route. `external` still proxies the request to `orchestrator`, `orchestrator` still chooses and queues the next processing version, and the UI uses the accepted processing version to load the matching durable lifecycle history.

The current retrieval context slice is read-only. `documents.context` in MCP and `POST /api/documents/context` in `external` use the same current-version pgvector search path as semantic search, then assemble the selected chunks into a cited context block with stable `[1]`, `[2]` references. The UI Search tab exposes that browser-facing context route with the same query, prefix, and metadata filters used for semantic search.

The current browser document-control slice is intentionally narrow. The UI Search tab can select a retrieval result, update one curated metadata key/value pair through `/api/documents/curation`, load and save guarded text edits through `/api/documents/edit-text`, or queue reprocessing through `/api/documents/reprocess`. `external` only validates the public request shape and proxies to `orchestrator`; `orchestrator` still owns document validation, raw text writes, metadata persistence, version selection, lifecycle history, and queueing.

When a browser reprocess or text-edit action returns a processing version, the UI immediately queries `POST /api/documents/history` with that `documentId` and `processingVersion`. This keeps action follow-up read-only and Postgres-backed while giving the user the specific queued/start/complete trail for the operation they just requested.

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

- richer retrieval APIs beyond the current inventory, search, and context tools
- richer citation UX beyond the current search-result citation labels and source links
- file-type expansion beyond `text/*` once extraction, failure handling, and citation policy are explicit
- RAGAS-backed quality gates for chunking, embedding, ranking, and metadata filter changes
- operations hardening for stuck lifecycle states, worker lag, and notification fan-out failures
- graph-style capabilities on top of the same document foundation
