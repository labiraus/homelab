# Homelab Troubleshooting Runbook

This runbook is for day-two document-platform and assistant operations.

Postgres-backed state is authoritative. NATS, SSE, and MCP notifications are delivery paths; do not treat notification absence as proof that the durable state is missing.

## Public HTTPS fails before application authentication

Check the **Public Endpoint Validation** workflow after app/Helm publication. It retries verified HTTPS and records sanitized DNS/TLS/discovery/auth results from GitHub's runner; compare that independent observation with the local network failure. A successful build only establishes artifact publication, and even a passing public probe does not establish the installed revision. Inspect Flux source/release status when an approved cluster connection is available.

After MCP publication, the check now additionally requires that image's stamped discovery version. On 2026-10-10, hosted runs `38082334514` and `38082259713` each failed ten attempts with TLS verification code 62 (hostname mismatch) against `87.117.64.111`, independently confirming the public TLS problem. MCP build/chart publication succeeded, but no application version could be read. Confirm WAN/DNS/NAT ownership and inspect installed Gateway/Certificate/Flux status before changing source that already declares the correct hostname. Do not infer stalled Flux from an inaccessible public route.

On 2026-10-10, `mcp.labiraus.com` resolved but the development network presented a Zscaler-issued certificate for unrelated `bprj.co.uk` names. Anonymous diagnostic responses were Zscaler HTTP 403 pages reporting “Access denied due to bad server certificate”. This is not an application authorization result. Do not disable certificate verification for normal use or send credentials through the failing connection. Have the network administrator investigate; on the approved LAN, compare DNS/NAT forwarding, Gateway certificate/listener readiness and HTTPRoute attachment read-only. Interception prevents concluding whether the origin certificate is also wrong. See [public validation evidence](../docs/worldbuilder/PUBLIC-VALIDATION.md).

## Document Processing Failures

Start with inventory and lifecycle history:

```bash
kubectl -n homelab logs deploy/homelab-orchestrator --since=30m --tail=200
kubectl -n homelab logs deploy/homelab-processor --since=30m --tail=300
```

Use MCP or the browser Inventory tab to inspect the affected document:

- `documents.inventory.list` with `documentId`, `status`, `prefix`, or exact metadata filters
- `documents.history.list` with `documentId` and, when available, `processingVersion`

Common readings:

- `status=unsupported`: the object was reconciled but is outside the current `text/*` extraction policy.
- `status=pending` with a recent queued event: the processor may not have claimed the job yet.
- `status=processing` with no recent started/completed/failed event: check processor pod health and NATS consumer lag.
- `status=failed` or `lastError` populated: inspect processor logs and the lifecycle event payload.

Recovery:

```bash
kubectl -n homelab rollout status deploy/homelab-orchestrator --timeout=3m
kubectl -n homelab rollout status deploy/homelab-processor --timeout=3m
flux reconcile helmrelease processor -n flux-system --with-source --timeout=10m
```

After fixing the root cause, queue a newer version through `documents.reprocess` or the browser Inventory/Search reprocess action. Follow that returned processing version with `documents.history.list`.

## Stuck Lifecycle States

A lifecycle timeline should usually show:

1. `documents.events.processor.queued`
2. `documents.events.processor.started`
3. `documents.events.processor.completed` or `documents.events.processor.failed`

If queued exists but started does not:

```bash
kubectl -n homelab get deploy,pod -l app.kubernetes.io/name=processor -o wide
kubectl -n homelab get scaledobject
kubectl -n nats get pod,svc
kubectl -n nats logs statefulset/nats --since=30m --tail=200
```

If started exists but no terminal event exists, inspect the current processor pod and previous logs:

```bash
kubectl -n homelab logs deploy/homelab-processor --tail=300
kubectl -n homelab logs deploy/homelab-processor --previous --tail=300
```

Do not manually edit `rag.documents.status` as a first response. Prefer a controlled reprocess request so `orchestrator` owns the processing version and event trail.

## NATS And KEDA Worker Lag

The processor chart scales from JetStream consumer lag through KEDA.

Checks:

```bash
kubectl -n homelab get scaledobject
kubectl -n homelab describe scaledobject homelab-processor
kubectl -n homelab get hpa,deploy,pod -l app.kubernetes.io/name=processor
kubectl -n nats get svc
```

Expected service names in this repo:

- client: `nats-nats.nats.svc.cluster.local:4222`
- monitor: `nats-nats-headless.nats.svc.cluster.local:8222`

If KEDA cannot read lag, check the processor chart values and the NATS service names before changing application code.

## vLLM Startup Or Gateway Failures

Run the smoke test first:

```bash
make vllm-gateway-smoke
```

If it fails, inspect:

```bash
kubectl get nodes -l node-llm=gpu -o wide
kubectl -n homelab get deploy,pod -l app.kubernetes.io/name=vllm -o wide
kubectl -n homelab logs deploy/homelab-vllm --tail=200
kubectl -n envoy-gateway-system get svc,pod -l gateway.envoyproxy.io/owning-gateway-name=vllm-ai-gateway -o wide
kubectl -n envoy-gateway-system logs -l gateway.envoyproxy.io/owning-gateway-name=vllm-ai-gateway --tail=120
```

Common causes:

- GPU node label or `nvidia.com/gpu` allocatable missing
- first-load model download or CUDA setup exceeds startup probe timing
- Envoy Gateway proxy is not ready
- processor-to-gateway NetworkPolicy labels drifted
- served model name no longer matches `LLM_MODEL`

Keep the default model conservative until this path is stable under repeated restarts.

## Assistant Proposal Recovery

Proposal rows are processor-owned assistant state. Raw document writes happen only after browser approval through `orchestrator`.

Checks:

```bash
kubectl -n homelab logs deploy/homelab-processor --since=30m --tail=200
kubectl -n homelab logs deploy/homelab-orchestrator --since=30m --tail=200
```

Use the Assistant tab or Postgres to compare:

- `assistant.file_proposals.status`
- `assistant.file_proposals.orchestrator_response`
- `rag.document_change_audits` rows with the same `proposal_id`
- `rag.document_lifecycle_events` for the queued processing version

Recovery guidance:

- pending proposal, no orchestrator call: approve or reject from the browser.
- approved proposal, orchestrator error response: fix the upstream document error and create a new proposal rather than mutating the decided proposal.
- audit row exists but processing failed: reprocess the affected document and follow the returned version.
- wrong raw content but a previous MinIO version marker is known: stage a browser revert from the audit row so `orchestrator` writes the reverted object and queues reingestion.

## Retention Policy

Current policy is preserve-by-default:

- MinIO `documents` bucket versioning stays enabled; current-object lifecycle expiration remains disabled.
- `rag.document_lifecycle_events` is retained indefinitely until a reviewed cleanup policy exists.
- `rag.document_change_audits` is retained indefinitely.
- assistant conversations, messages, memories, tool calls, proposals, and proposal decisions are retained indefinitely.
- NATS lifecycle notification retention is best-effort delivery only and is not an audit retention mechanism.

Before enabling destructive cleanup, document:

- retention duration
- legal/privacy reason
- backup coverage
- restore test result
- user-visible behavior after deletion

## Backup And Recovery Checks

Postgres:

```bash
kubectl -n data get cluster app-db -o yaml
kubectl -n data get backup,scheduledbackup
kubectl -n data get secret cnpg-backup-s3
```

Expected baseline:

- `app-db` reports `type=ContinuousArchiving`, `status=True` in `status.conditions`.
- at least one `ScheduledBackup` resource exists for `app-db`; on-demand `Backup` rows may be absent between rehearsals.

If `ContinuousArchiving=False`, inspect the current WAL archive failure before trusting recovery coverage:

```bash
kubectl -n data logs app-db-1 --tail=200 | rg "barman-cloud-wal-archive|SlowDownWrite|PutObject|archive command failed"
```

Current known bad reading:

- `SlowDownWrite` from `barman-cloud-wal-archive` means the external MinIO target is refusing WAL writes.
- while that condition is false, WAL-only recovery coverage is broken even if the cluster itself stays Ready.
- if `ScheduledBackup` is also absent, the cluster does not currently have the base-backup coverage needed for a real restore rehearsal.

When the archive error points at external MinIO, inspect the host path directly:

```bash
make minio-host-checks
```

Current known host-side failure:

- if `/srv/minio` is not mounted, MinIO falls back to the Pi root filesystem directory instead of the external data disk
- MinIO then starts logging `Storage resources are insufficient for the write operation` and `no online disks found`, which lines up with CNPG `SlowDownWrite` failures from `barman-cloud-wal-archive`

Safe recovery order after confirming the disk device is still present:

```bash
ssh svartalfheim 'lsblk -f'
ssh svartalfheim 'sudo mount /srv/minio'
ssh svartalfheim 'mount | grep "/srv/minio" && df -h /srv/minio && ls -ld /srv/minio /srv/minio/minio-data'
ssh svartalfheim 'sudo systemctl restart minio && systemctl status --no-pager minio | sed -n "1,20p"'
make minio-host-checks
make document-platform-checks
```

Post-recovery readout should change in this order:

- `/srv/minio` shows as mounted instead of `not-mounted`
- MinIO host errors stop reporting `Storage resources are insufficient` / `no online disks found`
- CNPG `ContinuousArchiving` returns to `True`
- `ScheduledBackup` and later `Backup` rows become meaningful again once the chart change is live

MinIO:

```bash
make ansible-minio-state
```

Secrets:

```bash
kubectl -n flux-system get secret ghcr-creds
kubectl -n homelab get secret documents-minio-credentials assistant-config external-data-connections mcp-data-connections orchestrator-config processor-config
kubectl -n data get secret app-db-bootstrap cnpg-backup-s3
```

Recovery rehearsal should prove:

- CNPG can restore the RAG and assistant schemas from backup.
- external MinIO documents and object versions are available from `svartalfheim`.
- generated Kubernetes secrets can be regenerated or reapplied from the documented Ansible/manual-secret paths.
- browser and MCP inventory/history reads still explain the recovered state.

## Minimum Operator Dashboard Checks

Until dedicated dashboards exist, start with:

```bash
make document-platform-checks
```

Then use these checks:

- ingestion throughput: recent `documents.events.processor.completed` lifecycle events
- processor failures: recent `documents.events.processor.failed` events and processor logs
- worker lag: KEDA ScaledObject status and NATS stream/consumer health
- retrieval latency: `external` and `mcp` request logs plus `/metrics` when scraped
- assistant proposal outcomes: counts by `assistant.file_proposals.status`
- vLLM health: `make vllm-gateway-smoke`, vLLM pod restarts, and Gateway proxy readiness

`make document-platform-checks` now also prints the CNPG `ContinuousArchiving` condition, current `ScheduledBackup` / `Backup` rows, and recent WAL archive failures so the default operator pass catches broken Postgres recovery coverage earlier.

The repo also includes `sql/rag/ops_dashboard_queries.pgsql` for the Postgres-backed portions of these checks. Run it through the local `psql` workflow after opening the usual app-db port-forward.

These checks reinforce the current service boundaries. Do not add a separate workflow engine for observability alone.

## Minecraft FTB release mismatch

If an FTB download page advertises the requested release but its installer ID resolves to another
version, check the live pack and version API before changing Ansible. On September 17, 2026, the
indexed Aero download page paired `1.10.0` with `100504`, while the live API identified that ID as
`1.11.0`. The correct Aero `1.10.0` ID is `100497`. See [Minecraft VM updates](../docs/MinecraftVM.md#updating-a-managed-server-version)
for cold backups and installed-version/readiness checks.

## Worldbuilder migration

Worldbuilder implementation and offline checks are tracked in [implementation status](../docs/worldbuilder/IMPLEMENTATION-STATUS.md). Use the [LAN runbook](../docs/worldbuilder/LAN-RUNBOOK.md) for discovery and controlled activation. Code publication on main is authorised; production migrations and feature activation require the separate LAN gate. Do not use the unfinished `scripts/pg-update.sh` placeholder for this migration.

## Worldbuilder disposable PostgreSQL host forwarding

If the test container is healthy but the devcontainer cannot reach its published port, use `WORLDBUILDER_TEST_TRANSPORT=exec bash scripts/worldbuilder/test-postgres.sh`. The local loopback socat bridge uses only the harness's generated test container and cleans up on exit. See [Worldbuilder troubleshooting](../docs/worldbuilder/TROUBLESHOOTING.md). This does not require cluster access or a production database connection.
