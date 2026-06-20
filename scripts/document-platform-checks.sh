#!/usr/bin/env bash
set -euo pipefail

if ! command -v kubectl >/dev/null 2>&1; then
	echo "kubectl is required for document platform checks" >&2
	exit 1
fi

if command -v rg >/dev/null 2>&1; then
	LOG_FILTER=(rg "barman-cloud-wal-archive|SlowDownWrite|PutObject|archive command failed")
else
	LOG_FILTER=(grep -E "barman-cloud-wal-archive|SlowDownWrite|PutObject|archive command failed")
fi

print_section() {
	printf '\n== %s ==\n' "$1"
}

format_wal_errors() {
	if ! command -v python3 >/dev/null 2>&1; then
		cat
		return
	fi

	python3 -c '
import json
import sys

for raw in sys.stdin:
    raw = raw.strip()
    if not raw:
        continue
    try:
        entry = json.loads(raw)
    except json.JSONDecodeError:
        print(raw)
        continue

    ts = entry.get("ts", "")
    logger = entry.get("logger", "")
    msg = entry.get("msg", "")

    if logger == "postgres":
        record = entry.get("record") or {}
        detail = record.get("detail") or record.get("message") or msg
        print(f"{ts} postgres {detail}")
        continue

    error = entry.get("error") or ""
    if error and error not in msg:
        print(f"{ts} {logger} {msg} :: {error}")
    else:
        print(f"{ts} {logger} {msg}")
'
}

print_section "Homelab Deployments"
kubectl -n homelab get deploy \
	homelab-orchestrator \
	homelab-processor \
	homelab-assistant \
	homelab-vllm

print_section "Homelab Pods"
kubectl -n homelab get pods -o wide

print_section "KEDA And NATS"
kubectl -n homelab get scaledobject || true
kubectl -n nats get pod,svc || true

print_section "Postgres Backup Baseline"
kubectl -n data get cluster app-db \
	-o jsonpath='{range .status.conditions[*]}{.type}={.status} {.reason}{"\n"}{end}' || true
echo
kubectl -n data get scheduledbackup || true
kubectl -n data get backup || true

print_section "Recent WAL Archive Errors"
kubectl -n data logs app-db-1 --tail=200 2>/dev/null | "${LOG_FILTER[@]}" | tail -n 20 | format_wal_errors || true

print_section "Envoy AI Gateway"
kubectl -n envoy-gateway-system get svc,pod -l gateway.envoyproxy.io/owning-gateway-name=vllm-ai-gateway || true

print_section "Follow-up Checks"
cat <<'EOF'
- Run `make ragas-chunking-eval` to measure retrieval quality against private gold cases.
- Treat `ContinuousArchiving=False`, missing `ScheduledBackup`, or repeated WAL `SlowDownWrite` logs as failed Postgres recovery coverage.
- Run `make vllm-gateway-smoke` to verify the assistant model path through Envoy AI Gateway.
- Run the SQL checks in `sql/rag/ops_dashboard_queries.pgsql` through the local psql workflow for lifecycle throughput, recent failures, and assistant proposal outcomes.
EOF
