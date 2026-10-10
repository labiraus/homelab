#!/usr/bin/env bash
# Disposable Docker only. Never loads operator .env files or cluster credentials.
set -euo pipefail
repo_dir=$(cd "$(dirname "$0")/../.." && pwd)
container_name="worldbuilder-test-${RANDOM}-$$"
docker run -d --name "$container_name" -e POSTGRES_PASSWORD=offline-test-only -e POSTGRES_DB=worldbuilder_test -p "127.0.0.1:${WORLDBUILDER_TEST_PORT:-}:5432" postgres:16.13 >/dev/null
relay_pid=""
cleanup() {
  if [[ -n "$relay_pid" ]]; then kill "$relay_pid" 2>/dev/null || true; fi
  docker rm -f "$container_name" >/dev/null
}
trap cleanup EXIT
for attempt in {1..30}; do
  if docker exec "$container_name" pg_isready -U postgres -d worldbuilder_test >/dev/null 2>&1; then break; fi
  sleep 1
done
sql() { docker exec -i "$container_name" psql -X -U postgres -d worldbuilder_test -v ON_ERROR_STOP=1 "$@"; }
sql < "$repo_dir/sql/worldbuilder/001_up.sql"
sql < "$repo_dir/sql/worldbuilder/001_up.sql"
sql < "$repo_dir/sql/worldbuilder/002_up.sql"
sql < "$repo_dir/sql/worldbuilder/002_up.sql"
sql < "$repo_dir/sql/worldbuilder/003_up.sql"
sql < "$repo_dir/sql/worldbuilder/003_up.sql"
sql < "$repo_dir/sql/worldbuilder/003_down.sql"
sql < "$repo_dir/sql/worldbuilder/002_down.sql"
sql < "$repo_dir/sql/worldbuilder/001_down.sql"
sql < "$repo_dir/sql/worldbuilder/001_up.sql"
sql < "$repo_dir/sql/worldbuilder/002_up.sql"
sql < "$repo_dir/sql/worldbuilder/003_up.sql"
sql -c 'CREATE ROLE worldbuilder_external NOINHERIT LOGIN; CREATE ROLE worldbuilder_relay NOINHERIT LOGIN; CREATE ROLE legacy_app NOINHERIT LOGIN;'
sql < "$repo_dir/sql/worldbuilder/grants.sql"
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
SQL
upgrade_sql < "$repo_dir/sql/worldbuilder/002_up.sql"
upgrade_sql < "$repo_dir/sql/worldbuilder/002_up.sql"
upgrade_sql < "$repo_dir/sql/worldbuilder/003_up.sql"
upgrade_sql < "$repo_dir/sql/worldbuilder/003_up.sql"
upgrade_sql <<'SQL'
DO $$ BEGIN
 IF (SELECT kind FROM worldbuilder.record_identity WHERE id='deleted') IS DISTINCT FROM 'source' THEN RAISE EXCEPTION 'deleted identity missing after upgrade'; END IF;
 IF (SELECT count(*) FROM worldbuilder.revision)<>2 THEN RAISE EXCEPTION 'upgrade changed history'; END IF;
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
(cd "$repo_dir/apps/orchestrator" && go test -timeout=120s -race -count=1 -run '^TestCanon' .)
if sql < "$repo_dir/sql/worldbuilder/001_down.sql" >/dev/null 2>&1; then echo 'FAIL: populated schema down migration succeeded'; exit 1; fi
if sql < "$repo_dir/sql/worldbuilder/002_down.sql" >/dev/null 2>&1; then echo 'FAIL: populated lifecycle down migration succeeded'; exit 1; fi
if sql < "$repo_dir/sql/worldbuilder/003_down.sql" >/dev/null 2>&1; then echo 'FAIL: populated inference down migration succeeded'; exit 1; fi
docker exec "$container_name" createdb -U postgres worldbuilder_restore
docker exec "$container_name" pg_dump -U postgres -d worldbuilder_test -Fc -f /tmp/worldbuilder.dump
docker exec "$container_name" pg_restore -U postgres -d worldbuilder_restore --exit-on-error /tmp/worldbuilder.dump
checksum_parts=()
for canon_table in world member revision record record_identity change_set audit outbox story_session story_turn story_outcome inference_attempt schema_version; do
  checksum_parts+=("SELECT '${canon_table}:' || row_to_json(t)::text AS item FROM worldbuilder.${canon_table} t")
done
checksum_union=${checksum_parts[0]}
for part in "${checksum_parts[@]:1}"; do checksum_union+=" UNION ALL $part"; done
checksum_query="SELECT md5(string_agg(item, ',' ORDER BY item)) FROM ($checksum_union) rows"
original=$(sql -Atc "$checksum_query")
restored=$(docker exec "$container_name" psql -X -U postgres -d worldbuilder_restore -Atc "$checksum_query")
[[ "$original" == "$restored" ]] || { echo 'FAIL: restored canon/lifecycle/story data differs'; exit 1; }
echo 'PASS: migrations 001/002/003 rerun/rollback, populated upgrade/conflict gates, role isolation, canon/lifecycle/story/outbox integration and all-table restore checksum'
