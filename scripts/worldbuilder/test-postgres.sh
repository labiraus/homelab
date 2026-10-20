#!/usr/bin/env bash
# Disposable Docker only. Never loads operator .env files or cluster credentials.
set -euo pipefail
repo_dir=$(cd "$(dirname "$0")/../.." && pwd)
container_name="worldbuilder-test-${RANDOM}-$$"
export WORLDBUILDER_DOCKER_CLI=$(command -v docker)
export WORLDBUILDER_POSTGRES_CONTAINER="$container_name"
# The streaming database bridge still uses the original CLI. Finite fixture
# commands require authoritative completion even when attach returns early.
docker() { python3 "$repo_dir/scripts/worldbuilder/docker-exec.py" "$@"; }
docker run -d --name "$container_name" --label "homelab.worldbuilder.postgres-fixture=$container_name" -e POSTGRES_PASSWORD=offline-test-only -e POSTGRES_DB=worldbuilder_test -p "127.0.0.1:${WORLDBUILDER_TEST_PORT:-}:5432" postgres:16.13 >/dev/null
relay_pid=""
cleanup() {
  if [[ -n "$relay_pid" ]]; then kill "$relay_pid" 2>/dev/null || true; fi
  docker rm -f "$container_name" >/dev/null
}
trap cleanup EXIT
for attempt in {1..30}; do
  # The image first starts a socket-only server for initdb, then shuts it down.
  # Wait for the final TCP server so migrations cannot race that handover.
  if docker exec "$container_name" pg_isready -h 127.0.0.1 -U postgres -d worldbuilder_test >/dev/null 2>&1; then break; fi
  sleep 1
done
sql() { docker exec -i "$container_name" psql -X -U postgres -d worldbuilder_test -v ON_ERROR_STOP=1 "$@"; }
sql < "$repo_dir/sql/worldbuilder/001_up.sql"
sql < "$repo_dir/sql/worldbuilder/001_up.sql"
sql < "$repo_dir/sql/worldbuilder/002_up.sql"
sql < "$repo_dir/sql/worldbuilder/002_up.sql"
sql < "$repo_dir/sql/worldbuilder/003_up.sql"
sql < "$repo_dir/sql/worldbuilder/003_up.sql"
sql < "$repo_dir/sql/worldbuilder/004_up.sql"
sql < "$repo_dir/sql/worldbuilder/004_up.sql"
sql < "$repo_dir/sql/worldbuilder/005_up.sql"
sql < "$repo_dir/sql/worldbuilder/005_up.sql"
sql < "$repo_dir/sql/worldbuilder/006_up.sql"
sql < "$repo_dir/sql/worldbuilder/006_up.sql"
sql < "$repo_dir/sql/worldbuilder/007_up.sql"
sql < "$repo_dir/sql/worldbuilder/007_up.sql"
sql < "$repo_dir/sql/worldbuilder/008_up.sql"
sql < "$repo_dir/sql/worldbuilder/008_up.sql"
sql < "$repo_dir/sql/worldbuilder/009_up.sql"
sql < "$repo_dir/sql/worldbuilder/009_up.sql"
sql < "$repo_dir/sql/worldbuilder/010_up.sql"
sql < "$repo_dir/sql/worldbuilder/010_up.sql"
sql < "$repo_dir/sql/worldbuilder/010_down.sql"
sql < "$repo_dir/sql/worldbuilder/009_down.sql"
sql < "$repo_dir/sql/worldbuilder/008_down.sql"
sql < "$repo_dir/sql/worldbuilder/007_down.sql"
sql < "$repo_dir/sql/worldbuilder/006_down.sql"
sql < "$repo_dir/sql/worldbuilder/005_down.sql"
sql < "$repo_dir/sql/worldbuilder/004_down.sql"
sql < "$repo_dir/sql/worldbuilder/003_down.sql"
sql < "$repo_dir/sql/worldbuilder/002_down.sql"
sql < "$repo_dir/sql/worldbuilder/001_down.sql"
sql < "$repo_dir/sql/worldbuilder/001_up.sql"
sql < "$repo_dir/sql/worldbuilder/002_up.sql"
sql < "$repo_dir/sql/worldbuilder/003_up.sql"
sql < "$repo_dir/sql/worldbuilder/004_up.sql"
sql < "$repo_dir/sql/worldbuilder/005_up.sql"
sql < "$repo_dir/sql/worldbuilder/006_up.sql"
sql < "$repo_dir/sql/worldbuilder/007_up.sql"
sql < "$repo_dir/sql/worldbuilder/008_up.sql"
sql < "$repo_dir/sql/worldbuilder/009_up.sql"
sql < "$repo_dir/sql/worldbuilder/009_up.sql"
sql < "$repo_dir/sql/worldbuilder/010_up.sql"
sql < "$repo_dir/sql/worldbuilder/010_up.sql"
sql -c 'CREATE ROLE worldbuilder_external NOINHERIT LOGIN; CREATE ROLE worldbuilder_relay NOINHERIT LOGIN; CREATE ROLE legacy_app NOINHERIT LOGIN;'
sql < "$repo_dir/sql/worldbuilder/grants.sql"
sql < "$repo_dir/sql/worldbuilder/auth-grants.sql"
# Rehearse the physical-backup seed against the actual latest schema before any
# expensive controller drill. A new table must have data, not a vacuous checksum.
docker exec "$container_name" createdb -U postgres -T worldbuilder_test worldbuilder_backup_seed
backup_sql() { docker exec -i "$container_name" psql -X -U postgres -d worldbuilder_backup_seed -v ON_ERROR_STOP=1 "$@"; }
backup_sql < "$repo_dir/scripts/worldbuilder/backup-seed.sql"
backup_sql <<'SQL'
DO $$ DECLARE t record; n bigint; BEGIN
 FOR t IN SELECT tablename FROM pg_tables WHERE schemaname='worldbuilder' LOOP
  EXECUTE format('SELECT count(*) FROM worldbuilder.%I',t.tablename) INTO n;
  IF n=0 THEN RAISE EXCEPTION 'physical backup seed leaves table % empty', t.tablename; END IF;
 END LOOP;
 IF NOT EXISTS(SELECT FROM auth.users) THEN RAISE EXCEPTION 'physical backup seed leaves registry empty'; END IF;
END $$;
SQL
echo 'PASS: physical backup seed populates every current canon table and identity registry'
# Upgrade a populated version-1 database independently of the empty up/down tests.
docker exec "$container_name" createdb -U postgres worldbuilder_upgrade
upgrade_sql() { docker exec -i "$container_name" psql -X -U postgres -d worldbuilder_upgrade -v ON_ERROR_STOP=1 "$@"; }
upgrade_sql < "$repo_dir/sql/worldbuilder/001_up.sql"
upgrade_sql <<'SQL'
INSERT INTO worldbuilder.world(id,name,revision) VALUES('00000000-0000-4000-8000-000000000001','Legacy world',1);
INSERT INTO worldbuilder.member VALUES('00000000-0000-4000-8000-000000000001','owner@example.test','owner');
INSERT INTO worldbuilder.revision(world_id,revision,snapshot,actor,reason) VALUES
 ('00000000-0000-4000-8000-000000000001',0,'{"main":{"id":"main","kind":"timeline","label":"Main"},"deleted":{"id":"deleted","kind":"source","label":"Old source"}}','owner@example.test','Created'),
 ('00000000-0000-4000-8000-000000000001',1,'{"main":{"id":"main","kind":"timeline","label":"Main"}}','owner@example.test','Deleted source');
INSERT INTO worldbuilder.story_session(world_id,id,owner,title,pinned_revision,timeline_id,version) VALUES('00000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000010','owner@example.test','Legacy story',1,'main',1);
INSERT INTO worldbuilder.story_turn(world_id,session_id,version,request_id,request,narration,state) VALUES('00000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000010',1,'00000000-0000-4000-8000-000000000011','{}','Legacy narration','{}');
SQL
upgrade_sql < "$repo_dir/sql/worldbuilder/002_up.sql"
upgrade_sql < "$repo_dir/sql/worldbuilder/002_up.sql"
upgrade_sql < "$repo_dir/sql/worldbuilder/003_up.sql"
upgrade_sql < "$repo_dir/sql/worldbuilder/003_up.sql"
upgrade_sql < "$repo_dir/sql/worldbuilder/004_up.sql"
upgrade_sql < "$repo_dir/sql/worldbuilder/004_up.sql"
upgrade_sql < "$repo_dir/sql/worldbuilder/005_up.sql"
upgrade_sql < "$repo_dir/sql/worldbuilder/005_up.sql"
upgrade_sql <<'SQL'
BEGIN;
INSERT INTO worldbuilder.world(id,name) VALUES('00000000-0000-4000-8000-000000000020','Numeric dates');
INSERT INTO worldbuilder.revision(world_id,revision,snapshot,actor,reason) VALUES
 ('00000000-0000-4000-8000-000000000020',0,'{"main":{"id":"main","kind":"timeline","label":"Main"},"person":{"id":"person","kind":"type","fields":{"birth":"number"}},"hero":{"id":"hero","kind":"entity","type":"person","label":"Hero"},"source":{"id":"source","kind":"source","label":"Notes"},"birth":{"id":"birth","kind":"fact","subject":"hero","predicate":"birth","valueType":"number","value":-9007199254740991,"timeline":"main","source":"source"}}','owner@example.test','Legacy numeric date');
INSERT INTO worldbuilder.record(world_id,id,kind,label,type_id,subject_id,source_id,timeline_id,predicate,value_type,value,fields)
 SELECT r.world_id,e.key,e.value->>'kind',coalesce(e.value->>'label',''),e.value->>'type',e.value->>'subject',e.value->>'source',e.value->>'timeline',coalesce(e.value->>'predicate',''),coalesce(e.value->>'valueType',''),e.value->'value',coalesce(e.value->'fields','{}')
 FROM worldbuilder.revision r CROSS JOIN LATERAL jsonb_each(r.snapshot) e WHERE r.world_id='00000000-0000-4000-8000-000000000020';
COMMIT;
SQL
calendar_upgrade_checksum="SELECT md5(string_agg(item, ',' ORDER BY item)) FROM (SELECT to_jsonb(r)::text AS item FROM worldbuilder.revision r UNION ALL SELECT (to_jsonb(r)-'calendar')::text FROM worldbuilder.record r) data"
calendar_before=$(upgrade_sql -Atc "$calendar_upgrade_checksum")
# Additional value constraints are schema drift, not permission to drop them.
docker exec "$container_name" createdb -U postgres -T worldbuilder_upgrade worldbuilder_calendar_drift
calendar_drift_sql() { docker exec -i "$container_name" psql -X -U postgres -d worldbuilder_calendar_drift -v ON_ERROR_STOP=1 "$@"; }
calendar_drift_sql -c "ALTER TABLE worldbuilder.record ADD CONSTRAINT custom_value_policy CHECK(length(value_type)<20);"
if calendar_drift_sql < "$repo_dir/sql/worldbuilder/006_up.sql" >/dev/null 2>&1; then echo 'FAIL: unexpected calendar schema drift accepted'; exit 1; fi
calendar_drift_sql -c "DO \$\$ BEGIN IF (SELECT max(version) FROM worldbuilder.schema_version)<>5 OR EXISTS(SELECT FROM information_schema.columns WHERE table_schema='worldbuilder' AND table_name='record' AND column_name='calendar') THEN RAISE EXCEPTION 'failed calendar migration changed schema'; END IF; END \$\$;"
upgrade_sql < "$repo_dir/sql/worldbuilder/006_up.sql"
upgrade_sql < "$repo_dir/sql/worldbuilder/006_up.sql"
[[ "$calendar_before" == "$(upgrade_sql -Atc "$calendar_upgrade_checksum")" ]] || { echo 'FAIL: calendar migration changed legacy records/history'; exit 1; }
upgrade_sql <<'SQL'
INSERT INTO worldbuilder.member(world_id,principal,role) VALUES('00000000-0000-4000-8000-000000000020','reviewer@example.test','reviewer');
INSERT INTO worldbuilder.change_set(world_id,id,author,base_revision,reason,operations,status,reviewer,review_reason,published_revision) VALUES
 ('00000000-0000-4000-8000-000000000020','00000000-0000-4000-8000-000000000021','author@example.test',0,'Legacy approved','[]','approved','reviewer@example.test','Reviewed',NULL),
 ('00000000-0000-4000-8000-000000000020','00000000-0000-4000-8000-000000000022','author@example.test',0,'Legacy published','[]','published','removed@example.test','Reviewed',0),
 ('00000000-0000-4000-8000-000000000020','00000000-0000-4000-8000-000000000023','author@example.test',0,'Legacy rejected','[]','rejected','reviewer@example.test','Rejected',NULL);
INSERT INTO worldbuilder.audit(world_id,actor,action,resource,detail,created_at) VALUES
 ('00000000-0000-4000-8000-000000000020','reviewer@example.test','change.review','00000000-0000-4000-8000-000000000021','{"decision":"approved","reason":"Reviewed"}','2026-01-01T00:00:00Z');
SQL
review_upgrade_checksum="SELECT md5(string_agg(item, ',' ORDER BY item)) FROM (SELECT to_jsonb(r)::text AS item FROM worldbuilder.revision r UNION ALL SELECT (to_jsonb(r)-'required_approvals'-'published_approvals')::text FROM worldbuilder.change_set r UNION ALL SELECT to_jsonb(r)::text FROM worldbuilder.audit r) data"
review_before=$(upgrade_sql -Atc "$review_upgrade_checksum")
upgrade_sql < "$repo_dir/sql/worldbuilder/007_up.sql"
upgrade_sql < "$repo_dir/sql/worldbuilder/007_up.sql"
upgrade_sql < "$repo_dir/sql/worldbuilder/008_up.sql"
upgrade_sql < "$repo_dir/sql/worldbuilder/008_up.sql"
[[ "$review_before" == "$(upgrade_sql -Atc "$review_upgrade_checksum")" ]] || { echo 'FAIL: review migration changed existing history/decisions'; exit 1; }
upgrade_sql <<'SQL'
DO $$ BEGIN
 IF (SELECT count(*) FROM worldbuilder.change_review)<>3 THEN RAISE EXCEPTION 'legacy reviews lost or duplicated'; END IF;
 IF (SELECT created_at FROM worldbuilder.change_review WHERE change_id='00000000-0000-4000-8000-000000000021') IS DISTINCT FROM '2026-01-01T00:00:00Z'::timestamptz THEN RAISE EXCEPTION 'known review timestamp changed'; END IF;
 IF (SELECT created_at FROM worldbuilder.change_review WHERE change_id='00000000-0000-4000-8000-000000000022') IS NOT NULL THEN RAISE EXCEPTION 'legacy review time fabricated'; END IF;
 IF (SELECT published_approvals FROM worldbuilder.change_set WHERE id='00000000-0000-4000-8000-000000000022') IS DISTINCT FROM ARRAY['removed@example.test'] THEN RAISE EXCEPTION 'published approval lost'; END IF;
 IF EXISTS(SELECT FROM worldbuilder.change_set WHERE required_approvals<>1) THEN RAISE EXCEPTION 'legacy policy changed'; END IF;
END $$;
SQL
upgrade_sql <<'SQL'
DO $$ BEGIN
 IF (SELECT kind FROM worldbuilder.record_identity WHERE id='deleted') IS DISTINCT FROM 'source' THEN RAISE EXCEPTION 'deleted identity missing after upgrade'; END IF;
 IF (SELECT count(*) FROM worldbuilder.revision)<>3 THEN RAISE EXCEPTION 'upgrade changed history'; END IF;
 IF (SELECT memory->>'throughVersion' FROM worldbuilder.story_session) IS DISTINCT FROM '0' OR (SELECT narration FROM worldbuilder.story_turn) IS DISTINCT FROM 'Legacy narration' THEN RAISE EXCEPTION 'upgrade changed legacy story content'; END IF;
END $$;
SQL
# Collective-author upgrade preserves published decisions and invalidates old pending authority.
pwa_upgrade_checksum="SELECT md5(string_agg(item, ',' ORDER BY item)) FROM (SELECT to_jsonb(r)::text AS item FROM worldbuilder.revision r UNION ALL SELECT (to_jsonb(r)-'policy_version')::text FROM worldbuilder.change_set r UNION ALL SELECT to_jsonb(r)::text FROM worldbuilder.audit r) data"
pwa_before=$(upgrade_sql -Atc "$pwa_upgrade_checksum")
upgrade_sql < "$repo_dir/sql/worldbuilder/009_up.sql"
upgrade_sql < "$repo_dir/sql/worldbuilder/009_up.sql"
upgrade_sql < "$repo_dir/sql/worldbuilder/010_up.sql"
upgrade_sql < "$repo_dir/sql/worldbuilder/010_up.sql"
[[ "$pwa_before" == "$(upgrade_sql -Atc "$pwa_upgrade_checksum")" ]] || { echo 'FAIL: PWA upgrade changed historical canon/decisions'; exit 1; }
upgrade_sql <<'SQL'
DO $$ BEGIN
 IF EXISTS(SELECT FROM worldbuilder.member WHERE role NOT IN ('author','reader')) THEN RAISE EXCEPTION 'legacy role remains'; END IF;
 IF EXISTS(SELECT FROM worldbuilder.change_set WHERE policy_version<>0) THEN RAISE EXCEPTION 'old pending authority silently renewed'; END IF;
 IF (SELECT published_approvals FROM worldbuilder.change_set WHERE id='00000000-0000-4000-8000-000000000022') IS DISTINCT FROM ARRAY['removed@example.test'] THEN RAISE EXCEPTION 'historical publication lost'; END IF;
END $$;
SQL
if upgrade_sql < "$repo_dir/sql/worldbuilder/002_down.sql" >/dev/null 2>&1; then echo 'FAIL: populated upgrade rollback succeeded'; exit 1; fi
# Inconsistent legacy identity must fail before changing the schema.
docker exec "$container_name" createdb -U postgres worldbuilder_conflict
conflict_sql() { docker exec -i "$container_name" psql -X -U postgres -d worldbuilder_conflict -v ON_ERROR_STOP=1 "$@"; }
conflict_sql < "$repo_dir/sql/worldbuilder/001_up.sql"
conflict_sql <<'SQL'
INSERT INTO worldbuilder.world(id,name,revision) VALUES('00000000-0000-4000-8000-000000000002','Conflicting legacy world',1);
INSERT INTO worldbuilder.revision(world_id,revision,snapshot,actor,reason) VALUES
 ('00000000-0000-4000-8000-000000000002',0,'{"reused":{"id":"reused","kind":"source","label":"Source"}}','owner@example.test','Created'),
 ('00000000-0000-4000-8000-000000000002',1,'{"reused":{"id":"reused","kind":"timeline","label":"Timeline"}}','owner@example.test','Reused');
SQL
if conflict_sql < "$repo_dir/sql/worldbuilder/002_up.sql" >/dev/null 2>&1; then echo 'FAIL: conflicting historical identity migrated silently'; exit 1; fi
conflict_sql -c "DO \$\$ BEGIN IF (SELECT max(version) FROM worldbuilder.schema_version)<>1 THEN RAISE EXCEPTION 'failed migration advanced schema'; END IF; END \$\$;"
sql -c "SELECT has_schema_privilege('legacy_app','worldbuilder','USAGE') AS must_be_false;"
if sql -c 'SET ROLE legacy_app; SELECT * FROM worldbuilder.world;' >/dev/null 2>&1; then echo 'FAIL: legacy user can access canon'; exit 1; fi
if sql -c 'SET ROLE worldbuilder_relay; INSERT INTO worldbuilder.world VALUES(gen_random_uuid(), '\''bypass'\'', 0, now());' >/dev/null 2>&1; then echo 'FAIL: relay can mutate canon'; exit 1; fi
port=$(docker port "$container_name" 5432/tcp | awk -F: 'NR==1 {print $NF}')
test_host=""
# Container socket readiness can precede host forwarding (and the image's final TCP server).
if [[ "${WORLDBUILDER_TEST_TRANSPORT:-tcp}" != exec ]]; then
 for attempt in {1..15}; do
  for candidate_host in 127.0.0.1 host.docker.internal; do
    if PGCONNECT_TIMEOUT=2 PGPASSWORD=offline-test-only psql -h "$candidate_host" -p "$port" -U postgres -d worldbuilder_test -Atc 'SELECT 1' >/dev/null 2>&1; then
      test_host=$candidate_host
      break 2
    fi
  done
  sleep 1
 done
fi
if [[ -z "$test_host" ]] && command -v socat >/dev/null; then
  # Docker-outside-Docker may expose its API while host port forwarding is broken.
  # Relay only a local loopback socket through docker exec into our own container.
  relay_log=$(mktemp /tmp/worldbuilder-relay-XXXXXX.log)
  socat -d -d TCP-LISTEN:0,bind=127.0.0.1,reuseaddr,fork "EXEC:bash '$repo_dir/scripts/worldbuilder/postgres-exec-bridge.sh' '$container_name'" 2>"$relay_log" &
  relay_pid=$!
  for attempt in {1..20}; do
    relay_port=$(sed -nE 's/.*listening on.*127\.0\.0\.1:([0-9]+).*/\1/p' "$relay_log" | head -1)
    if [[ -n "$relay_port" ]] && PGCONNECT_TIMEOUT=2 PGPASSWORD=offline-test-only psql -h 127.0.0.1 -p "$relay_port" -U postgres -d worldbuilder_test -Atc 'SELECT 1' >/dev/null 2>&1; then
      test_host=127.0.0.1
      port=$relay_port
      echo "Using disposable PostgreSQL Docker-exec bridge on loopback port $port"
      break
    fi
    sleep 0.1
  done
fi
if [[ -z "$test_host" ]]; then
  echo "FAIL: disposable PostgreSQL host port is unreachable; try WORLDBUILDER_TEST_PORT=25433 in Docker-outside-Docker" >&2
  exit 1
fi
export WORLDBUILDER_TEST_DATABASE_URL="postgres://postgres:offline-test-only@${test_host}:${port}/worldbuilder_test?sslmode=disable&connect_timeout=5"
export GOCACHE="${GOCACHE:-/tmp/worldbuilder-go-cache}"
export GOMODCACHE="${GOMODCACHE:-/tmp/worldbuilder-go-mod}"
(cd "$repo_dir/apps/external" && go test -timeout=120s -race -count=1 ./worldbuilder)
(cd "$repo_dir/apps/orchestrator" && go test -timeout=120s -race -count=1 -run '^Test(Canon|Generation|DocumentReconciliation)' .)
# Native MCP OAuth uses an independent database and a non-owner identity. Never
# grant the MCP authorization service canon privileges or use a production DSN.
docker exec "$container_name" createdb -U postgres mcp_oauth_test
oauth_sql() { docker exec -i "$container_name" psql -X -U postgres -d mcp_oauth_test -v ON_ERROR_STOP=1 "$@"; }
oauth_sql <<'SQL'
CREATE SCHEMA auth;
CREATE TABLE auth.users(email text PRIMARY KEY, display_name text NOT NULL);
INSERT INTO auth.users VALUES ('owner@example.test','Fixture owner');
CREATE SCHEMA worldbuilder;
REVOKE ALL ON SCHEMA worldbuilder FROM PUBLIC;
CREATE ROLE mcp_oauth_runtime LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS PASSWORD 'offline-oauth-only';
SQL
oauth_sql < "$repo_dir/sql/mcp/001_up.sql"
oauth_sql < "$repo_dir/sql/mcp/001_up.sql"
oauth_sql < "$repo_dir/sql/mcp/grants.sql"
MCP_OAUTH_TEST_DATABASE_URL="postgres://mcp_oauth_runtime:offline-oauth-only@${test_host}:${port}/mcp_oauth_test?sslmode=disable&connect_timeout=5" \
MCP_OAUTH_TEST_ADMIN_DATABASE_URL="postgres://postgres:offline-test-only@${test_host}:${port}/mcp_oauth_test?sslmode=disable&connect_timeout=5" \
  bash -c 'cd "$1/apps/external" && go test -timeout=120s -race -count=1 ./mcpauth && go test -timeout=120s -race -count=1 -run TestMCPOAuthStartup .' _ "$repo_dir"
# Restored credential state can precede revocation. Preserve the source and
# invalidate only the independently restored OAuth database before exposing it.
docker exec "$container_name" createdb -U postgres mcp_oauth_restore
docker exec "$container_name" pg_dump -U postgres -d mcp_oauth_test -Fc -f /tmp/mcp-oauth.dump
docker exec "$container_name" pg_restore -U postgres -d mcp_oauth_restore --exit-on-error /tmp/mcp-oauth.dump
docker exec "$container_name" psql -X -U postgres -d mcp_oauth_restore -v ON_ERROR_STOP=1 -c "DO \$\$ BEGIN IF NOT EXISTS(SELECT 1 FROM mcp_oauth.token WHERE NOT revoked AND expires_at>now()) THEN RAISE EXCEPTION 'restore test has no valid source grant'; END IF; END \$\$;"
docker exec -i "$container_name" psql -X -U postgres -d mcp_oauth_restore -v ON_ERROR_STOP=1 < "$repo_dir/sql/mcp/revoke-after-restore.sql"
docker exec "$container_name" psql -X -U postgres -d mcp_oauth_restore -v ON_ERROR_STOP=1 -c "DO \$\$ BEGIN IF EXISTS(SELECT 1 FROM mcp_oauth.token WHERE NOT revoked) OR EXISTS(SELECT 1 FROM mcp_oauth.authorization) OR EXISTS(SELECT 1 FROM mcp_oauth.code) THEN RAISE EXCEPTION 'restored credential remains usable'; END IF; IF NOT EXISTS(SELECT 1 FROM mcp_oauth.client WHERE approved) THEN RAISE EXCEPTION 'approved client identity lost'; END IF; END \$\$;"
echo 'PASS: native MCP authorization database restore requires fresh consent and retains client identities'
if sql < "$repo_dir/sql/worldbuilder/001_down.sql" >/dev/null 2>&1; then echo 'FAIL: populated schema down migration succeeded'; exit 1; fi
if sql < "$repo_dir/sql/worldbuilder/002_down.sql" >/dev/null 2>&1; then echo 'FAIL: populated lifecycle down migration succeeded'; exit 1; fi
if sql < "$repo_dir/sql/worldbuilder/003_down.sql" >/dev/null 2>&1; then echo 'FAIL: populated inference down migration succeeded'; exit 1; fi
if sql < "$repo_dir/sql/worldbuilder/004_down.sql" >/dev/null 2>&1; then echo 'FAIL: populated story-definition down migration succeeded'; exit 1; fi
if sql < "$repo_dir/sql/worldbuilder/005_down.sql" >/dev/null 2>&1; then echo 'FAIL: populated generation down migration succeeded'; exit 1; fi
if sql < "$repo_dir/sql/worldbuilder/006_down.sql" >/dev/null 2>&1; then echo 'FAIL: populated calendar down migration succeeded'; exit 1; fi
if sql < "$repo_dir/sql/worldbuilder/007_down.sql" >/dev/null 2>&1; then echo 'FAIL: populated review-policy down migration succeeded'; exit 1; fi
if sql < "$repo_dir/sql/worldbuilder/008_down.sql" >/dev/null 2>&1; then echo 'FAIL: populated superuser down migration succeeded'; exit 1; fi
if sql < "$repo_dir/sql/worldbuilder/010_down.sql" >/dev/null 2>&1; then echo "FAIL: populated PWA downgrade succeeded"; exit 1; fi
sql < "$repo_dir/sql/worldbuilder/ops_dashboard_queries.pgsql"
docker exec "$container_name" createdb -U postgres worldbuilder_restore
docker exec "$container_name" pg_dump -U postgres -d worldbuilder_test -Fc -f /tmp/worldbuilder.dump
docker exec "$container_name" pg_restore -U postgres -d worldbuilder_restore --exit-on-error /tmp/worldbuilder.dump
checksum_parts=()
for canon_table in established_account conversation conversation_message world member revision record record_identity change_set change_review audit outbox story_session story_turn story_outcome story_definition generation_job generation_outbox inference_attempt schema_version superuser account_audit; do
  checksum_parts+=("SELECT '${canon_table}:' || row_to_json(t)::text AS item FROM worldbuilder.${canon_table} t")
done
checksum_parts+=("SELECT 'auth.users:' || md5(coalesce(string_agg(row_to_json(t)::text, E'\\n' ORDER BY row_to_json(t)::text), '')) AS item FROM auth.users t")
checksum_union=${checksum_parts[0]}
for part in "${checksum_parts[@]:1}"; do checksum_union+=" UNION ALL $part"; done
checksum_query="SELECT md5(string_agg(item, ',' ORDER BY item)) FROM ($checksum_union) rows"
original=$(sql -Atc "$checksum_query")
restored=$(docker exec "$container_name" psql -X -U postgres -d worldbuilder_restore -Atc "$checksum_query")
[[ "$original" == "$restored" ]] || { echo 'FAIL: restored canon/lifecycle/story data differs'; exit 1; }
echo 'PASS: migrations 001–010 rerun/rollback, populated upgrade/conflict gates, role isolation, canon/lifecycle/story/outbox integration and all-table restore checksum'
