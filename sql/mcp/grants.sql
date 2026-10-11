-- Provision mcp_oauth_runtime independently with LOGIN NOINHERIT NOSUPERUSER
-- NOCREATEDB NOCREATEROLE NOBYPASSRLS, and an operator-managed Secret password.
-- This identity owns no tables, has no canon grants and cannot register users.
BEGIN;
GRANT USAGE ON SCHEMA mcp_oauth, auth TO mcp_oauth_runtime;
GRANT SELECT ON auth.users TO mcp_oauth_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON mcp_oauth.client, mcp_oauth.authorization, mcp_oauth.code, mcp_oauth.token TO mcp_oauth_runtime;
GRANT SELECT ON mcp_oauth.schema_version TO mcp_oauth_runtime;
REVOKE CREATE ON SCHEMA mcp_oauth, auth FROM mcp_oauth_runtime;
COMMIT;
