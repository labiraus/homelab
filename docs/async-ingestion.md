# Async Ingestion Slice

This document describes the first coherent ingestion slice for the current architecture.

## Goal

Start with reconciliation of MinIO objects into Postgres, then layer asynchronous processing on top.

This keeps:

- MinIO as canonical raw storage
- Postgres as the source of truth
- Kafka as execution transport
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

### Phase 2

When `orchestrator` determines a document is new, changed, or needs reprocessing, it emits a Kafka job.

Kafka is used to trigger work and scale workers, but job completion and system state are still reflected back into Postgres.

### Phase 3

`processor` consumes the job, performs extraction, chunking, and embedding, and writes derived results back to Postgres.

This includes:

- chunk rows
- embedding rows
- processing timestamps
- updated processing state on the owning document row

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

- richer retrieval APIs through `external` and `mcp`
- reprocessing and versioned derivations
- citation UX in the UI
- richer context assembly and graph-style capabilities on top of the same document foundation
