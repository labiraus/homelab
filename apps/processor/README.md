# processor

`processor` is the internal stateless TypeScript worker for document-processing jobs.

It consumes NATS JetStream events, claims pending document work from Postgres, fetches MinIO-backed text documents, chunks them, creates embeddings, and writes derived state back to Postgres.

`processor` is part of the data plane. It should not own global document lifecycle state; that remains the job of `orchestrator` and Postgres-backed control-plane records.

The current lifecycle for a claimed job is:

1. claim `pending -> processing`
2. load the referenced supported object from MinIO
3. extract chunkable text from the raw object, including HTML-specific visible-text extraction for `text/html`
4. write `rag.chunks` and `rag.embeddings`
5. finalize the owning `rag.documents` row as `processed`

If work fails after claim, the worker records `last_error`, returns the document to `pending`, and lets JetStream retry the message.

The worker publishes `documents.events.processor.started`, `documents.events.processor.completed`, and `documents.events.processor.failed` lifecycle notifications on a best-effort basis. After a notification publish succeeds, it also appends the event payload to `rag.document_lifecycle_events` and updates the quick summary columns on `rag.documents`.

Embeddings are stored in `rag.embeddings.vector` as `vector(384)` so the pgvector HNSW cosine index can be built. If `EMBEDDING_MODEL` changes to a model with a different vector size, update the processor bootstrap schema and `sql/rag/schema.pgsql` together.

When `EMBEDDING_MODEL=local-embeddings` and `EMBEDDING_ENDPOINT` is empty, the worker uses the built-in deterministic 384-dimensional local embedding function. Set `EMBEDDING_ENDPOINT` only when routing to an actual OpenAI-compatible embeddings service.

The current ingestion baseline is UTF-8 `text/*`, with HTML-aware extraction for `text/html`. `rag.chunks.chunk_metadata` now carries chunk-level citation hints such as HTML titles, heading paths, and extraction warnings so retrieval surfaces can render richer labels without breaking older plain-text chunks.

Future file-type expansion must still follow `docs/FileTypeExpansion.md`: document extraction rules, failure handling, citation policy, tests, and runtime dependencies before changing the worker container.
