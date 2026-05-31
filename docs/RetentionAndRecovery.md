# Retention And Recovery

The current platform policy is preserve-by-default while the document and assistant surfaces are still young.

## Retention Defaults

- MinIO `documents` bucket: versioning enabled, current-object expiration disabled.
- Postgres `rag.documents`, `rag.chunks`, and `rag.embeddings`: retained until an explicit reprocessing or cleanup policy exists.
- `rag.document_lifecycle_events`: retained indefinitely as processing audit history.
- `rag.document_change_audits`: retained indefinitely as create/edit/revert audit history.
- assistant conversations, messages, memories, tool calls, file proposals, and proposal decisions: retained indefinitely and scoped by authenticated email at query time.
- NATS JetStream processing jobs and lifecycle notifications: operational transport only, not the durable retention layer.

## Cleanup Gate

Before enabling destructive lifecycle cleanup, update this file and the relevant chart/app docs with:

- the records or objects affected
- the retention duration
- the backup source that can restore them
- the restore procedure tested
- the user or operator behavior after cleanup
- the metrics or logs that prove cleanup is working

## Recovery Priorities

Recover in this order:

1. Postgres CNPG app database, including `rag` and `assistant` schemas.
2. External MinIO `documents` bucket on `svartalfheim`, including object versions.
3. Generated Kubernetes secrets used by app, MinIO, CNPG backup, and Flux registry access.
4. Flux app releases from the published OCI charts.
5. Derived chunks and embeddings through controlled reprocessing when needed.

Derived state can be rebuilt from raw MinIO objects, but audit history and user-approved assistant state live in Postgres and should be treated as primary data.

## Verification

After recovery:

- run `documents.inventory.list` for a known prefix
- run `documents.history.list` for a known document
- run `documents.search` for a known gold query
- run `make ragas-chunking-eval` against the private gold cases
- run `make vllm-gateway-smoke`
- open the Assistant tab and verify conversations, memories, proposals, and audit rows load for the authenticated user
