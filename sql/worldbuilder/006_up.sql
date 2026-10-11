-- Reviewed calendars and full-range typed date facts. No historical rewrites.
BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ DECLARE checks text[]; BEGIN
 IF NOT EXISTS(SELECT FROM worldbuilder.schema_version WHERE version=5) THEN RAISE EXCEPTION 'apply migration 005 first'; END IF;
 IF NOT EXISTS(SELECT FROM worldbuilder.schema_version WHERE version=6) THEN
  -- Migration 001 left this compound constraint unnamed. Identify it by the
  -- value_type column, refusing unexpected schema drift instead of guessing.
  SELECT array_agg(c.conname) INTO checks FROM pg_constraint c
   JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=ANY(c.conkey)
   WHERE c.conrelid='worldbuilder.record'::regclass AND c.contype='c' AND a.attname='value_type';
  IF cardinality(checks) IS DISTINCT FROM 1 THEN RAISE EXCEPTION 'expected one fact value constraint'; END IF;
  EXECUTE format('ALTER TABLE worldbuilder.record DROP CONSTRAINT %I', checks[1]);
  ALTER TABLE worldbuilder.record ADD CONSTRAINT record_fact_value_check CHECK(kind <> 'fact' OR
   (subject_id IS NOT NULL AND source_id IS NOT NULL AND predicate <> '' AND value IS NOT NULL AND value_type IN ('string','number','boolean','entity','tick')));
  ALTER TABLE worldbuilder.record ADD COLUMN calendar jsonb;
  ALTER TABLE worldbuilder.record ADD CONSTRAINT record_calendar_check CHECK(calendar IS NULL OR (kind='world_settings' AND jsonb_typeof(calendar)='object'));
 END IF;
END $$;
INSERT INTO worldbuilder.schema_version(version) VALUES(6) ON CONFLICT DO NOTHING;
COMMIT;
