-- Explicit operator migration. Never run the legacy auth seed against live users.
BEGIN;
CREATE SCHEMA IF NOT EXISTS mcp_oauth;
REVOKE ALL ON SCHEMA mcp_oauth FROM PUBLIC;
CREATE TABLE IF NOT EXISTS mcp_oauth.schema_version (
    version integer PRIMARY KEY CHECK (version > 0),
    applied_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO mcp_oauth.schema_version(version) VALUES (1) ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS mcp_oauth.client (
    id text PRIMARY KEY,
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    redirect_uris jsonb NOT NULL CHECK (jsonb_typeof(redirect_uris) = 'array'),
    approved boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS mcp_oauth.authorization (
    id_hash text PRIMARY KEY,
    client_id text NOT NULL REFERENCES mcp_oauth.client(id),
    email text NOT NULL REFERENCES auth.users(email) ON DELETE CASCADE,
    redirect_uri text NOT NULL,
    resource text NOT NULL,
    challenge text NOT NULL,
    state text NOT NULL,
    csrf_hash text NOT NULL,
    expires_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS mcp_oauth.code (
    code_hash text PRIMARY KEY,
    client_id text NOT NULL REFERENCES mcp_oauth.client(id),
    email text NOT NULL REFERENCES auth.users(email) ON DELETE CASCADE,
    redirect_uri text NOT NULL,
    resource text NOT NULL,
    challenge text NOT NULL,
    expires_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS mcp_oauth.token (
    access_hash text PRIMARY KEY,
    refresh_hash text UNIQUE NOT NULL,
    family text NOT NULL,
    client_id text NOT NULL REFERENCES mcp_oauth.client(id),
    email text NOT NULL REFERENCES auth.users(email) ON DELETE CASCADE,
    resource text NOT NULL,
    expires_at timestamptz NOT NULL,
    refresh_expires_at timestamptz NOT NULL,
    revoked boolean NOT NULL DEFAULT false
);
CREATE INDEX IF NOT EXISTS mcp_oauth_token_family ON mcp_oauth.token(family);
REVOKE ALL ON ALL TABLES IN SCHEMA mcp_oauth FROM PUBLIC;
COMMIT;
