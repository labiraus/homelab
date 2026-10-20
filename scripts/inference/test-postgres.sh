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
docker run -d --name "$container_name" --label "homelab.worldbuilder.postgres-fixture=$container_name" -e POSTGRES_PASSWORD=offline-test-only -e POSTGRES_DB=inference_test -p "127.0.0.1:${WORLDBUILDER_TEST_PORT:-}:5432" postgres:16.13 >/dev/null
relay_pid=""
cleanup() {
  if [[ -n "$relay_pid" ]]; then kill "$relay_pid" 2>/dev/null || true; fi
  docker rm -f "$container_name" >/dev/null
}
trap cleanup EXIT
for attempt in {1..30}; do
  # The image first starts a socket-only server for initdb, then shuts it down.
  # Wait for the final TCP server so migrations cannot race that handover.
  if docker exec "$container_name" pg_isready -h 127.0.0.1 -U postgres -d inference_test >/dev/null 2>&1; then break; fi
  sleep 1
done
sql() { docker exec -i "$container_name" psql -X -U postgres -d inference_test -v ON_ERROR_STOP=1 "$@"; }
sql -c "CREATE ROLE inference_runtime LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS PASSWORD 'offline-inference-only'; CREATE SCHEMA worldbuilder; REVOKE ALL ON SCHEMA worldbuilder FROM PUBLIC; CREATE TABLE worldbuilder.world(id integer);"
sql < "$repo_dir/sql/inference/001_up.sql"
sql < "$repo_dir/sql/inference/001_up.sql"
sql < "$repo_dir/sql/inference/grants.sql"
port=$(docker port "$container_name" 5432/tcp | awk -F: 'NR==1 {print $NF}')
test_host=""
# Container socket readiness can precede host forwarding (and the image's final TCP server).
if [[ "${WORLDBUILDER_TEST_TRANSPORT:-tcp}" != exec ]]; then
 for attempt in {1..15}; do
  for candidate_host in 127.0.0.1 host.docker.internal; do
    if PGCONNECT_TIMEOUT=2 PGPASSWORD=offline-test-only psql -h "$candidate_host" -p "$port" -U postgres -d inference_test -Atc 'SELECT 1' >/dev/null 2>&1; then
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
    if [[ -n "$relay_port" ]] && PGCONNECT_TIMEOUT=2 PGPASSWORD=offline-test-only psql -h 127.0.0.1 -p "$relay_port" -U postgres -d inference_test -Atc 'SELECT 1' >/dev/null 2>&1; then
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
export INFERENCE_TEST_DATABASE_URL="postgres://postgres:offline-test-only@${test_host}:${port}/inference_test?sslmode=disable&connect_timeout=5"
export GOPATH="${GOPATH:-/tmp/inference-go}"
export GOCACHE="${GOCACHE:-/tmp/inference-gocache}"
export GOMODCACHE="${GOMODCACHE:-/tmp/inference-gomod}"
(cd "$repo_dir/apps/inference" && go test -timeout=120s -race -count=1 ./...)
# Restore the entire ledger and prove reservations, immutable history and grants survive.
snapshot=$(mktemp /tmp/inference-dump-XXXXXX.sql)
trap 'rm -f "$snapshot"; cleanup' EXIT
docker exec "$container_name" pg_dump -U postgres -d inference_test --schema=inference > "$snapshot"
before=$(sql -Atc "SELECT md5(string_agg(to_jsonb(a)::text,',' ORDER BY id)) FROM inference.attempt a")
docker exec "$container_name" createdb -U postgres inference_restore
docker exec -i "$container_name" psql -X -U postgres -d inference_restore -v ON_ERROR_STOP=1 < "$snapshot"
after=$(docker exec "$container_name" psql -X -U postgres -d inference_restore -Atc "SELECT md5(string_agg(to_jsonb(a)::text,',' ORDER BY id)) FROM inference.attempt a")
[[ "$before" == "$after" ]] || { echo 'FAIL: inference restore checksum mismatch'; exit 1; }
if docker exec "$container_name" psql -X -U postgres -d inference_restore -v ON_ERROR_STOP=1 -c 'SET ROLE inference_runtime; DELETE FROM inference.attempt;' >/dev/null 2>&1; then echo 'FAIL: restored role can erase ledger'; exit 1; fi
echo 'PASS: inference migrations, concurrent accounting, role isolation and populated restore'
