BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
DO $$ BEGIN
 IF EXISTS(SELECT FROM worldbuilder.world) THEN RAISE EXCEPTION 'refusing populated review-policy rollback'; END IF;
END $$;
DROP TRIGGER IF EXISTS worldbuilder_member_authority ON worldbuilder.member;
DROP TRIGGER IF EXISTS worldbuilder_change_content ON worldbuilder.change_set;
DROP TABLE IF EXISTS worldbuilder.change_review;
DROP FUNCTION IF EXISTS worldbuilder.guard_review(),worldbuilder.rotate_review_authority(),worldbuilder.guard_change_content();
ALTER TABLE worldbuilder.member DROP COLUMN IF EXISTS authority_id;
ALTER TABLE worldbuilder.record DROP COLUMN IF EXISTS review_policy;
ALTER TABLE worldbuilder.change_set DROP COLUMN IF EXISTS required_approvals,DROP COLUMN IF EXISTS published_approvals;
DELETE FROM worldbuilder.schema_version WHERE version=7;
COMMIT;
