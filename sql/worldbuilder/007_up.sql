-- Reviewed human quorum and immutable decisions; preserve existing history.
BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM worldbuilder.schema_version WHERE version=6) THEN RAISE EXCEPTION 'apply migration 006 first'; END IF;
END $$;
ALTER TABLE worldbuilder.member ADD COLUMN IF NOT EXISTS authority_id uuid NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE worldbuilder.record ADD COLUMN IF NOT EXISTS review_policy jsonb;
ALTER TABLE worldbuilder.change_set ADD COLUMN IF NOT EXISTS required_approvals integer NOT NULL DEFAULT 1 CHECK(required_approvals BETWEEN 1 AND 5);
ALTER TABLE worldbuilder.change_set ADD COLUMN IF NOT EXISTS published_approvals text[] NOT NULL DEFAULT '{}';
CREATE TABLE IF NOT EXISTS worldbuilder.change_review (
 world_id uuid NOT NULL, change_id uuid NOT NULL, reviewer text NOT NULL, authority_id uuid NOT NULL,
 decision text NOT NULL CHECK(decision IN ('approved','rejected')), reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 2000),
 created_at timestamptz DEFAULT now(), legacy boolean NOT NULL DEFAULT false,
 PRIMARY KEY(world_id,change_id,reviewer,authority_id),
 FOREIGN KEY(world_id,change_id) REFERENCES worldbuilder.change_set(world_id,id)
);
-- Retain old decisions without inventing a review time. The old audit provides
-- it when available. Removed/ineligible reviewers get an unmatched authority ID.
INSERT INTO worldbuilder.change_review(world_id,change_id,reviewer,authority_id,decision,reason,created_at,legacy)
 SELECT c.world_id,c.id,c.reviewer,
  CASE WHEN m.role IN ('owner','admin','reviewer') THEN m.authority_id ELSE gen_random_uuid() END,
  CASE WHEN c.status='rejected' THEN 'rejected' ELSE 'approved' END,c.review_reason,
  (SELECT a.created_at FROM worldbuilder.audit a WHERE a.world_id=c.world_id AND a.resource=c.id::text
    AND a.actor=c.reviewer AND a.action='change.review' AND a.detail->>'reason'=c.review_reason ORDER BY a.id DESC LIMIT 1),true
 FROM worldbuilder.change_set c LEFT JOIN worldbuilder.member m ON m.world_id=c.world_id AND m.principal=c.reviewer
 WHERE c.reviewer IS NOT NULL AND NOT EXISTS(SELECT FROM worldbuilder.schema_version WHERE version=7);
UPDATE worldbuilder.change_set SET published_approvals=ARRAY[reviewer]
 WHERE status='published' AND NOT EXISTS(SELECT FROM worldbuilder.schema_version WHERE version=7);
CREATE OR REPLACE FUNCTION worldbuilder.rotate_review_authority() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (NEW.world_id,NEW.principal,NEW.role) IS DISTINCT FROM (OLD.world_id,OLD.principal,OLD.role) THEN NEW.authority_id=gen_random_uuid();
 ELSE NEW.authority_id=OLD.authority_id; END IF;
 RETURN NEW;
END $$;
CREATE OR REPLACE FUNCTION worldbuilder.guard_review() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.reviewer=(SELECT author FROM worldbuilder.change_set WHERE world_id=NEW.world_id AND id=NEW.change_id) THEN RAISE EXCEPTION 'review must be independent of author'; END IF;
 RETURN NEW;
END $$;
CREATE OR REPLACE FUNCTION worldbuilder.guard_change_content() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (NEW.world_id,NEW.id,NEW.author,NEW.base_revision,NEW.reason,NEW.operations,NEW.required_approvals,NEW.created_at)
   IS DISTINCT FROM (OLD.world_id,OLD.id,OLD.author,OLD.base_revision,OLD.reason,OLD.operations,OLD.required_approvals,OLD.created_at)
   OR (OLD.status='published' AND NEW IS DISTINCT FROM OLD) THEN RAISE EXCEPTION 'proposal content and published decision are immutable'; END IF;
 RETURN NEW;
END $$;
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM pg_trigger WHERE tgrelid='worldbuilder.change_review'::regclass AND tgname='worldbuilder_review_immutable') THEN
  CREATE TRIGGER worldbuilder_review_immutable BEFORE UPDATE OR DELETE ON worldbuilder.change_review FOR EACH ROW EXECUTE FUNCTION worldbuilder.immutable();
  CREATE TRIGGER worldbuilder_review_independent BEFORE INSERT ON worldbuilder.change_review FOR EACH ROW EXECUTE FUNCTION worldbuilder.guard_review();
  CREATE TRIGGER worldbuilder_member_authority BEFORE UPDATE ON worldbuilder.member FOR EACH ROW EXECUTE FUNCTION worldbuilder.rotate_review_authority();
  CREATE TRIGGER worldbuilder_change_content BEFORE UPDATE ON worldbuilder.change_set FOR EACH ROW EXECUTE FUNCTION worldbuilder.guard_change_content();
  ALTER TABLE worldbuilder.record ADD CONSTRAINT record_review_policy_check CHECK(review_policy IS NULL OR (kind='world_settings' AND jsonb_typeof(review_policy)='object'));
 END IF;
END $$;
INSERT INTO worldbuilder.schema_version(version) VALUES(7) ON CONFLICT DO NOTHING;
COMMIT;
