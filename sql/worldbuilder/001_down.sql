-- Disposable EMPTY foundation rollback only. Production rollback disables the feature and preserves data.
BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ BEGIN
 IF EXISTS(SELECT FROM worldbuilder.world) THEN
  RAISE EXCEPTION 'Refusing to remove populated Worldbuilder schema; disable feature and retain data';
 END IF;
END $$;
DROP SCHEMA worldbuilder CASCADE;
COMMIT;
