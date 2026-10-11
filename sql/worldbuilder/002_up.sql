-- Explicit, additive lifecycle/settings migration. Never runs on application startup.
BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM worldbuilder.schema_version WHERE version=1) THEN
  RAISE EXCEPTION 'apply Worldbuilder migration 001 first';
 END IF;
 IF EXISTS(SELECT 1 FROM worldbuilder.revision r CROSS JOIN LATERAL jsonb_each(r.snapshot) entry
   GROUP BY r.world_id,entry.key HAVING count(DISTINCT entry.value->>'kind')>1) THEN
  RAISE EXCEPTION 'historical record IDs have conflicting kinds; inventory and reconcile explicitly before migration 002';
 END IF;
END $$;
ALTER TABLE worldbuilder.world ADD COLUMN IF NOT EXISTS default_timeline text NOT NULL DEFAULT 'main';
ALTER TABLE worldbuilder.world ADD COLUMN IF NOT EXISTS archived boolean NOT NULL DEFAULT false;
ALTER TABLE worldbuilder.world ADD COLUMN IF NOT EXISTS admin_version bigint NOT NULL DEFAULT 0 CHECK(admin_version>=0);
CREATE UNIQUE INDEX IF NOT EXISTS world_single_owner ON worldbuilder.member(world_id) WHERE role='owner';
CREATE TABLE IF NOT EXISTS worldbuilder.record_identity (
 world_id uuid NOT NULL REFERENCES worldbuilder.world(id), id text NOT NULL, kind text NOT NULL,
 PRIMARY KEY(world_id,id), UNIQUE(world_id,id,kind)
);
-- Seed tombstones from every immutable revision, including currently deleted IDs.
INSERT INTO worldbuilder.record_identity(world_id,id,kind)
 SELECT DISTINCT r.world_id,entry.key,entry.value->>'kind'
 FROM worldbuilder.revision r CROSS JOIN LATERAL jsonb_each(r.snapshot) entry
 ON CONFLICT DO NOTHING;
ALTER TABLE worldbuilder.record DROP CONSTRAINT IF EXISTS record_kind_check;
ALTER TABLE worldbuilder.record ADD CONSTRAINT record_kind_check CHECK(kind IN ('type','timeline','entity','fact','relationship','event','rule','source','world_settings'));
CREATE UNIQUE INDEX IF NOT EXISTS world_single_settings ON worldbuilder.record(world_id) WHERE kind='world_settings';
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM pg_constraint WHERE conname='record_stable_identity' AND conrelid='worldbuilder.record'::regclass) THEN
  ALTER TABLE worldbuilder.record ADD CONSTRAINT record_stable_identity FOREIGN KEY(world_id,id,kind) REFERENCES worldbuilder.record_identity(world_id,id,kind);
 END IF;
END $$;
CREATE OR REPLACE FUNCTION worldbuilder.register_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO worldbuilder.record_identity VALUES(NEW.world_id,NEW.id,NEW.kind) ON CONFLICT DO NOTHING;
 RETURN NEW;
END $$;
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM pg_trigger WHERE tgrelid='worldbuilder.record'::regclass AND tgname='worldbuilder_register_identity') THEN
  CREATE TRIGGER worldbuilder_register_identity BEFORE INSERT ON worldbuilder.record FOR EACH ROW EXECUTE FUNCTION worldbuilder.register_identity();
  CREATE TRIGGER worldbuilder_identity_immutable BEFORE UPDATE OR DELETE ON worldbuilder.record_identity FOR EACH ROW EXECUTE FUNCTION worldbuilder.immutable();
 END IF;
END $$;
INSERT INTO worldbuilder.schema_version(version) VALUES(2) ON CONFLICT DO NOTHING;
COMMIT;
