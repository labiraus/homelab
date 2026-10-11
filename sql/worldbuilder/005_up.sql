BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM worldbuilder.schema_version WHERE version=4) THEN RAISE EXCEPTION 'apply migration 004 first'; END IF;
END $$;
CREATE TABLE IF NOT EXISTS worldbuilder.generation_job (
 world_id uuid NOT NULL REFERENCES worldbuilder.world(id), id uuid NOT NULL, owner text NOT NULL,
 request jsonb NOT NULL, status text NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','running','succeeded','failed','uncertain','cancelled')),
 step integer NOT NULL DEFAULT 0 CHECK(step BETWEEN 0 AND 4), parts integer NOT NULL CHECK(parts BETWEEN 1 AND 4),
 lease_token uuid, lease_until timestamptz, results jsonb NOT NULL DEFAULT '[]', error_code text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(world_id,id), CHECK(step<=parts)
);
CREATE INDEX IF NOT EXISTS generation_owner ON worldbuilder.generation_job(world_id,owner,created_at);
CREATE INDEX IF NOT EXISTS generation_active_owner ON worldbuilder.generation_job(owner) WHERE status IN ('queued','running');
CREATE TABLE IF NOT EXISTS worldbuilder.generation_outbox (
 id uuid PRIMARY KEY, world_id uuid NOT NULL, job_id uuid NOT NULL, payload jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), delivered_at timestamptz,
 UNIQUE(world_id,job_id), FOREIGN KEY(world_id,job_id) REFERENCES worldbuilder.generation_job(world_id,id)
);
CREATE INDEX IF NOT EXISTS generation_pending ON worldbuilder.generation_outbox(created_at) WHERE delivered_at IS NULL;
-- Opaque attribution avoids a cross-connection FK lock during generation.
ALTER TABLE worldbuilder.inference_attempt ADD COLUMN IF NOT EXISTS generation_job_id uuid;
ALTER TABLE worldbuilder.inference_attempt ADD COLUMN IF NOT EXISTS generation_part integer CHECK(generation_part BETWEEN 1 AND 4);
INSERT INTO worldbuilder.schema_version(version) VALUES(5) ON CONFLICT DO NOTHING;
COMMIT;
