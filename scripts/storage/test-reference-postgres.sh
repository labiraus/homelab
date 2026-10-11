#!/usr/bin/env bash
# Disposable database and local psql only; never loads workstation/LAN settings.
set -euo pipefail
repo_dir=$(cd "$(dirname "$0")/../.." && pwd)
container_name="worldbuilder-test-reference-${RANDOM}-$$"
work_dir=$(mktemp -d)
relay_pid=''
cleanup() {
  if [[ -n "$relay_pid" ]]; then kill "$relay_pid" 2>/dev/null || true; fi
  docker rm -f "$container_name" >/dev/null 2>&1 || true
  rm -rf "$work_dir"
}
trap cleanup EXIT
docker run -d --name "$container_name" -e POSTGRES_PASSWORD=offline-reference-only -e POSTGRES_DB=storage_reference_test postgres:16.13 >/dev/null
ready=false
for attempt in {1..30}; do
  if docker exec "$container_name" pg_isready -h 127.0.0.1 -U postgres -d storage_reference_test >/dev/null 2>&1; then ready=true; break; fi
  sleep 1
done
[[ "$ready" == true ]] || { echo 'Disposable reference database not ready'; exit 1; }
# Exec bridge avoids Docker-outside-Docker host port reachability differences.
socat -d -d TCP-LISTEN:0,bind=127.0.0.1,reuseaddr,fork "EXEC:bash '$repo_dir/scripts/worldbuilder/postgres-exec-bridge.sh' '$container_name'" 2>"$work_dir/bridge.log" &
relay_pid=$!
for attempt in {1..30}; do
  port=$(sed -n 's/.*listening on AF=2 127.0.0.1:\([0-9]*\).*/\1/p' "$work_dir/bridge.log" | head -1)
  [[ -n "$port" ]] && break
  sleep 0.1
done
[[ -n "$port" ]] || { echo 'Disposable reference bridge not ready'; exit 1; }
export PGHOST=127.0.0.1 PGPORT="$port" PGDATABASE=storage_reference_test PGUSER=postgres PGPASSWORD=offline-reference-only PGCONNECT_TIMEOUT=5
unset PGSERVICE PGSERVICEFILE PGOPTIONS PGHOSTADDR
cd "$repo_dir"
STORAGE_REFERENCE_DATABASE_TEST=1 python3 -m unittest discover -s scripts/storage -p test_reference_postgres.py -v
