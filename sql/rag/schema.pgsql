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

CREATE INDEX IF NOT EXISTS rag_documents_metadata_gin_idx
    ON rag.documents
    USING GIN (metadata jsonb_path_ops)
    WHERE metadata IS NOT NULL;

CREATE TABLE IF NOT EXISTS rag.document_lifecycle_events (
    id BIGSERIAL PRIMARY KEY,
    document_pk BIGINT REFERENCES rag.documents(id) ON DELETE CASCADE,
    document_id TEXT NOT NULL,
    subject TEXT NOT NULL,
    processing_version INTEGER NOT NULL DEFAULT 0,
    event_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS rag_document_lifecycle_events_document_idx
    ON rag.document_lifecycle_events (document_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS rag_document_lifecycle_events_version_idx
    ON rag.document_lifecycle_events (document_id, processing_version, occurred_at DESC);

CREATE INDEX IF NOT EXISTS rag_document_lifecycle_events_subject_idx
    ON rag.document_lifecycle_events (subject, occurred_at DESC);

CREATE TABLE IF NOT EXISTS rag.document_change_audits (
    id BIGSERIAL PRIMARY KEY,
    audit_id TEXT NOT NULL UNIQUE,
    document_pk BIGINT REFERENCES rag.documents(id) ON DELETE SET NULL,
    document_id TEXT NOT NULL,
    bucket_name TEXT,
    object_key TEXT NOT NULL,
    action TEXT NOT NULL,
    actor_email TEXT NOT NULL,
    conversation_id TEXT,
    proposal_id TEXT,
    old_version_marker TEXT,
    new_version_marker TEXT,
    reverted_to_version_marker TEXT,
    processing_version INTEGER NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS rag_document_change_audits_document_created_idx
    ON rag.document_change_audits (document_id, created_at DESC);

CREATE INDEX IF NOT EXISTS rag_document_change_audits_actor_created_idx
    ON rag.document_change_audits (actor_email, created_at DESC);

CREATE INDEX IF NOT EXISTS rag_document_change_audits_proposal_idx
    ON rag.document_change_audits (proposal_id)
    WHERE proposal_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS rag.chunks (
    id BIGSERIAL PRIMARY KEY,
    document_pk BIGINT NOT NULL REFERENCES rag.documents(id) ON DELETE CASCADE,
    processing_version INTEGER NOT NULL DEFAULT 1,
    chunk_index INTEGER NOT NULL,
    chunk_text TEXT NOT NULL,
    token_count INTEGER NOT NULL,
    chunk_metadata JSONB,
    content_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (document_pk, processing_version, chunk_index)
);

ALTER TABLE IF EXISTS rag.chunks
    ADD COLUMN IF NOT EXISTS chunk_metadata JSONB;

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

ALTER TABLE IF EXISTS rag.embeddings
    ALTER COLUMN vector TYPE vector(384) USING vector::vector(384);

CREATE INDEX IF NOT EXISTS rag_embeddings_chunk_id_idx
    ON rag.embeddings (chunk_id);

CREATE INDEX IF NOT EXISTS rag_embeddings_model_idx
    ON rag.embeddings (model);

CREATE INDEX IF NOT EXISTS rag_embeddings_vector_cosine_idx
    ON rag.embeddings
    USING hnsw (vector vector_cosine_ops)
    WHERE vector IS NOT NULL;
