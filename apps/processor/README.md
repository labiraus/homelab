# processor

`processor` is the internal stateless TypeScript worker for document-processing jobs.

It consumes NATS JetStream events, claims pending document work from Postgres, fetches MinIO-backed text documents, chunks them, creates embeddings, and writes derived state back to Postgres.

`processor` is part of the data plane. It should not own global document lifecycle state; that remains the job of `orchestrator` and Postgres-backed control-plane records.

The current lifecycle for a claimed job is:

1. claim `pending -> processing`
2. load the referenced `text/*` object from MinIO
3. chunk and embed the document text
4. write `rag.chunks` and `rag.embeddings`
5. finalize the owning `rag.documents` row as `processed`

If work fails after claim, the worker records `last_error`, returns the document to `pending`, and lets JetStream retry the message.
