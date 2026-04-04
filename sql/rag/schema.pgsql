CREATE EXTENSION IF NOT EXISTS vector;

CREATE SCHEMA IF NOT EXISTS rag_pipeline;

CREATE TABLE IF NOT EXISTS rag_pipeline.documents (
    id BIGSERIAL PRIMARY KEY,
    document_id TEXT NOT NULL UNIQUE,
    source_uri TEXT NOT NULL,
    content_type TEXT NOT NULL,
    metadata JSONB,
    requested_at TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS rag_pipeline.chunks (
    id BIGSERIAL PRIMARY KEY,
    document_id BIGINT NOT NULL REFERENCES rag_pipeline.documents(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    chunk_text TEXT NOT NULL,
    token_count INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (document_id, chunk_index)
);

CREATE TABLE IF NOT EXISTS rag_pipeline.embeddings (
    id BIGSERIAL PRIMARY KEY,
    chunk_id BIGINT NOT NULL UNIQUE REFERENCES rag_pipeline.chunks(id) ON DELETE CASCADE,
    model TEXT NOT NULL,
    vector vector,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
