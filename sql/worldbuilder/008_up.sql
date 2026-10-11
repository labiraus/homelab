-- Operator-granted superusers may register users and review their own proposals.
BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM worldbuilder.schema_version WHERE version=7) THEN RAISE EXCEPTION 'apply migration 007 first'; END IF;
END $$;
CREATE TABLE IF NOT EXISTS worldbuilder.superuser (
 principal text PRIMARY KEY CHECK(principal=lower(trim(principal)) AND principal LIKE '%@%'),
 authority_id uuid NOT NULL DEFAULT gen_random_uuid(),
 granted_at timestamptz NOT NULL DEFAULT now(), granted_by text NOT NULL
);
CREATE TABLE IF NOT EXISTS worldbuilder.account_audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 actor text NOT NULL, action text NOT NULL, principal text NOT NULL,
 detail jsonb NOT NULL DEFAULT '{}', created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE worldbuilder.change_review ADD COLUMN IF NOT EXISTS superuser_authority_id uuid;
ALTER TABLE worldbuilder.change_review ADD COLUMN IF NOT EXISTS authority_scope uuid
 GENERATED ALWAYS AS (coalesce(superuser_authority_id,'00000000-0000-0000-0000-000000000000'::uuid)) STORED;
ALTER TABLE worldbuilder.change_review DROP CONSTRAINT IF EXISTS change_review_pkey;
ALTER TABLE worldbuilder.change_review ADD PRIMARY KEY(world_id,change_id,reviewer,authority_id,authority_scope);
CREATE OR REPLACE FUNCTION worldbuilder.lock_superuser(actor_email text) RETURNS uuid
 LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT s.authority_id FROM worldbuilder.superuser s WHERE s.principal=$1 FOR SHARE
$$;
REVOKE ALL ON FUNCTION worldbuilder.lock_superuser(text) FROM PUBLIC;
-- Replace only the original author/reviewer check; retain all other constraints.
DO $$ DECLARE constraint_name text; BEGIN
 FOR constraint_name IN SELECT conname FROM pg_constraint
  WHERE conrelid='worldbuilder.change_set'::regclass AND contype='c'
    AND pg_get_constraintdef(oid) LIKE '%reviewer <> author%'
 LOOP EXECUTE format('ALTER TABLE worldbuilder.change_set DROP CONSTRAINT %I',constraint_name); END LOOP;
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
CREATE OR REPLACE FUNCTION worldbuilder.audit_superuser() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='INSERT' THEN
  INSERT INTO worldbuilder.account_audit(actor,action,principal,detail) VALUES(current_user,'superuser.grant',NEW.principal,jsonb_build_object('authorityId',NEW.authority_id,'grantedBy',NEW.granted_by));
  RETURN NEW;
 END IF;
 INSERT INTO worldbuilder.account_audit(actor,action,principal,detail) VALUES(current_user,'superuser.revoke',OLD.principal,jsonb_build_object('authorityId',OLD.authority_id));
 RETURN OLD;
END $$;
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM pg_trigger WHERE tgrelid='worldbuilder.account_audit'::regclass AND tgname='worldbuilder_account_audit_immutable') THEN
  CREATE TRIGGER worldbuilder_account_audit_immutable BEFORE UPDATE OR DELETE ON worldbuilder.account_audit FOR EACH ROW EXECUTE FUNCTION worldbuilder.immutable();
  CREATE TRIGGER worldbuilder_superuser_immutable BEFORE UPDATE ON worldbuilder.superuser FOR EACH ROW EXECUTE FUNCTION worldbuilder.immutable();
  CREATE TRIGGER worldbuilder_superuser_audit AFTER INSERT OR DELETE ON worldbuilder.superuser FOR EACH ROW EXECUTE FUNCTION worldbuilder.audit_superuser();
  CREATE TRIGGER worldbuilder_self_reviewer BEFORE INSERT OR UPDATE ON worldbuilder.change_set FOR EACH ROW EXECUTE FUNCTION worldbuilder.guard_self_reviewer();
 END IF;
END $$;
INSERT INTO worldbuilder.schema_version(version) VALUES(8) ON CONFLICT DO NOTHING;
COMMIT;
