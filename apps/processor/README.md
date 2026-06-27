# processor

`processor` is the internal stateless TypeScript worker for document-processing jobs.

It consumes NATS JetStream events, claims pending document work from Postgres, fetches MinIO-backed text documents, extracts text, and indexes derived retrieval state into OpenSearch.

`processor` is part of the data plane. It should not own global document lifecycle state; that remains the job of `orchestrator` and Postgres-backed control-plane records.

The current lifecycle for a claimed job is:

1. claim `pending -> processing`
2. load the referenced supported object from MinIO
3. extract chunkable text from the raw object, including HTML-specific visible-text extraction for `text/html`
4. index the extracted text into OpenSearch through the configured ingest pipeline
5. finalize the owning `rag.documents` row as `processed`

If work fails after claim, the worker records `last_error`, returns the document to `pending`, and lets JetStream retry the message.

The worker publishes `documents.events.processor.started`, `documents.events.processor.completed`, and `documents.events.processor.failed` lifecycle notifications on a best-effort basis. After a notification publish succeeds, it also appends the event payload to `rag.document_lifecycle_events` and updates the quick summary columns on `rag.documents`.

OpenSearch stores nested chunk text and vectors. `OPENSEARCH_RAG_MODEL_ID` should identify an OpenSearch ML Commons model whose connector routes inference through Envoy AI Gateway.

The current ingestion baseline is UTF-8 `text/*`, with HTML-aware extraction for `text/html`. OpenSearch-native chunking may initially carry less precise HTML heading metadata than the previous processor chunker until section metadata is mapped into the ingest pipeline.

Future file-type expansion must still follow `docs/FileTypeExpansion.md`: document extraction rules, failure handling, citation policy, tests, and runtime dependencies before changing the worker container.
