BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ BEGIN
 IF EXISTS(SELECT FROM worldbuilder.world) OR EXISTS(SELECT FROM worldbuilder.story_definition) THEN RAISE EXCEPTION 'refusing populated story rollback'; END IF;
END $$;
ALTER TABLE worldbuilder.story_session DROP CONSTRAINT IF EXISTS story_session_definition;
ALTER TABLE worldbuilder.story_session DROP COLUMN IF EXISTS definition_id, DROP COLUMN IF EXISTS memory;
ALTER TABLE worldbuilder.story_turn DROP COLUMN IF EXISTS memory;
DROP TABLE IF EXISTS worldbuilder.story_definition;
DELETE FROM worldbuilder.schema_version WHERE version=4;
COMMIT;
