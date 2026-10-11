-- Run separately as the existing auth schema/table owner or database operator.
-- The canon migration owner must not acquire the legacy app role.
BEGIN;
-- Preserve existing users; create only the empty shape for a fresh installation.
CREATE SCHEMA IF NOT EXISTS auth;
CREATE TABLE IF NOT EXISTS auth.users (
 email text PRIMARY KEY, display_name text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
GRANT USAGE ON SCHEMA auth TO worldbuilder_external;
GRANT SELECT(email,display_name,created_at,updated_at), INSERT(email,display_name) ON auth.users TO worldbuilder_external;
COMMIT;
