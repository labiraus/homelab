BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ BEGIN
 IF EXISTS(SELECT FROM worldbuilder.world) OR EXISTS(SELECT FROM worldbuilder.superuser) OR EXISTS(SELECT FROM worldbuilder.account_audit) THEN
  RAISE EXCEPTION 'refusing populated superuser rollback';
 END IF;
END $$;
CREATE OR REPLACE FUNCTION worldbuilder.guard_review() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.reviewer=(SELECT author FROM worldbuilder.change_set WHERE world_id=NEW.world_id AND id=NEW.change_id) THEN RAISE EXCEPTION 'review must be independent of author'; END IF;
 RETURN NEW;
END $$;
ALTER TABLE worldbuilder.change_set ADD CONSTRAINT change_set_independent_reviewer CHECK(reviewer IS NULL OR reviewer <> author);
ALTER TABLE worldbuilder.change_review DROP COLUMN IF EXISTS authority_scope;
ALTER TABLE worldbuilder.change_review DROP COLUMN IF EXISTS superuser_authority_id;
ALTER TABLE worldbuilder.change_review ADD PRIMARY KEY(world_id,change_id,reviewer,authority_id);
DROP TRIGGER IF EXISTS worldbuilder_self_reviewer ON worldbuilder.change_set;
DROP FUNCTION IF EXISTS worldbuilder.lock_superuser(text);
DROP TABLE IF EXISTS worldbuilder.account_audit,worldbuilder.superuser;
DROP FUNCTION IF EXISTS worldbuilder.guard_self_reviewer(),worldbuilder.audit_superuser();
-- auth.users is shared legacy data and is deliberately preserved.
DELETE FROM worldbuilder.schema_version WHERE version=8;
COMMIT;
