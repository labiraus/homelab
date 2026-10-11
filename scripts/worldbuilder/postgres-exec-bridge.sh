#!/usr/bin/env bash
# Binary PostgreSQL transport to a disposable test container only. No TTY.
set -euo pipefail
container_name=${1:?disposable container name required}
[[ "$container_name" == worldbuilder-test-* ]] || { echo 'Refusing non-test container' >&2; exit 1; }
exec docker exec -i "$container_name" bash -c '
  exec 3<>/dev/tcp/127.0.0.1/5432
  cat <&3 &
  receive_pid=$!
  cat >&3
  kill "$receive_pid" 2>/dev/null || true
  wait "$receive_pid" 2>/dev/null || true
'
