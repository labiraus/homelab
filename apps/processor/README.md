# processor

`processor` is an internal TypeScript worker that consumes Kafka document events, chunks text, generates embeddings through an internal embedding endpoint, and writes chunks plus embeddings to Postgres.
