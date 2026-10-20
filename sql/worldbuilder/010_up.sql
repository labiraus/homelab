-- Independent inference proxy; historical allocations remain unchanged.
BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM worldbuilder.schema_version WHERE version=9) THEN RAISE EXCEPTION 'apply migration 009 first'; END IF;
END $$;
ALTER TABLE worldbuilder.inference_attempt ADD COLUMN IF NOT EXISTS accounting_mode text NOT NULL DEFAULT 'legacy';
ALTER TABLE worldbuilder.inference_attempt ADD COLUMN IF NOT EXISTS inference_profile jsonb NOT NULL DEFAULT '{"mode":"legacy"}';
ALTER TABLE worldbuilder.inference_attempt DROP CONSTRAINT IF EXISTS inference_attempt_allocated_micros_check;
ALTER TABLE worldbuilder.inference_attempt DROP CONSTRAINT IF EXISTS inference_attempt_accounting_check;
ALTER TABLE worldbuilder.inference_attempt ADD CONSTRAINT inference_attempt_accounting_check CHECK
 ((accounting_mode='legacy' AND allocated_micros>0) OR (accounting_mode='proxy' AND allocated_micros=0 AND currency='USD'));
ALTER TABLE worldbuilder.generation_job ADD COLUMN IF NOT EXISTS inference_profile jsonb NOT NULL DEFAULT '{"mode":"legacy"}';
ALTER TABLE worldbuilder.conversation_message ADD COLUMN IF NOT EXISTS inference_profile jsonb NOT NULL DEFAULT '{"mode":"legacy"}';
INSERT INTO worldbuilder.schema_version VALUES(10) ON CONFLICT DO NOTHING;
COMMIT;
