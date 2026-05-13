CREATE SCHEMA IF NOT EXISTS assistant;

CREATE TABLE IF NOT EXISTS assistant.conversations (
    id BIGSERIAL PRIMARY KEY,
    conversation_id TEXT NOT NULL UNIQUE,
    user_email TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT 'New conversation',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS assistant_conversations_user_updated_idx
    ON assistant.conversations (user_email, updated_at DESC);

CREATE TABLE IF NOT EXISTS assistant.messages (
    id BIGSERIAL PRIMARY KEY,
    message_id TEXT NOT NULL UNIQUE,
    conversation_pk BIGINT NOT NULL REFERENCES assistant.conversations(id) ON DELETE CASCADE,
    conversation_id TEXT NOT NULL,
    user_email TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    citations JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS assistant_messages_conversation_created_idx
    ON assistant.messages (conversation_id, created_at ASC);

CREATE TABLE IF NOT EXISTS assistant.tool_calls (
    id BIGSERIAL PRIMARY KEY,
    tool_call_id TEXT NOT NULL UNIQUE,
    conversation_pk BIGINT REFERENCES assistant.conversations(id) ON DELETE CASCADE,
    conversation_id TEXT,
    message_id TEXT,
    user_email TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    arguments JSONB NOT NULL DEFAULT '{}'::jsonb,
    result JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_error BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS assistant_tool_calls_conversation_created_idx
    ON assistant.tool_calls (conversation_id, created_at DESC);

CREATE TABLE IF NOT EXISTS assistant.memories (
    id BIGSERIAL PRIMARY KEY,
    memory_id TEXT NOT NULL UNIQUE,
    user_email TEXT NOT NULL,
    text TEXT NOT NULL,
    source_conversation_id TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS assistant_memories_user_status_created_idx
    ON assistant.memories (user_email, status, created_at DESC);

CREATE TABLE IF NOT EXISTS assistant.file_proposals (
    id BIGSERIAL PRIMARY KEY,
    proposal_id TEXT NOT NULL UNIQUE,
    conversation_id TEXT NOT NULL,
    user_email TEXT NOT NULL,
    action TEXT NOT NULL,
    document_id TEXT,
    object_key TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'text/plain; charset=utf-8',
    proposed_text TEXT NOT NULL,
    rationale TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    orchestrator_response JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decided_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS assistant_file_proposals_conversation_created_idx
    ON assistant.file_proposals (conversation_id, created_at DESC);

CREATE INDEX IF NOT EXISTS assistant_file_proposals_user_status_idx
    ON assistant.file_proposals (user_email, status, created_at DESC);
