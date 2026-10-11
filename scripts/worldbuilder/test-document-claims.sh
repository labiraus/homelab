#!/usr/bin/env bash
# Disposable database only; never loads operator settings or production DSNs.
set -euo pipefail
repo_dir=$(cd "$(dirname "$0")/../.." && pwd)
container_name="worldbuilder-test-${RANDOM}-$$"
export WORLDBUILDER_DOCKER_CLI=$(command -v docker)
export WORLDBUILDER_POSTGRES_CONTAINER="$container_name"
docker() { python3 "$repo_dir/scripts/worldbuilder/docker-exec.py" "$@"; }
work_dir=$(mktemp -d /tmp/worldbuilder-claims-XXXXXX)
relay_pid=''
cleanup() {
  if [[ -n "$relay_pid" ]]; then kill "$relay_pid" 2>/dev/null || true; wait "$relay_pid" 2>/dev/null || true; fi
  docker rm -f "$container_name" >/dev/null
  rm -rf "$work_dir"
}
trap cleanup EXIT
docker run -d --name "$container_name" --label "homelab.worldbuilder.postgres-fixture=$container_name" -e POSTGRES_PASSWORD=offline-claims-only -e POSTGRES_DB=document_claims_test postgres:16.13 >/dev/null
ready=false
for attempt in {1..30}; do
  if docker exec "$container_name" pg_isready -h 127.0.0.1 -U postgres -d document_claims_test >/dev/null 2>&1; then ready=true; break; fi
  sleep 1
done
[[ "$ready" == true ]] || { echo 'Disposable claim database did not start'; exit 1; }
socat -d -d TCP-LISTEN:0,bind=127.0.0.1,reuseaddr,fork "EXEC:bash '$repo_dir/scripts/worldbuilder/postgres-exec-bridge.sh' '$container_name'" 2>"$work_dir/bridge.log" &
relay_pid=$!
port=''
for attempt in {1..30}; do
  port=$(sed -nE 's/.*listening on.*127\.0\.0\.1:([0-9]+).*/\1/p' "$work_dir/bridge.log" | head -1)
  [[ -n "$port" ]] && break
  sleep 0.1
done
[[ -n "$port" ]] || { echo 'Disposable claim bridge did not start'; exit 1; }
PGPASSWORD=offline-claims-only PGCONNECT_TIMEOUT=5 psql -X -h 127.0.0.1 -p "$port" -U postgres -d document_claims_test -Atc 'SELECT 1' >/dev/null
cd "$repo_dir/apps/processor"
DOCUMENT_CLAIMS_TEST_DATABASE_URL="postgres://postgres:offline-claims-only@127.0.0.1:${port}/document_claims_test?sslmode=disable&connect_timeout=5" \
  node --import tsx --test test/db.integration.test.ts
