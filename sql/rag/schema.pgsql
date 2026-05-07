CREATE EXTENSION IF NOT EXISTS vector;

CREATE SCHEMA IF NOT EXISTS rag;

CREATE TABLE IF NOT EXISTS rag.documents (
    id BIGSERIAL PRIMARY KEY,
    document_id TEXT NOT NULL UNIQUE,
    bucket_name TEXT,
    object_key TEXT,
    source_uri TEXT NOT NULL,
    content_type TEXT,
    version_marker TEXT,
    etag TEXT,
    size_bytes BIGINT,
    last_modified TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'pending',
    metadata JSONB,
    desired_processing_version INTEGER NOT NULL DEFAULT 1,
    current_processing_version INTEGER NOT NULL DEFAULT 0,
    last_reconciled_at TIMESTAMPTZ,
    last_processed_at TIMESTAMPTZ,
    last_event_subject TEXT,
    last_event_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE IF EXISTS rag.documents
    ADD COLUMN IF NOT EXISTS last_event_subject TEXT;

ALTER TABLE IF EXISTS rag.documents
    ADD COLUMN IF NOT EXISTS last_event_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS rag_documents_bucket_object_key_idx
    ON rag.documents (bucket_name, object_key)
    WHERE bucket_name IS NOT NULL AND object_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS rag_documents_status_idx
    ON rag.documents (status);

CREATE INDEX IF NOT EXISTS rag_documents_processing_version_idx
    ON rag.documents (desired_processing_version, current_processing_version);

CREATE TABLE IF NOT EXISTS rag.chunks (
    id BIGSERIAL PRIMARY KEY,
    document_pk BIGINT NOT NULL REFERENCES rag.documents(id) ON DELETE CASCADE,
    processing_version INTEGER NOT NULL DEFAULT 1,
    chunk_index INTEGER NOT NULL,
    chunk_text TEXT NOT NULL,
    token_count INTEGER NOT NULL,
    content_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (document_pk, processing_version, chunk_index)
);

CREATE INDEX IF NOT EXISTS rag_chunks_document_pk_idx
    ON rag.chunks (document_pk);

CREATE TABLE IF NOT EXISTS rag.embeddings (
    id BIGSERIAL PRIMARY KEY,
    chunk_id BIGINT NOT NULL REFERENCES rag.chunks(id) ON DELETE CASCADE,
    model TEXT NOT NULL,
    vector vector(384),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (chunk_id, model)
);

CREATE INDEX IF NOT EXISTS rag_embeddings_chunk_id_idx
    ON rag.embeddings (chunk_id);

CREATE INDEX IF NOT EXISTS rag_embeddings_model_idx
    ON rag.embeddings (model);

CREATE INDEX IF NOT EXISTS rag_embeddings_vector_cosine_idx
    ON rag.embeddings
    USING hnsw (vector vector_cosine_ops)
    WHERE vector IS NOT NULL;
