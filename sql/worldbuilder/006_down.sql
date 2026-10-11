-- Disposable empty databases only. Populated production rollback disables the feature.
BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ BEGIN
 IF EXISTS(SELECT FROM worldbuilder.world) THEN RAISE EXCEPTION 'refusing populated calendar rollback'; END IF;
END $$;
ALTER TABLE worldbuilder.record DROP COLUMN IF EXISTS calendar;
ALTER TABLE worldbuilder.record DROP CONSTRAINT IF EXISTS record_fact_value_check;
ALTER TABLE worldbuilder.record ADD CONSTRAINT record_fact_value_check CHECK(kind <> 'fact' OR
 (subject_id IS NOT NULL AND source_id IS NOT NULL AND predicate <> '' AND value IS NOT NULL AND value_type IN ('string','number','boolean','entity')));
DELETE FROM worldbuilder.schema_version WHERE version=6;
COMMIT;
