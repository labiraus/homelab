# processor

`processor` is the internal stateless TypeScript worker for document-processing jobs.

It consumes NATS JetStream events, performs extraction-oriented work such as chunking and embedding, and writes derived state back to Postgres.

`processor` is part of the data plane. It should not own global document lifecycle state; that remains the job of `orchestrator` and Postgres-backed control-plane records.
