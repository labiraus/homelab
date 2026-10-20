BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ BEGIN
 IF EXISTS(SELECT FROM worldbuilder.world) OR EXISTS(SELECT FROM worldbuilder.inference_attempt) THEN RAISE EXCEPTION 'preserve populated inference history; disable proxy for rollback'; END IF;
END $$;
ALTER TABLE worldbuilder.conversation_message DROP COLUMN IF EXISTS inference_profile;
ALTER TABLE worldbuilder.generation_job DROP COLUMN IF EXISTS inference_profile;
ALTER TABLE worldbuilder.inference_attempt DROP CONSTRAINT IF EXISTS inference_attempt_accounting_check;
ALTER TABLE worldbuilder.inference_attempt DROP COLUMN IF EXISTS accounting_mode;
ALTER TABLE worldbuilder.inference_attempt DROP COLUMN IF EXISTS inference_profile;
ALTER TABLE worldbuilder.inference_attempt ADD CONSTRAINT inference_attempt_allocated_micros_check CHECK(allocated_micros>0);
DELETE FROM worldbuilder.schema_version WHERE version=10;
COMMIT;
