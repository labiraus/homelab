BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM worldbuilder.schema_version WHERE version=3) THEN RAISE EXCEPTION 'apply migration 003 first'; END IF;
END $$;
CREATE TABLE IF NOT EXISTS worldbuilder.story_definition (
 world_id uuid NOT NULL, id uuid NOT NULL, owner text NOT NULL,
 pinned_revision bigint NOT NULL, definition jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(world_id,id), FOREIGN KEY(world_id,pinned_revision) REFERENCES worldbuilder.revision(world_id,revision)
);
CREATE INDEX IF NOT EXISTS story_definition_owner ON worldbuilder.story_definition(world_id,owner,created_at);
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM pg_trigger WHERE tgrelid='worldbuilder.story_definition'::regclass AND tgname='worldbuilder_definition_immutable') THEN
  CREATE TRIGGER worldbuilder_definition_immutable BEFORE UPDATE OR DELETE ON worldbuilder.story_definition FOR EACH ROW EXECUTE FUNCTION worldbuilder.immutable();
 END IF;
END $$;
ALTER TABLE worldbuilder.story_session ADD COLUMN IF NOT EXISTS definition_id uuid;
ALTER TABLE worldbuilder.story_session ADD COLUMN IF NOT EXISTS memory jsonb NOT NULL DEFAULT '{"summary":"","throughVersion":0,"choices":[]}';
ALTER TABLE worldbuilder.story_turn ADD COLUMN IF NOT EXISTS memory jsonb NOT NULL DEFAULT '{"summary":"","throughVersion":0,"choices":[]}';
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM pg_constraint WHERE conname='story_session_definition' AND conrelid='worldbuilder.story_session'::regclass) THEN
  ALTER TABLE worldbuilder.story_session ADD CONSTRAINT story_session_definition FOREIGN KEY(world_id,definition_id) REFERENCES worldbuilder.story_definition(world_id,id);
 END IF;
END $$;
INSERT INTO worldbuilder.schema_version(version) VALUES(4) ON CONFLICT DO NOTHING;
COMMIT;
