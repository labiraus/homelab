# Homelab Troubleshooting Runbook

This runbook is for day-two document-platform and assistant operations.

If the UI image smoke check passes image-pull progress along with a container ID to `docker start`, keep stderr separate when parsing the stdout identifier from `docker create`. A cached image can hide this harness failure locally. Require a corrected actual-image check and successful CI chart publication; a published image alone is not a completed release.

If a local Node image build reports `npm error Exit handler never called!`, verify registry TLS inside that Docker worker before blaming the lockfile. On this offline workstation, `node:24.13.0-bookworm-slim` could be pulled, but its direct fetch of `https://registry.npmjs.org/typescript` failed with `UNABLE_TO_GET_ISSUER_CERT_LOCALLY`. Host/devcontainer npm success does not prove container trust. Use hosted CI's complete image/runtime checks while correcting workstation trust separately; retain `npm ci` and certificate verification. Do not add a private workstation CA or disable TLS verification in production Dockerfiles to bypass a local interception issue.

For Worldbuilder dates outside JavaScript's safe integer range, inspect the raw HTTP JSON before ordinary `response.json()`/`JSON.parse()` converts numbers. The UI/processor preserve original date tokens and send large dates as decimal strings; external keeps exact numeric history. An “Exact fictional dates require … JSON source support” error means the browser needs updating, not that canon should be truncated or rewritten. See the [date wire contract](../docs/worldbuilder/API.md). Calendar formatting and life-bound fact types remain separate from this transport behavior.

Postgres-backed state is authoritative. NATS, SSE, and MCP notifications are delivery paths; do not treat notification absence as proof that the durable state is missing.

## SSH CA recovery (2026-10-11)

The workstation SSH login certificate expired on 2026-07-20 and the old signing CA passphrase was unavailable. Recovery used a staged SSH trust rotation, with no VM recreation, kubeadm reset, or storage deletion. See the [rotation procedure](../docs/Setup.md#recovering-a-lost-ssh-ca-passphrase) and [local secret custody](../docs/Secrets.md).

The replacement Ed25519 CA fingerprint is `SHA256:cF9DuLWENcuTmQYNZKy6yjlZ7U5gc/YfE31q41SfJxU`. Its initial login certificate expires on 2026-11-10. The workstation CA/public key/certificate and all five Terraform node-layer public CA entries were updated; Terraform was not applied. Old trust is retained while offline hosts remain outstanding.

- All four Proxmox hosts accept the replacement certificate for root. The initial console attempt installed a bare public CA key in `/etc/pve/priv/authorized_keys`; adding `cert-authority,principals="root"` fixed authentication. The operator verified the root authorized-key symlink and permissions. Fresh client traces confirmed `ED25519-CERT` acceptance on every host.
- `jotunheim`: the Proxmox guest agent installed additional public CA trust. New certificate SSH and sudo passed.
- `alfheim` and `nidavellir`: an existing authorized login key provided recovery access. New certificate SSH and sudo passed.
- `niflheim`: its changed host key was independently verified by reading `/etc/ssh/ssh_host_ed25519_key.pub` from VM 3104's root partition through authenticated Proxmox SSH, using a read-only loop device and read-only `debugfs`, without mounting or changing the guest filesystem. The host fingerprint is `SHA256:QIBi+PJH9CCUOKgIyQhL7WTF8lbqNgNJwJRQEa7gAF4`. The loop device was detached. Strict SSH verification against that key and the existing login key allowed CA installation; new certificate authentication and sudo passed. Only this host's workstation `known_hosts` entries were replaced, with the original saved locally. The guest-agent service is absent; cloud-init reports completion.
- Existing authorized-key trust on these four guests was preserved, with `authorized_keys.before-ca-20261011` backups beside their authorized-key files.
- `yggdrasil` recovered from an initramfs boot failure after the operator performed the guided filesystem repair. Root UUID `4503a76b-f0a0-4337-8c84-32ca0091e510` matches ext4 `/dev/mmcblk1p2`; `/dev/mmcblk1p1` is vfat. The read-only check reported orphaned inodes and block bitmap differences. After repair, root is mounted read/write, SSH/kubelet/containerd are active, time is synchronized, and systemd lists no failed units. This establishes boot recovery, not a full application-data integrity check.
- The recovered API allowed a temporary, node-pinned recovery Pod on `yggdrasil` to add the public CA to its verified `/etc/ssh/trusted-user-ca-keys.pub`. It used an already cached digest-pinned image, no service-account token, dropped capabilities, and only the host SSH directory mount. New certificate SSH and sudo passed; the Pod was deleted and its absence verified. The previous CA file remains at `/etc/ssh/trusted-user-ca-keys.pub.before-ca-20261011`.
- `svartalfheim`: new certificate SSH as `oliver` and passwordless sudo are verified at `192.168.8.177`. Effective `TrustedUserCAKeys` is `/etc/ssh/trusted-user-ca-keys.pub`; both CA fingerprints are present and the `.before-ca` backup exists. Initial post-install network timeouts cleared on retry without further trust changes. `/dev/sda2` is mounted at `/srv/minio`, with MinIO and Samba active. SeaweedFS is inactive: this is access recovery, not the storage migration. The temporary public-key-only service on `yggdrasil:18080` was stopped, port closure verified, and owned server files removed. The verified temporary `/tmp/ca.pub` download was removed from the Pi. Local cleanup metadata remains in ignored `.local/ssh-ca-recovery-20261011/public-ca-transfer.json`.
- API `/readyz` passes, including etcd. Scheduler/controller-manager leases initially retained old timestamps, but both resumed renewal during observation without manual restarts. `yggdrasil`, `jotunheim`, `alfheim`, and `niflheim` have current kubelet leases. Do not mistake cached Node Ready, mirrored Pod phases, or CNPG status for live health immediately after an outage.
- The initial Longhorn attachment failure for PostgreSQL volume `pvc-6e1da232-b267-4e81-afb4-21433f704195` cleared during reconciliation: it is attached and healthy on `alfheim`, and `data/app-db-1` is Running/Ready. No volume salvage, deletion, or database restore was attempted. SQL/application-data integrity and backup recovery were not verified.
- `helheim` subsequently recovered: with all four Proxmox members quorate, restarting only host3's corosync released its abnormal 5.2 GiB RSS allocation (about 180 MiB afterward). Four-member quorum was rechecked, available memory exceeded the VM allocation, and existing VM 3103 started successfully. Existing authorized-key access allowed the public CA entry scoped to `ubuntu` to be appended with a rollback backup; fresh `ED25519-CERT` authentication and sudo passed. The node is Ready with active kubelet/containerd. All seven Longhorn volumes then became attached/healthy and all three NATS members Ready. `midgard` remains `NotReady,SchedulingDisabled`; application rollout and data recovery checks are still ongoing.

For Proxmox console recovery, first check the effective SSH trust paths and verify that `readlink -f /root/.ssh/authorized_keys` resolves to `/etc/pve/priv/authorized_keys`. Preserve existing entries, back up the file, then append the root-scoped public CA entry if absent. This single-line form survives consoles that flatten copied line breaks:

```bash
f=/etc/pve/priv/authorized_keys; test -e /root/authorized_keys.before-ca-20261011 || cp -p "$f" /root/authorized_keys.before-ca-20261011; entry='cert-authority,principals="root" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJ/khxdQQQI9UpA1WoPwUbH0bikOq9A3malUSS0IIRbf homelab-user-ca-20261011'; grep -qxF "$entry" "$f" || printf '\n%s\n' "$entry" >> "$f"
```

Verify fresh certificate authentication afterward; no workload restart is required. Never copy the CA private key or passphrase to a host.

## MCP discovery works but transport authentication is missing

The October LAN check read the registered-user count anonymously through `/mcp`: allowing the gateway's mesh identity had not authenticated its caller. A temporary `ingress/mcp-recovery-auth` CUSTOM policy now requires the existing oauth2-proxy provider for `/mcp`, `/sse`, `/message` and `/messages`. It preserves public discovery and preflight. The permanent MCP chart policy and app registered-user check must reconcile before removing the temporary policy. Use `scripts/validation/public-endpoints.py` to verify anonymous/forged-header refusal as well as TLS/discovery, and separately prove authenticated calls. A login redirect is refusal, not successful MCP data access.

Envoy prefixes rather than replaces the auth-check path. `/oauth2/auth/mcp` reaches the proxy's ordinary login/upstream flow; `/oauth2/auth?/mcp` reaches its exact auth-only endpoint. Keep the provider's query delimiter and verify actual 401/202 responses rather than treating a redirect as authenticated-path acceptance.

The provider is also supplied explicitly by `bootstrap/flux-bootstrap`, so updating only the Istio child defaults leaves the effective prefix unchanged. The publication regression now compares both rendered provider configurations. MCP `0.0.0-uat.38164253531` and its permanent edge policy are live; anonymous/forged-header probes passed before removing the temporary recovery policy. Browser login is user-confirmed; authenticated native MCP still needs acceptance.

## S3 startup fails despite a healthy storage host

On 2026-10-11, `svartalfheim` resolved inside the new orchestrator to Wi-Fi `192.168.8.202`, but the workload's egress policy permits the verified wired address `192.168.8.177/32`. Both host listeners were healthy; the application timed out/reset on the disallowed address. Published application, CNPG and prepared Velero endpoints now use the wired address explicitly. Source MinIO stays on port 9000; SeaweedFS cutover remains a separate port-8333/credential migration. Keep endpoints and egress rules aligned rather than broadening access to the LAN.

## OpenSearch cannot downgrade after an interrupted upgrade

The October recovery exposed a failed 3.9.0 upgrade followed by an automatic rollback to 3.7.0. The ML node refused its already-upgraded metadata with `cannot downgrade a node`. Preserve the PVCs and retry forward to the observed newer version. The OpenSearch HelmRelease now uses Flux `RetryOnFailure` for upgrades, avoiding automatic binary downgrade. Istiod also retries failed upgrades so a temporary unavailable worker does not permanently stall gateway dependencies. Check the actual installed Flux CRD/controller supports the strategy before publication.

After both OpenSearch 3.9.0 Pods became Ready, `_cluster/health` still reported one node: data had `discovery.type=single-node`, and the ingress policy allowed no port-9300 peer transport. The data chart now enables normal discovery and permits transport only between this release's data/ML Pods in the same namespace. Preserve both PVCs and verify two actual cluster members after reconciliation; Pod readiness alone does not establish ML membership. One data node can still leave replica shards unassigned.

## Prometheus wrapper conflicts with its own HelmRelease

After repairing empty-overlay publication, the live API dry-run revealed a second issue: the Prometheus wrapper rendered `flux-system/prometheus`, the name already occupied by its owning HelmRelease. Applying it would combine the parent's `chartRef` with the child's `chart`, which the installed CRD rejects. The upstream child is now named `prometheus-stack`, while its Helm release name stays `prometheus` so service names and monitor selectors remain stable. Check both Flux objects after publication.

## Published Helm chart loses defaults with an empty overlay

On 2026-10-11, Flux fetched Prometheus `0.0.0-uat.38152999164`, but install failed at `.Values.worldbuilderObservability.enabled`. `helm show values` on that exact OCI artifact returned no defaults, although source rendering passed. The publication action's old yq expression returned empty output for the zero-byte `values-ghcr.yaml`, then replaced `values.yaml` successfully.

Publication and the local merge helper now share a merger that normalizes empty/null documents to maps, validates single-document mapping inputs and replaces output only after validation. The regression packages the real Prometheus chart, verifies retained defaults and compares its render with source plus overlay. Publish the corrected charts through GitHub Actions and verify Flux against the new artifact; a local render alone does not repair the installed release.

## Devcontainer build fails while installing TFLint

If the build reports `bash: line 1: 404:: command not found` at the TFLint step, check for the obsolete `terraform-linters/tflint/master/install_linux.sh` download in `.devcontainer/Dockerfile`. That upstream URL returns HTTP 404. TFLint is already installed by the pinned Terraform feature in `.devcontainer/devcontainer.json`; keep that feature as the single installation source.

After removing the duplicate Dockerfile installer, run **Dev Containers: Rebuild Container** in VS Code. Verify `terraform version` and `tflint --version` inside the rebuilt container.

## Devcontainer storage tooling and download failures

On 2026-10-11, `https://dl.min.io/client/mc/release/linux-amd64/mc` returned HTTP 410. The devcontainer now installs the SeaweedFS `weed` CLI instead of `mc`, following the storage pivot. It downloads the official [4.48 release](https://github.com/seaweedfs/seaweedfs/releases/tag/4.48) and checks the architecture-specific SHA-256 before extraction. When updating it, update the release and both amd64/arm64 checksums together. Rebuild and verify `weed version`. Existing MinIO-specific Ansible state playbooks still require `mc` and must be replaced as part of the service migration; installing `weed` alone does not migrate storage or backups.

For command-line Dev Containers builds on a Zscaler-inspected Windows host, set `NODE_EXTRA_CA_CERTS` to the absolute path of the existing `.devcontainer/zscaler-ca.crt` before running the CLI. The Dockerfile CA installation covers build steps inside the image; the host Node process needs its own CA configuration for feature downloads.

## SeaweedFS returns read errors immediately after restart

The S3 listener and bucket listing can become ready before volume servers register with the master. The disposable storage smoke test observed transient `InternalError` / `volume not found` responses during that interval. Check a known object's contents as well as listener/list readiness, using bounded retries; persistent failures require checking volume files, the disk mount and filer/master metadata. Never treat a successful bucket listing alone as proof that object data is available. See [migration gates](../docs/SeaweedFSMigration.md).

## Public HTTPS fails before application authentication

Check the **Public Endpoint Validation** workflow after app/Helm publication. It retries verified HTTPS and records sanitized DNS/TLS/discovery/auth results from GitHub's runner; compare that independent observation with the local network failure. A successful build only establishes artifact publication, and even a passing public probe does not establish the installed revision. Inspect Flux source/release status when an approved cluster connection is available.

After MCP publication, the check now additionally requires that image's stamped discovery version. On 2026-10-10, hosted runs `38082334514` and `38082259713` each failed ten attempts with TLS verification code 62 (hostname mismatch) against `87.117.64.111`, independently confirming the public TLS problem. MCP build/chart publication succeeded, but no application version could be read. Confirm WAN/DNS/NAT ownership and inspect installed Gateway/Certificate/Flux status before changing source that already declares the correct hostname. Do not infer stalled Flux from an inaccessible public route.

Hosted run `38125438853` on 2026-10-11 additionally observed the origin's date-valid Let's Encrypt/YE2 certificate for `bath.bprj.co.uk` and related `bprj.co.uk` names, and a timeout on the anonymous HTTP-01 challenge-path probe. The mismatch therefore exists beyond local Zscaler interception. Final failed checks retain public certificate metadata with verification enabled and HTTP-01 status/redacted redirects without application credentials or response bodies. A date-valid certificate for another hostname is not fixed by expiry/CA-trust bypasses.

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

## Minecraft installer interrupted by Ansible

If a cold upgrade starts Minecraft and immediately logs `Stopping minecraft.service` while the
modpack installer is still downloading, check the role's handler ordering. Configuration templates
notify a restart; flushing that handler after `state: started` interrupts the newly started installer.
The role now flushes pending restarts before ensuring the service is enabled and started. A stopped
service with changed configuration starts once, while an unchanged stopped service still starts.

## Worldbuilder migration

Worldbuilder implementation and offline checks are tracked in [implementation status](../docs/worldbuilder/IMPLEMENTATION-STATUS.md). Use the [LAN runbook](../docs/worldbuilder/LAN-RUNBOOK.md) for discovery and controlled activation. Code publication on main is authorised; production migrations and feature activation require the separate LAN gate. Do not use the unfinished `scripts/pg-update.sh` placeholder for this migration.

## Worldbuilder generation broker configuration

An `incompatible generation stream` startup error now includes durability drift: inspect subjects, file storage, three replicas and seven-day limits retention. A consumer replica mismatch also stops startup. Do not delete or silently recreate existing streams/consumers to bypass the error; inspect and back up their pending work before controlled repair. The [disposable broker drill](../docs/worldbuilder/BROKER-RECOVERY.md) verifies leader loss, quorum loss and restart locally; installed topology and storage remain LAN checks.

## Worldbuilder disposable API-server acceptance

Use `bash scripts/worldbuilder/test-platform.sh` with the dependencies in [offline acceptance](../docs/worldbuilder/OFFLINE-ACCEPTANCE.md). It downloads pinned/checksummed test binaries and uses generated loopback TLS credentials; setting an operator kubeconfig or `USE_EXISTING_CLUSTER` must never redirect the test. A checksum mismatch, missing API/schema, strict field error or failed negative fixture is a failed gate. Preserve the printed public-schema/render artifact directory and inspect the named resource. Helm stderr is stored separately from YAML. Set `KUBEBUILDER_ATTACH_CONTROL_PLANE_OUTPUT=true` for local startup diagnostics.

New CRDs can initially have absent/null status conditions; the harness waits for `Established` with a deadline. It refuses remote conversion-webhook configurations rather than rewriting their source or connecting outside the fixture. External operators/webhooks need separate disposable fixtures; do not bypass an acceptance failure by selecting a real cluster. No test success proves live admission or workload readiness.

If a local Go cache reports `missing ziphash`, use a fresh task-specific `GOPATH`/`GOMODCACHE` and rerun the existing gate with module verification intact; do not reuse incomplete unpacked modules or disable checksum verification.

For a structural parser error on a bare `=` in a public CRD enum, use the repository's Kubernetes-aware safe loader. It preserves this legal string while rejecting unsafe YAML tags; do not rewrite the upstream CRD or skip its validation. The disposable API-server gate must also pass the original source.

## Worldbuilder Prometheus admission acceptance

The [controller fixture](../docs/worldbuilder/CONTROLLER-ACCEPTANCE.md) runs the chart’s real certificate Jobs before testing admission. Null Helm annotation maps are valid; completed hook Pods share the operator’s name label, so use the Deployment selector for shutdown/replacement. Invalid PromQL may return Kubernetes `Invalid` without a webhook prefix. Check the semantic reason, and prove TLS trust and unavailable-webhook refusal separately. After a Ready operator Pod, wait for actual admission routing within the bounded deadline. Do not disable TLS or set `failurePolicy: Ignore` to make acceptance pass.

## Worldbuilder physical backup acceptance

The [CNPG backup fixture](../docs/worldbuilder/BACKUP-ACCEPTANCE.md) verifies base backup/WAL and latest/PITR recovery after removing only its synthetic source Cluster/Pods/PVCs. Do not reuse those deletion commands against an operator context. Inspect its `backup-result.json` together with the final workflow conclusion and cleanup logs; a configured/completed backup alone does not prove restore. The fixture tests all 17 populated tables and refuses wrong S3 credentials.

CNPG webhook errors may print `Cluster ... is invalid` without an `admission webhook` prefix. The acceptance harness checks the CNPG rejection and separately stops/restarts its own operator to prove fail-closed admission and recovery. Local database fallback only follows a transport failure; password, SQL and certificate errors must not be bypassed by switching to peer authentication.

A newly created NodePort can briefly accept and close a connection before kube-proxy installs its route. The local `psql` preflight retries this startup condition for a bounded interval; a persistent closed connection still fails, and does not select peer authentication. Hosted CI exposed this race after successful CNPG admission checks; test both recovery on a later connection and persistent-failure refusal.

## Worldbuilder Velero recovery

Use the [Velero fixture](../docs/worldbuilder/VELERO-ACCEPTANCE.md) for disposable file/resource restore, and inspect `velero-state.log`, controller/node-agent/restore logs and the final workflow result. A completed metadata backup is insufficient: require a completed PodVolumeBackup/Kopia snapshot, removed source storage, completed PodVolumeRestore and matching file/resource evidence. Failed, partial or warning-bearing results cannot pass.

Default kind local-path PVCs may produce unsupported hostPath PVs. The fixture requests `volumeType: local` and verifies the actual PV and claim UID. It excludes generated `kube-root-ca.crt` with the `velero.io/exclude-from-backup=true` label so the mapped namespace keeps its own current trust. An annotation with that name does not exclude the resource and produces an existing-ConfigMap mismatch. Do not suppress arbitrary restore warnings or copy stale cluster trust to make the test pass. Shared SeaweedFS fixtures use only the target's scoped `cnpg` or `velero` identity actions.

## Worldbuilder metrics and dashboard provisioning

For disposable controller failures, use [controller acceptance](../docs/worldbuilder/CONTROLLER-ACCEPTANCE.md) and retain `kind.log`, `pods.log`, `events.log` and the workflow conclusion. A kind bootstrap that appears to finish kubeadm early can be a Docker exec transport failure: this environment returned exit 0 before a delayed command actually exited 7. The fixture now inspects exec IDs to completion and verifies bounded output transfers. A failed/partial copy or timed-out exec must not become a passing gate. Creation and teardown both use generated kubeconfig; do not allow kind cleanup to edit the operator's home config.

Use the two acceptance commands in [observability](../docs/worldbuilder/OBSERVABILITY.md) before activation. Empty panels require separate checks of the configured datasource UID, application namespace, `worldbuilder-external` job, monitor/Service selection and actual scrape health. A rendered ServiceMonitor alone does not establish discovery or mTLS. Keep STRICT authentication and use the observed scraper identity; do not open the endpoint to all Pods to make a scrape pass. The disposable fixture uses an internal Docker network and needs no reachable published host ports.

Grafana file provisioning does not resolve manual-import `__inputs`. The chart binds UIDs and namespace explicitly, supplies the provider and mounts its ConfigMap. Inspect both the selected upstream chart's volume/provider render and Grafana's provisioned dashboard API. Keep one dashboard JSON source under the Prometheus chart; avoid a separate docs copy drifting from the published artifact.

## Worldbuilder disposable PostgreSQL host forwarding

If the test container is healthy but the devcontainer cannot reach its published port, use `WORLDBUILDER_TEST_TRANSPORT=exec bash scripts/worldbuilder/test-postgres.sh`. The local loopback socat bridge uses only the harness's generated test container and cleans up on exit. Finite commands use labelled exec-ID completion because Docker attach may return before database creation or restore finishes. See [Worldbuilder troubleshooting](../docs/worldbuilder/TROUBLESHOOTING.md). This does not require cluster access or a production database connection.

## Storage publication and historical reverts during migration

App CI publishes the matching Helm chart as well as the image. Keep source endpoint/Secret/egress defaults until the storage gates pass; prepare SeaweedFS settings in explicit cutover overlays. Publishing application code must not implicitly switch object stores. `scripts/storage/check_charts.py` verifies base/GHCR/ECR source and target combinations. A published chart is not evidence of Flux reconciliation or a successful migration.

If an old audit marker cannot be reverted after copying objects, compare the exact bucket/key/version against the verified migration map. New S3 writes assign new IDs; a current-only copy cannot preserve historical reads. Configure both `S3_VERSION_MAP_KEY` and its immutable `S3_VERSION_MAP_VERSION` only after validating the map and restore. The resolver rejects endpoint/bucket mismatch, deleted versions and hash/size mismatch; do not work around those failures by reading the latest object. See [migration/recovery procedure](../docs/StorageVersionMigration.md).

The disposable storage test needs `systemd-socket-activate` and `systemd-socket-proxyd`, plus the role-pinned `weed` binary. Minimal devcontainers may lack the systemd tools even when production hosts have them. CI uses an Ubuntu runner with these tools. SeaweedFS interprets environment variables as flags: an inherited `debug=release` caused startup failure on 2026-10-11. The harness now starts it with a minimal environment and prints the beginning as well as the end of failed startup logs.

## Storage archive ordering evidence refused

Schema-2 archives require both `order/before/` and `order/after/` XML page sets plus their manifest hashes. Do not delete a page or reorder tied manifest entries. A missing page, corrupt checksum, mismatched marker/scope, changed inventory, duplicate version, inconsistent latest flag or non-monotonic provider history stops export/import. SeaweedFS can list a `null` version last regardless of its age; the tool repositions it only when timestamps/latest flags establish one position. Latest null ties and a non-latest null tied only with the latest peer are provable; other null ties require independent source chronology and stay refused. Inspect the installed provider release and original history, preserving source data; changing timestamps or sorting opaque IDs does not establish order. See [retained source ordering](../docs/StorageVersionMigration.md).

Hosted disposable acceptance encountered `No writable volumes and no free volumes left` after allocating 84 mostly empty volumes across fixture buckets. SeaweedFS's automatic slot count divides free disk by the configured 1 GiB volume limit, while default growth creates seven volumes per bucket. The harness now uses a bounded 256-slot override without preallocation, plus the separate large-fixture free-disk check. This is test sizing, not a production repair: actual disk usage, writable volumes, replication and growth capacity require inspection before changing a live server limit.

## Interrupted multipart migration

Retain `multipart-journal.jsonl` and `import-journal.jsonl`; neither authorises activation. Known upload IDs receive a best-effort abort, but a lost initiation reply can leave an unknown orphan and a lost completion reply can leave a committed version even after an acknowledged abort. Inspect versions and unfinished uploads for the recorded key, preserving evidence before controlled cleanup. A disk-full journal or hard stop can leave intent without an outcome. The importer refuses prior import evidence, populated destinations and pre-existing uploads; do not remove these safeguards to rerun it. Prepare a separately empty destination after reconciliation. See [multipart recovery boundaries](../docs/StorageVersionMigration.md).

For the disposable large-object round trip, use `--large-migration` with 12 GiB free temporary disk. If locally extracted systemd tools fail before the server checks, verify their dependencies with `ldd`: both the directory containing `libsystemd-shared-252.so` and the extracted architecture library directory containing `libkmod.so.2` must be in `LD_LIBRARY_PATH`. Check `systemd-socket-activate --version` before restarting the harness.

## Storage reference coverage refuses completion

Use the [reference coverage procedure](../docs/StorageReferenceCoverage.md) to interpret explicit missing-version/context/delete-marker outcomes. A failed PostgreSQL export has no sealed `manifest.json`; inspect its private `psql.log` for permissions, RLS or schema errors. Do not enable filtered row visibility or substitute a current object to obtain a green result. Resolve verified missing context through an export-bound reviewed file, preserving original database/history values. S3 delete-marker map entries have no body size/hash, and literal `null` is a valid source version ID; neither is a processing-generation number.

## Git push selects the wrong GitHub account

On 2026-10-11, the forwarded IDE credential helper selected a different GitHub account for a documentation push and GitHub returned HTTP 403, despite an earlier successful implementation push. For this repository, `git -c credential.username=labiraus -c credential.useHttpPath=true push origin main` selected the existing authorised repository account and succeeded. These are per-command hints, not new credentials or a permission bypass. Check the account named in the rejection before assuming repository access was revoked; do not print tokens, change global account selection or force-push to work around authentication.

If disposable PostgreSQL migrations encounter “the database system is shutting down” immediately after container startup, wait for `pg_isready -h 127.0.0.1` inside that container. The official image starts a temporary socket-only server during initdb; socket readiness can race its shutdown. The Worldbuilder harness now waits for the final TCP server before running migrations.

A Worldbuilder proposal can remain labelled `approved` after membership changes while its eligible approval count falls below quorum. Publication correctly returns 409. Inspect review history/current count and obtain fresh independent reviews; restoring an old role does not resurrect its old approval. Published approval counts remain historical and do not change when members leave. See [review policy](../docs/worldbuilder/REVIEW-POLICY.md).

## Worldbuilder persistent TSDB acceptance

Use the [TSDB recovery gate](../docs/worldbuilder/TSDB-RECOVERY.md) to distinguish scrape recovery from history recovery. Require stopped producers, quiescent fixed-time raw samples, WAL replay and a distinct restored volume after source deletion. The operator mounts its PVC through `prometheus-db`; derive and check the actual mount/subPath before reading a snapshot. Inspect file hashes before startup and require a causal corrupt-index failure, not an unrelated Pod error. Production storage and admin snapshot access remain separate controlled changes.

Prometheus recovery uses forward retries with a 15-minute child install/upgrade timeout and a 20-minute wrapper timeout. Slow first image pulls must not trigger repeated uninstall/recreate cycles; inspect Pod events and actual image progress before changing the underlying resources.

`midgard` is the intentionally offline/cordoned burst GPU host. The observed Helm readiness check waits for its otherwise-unschedulable node-exporter Pod indefinitely. The observability bootstrap explicitly excludes this host through `stack.nodeExporterExcludedNodes`; remove the exclusion through Flux when returning it to service, then verify its exporter and kubelet targets. Do not disable readiness checks for the active cluster to hide an offline DaemonSet member.

## Late application build restores older source

A Helm workflow may finish after a newer application commit, then dispatch its older SHA with a larger global run ID. On 2026-10-11, MCP run `38164323246` checked out `d4cecf9` even though the run API showed branch head `accf6d6`; that displaced the verified authentication release `38164253531`. Inspect the actual checkout/artifact provenance, not only the run's head SHA or success status. Keep temporary edge containment until both permanent policy and registered-user validation are observed again.

Helm dispatch now selects the current branch revision. The shared Docker publisher runs `check-app-publish-source.sh` against current branch content for the app, shared code, app chart/libraries and build/workflow inputs, refusing stale promotion. App workflows retest their selected revision. New images record the actual checkout in `org.opencontainers.image.revision`; charts use `homelab.labiraus.com/source-revision`. The GHCR bootstrap temporarily pins MCP's verified chart with `mcp.chartVersion`; a temporary source-controller suspension holds the known artifact until that durable pin is published, then must be resumed. Advance the pin only after real authentication probes pass on the intended replacement.
