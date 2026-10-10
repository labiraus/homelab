-- Empty test databases only. Roll production back by disabling the feature.
BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ BEGIN
 IF EXISTS(SELECT FROM worldbuilder.world) THEN
  RAISE EXCEPTION 'refusing lifecycle rollback with world data; disable the feature and preserve history';
 END IF;
END $$;
DROP TRIGGER IF EXISTS worldbuilder_register_identity ON worldbuilder.record;
DROP FUNCTION IF EXISTS worldbuilder.register_identity();
ALTER TABLE worldbuilder.record DROP CONSTRAINT IF EXISTS record_stable_identity;
DROP TABLE IF EXISTS worldbuilder.record_identity;
DROP INDEX IF EXISTS worldbuilder.world_single_owner;
DROP INDEX IF EXISTS worldbuilder.world_single_settings;
ALTER TABLE worldbuilder.record DROP CONSTRAINT IF EXISTS record_kind_check;
ALTER TABLE worldbuilder.record ADD CONSTRAINT record_kind_check CHECK(kind IN ('type','timeline','entity','fact','relationship','event','rule','source'));
ALTER TABLE worldbuilder.world DROP COLUMN IF EXISTS default_timeline, DROP COLUMN IF EXISTS archived, DROP COLUMN IF EXISTS admin_version;
DELETE FROM worldbuilder.schema_version WHERE version=2;
COMMIT;
