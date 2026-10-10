#!/usr/bin/env bash
# Disposable Docker only. Never loads operator .env files or cluster credentials.
set -euo pipefail
repo_dir=$(cd "$(dirname "$0")/../.." && pwd)
container_name="worldbuilder-test-${RANDOM}-$$"
docker run -d --name "$container_name" -e POSTGRES_PASSWORD=offline-test-only -e POSTGRES_DB=worldbuilder_test -p "127.0.0.1:${WORLDBUILDER_TEST_PORT:-}:5432" postgres:16.13 >/dev/null
trap 'docker rm -f "$container_name" >/dev/null' EXIT
for attempt in {1..30}; do
  if docker exec "$container_name" pg_isready -U postgres -d worldbuilder_test >/dev/null 2>&1; then break; fi
  sleep 1
done
sql() { docker exec -i "$container_name" psql -X -U postgres -d worldbuilder_test -v ON_ERROR_STOP=1 "$@"; }
sql < "$repo_dir/sql/worldbuilder/001_up.sql"
sql < "$repo_dir/sql/worldbuilder/001_up.sql"
sql < "$repo_dir/sql/worldbuilder/001_down.sql"
sql < "$repo_dir/sql/worldbuilder/001_up.sql"
sql -c 'CREATE ROLE worldbuilder_external NOINHERIT LOGIN; CREATE ROLE worldbuilder_relay NOINHERIT LOGIN; CREATE ROLE legacy_app NOINHERIT LOGIN;'
sql < "$repo_dir/sql/worldbuilder/grants.sql"
sql -c "SELECT has_schema_privilege('legacy_app','worldbuilder','USAGE') AS must_be_false;"
if sql -c 'SET ROLE legacy_app; SELECT * FROM worldbuilder.world;' >/dev/null 2>&1; then echo 'FAIL: legacy user can access canon'; exit 1; fi
if sql -c 'SET ROLE worldbuilder_relay; INSERT INTO worldbuilder.world VALUES(gen_random_uuid(), '\''bypass'\'', 0, now());' >/dev/null 2>&1; then echo 'FAIL: relay can mutate canon'; exit 1; fi
port=$(docker port "$container_name" 5432/tcp | awk -F: 'NR==1 {print $NF}')
test_host=""
# Container socket readiness can precede host forwarding (and the image's final TCP server).
for attempt in {1..15}; do
  for candidate_host in 127.0.0.1 host.docker.internal; do
    if PGCONNECT_TIMEOUT=2 PGPASSWORD=offline-test-only psql -h "$candidate_host" -p "$port" -U postgres -d worldbuilder_test -Atc 'SELECT 1' >/dev/null 2>&1; then
      test_host=$candidate_host
      break 2
    fi
  done
  sleep 1
done
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
docker exec "$container_name" createdb -U postgres worldbuilder_restore
docker exec "$container_name" pg_dump -U postgres -d worldbuilder_test -Fc -f /tmp/worldbuilder.dump
docker exec "$container_name" pg_restore -U postgres -d worldbuilder_restore --exit-on-error /tmp/worldbuilder.dump
original=$(sql -Atc "SELECT md5(string_agg(world_id::text || ':' || revision::text || ':' || snapshot::text, ',' ORDER BY world_id,revision)) FROM worldbuilder.revision")
restored=$(docker exec "$container_name" psql -X -U postgres -d worldbuilder_restore -Atc "SELECT md5(string_agg(world_id::text || ':' || revision::text || ':' || snapshot::text, ',' ORDER BY world_id,revision)) FROM worldbuilder.revision")
[[ "$original" == "$restored" ]] || { echo 'FAIL: restored canon differs'; exit 1; }
echo 'PASS: up/up/down/up, role isolation, publication/story/outbox integration, populated-down refusal and restore checksum'
