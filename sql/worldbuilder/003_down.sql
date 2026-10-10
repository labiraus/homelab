-- Empty disposable databases only; populated rollback preserves billing evidence.
BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ BEGIN
 IF EXISTS(SELECT FROM worldbuilder.world) OR EXISTS(SELECT FROM worldbuilder.inference_attempt) THEN
  RAISE EXCEPTION 'refusing inference rollback with data; disable inference and retain evidence';
 END IF;
END $$;
DROP TABLE IF EXISTS worldbuilder.inference_attempt;
DELETE FROM worldbuilder.schema_version WHERE version=3;
COMMIT;
