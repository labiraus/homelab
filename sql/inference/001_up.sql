BEGIN;
SELECT pg_advisory_xact_lock(719341026);
CREATE SCHEMA IF NOT EXISTS inference;
CREATE TABLE IF NOT EXISTS inference.schema_version(version integer PRIMARY KEY);
CREATE TABLE IF NOT EXISTS inference.attempt (
 id uuid PRIMARY KEY,
 client_id text NOT NULL, request_hash text NOT NULL,
 profile text NOT NULL, profile_revision text NOT NULL, model text NOT NULL,
 price_version text NOT NULL, input_rate bigint NOT NULL CHECK(input_rate>0), output_rate bigint NOT NULL CHECK(output_rate>0),
 created_at timestamptz NOT NULL DEFAULT now(),
 period date NOT NULL DEFAULT date_trunc('month',now() AT TIME ZONE 'UTC')::date,
 reserved_micros bigint NOT NULL CHECK(reserved_micros>0),
 settled_micros bigint CHECK(settled_micros>=0 AND settled_micros<=reserved_micros),
 input_tokens bigint CHECK(input_tokens>=0), output_tokens bigint CHECK(output_tokens>=0),
 lease_until timestamptz NOT NULL, finished_at timestamptz,
 CHECK ((settled_micros IS NULL AND finished_at IS NULL AND input_tokens IS NULL AND output_tokens IS NULL)
     OR (settled_micros IS NOT NULL AND finished_at IS NOT NULL AND input_tokens IS NOT NULL AND output_tokens IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS inference_attempt_period ON inference.attempt(period);
CREATE OR REPLACE FUNCTION inference.guard_attempt() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 IF OLD.settled_micros IS NOT NULL OR
    (to_jsonb(NEW)-ARRAY['settled_micros','input_tokens','output_tokens','finished_at']) IS DISTINCT FROM
    (to_jsonb(OLD)-ARRAY['settled_micros','input_tokens','output_tokens','finished_at']) THEN
  RAISE EXCEPTION 'inference accounting history is immutable';
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS guard_attempt ON inference.attempt;
CREATE TRIGGER guard_attempt BEFORE UPDATE ON inference.attempt FOR EACH ROW EXECUTE FUNCTION inference.guard_attempt();
INSERT INTO inference.schema_version VALUES(1) ON CONFLICT DO NOTHING;
COMMIT;
