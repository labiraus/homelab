-- Populated upgrades are deliberately not destructively downgraded.
BEGIN;
DO $$ BEGIN
 IF EXISTS(SELECT FROM worldbuilder.world) THEN RAISE EXCEPTION 'populated schema 9 must be preserved; disable feature for rollback'; END IF;
END $$;
DROP TRIGGER IF EXISTS worldbuilder_established_guard ON worldbuilder.revision;
DROP TRIGGER IF EXISTS worldbuilder_established_capture ON worldbuilder.revision;
DROP TABLE IF EXISTS worldbuilder.conversation_message,worldbuilder.conversation,worldbuilder.established_account;
DROP FUNCTION IF EXISTS worldbuilder.guard_conversation_message();
DROP FUNCTION IF EXISTS worldbuilder.guard_established_revision(),worldbuilder.capture_established_revision(),worldbuilder.set_administrator(text,text,boolean,text);
ALTER TABLE worldbuilder.world DROP COLUMN publication_policy, DROP COLUMN policy_version;
ALTER TABLE worldbuilder.change_set DROP COLUMN policy_version;
ALTER TABLE worldbuilder.change_review DROP COLUMN policy_version;
ALTER TABLE worldbuilder.story_session DROP COLUMN mode, DROP COLUMN scene;
ALTER TABLE worldbuilder.story_turn DROP COLUMN scene;
ALTER TABLE worldbuilder.member DROP CONSTRAINT member_role_check;
ALTER TABLE worldbuilder.member ADD CONSTRAINT member_role_check CHECK(role IN ('owner','admin','editor','reviewer','reader'));
CREATE UNIQUE INDEX world_single_owner ON worldbuilder.member(world_id) WHERE role='owner';
ALTER TABLE worldbuilder.change_set DROP CONSTRAINT change_set_required_approvals_check;
ALTER TABLE worldbuilder.change_set ADD CONSTRAINT change_set_required_approvals_check CHECK(required_approvals BETWEEN 1 AND 5);
ALTER TABLE worldbuilder.change_set ADD CONSTRAINT change_set_reviewed_check CHECK(status NOT IN ('approved','published') OR reviewer IS NOT NULL);
ALTER TABLE worldbuilder.record DROP CONSTRAINT record_kind_check;
ALTER TABLE worldbuilder.record ADD CONSTRAINT record_kind_check CHECK(kind IN ('type','entity','source','timeline','fact','relationship','event','rule','world_settings'));
DELETE FROM worldbuilder.schema_version WHERE version=9;
CREATE OR REPLACE FUNCTION worldbuilder.guard_change_content() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (NEW.world_id,NEW.id,NEW.author,NEW.base_revision,NEW.reason,NEW.operations,NEW.required_approvals,NEW.created_at)
   IS DISTINCT FROM (OLD.world_id,OLD.id,OLD.author,OLD.base_revision,OLD.reason,OLD.operations,OLD.required_approvals,OLD.created_at)
   OR (OLD.status='published' AND NEW IS DISTINCT FROM OLD) THEN RAISE EXCEPTION 'proposal content and published decision are immutable'; END IF;
 RETURN NEW;
END $$;
CREATE OR REPLACE FUNCTION worldbuilder.guard_review() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.reviewer=(SELECT author FROM worldbuilder.change_set WHERE world_id=NEW.world_id AND id=NEW.change_id) THEN
  NEW.superuser_authority_id=worldbuilder.lock_superuser(NEW.reviewer);
  IF NEW.superuser_authority_id IS NULL THEN RAISE EXCEPTION 'self review requires current superuser authority'; END IF;
 ELSE NEW.superuser_authority_id=NULL;
 END IF;
 RETURN NEW;
END $$;
CREATE OR REPLACE FUNCTION worldbuilder.guard_self_reviewer() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.reviewer=NEW.author AND NOT EXISTS(SELECT FROM worldbuilder.superuser WHERE principal=NEW.author) THEN
  RAISE EXCEPTION 'self reviewer requires current superuser authority';
 END IF;
 RETURN NEW;
END $$;
COMMIT;
