-- Run explicitly with psql -X -v ON_ERROR_STOP=1. Never run on application startup.
BEGIN;
SELECT pg_advisory_xact_lock(hashtext('worldbuilder-migrations'));
CREATE SCHEMA IF NOT EXISTS worldbuilder;
REVOKE ALL ON SCHEMA worldbuilder FROM PUBLIC;
CREATE TABLE IF NOT EXISTS worldbuilder.schema_version(version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS worldbuilder.world (
 id uuid PRIMARY KEY, name text NOT NULL CHECK(length(name) BETWEEN 1 AND 200),
 revision bigint NOT NULL DEFAULT 0 CHECK(revision >= 0), created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS worldbuilder.member (
 world_id uuid NOT NULL REFERENCES worldbuilder.world(id), principal text NOT NULL,
 role text NOT NULL CHECK(role IN ('owner','admin','editor','reviewer','reader')),
 PRIMARY KEY(world_id,principal)
);
CREATE TABLE IF NOT EXISTS worldbuilder.revision (
 world_id uuid NOT NULL REFERENCES worldbuilder.world(id), revision bigint NOT NULL CHECK(revision >= 0),
 snapshot jsonb NOT NULL, actor text NOT NULL, reason text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(world_id,revision)
);
-- Structured current projection. JSONB is limited to typed value, schema and presentation extensions.
CREATE TABLE IF NOT EXISTS worldbuilder.record (
 world_id uuid NOT NULL REFERENCES worldbuilder.world(id), id text NOT NULL,
 kind text NOT NULL CHECK(kind IN ('type','timeline','entity','fact','relationship','event','rule','source')),
 label text NOT NULL DEFAULT '', type_id text, subject_id text, target_id text, timeline_id text, source_id text,
 predicate text NOT NULL DEFAULT '', value_type text NOT NULL DEFAULT '', value jsonb,
 valid_from bigint, valid_until bigint, attributes jsonb NOT NULL DEFAULT '{}',
 fields jsonb NOT NULL DEFAULT '{}', relations jsonb NOT NULL DEFAULT '{}',
 PRIMARY KEY(world_id,id),
 FOREIGN KEY(world_id,type_id) REFERENCES worldbuilder.record(world_id,id) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(world_id,subject_id) REFERENCES worldbuilder.record(world_id,id) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(world_id,target_id) REFERENCES worldbuilder.record(world_id,id) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(world_id,timeline_id) REFERENCES worldbuilder.record(world_id,id) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(world_id,source_id) REFERENCES worldbuilder.record(world_id,id) DEFERRABLE INITIALLY DEFERRED,
 CHECK(valid_until IS NULL OR valid_from IS NULL OR valid_until > valid_from),
 CHECK(kind NOT IN ('fact','relationship','event') OR timeline_id IS NOT NULL),
 CHECK(kind <> 'entity' OR type_id IS NOT NULL),
 CHECK(kind <> 'fact' OR (subject_id IS NOT NULL AND source_id IS NOT NULL AND predicate <> '' AND value IS NOT NULL AND value_type IN ('string','number','boolean','entity'))),
 CHECK(kind <> 'relationship' OR (subject_id IS NOT NULL AND target_id IS NOT NULL AND predicate <> '' AND source_id IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS record_scope ON worldbuilder.record(world_id,kind,timeline_id,subject_id);
CREATE TABLE IF NOT EXISTS worldbuilder.change_set (
 world_id uuid NOT NULL REFERENCES worldbuilder.world(id), id uuid NOT NULL,
 author text NOT NULL, base_revision bigint NOT NULL, reason text NOT NULL,
 operations jsonb NOT NULL, status text NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','ready_for_review','approved','rejected','published')),
 reviewer text, review_reason text, published_revision bigint,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(world_id,id),
 FOREIGN KEY(world_id,base_revision) REFERENCES worldbuilder.revision(world_id,revision),
 FOREIGN KEY(world_id,published_revision) REFERENCES worldbuilder.revision(world_id,revision),
 CHECK(reviewer IS NULL OR reviewer <> author),
 CHECK(status NOT IN ('approved','published') OR reviewer IS NOT NULL)
);
CREATE TABLE IF NOT EXISTS worldbuilder.audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, world_id uuid NOT NULL REFERENCES worldbuilder.world(id),
 actor text NOT NULL, action text NOT NULL, resource text NOT NULL, detail jsonb NOT NULL DEFAULT '{}',
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS worldbuilder.outbox (
 id uuid PRIMARY KEY, world_id uuid NOT NULL, revision bigint NOT NULL,
 subject text NOT NULL DEFAULT 'worldbuilder.canon.published', payload jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), delivered_at timestamptz,
 FOREIGN KEY(world_id,revision) REFERENCES worldbuilder.revision(world_id,revision), UNIQUE(world_id,revision)
);
CREATE INDEX IF NOT EXISTS outbox_pending ON worldbuilder.outbox(created_at) WHERE delivered_at IS NULL;
CREATE TABLE IF NOT EXISTS worldbuilder.story_session (
 world_id uuid NOT NULL, id uuid NOT NULL, owner text NOT NULL, title text NOT NULL,
 pinned_revision bigint NOT NULL, timeline_id text NOT NULL, version bigint NOT NULL DEFAULT 0,
 state jsonb NOT NULL DEFAULT '{}', created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(world_id,id), FOREIGN KEY(world_id,pinned_revision) REFERENCES worldbuilder.revision(world_id,revision)
);
CREATE TABLE IF NOT EXISTS worldbuilder.story_turn (
 world_id uuid NOT NULL, session_id uuid NOT NULL, version bigint NOT NULL,
 request_id uuid NOT NULL, request jsonb NOT NULL, narration text NOT NULL,
 state jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(world_id,session_id,version), UNIQUE(world_id,session_id,request_id),
 FOREIGN KEY(world_id,session_id) REFERENCES worldbuilder.story_session(world_id,id)
);
CREATE TABLE IF NOT EXISTS worldbuilder.story_outcome (
 world_id uuid NOT NULL, session_id uuid NOT NULL, request_id uuid NOT NULL, change_id uuid NOT NULL,
 request jsonb NOT NULL, PRIMARY KEY(world_id,session_id,request_id),
 FOREIGN KEY(world_id,session_id) REFERENCES worldbuilder.story_session(world_id,id),
 FOREIGN KEY(world_id,change_id) REFERENCES worldbuilder.change_set(world_id,id)
);
-- History cannot be rewritten even by accidental runtime UPDATE/DELETE statements.
CREATE OR REPLACE FUNCTION worldbuilder.immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'worldbuilder history is immutable'; END $$;
DO $$ BEGIN
 IF NOT EXISTS(SELECT FROM pg_trigger WHERE tgname='worldbuilder_revision_immutable') THEN
  CREATE TRIGGER worldbuilder_revision_immutable BEFORE UPDATE OR DELETE ON worldbuilder.revision FOR EACH ROW EXECUTE FUNCTION worldbuilder.immutable();
  CREATE TRIGGER worldbuilder_audit_immutable BEFORE UPDATE OR DELETE ON worldbuilder.audit FOR EACH ROW EXECUTE FUNCTION worldbuilder.immutable();
  CREATE TRIGGER worldbuilder_turn_immutable BEFORE UPDATE OR DELETE ON worldbuilder.story_turn FOR EACH ROW EXECUTE FUNCTION worldbuilder.immutable();
 END IF;
END $$;
INSERT INTO worldbuilder.schema_version(version) VALUES(1) ON CONFLICT DO NOTHING;
COMMIT;
