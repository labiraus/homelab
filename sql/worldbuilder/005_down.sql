BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ BEGIN
 IF EXISTS(SELECT FROM worldbuilder.world) OR EXISTS(SELECT FROM worldbuilder.generation_job) OR EXISTS(SELECT FROM worldbuilder.inference_attempt) THEN RAISE EXCEPTION 'refusing populated generation rollback'; END IF;
END $$;
ALTER TABLE worldbuilder.inference_attempt DROP COLUMN IF EXISTS generation_job_id, DROP COLUMN IF EXISTS generation_part;
DROP TABLE IF EXISTS worldbuilder.generation_outbox;
DROP TABLE IF EXISTS worldbuilder.generation_job;
DELETE FROM worldbuilder.schema_version WHERE version=5;
COMMIT;
