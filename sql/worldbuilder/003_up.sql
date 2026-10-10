-- Durable inference allowances; external installs nothing at startup.
BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM worldbuilder.schema_version WHERE version=2) THEN
  RAISE EXCEPTION 'apply Worldbuilder migration 002 first';
 END IF;
END $$;
-- No world FK: reservations commit on a separate connection while the authorised
-- canon transaction holds the world row lock. They must survive its rollback.
CREATE TABLE IF NOT EXISTS worldbuilder.inference_attempt (
 id uuid PRIMARY KEY, world_id uuid NOT NULL, actor text NOT NULL,
 workflow text NOT NULL, revision bigint NOT NULL CHECK(revision>=0), timeline text NOT NULL,
 session_id text NOT NULL DEFAULT '', request_hash text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), lease_until timestamptz NOT NULL,
 status text NOT NULL DEFAULT 'reserved' CHECK(status IN ('reserved','succeeded','failed')),
 allocated_micros bigint NOT NULL CHECK(allocated_micros>0), currency text NOT NULL,
 gateway_route text NOT NULL, model text NOT NULL DEFAULT '', usage jsonb NOT NULL DEFAULT '{}',
 latency_ms bigint NOT NULL DEFAULT 0 CHECK(latency_ms>=0), finished_at timestamptz
);
CREATE INDEX IF NOT EXISTS inference_actor_day ON worldbuilder.inference_attempt(actor,created_at);
INSERT INTO worldbuilder.schema_version(version) VALUES(3) ON CONFLICT DO NOTHING;
COMMIT;
