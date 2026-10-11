\set ON_ERROR_STOP on
-- Operator-only aggregate diagnostics. No prompts, output, email or world names.
BEGIN READ ONLY;
SET LOCAL statement_timeout = '5s';
\echo 'Schema and canonical publication backlog'
SELECT max(version) AS schema_version FROM worldbuilder.schema_version;
SELECT count(*) AS pending, min(created_at) AS oldest_pending FROM worldbuilder.outbox WHERE delivered_at IS NULL;
\echo 'Generation dispatch backlog and state'
SELECT count(*) AS pending, min(created_at) AS oldest_pending FROM worldbuilder.generation_outbox WHERE delivered_at IS NULL;
SELECT status,count(*) AS jobs,sum(step) AS completed_parts,min(created_at) AS oldest FROM worldbuilder.generation_job GROUP BY status ORDER BY status;
SELECT count(*) AS expired_running FROM worldbuilder.generation_job WHERE status='running' AND lease_until<now();
\echo 'Inference attempts and allocated allowance in last 24 hours (not invoices)'
SELECT workflow,status,currency,count(*) AS attempts,sum(allocated_micros) AS allocated_micros,round(avg(latency_ms)) AS mean_latency_ms FROM worldbuilder.inference_attempt WHERE created_at>=now()-interval '24 hours' GROUP BY workflow,status,currency ORDER BY workflow,status,currency;
SELECT count(*) AS expired_reservations FROM worldbuilder.inference_attempt WHERE status='reserved' AND lease_until<now();
\echo 'Governance and story counts'
SELECT status,count(*) AS changes FROM worldbuilder.change_set GROUP BY status ORDER BY status;
SELECT count(*) AS sessions,coalesce(sum(version),0) AS saved_turn_versions FROM worldbuilder.story_session;
SELECT action,count(*) AS actions FROM worldbuilder.audit WHERE created_at>=now()-interval '24 hours' GROUP BY action ORDER BY action;
COMMIT;
