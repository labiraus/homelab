-- Apply to a restored database before exposing native MCP authorization.
-- A restored snapshot can predate a revocation; require fresh browser consent.
BEGIN;
DELETE FROM mcp_oauth.authorization;
DELETE FROM mcp_oauth.code;
UPDATE mcp_oauth.token SET revoked=true;
COMMIT;
