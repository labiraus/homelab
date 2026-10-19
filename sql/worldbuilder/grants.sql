-- Provision these NOINHERIT login roles separately with operator-managed secrets.
-- Run as migration owner, never as the shared legacy app user.
BEGIN;
REVOKE ALL ON SCHEMA worldbuilder FROM PUBLIC;
GRANT USAGE ON SCHEMA worldbuilder TO worldbuilder_external, worldbuilder_relay;
GRANT SELECT, INSERT, UPDATE, DELETE ON worldbuilder.world, worldbuilder.member, worldbuilder.record, worldbuilder.change_set, worldbuilder.story_session TO worldbuilder_external;
GRANT SELECT, INSERT ON worldbuilder.revision, worldbuilder.audit, worldbuilder.story_turn, worldbuilder.story_outcome, worldbuilder.outbox TO worldbuilder_external;
GRANT SELECT ON worldbuilder.schema_version TO worldbuilder_external;
GRANT SELECT ON worldbuilder.superuser TO worldbuilder_external;
GRANT EXECUTE ON FUNCTION worldbuilder.lock_superuser(text) TO worldbuilder_external;
GRANT SELECT, INSERT ON worldbuilder.account_audit TO worldbuilder_external;
GRANT SELECT, INSERT ON worldbuilder.record_identity TO worldbuilder_external;
GRANT SELECT, INSERT ON worldbuilder.change_review TO worldbuilder_external;
GRANT SELECT, INSERT ON worldbuilder.story_definition TO worldbuilder_external;
GRANT SELECT, INSERT ON worldbuilder.inference_attempt TO worldbuilder_external;
GRANT UPDATE(status,model,usage,latency_ms,finished_at) ON worldbuilder.inference_attempt TO worldbuilder_external;
GRANT SELECT, INSERT ON worldbuilder.generation_job,worldbuilder.generation_outbox TO worldbuilder_external;
GRANT UPDATE(status,step,lease_token,lease_until,results,error_code,updated_at) ON worldbuilder.generation_job TO worldbuilder_external;
GRANT SELECT ON worldbuilder.generation_outbox,worldbuilder.schema_version TO worldbuilder_relay;
GRANT UPDATE(delivered_at) ON worldbuilder.generation_outbox TO worldbuilder_relay;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA worldbuilder TO worldbuilder_external;
GRANT SELECT ON worldbuilder.outbox, worldbuilder.revision TO worldbuilder_relay;
GRANT UPDATE(delivered_at) ON worldbuilder.outbox TO worldbuilder_relay;
GRANT SELECT ON worldbuilder.established_account TO worldbuilder_external;
GRANT SELECT, INSERT ON worldbuilder.conversation, worldbuilder.conversation_message TO worldbuilder_external;
GRANT UPDATE(status,result) ON worldbuilder.conversation_message TO worldbuilder_external;
GRANT EXECUTE ON FUNCTION worldbuilder.set_administrator(text,text,boolean,text) TO worldbuilder_external;
COMMIT;
