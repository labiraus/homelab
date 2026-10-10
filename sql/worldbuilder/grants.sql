-- Provision these NOINHERIT login roles separately with operator-managed secrets.
-- Run as migration owner, never as the shared legacy app user.
BEGIN;
REVOKE ALL ON SCHEMA worldbuilder FROM PUBLIC;
GRANT USAGE ON SCHEMA worldbuilder TO worldbuilder_external, worldbuilder_relay;
GRANT SELECT, INSERT, UPDATE, DELETE ON worldbuilder.world, worldbuilder.member, worldbuilder.record, worldbuilder.change_set, worldbuilder.story_session TO worldbuilder_external;
GRANT SELECT, INSERT ON worldbuilder.revision, worldbuilder.audit, worldbuilder.story_turn, worldbuilder.story_outcome, worldbuilder.outbox TO worldbuilder_external;
GRANT SELECT ON worldbuilder.schema_version TO worldbuilder_external;
GRANT SELECT, INSERT ON worldbuilder.record_identity TO worldbuilder_external;
GRANT SELECT, INSERT ON worldbuilder.story_definition TO worldbuilder_external;
GRANT SELECT, INSERT ON worldbuilder.inference_attempt TO worldbuilder_external;
GRANT UPDATE(status,model,usage,latency_ms,finished_at) ON worldbuilder.inference_attempt TO worldbuilder_external;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA worldbuilder TO worldbuilder_external;
GRANT SELECT ON worldbuilder.outbox, worldbuilder.revision TO worldbuilder_relay;
GRANT UPDATE(delivered_at) ON worldbuilder.outbox TO worldbuilder_relay;
COMMIT;
