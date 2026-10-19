-- PWA domain: collective authorship, established accounts and shared conversations.
-- Apply explicitly as migration owner; immutable historical snapshots are unchanged.
BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM worldbuilder.schema_version WHERE version=8) THEN RAISE EXCEPTION 'apply migration 008 first'; END IF;
END $$;
ALTER TABLE worldbuilder.world ADD COLUMN IF NOT EXISTS publication_policy jsonb NOT NULL DEFAULT '{"mode":"independent","requiredApprovals":1}';
ALTER TABLE worldbuilder.world ADD COLUMN IF NOT EXISTS policy_version bigint NOT NULL DEFAULT 1;
ALTER TABLE worldbuilder.change_set ADD COLUMN IF NOT EXISTS policy_version bigint NOT NULL DEFAULT 0;
ALTER TABLE worldbuilder.change_review ADD COLUMN IF NOT EXISTS policy_version bigint NOT NULL DEFAULT 0;
DROP INDEX IF EXISTS worldbuilder.world_single_owner;
ALTER TABLE worldbuilder.member DROP CONSTRAINT IF EXISTS member_role_check;
UPDATE worldbuilder.member SET role='author' WHERE role IN ('owner','admin','editor','reviewer');
ALTER TABLE worldbuilder.member ADD CONSTRAINT member_role_check CHECK(role IN ('author','reader'));
UPDATE worldbuilder.world w SET publication_policy=jsonb_build_object('mode','independent','requiredApprovals',COALESCE((r.snapshot->'world_settings'->'reviewPolicy'->>'requiredApprovals')::integer,1))
 FROM worldbuilder.revision r WHERE r.world_id=w.id AND r.revision=w.revision AND NOT EXISTS(SELECT FROM worldbuilder.schema_version WHERE version=9);
ALTER TABLE worldbuilder.change_set DROP CONSTRAINT IF EXISTS change_set_required_approvals_check;
ALTER TABLE worldbuilder.change_set ADD CONSTRAINT change_set_required_approvals_check CHECK(required_approvals BETWEEN 0 AND 5000);
-- Direct publication does not require a legacy single-reviewer column.
DO $$ DECLARE c text; BEGIN
 FOR c IN SELECT conname FROM pg_constraint WHERE conrelid='worldbuilder.change_set'::regclass AND contype='c' AND pg_get_constraintdef(oid) LIKE '%reviewer IS NOT NULL%'
 LOOP EXECUTE format('ALTER TABLE worldbuilder.change_set DROP CONSTRAINT %I',c); END LOOP;
 FOR c IN SELECT conname FROM pg_constraint WHERE conrelid='worldbuilder.record'::regclass AND contype='c' AND pg_get_constraintdef(oid) LIKE '%kind = ANY%'
 LOOP
  IF pg_get_constraintdef((SELECT oid FROM pg_constraint WHERE conrelid='worldbuilder.record'::regclass AND conname=c)) LIKE '%world_settings%' THEN
   EXECUTE format('ALTER TABLE worldbuilder.record DROP CONSTRAINT %I',c);
  END IF;
 END LOOP;
END $$;
ALTER TABLE worldbuilder.record DROP CONSTRAINT IF EXISTS record_kind_check;
ALTER TABLE worldbuilder.record ADD CONSTRAINT record_kind_check CHECK(kind IN ('type','entity','source','timeline','fact','relationship','event','rule','world_settings','established_account'));
ALTER TABLE worldbuilder.story_session ADD COLUMN IF NOT EXISTS mode text NOT NULL DEFAULT 'canon_eligible' CHECK(mode IN ('canon_eligible','sandbox'));
ALTER TABLE worldbuilder.story_session ADD COLUMN IF NOT EXISTS scene jsonb NOT NULL DEFAULT '{}';
ALTER TABLE worldbuilder.story_turn ADD COLUMN IF NOT EXISTS scene jsonb NOT NULL DEFAULT '{}';
CREATE TABLE IF NOT EXISTS worldbuilder.established_account (
 world_id uuid NOT NULL, id text NOT NULL, timeline text NOT NULL, revision bigint NOT NULL,
 record jsonb NOT NULL CHECK(record->>'kind'='established_account' AND record->>'id'=id AND record->>'timeline'=timeline AND length(trim(record->>'synopsis'))>0), supersedes text, actor text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(world_id,id), UNIQUE(world_id,timeline,supersedes),
 FOREIGN KEY(world_id,revision) REFERENCES worldbuilder.revision(world_id,revision)
);
CREATE OR REPLACE FUNCTION worldbuilder.guard_established_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT FROM worldbuilder.established_account a WHERE a.world_id=NEW.world_id AND (NEW.snapshot->a.id) IS DISTINCT FROM a.record) THEN
  RAISE EXCEPTION 'established accounts cannot be changed or removed; supersede explicitly';
 END IF;
 RETURN NEW;
END $$;
CREATE OR REPLACE FUNCTION worldbuilder.capture_established_revision() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 INSERT INTO worldbuilder.established_account(world_id,id,timeline,revision,record,supersedes,actor)
 SELECT NEW.world_id,e.key,e.value->>'timeline',NEW.revision,e.value,nullif(e.value->>'supersedes',''),NEW.actor
 FROM jsonb_each(NEW.snapshot) e WHERE e.value->>'kind'='established_account'
 AND NOT EXISTS(SELECT FROM worldbuilder.established_account a WHERE a.world_id=NEW.world_id AND a.id=e.key);
 RETURN NEW;
END $$;
CREATE TABLE IF NOT EXISTS worldbuilder.conversation (
 world_id uuid NOT NULL REFERENCES worldbuilder.world(id), id uuid NOT NULL, title text NOT NULL CHECK(length(title) BETWEEN 1 AND 200),
 creator text NOT NULL, revision bigint NOT NULL, timeline text NOT NULL, at bigint, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(world_id,id), FOREIGN KEY(world_id,revision) REFERENCES worldbuilder.revision(world_id,revision)
);
CREATE TABLE IF NOT EXISTS worldbuilder.conversation_message (
 world_id uuid NOT NULL, conversation_id uuid NOT NULL, id uuid NOT NULL, sequence bigint GENERATED ALWAYS AS IDENTITY,
 actor text NOT NULL, request jsonb NOT NULL, result jsonb, status text NOT NULL DEFAULT 'saved' CHECK(status IN ('saved','running','complete','failed','uncertain')),
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(world_id,conversation_id,id),
 FOREIGN KEY(world_id,conversation_id) REFERENCES worldbuilder.conversation(world_id,id)
);
CREATE OR REPLACE FUNCTION worldbuilder.guard_conversation_message() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'conversation messages are retained'; END IF;
 IF (NEW.world_id,NEW.conversation_id,NEW.id,NEW.sequence,NEW.actor,NEW.request,NEW.created_at) IS DISTINCT FROM (OLD.world_id,OLD.conversation_id,OLD.id,OLD.sequence,OLD.actor,OLD.request,OLD.created_at)
 OR (OLD.status IN ('complete','failed','uncertain') AND NEW IS DISTINCT FROM OLD)
 OR (OLD.status='running' AND NEW.status='saved') THEN RAISE EXCEPTION 'conversation request and completed result are immutable'; END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS worldbuilder_conversation_immutable ON worldbuilder.conversation;
CREATE TRIGGER worldbuilder_conversation_immutable BEFORE UPDATE OR DELETE ON worldbuilder.conversation FOR EACH ROW EXECUTE FUNCTION worldbuilder.immutable();
DROP TRIGGER IF EXISTS worldbuilder_message_guard ON worldbuilder.conversation_message;
CREATE TRIGGER worldbuilder_message_guard BEFORE UPDATE OR DELETE ON worldbuilder.conversation_message FOR EACH ROW EXECUTE FUNCTION worldbuilder.guard_conversation_message();
CREATE OR REPLACE FUNCTION worldbuilder.guard_review() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE p jsonb; proposer text; BEGIN
 SELECT publication_policy INTO p FROM worldbuilder.world WHERE id=NEW.world_id;
 SELECT author INTO proposer FROM worldbuilder.change_set WHERE world_id=NEW.world_id AND id=NEW.change_id;
 IF NEW.reviewer=proposer THEN
  NEW.superuser_authority_id=worldbuilder.lock_superuser(NEW.reviewer);
  IF p->>'mode'<>'unanimous' THEN RAISE EXCEPTION 'self review requires unanimous policy'; END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE OR REPLACE FUNCTION worldbuilder.guard_self_reviewer() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.reviewer=NEW.author AND (SELECT publication_policy->>'mode' FROM worldbuilder.world WHERE id=NEW.world_id)<>'unanimous' THEN
  RAISE EXCEPTION 'self review requires unanimous policy';
 END IF;
 RETURN NEW;
END $$;
CREATE OR REPLACE FUNCTION worldbuilder.guard_change_content() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (NEW.world_id,NEW.id,NEW.author,NEW.base_revision,NEW.reason,NEW.operations,NEW.required_approvals,NEW.policy_version,NEW.created_at)
 IS DISTINCT FROM (OLD.world_id,OLD.id,OLD.author,OLD.base_revision,OLD.reason,OLD.operations,OLD.required_approvals,OLD.policy_version,OLD.created_at)
 OR (OLD.status='published' AND NEW IS DISTINCT FROM OLD) THEN RAISE EXCEPTION 'proposal content and published decision are immutable'; END IF;
 RETURN NEW;
END $$;
-- Narrow audited Administrator management; application receives no broad grant write permission.
CREATE OR REPLACE FUNCTION worldbuilder.set_administrator(actor_email text,target_email text,enabled boolean,reason text) RETURNS void
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 PERFORM pg_advisory_xact_lock(hashtext('worldbuilder-administrators'));
 IF NOT EXISTS(SELECT FROM worldbuilder.superuser WHERE principal=actor_email) THEN RAISE EXCEPTION 'Administrator required'; END IF;
 IF length(trim(reason)) NOT BETWEEN 1 AND 2000 OR position('@' in target_email)=0 THEN RAISE EXCEPTION 'registered user and reason required'; END IF;
 IF enabled THEN
  INSERT INTO worldbuilder.superuser(principal,granted_by) VALUES(target_email,actor_email) ON CONFLICT DO NOTHING;
 ELSE
  IF EXISTS(SELECT FROM worldbuilder.superuser WHERE principal=target_email) AND (SELECT count(*) FROM worldbuilder.superuser)<=1 THEN RAISE EXCEPTION 'cannot remove final Administrator'; END IF;
  DELETE FROM worldbuilder.superuser WHERE principal=target_email;
 END IF;
 INSERT INTO worldbuilder.account_audit(actor,action,principal,detail) VALUES(actor_email,'administrator.set',target_email,jsonb_build_object('enabled',enabled,'reason',reason));
END $$;
REVOKE ALL ON FUNCTION worldbuilder.set_administrator(text,text,boolean,text) FROM PUBLIC;
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM pg_trigger WHERE tgname='worldbuilder_established_immutable') THEN
  CREATE TRIGGER worldbuilder_established_immutable BEFORE UPDATE OR DELETE ON worldbuilder.established_account FOR EACH ROW EXECUTE FUNCTION worldbuilder.immutable();
  CREATE TRIGGER worldbuilder_established_guard BEFORE INSERT ON worldbuilder.revision FOR EACH ROW EXECUTE FUNCTION worldbuilder.guard_established_revision();
  CREATE TRIGGER worldbuilder_established_capture AFTER INSERT ON worldbuilder.revision FOR EACH ROW EXECUTE FUNCTION worldbuilder.capture_established_revision();
 END IF;
END $$;
INSERT INTO worldbuilder.schema_version(version) VALUES(9) ON CONFLICT DO NOTHING;
COMMIT;
