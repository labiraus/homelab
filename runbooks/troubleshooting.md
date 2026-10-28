# Troubleshooting

Start with the symptom and verify actual context, identity, ownership and artifact.
[Status](../docs/status.md) owns dated incident evidence; procedures here do not
establish current health. Do not run destructive recovery merely because a prior
incident used it.

## Access and browser symptoms

| Symptom | Check and next action |
| --- | --- |
| Public HTTPS times out or presents another hostname | Compare ordinary DNS, current WAN and private Gateway; preserve Host/SNI and TLS verification for diagnostic overrides. Check hosted results independently. See [operations](../docs/operations.md#public-connectivity). |
| Google rejects outside-domain users | Inspect the existing project's audience and proxy wildcard domains. Register intended emails separately; test registered and unregistered users. See [authentication](../docs/authentication.md). |
| Browser login loops, plain Authenticated or RBAC denial | Inspect selected provider, exact service/Service port, redirect and reverse-proxy upstreams. Preserve public Host plus proxy-scoped mesh routing; parent values can replace child provider lists. |
| Signed out after Google login; reason is `identity validation failed` | The API received identity but its database lookup failed. Check CNPG readiness, ready `app-db-rw` endpoints and database transport; repeated login or account registration does not repair an unavailable database. See [authentication](../docs/authentication.md). |
| MCP discovery works but tools fail | Discovery is public. Verify actual native consent/credential, both transports, registered-user checks and private authority health; forwarded headers alone are insufficient. |
| Native login fails at Google registration | Native clients use the local MCP issuer, not Google's DCR. Browser Google cookies alone do not establish native login. |
| SSH permission denied | Inspect certificate validity before changing trust. Lost CA passphrase needs staged trust rotation, not VM recreation or Kubernetes PKI reset. See [SSH recovery](../docs/recovery.md#ssh-trust-recovery). |
| Physical host answers ping but SSH has no banner | Use physical console/kernel/power/disk evidence. `yggdrasil` is a physical Chromebook, not a Proxmox VM; TCP acceptance is not userspace health. |
| UI links/composer are visible but unreachable | Measure full bounds and `elementFromPoint` after scrolling/resizing. Header/footer/grid containment and real hit targets matter; auto-scrolling clicks can hide the defect. |
| World-menu trigger moves or Worlds opens an empty screen | Keep trigger in normal layout, popup separate; verify library navigation, Escape/focus return, outside dismissal and narrow widths. |
| Saved candidate disappears | Unsaved inline edits are memory-only. Shared saved messages/results persist separately; device-queued saves are not yet server confirmations. See [experience](../docs/worldbuilder/experience.md). |
| Approved proposal returns 409 after membership change | Check eligible approval count against current policy. Removed authority does not revive by restoring an old role; obtain fresh eligible approvals. Historical published counts remain unchanged. |
| Stale proposal or story turn | Retain original revision/session version. Use three-way reconciliation or explicitly reauthor; never substitute a fresh pin to bypass conflict. |
| Administration returns 409 | Reload administration version. Archived worlds must reopen before other writes; policy changes require fresh proposal pins. Protect final Authors/Administrators. |
| Generation is queued/uncertain | Inspect job, claim, dispatch and reservation. Completed parts remain; uncertain dispatched work is never automatically repeated or refunded. See [recovery](../docs/recovery.md). |
| Offline queue does not replay | Keep the app open, authenticate the same account, inspect capability/conflict and transactional lease (up to 30 seconds after interruption). Never alter queued IDs, payloads or expected versions. |
| Quota/storage failure or blocked worker update | Treat failed IndexedDB writes as failed saves. Copy/export before clearing data; finish registered local writes before service-worker activation. |

Native MCP tokens are not a Google browser session. Use a legitimate browser
session for production browser acceptance; never mint proxy cookies or inject
identity headers. Keep exported browser state ignored/private. A fixture served
from an owned loopback forward is a different evidence tier.

## Platform symptoms

| Symptom | Check and next action |
| --- | --- |
| Local render works but Flux fails | Inspect the actual OCI archive, tag/digest and effective parent values. A stale digest pin or retained child values can keep Kafka-era/scaler settings alive. Publish/repoint through reviewed GitOps; do not assume local edits deployed. |
| Release appears missing | Discover HelmRelease/OCI namespace independently of workload namespace; app releases live in `flux-system`. |
| Helm releases stop reconciling after a worker outage | Inspect the Helm controller's actual Pod, node selector and leader lease. A hard hostname pin can strand its replacement on the failed worker while old releases retain Ready conditions. See [Flux recovery](../docs/recovery.md#flux-reconciliation-recovery). |
| Empty defaults in published chart | Inspect `helm show values` on that artifact; use shared overlay merger and packaged-chart parity test. Empty/null overlays must preserve base defaults. |
| Late build restores old code | Inspect actual checkout SHA and publication freshness guard; match image/chart and intended source before changing the pin. |
| Prometheus wrapper conflicts with itself | Wrapper owns `flux-system/prometheus`; upstream child is `prometheus-stack` while Helm release name stays `prometheus`. A merged parent chartRef/child chart is invalid. |
| NATS-dependent app rollout times out | Check real broker Service DNS and its monitoring Service. KEDA uses `nats-jetstream`, not obsolete Kafka fields. Compare effective chart values with the deployed ScaledObject. |
| OpenSearch refuses downgrade | Preserve PVCs and retry forward to the observed newer metadata-compatible version. Do not delete data or force the older binary. |
| OpenSearch Pods Ready but only one member | Check normal discovery (not data single-node mode), port-9300 peer policy and actual cluster membership. Parent allowlists replace child arrays. |
| S3 host healthy but apps time out | Compare wired `.177` endpoint with DNS returning Wi-Fi `.202`; egress and endpoint must agree. Do not broaden LAN access to hide drift. |
| S3 lists buckets but reads fail after restart | Bound retries while volume registration settles, then verify known object bytes and metadata/mounts; listing readiness is insufficient. |
| CNPG SlowDownWrite / no online disks | Check `/srv/minio` mount UUID/device before credentials or policy. See [storage mount recovery](../docs/storage.md). |
| Deployment rejects Recreate strategy | Remove the incompatible `rollingUpdate` fields from effective values/rendered spec; inspect the published parent/child chart before retrying. |
| Longhorn PVC Pending | Inspect disk scheduling, capacity and expected disk UUID after VM recreation. Do not replace existing data to satisfy a new claim. |
| Drain blocked by Longhorn | Drain is not replica eviction; inspect PDBs and replica health. Deleting instance managers/PDBs is not a data-bearing-cluster recovery default. |
| Worker join missing JWS signature | Check controller-manager bootstrap signing and the exact token entry in cluster-info before retrying; see [bootstrap discovery](../docs/recovery.md#bootstrap-token-discovery). |
| CNI unauthorized / ContainerCreating | Check node-local stale Istio CNI configuration, Cilium and injection state before app code. Follow [node-local recovery](../docs/recovery.md#node-local-cni-and-longhorn-recovery) with healthy volume coverage. |
| New STRICT-mTLS workload unreachable | Verify namespace injection, native sidecar initContainers and actual certificate, Service ports and permitted identities. Ready alone is insufficient. |
| Control plane slow / leader election lost | Inspect local API/etcd health with verified trust, CPU/load and connection counts. A high-connection external watcher can saturate the Pi; identify it before restarting workloads. |
| vLLM GPU unavailable | Check real compute device and allocatable GPU, namespace/release ownership and any stale pre-Flux Helm release occupying the GPU. Do not remove the authoritative Flux release. |
| Model loading seems stuck | Inspect image/weight download and CUDA setup; use the pinned tested profile/eager mode and bounded cold-start allowance. Never silently switch model/profile for queued work. |
| Embeddings work singly, fail in a batch | Check gateway buffer and full eight-input response; preserve input bounds and truncation refusal. See [documents](../docs/documents.md). |
| Search appears semantic but is incomplete | Inspect actual query, canonical reconciliation, profile and returned coverage. Sorted nested hits may have null `_score`; validate score/ID sort values. Token hashes are not model embeddings. |
| Legacy pgvector index says no dimensions | Inspect the legacy column's fixed vector dimension; do not confuse that retained comparison path with the active OpenSearch pipeline. |
| Terraform image import refused | Use Proxmox `import`/`import_from`, not ISO; use `download_ubuntu_image=false` only after verifying the existing expected image. |
| VM failed after apply / identity mismatch | Inspect live `qm status/config`, tasks, memory headroom and old node identity before retry. Follow [worker recovery](../docs/recovery.md#worker-vm-recovery). |

Envoy/Gateway CRD ownership is split: Flux bootstrap owns Gateway API CRDs; the
Envoy wrapper owns Envoy Gateway GitRepository/Kustomization and a dedicated
OCI-backed HelmRelease for AI Gateway CRDs; child controller HelmReleases disable
their own CRD management. Inspect that
ownership before trying to repair conflicting installs.

For image pull failures check the actual pinned repository/tag (oauth2-proxy tags
include a leading `v`). Do not substitute floating images. For `helheim`, host
power profiles and worker NVIDIA modules are distinct from mere PCI visibility.

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

A pending row whose desired generation is no newer than its completed generation can be the old scanner bug: changed source identities were queued at default generation 1 and correctly deduplicated by the worker. Confirm source version/ETag, then use the normal document reprocess endpoint for those specific rows. The corrected scanner advances changed sources beyond both stored generations and preserves unchanged generations.

Before the session-lock worker protocol, a killed or scaled-down worker could leave `processing` indefinitely, with one JetStream message repeatedly rejected by healthy workers. Check actual ingest activity and retained history; do not clear completed generations or audit rows. New workers retain a PostgreSQL session lock through checkpointing, recover after owner loss, and fence late index/database writes. The first upgrade requires draining and pausing document consumption until every worker uses the new protocol. A successful index write alone does not prove the SQL checkpoint completed.

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

## Git push selects the wrong GitHub account

If a forwarded IDE credential helper selects the wrong account, inspect the account
named in GitHub's rejection. A per-command `credential.username=labiraus` and
`credential.useHttpPath=true` can select the existing authorized repository account.
These are account-selection hints, not new credentials or a permission bypass.
Do not print tokens, change global account selection or force-push around a refusal.

## Network Instability

Install the watchdog script and `systemd` timer on Proxmox:

```bash
scp scripts/network-watch.sh root@proxmox-node4:/usr/local/sbin/network-watch.sh
scp scripts/network-watch.service root@proxmox-node4:/etc/systemd/system/network-watch.service
scp scripts/network-watch.timer root@proxmox-node4:/etc/systemd/system/network-watch.timer
ssh root@proxmox-node4 'chmod 755 /usr/local/sbin/network-watch.sh && systemctl daemon-reload && systemctl enable --now network-watch.timer && systemctl status --no-pager network-watch.timer'
```

The watchdog derives the management path from the current default route. On a routed bridge such as `vmbr0`, it pings the gateway through the bridge interface and restarts the bridge member that is actually carrying the uplink instead of assuming the physical NIC is always named `nic0`.

On the newer `cdc_ncm` USB dongles used on `proxmox-node1` and `proxmox-node4`, the Linux bridge's implicit promiscuous handling was not enough to keep VM return traffic working reliably. A direct `ip link set dev <uplink> promisc on` on the physical bridge member restored bidirectional traffic from `yggdrasil` to the worker VMs. The watchdog now reapplies explicit promiscuous mode on the detected uplink before testing or bouncing it.

If a Proxmox host is migrated to a new USB Ethernet dongle, stop the watchdog before changing the bridge or hot-plugging multiple candidate uplinks. A stale watchdog that still targets the previous NIC can flap the healthy interface while you are trying to recover the host:

```bash
systemctl stop network-watch.timer network-watch.service
systemctl disable network-watch.timer
```

USB Ethernet adapters can also fail in ways that look like Kubernetes or Flux problems but are really host-level link issues. In the April 8, 2026 outage, a new ASIX `AX88179` dongle was detected by Linux and exposed as `enx...`, but it was not the active bridge member yet, and one host showed intermittent USB registration failures while the old Realtek uplink was still the live management path. When diagnosing a Proxmox node that vanished from the cluster, confirm the host uplink first:

```bash
ip -br link
ip -br addr
ip route
bridge link
ethtool <candidate-uplink>
journalctl -k -b | egrep -i 'usb|asix|r8152|cdc_ncm|link|carrier'
```

If a newly attached USB dongle reports `LOWER_UP` / `Link detected: yes` but the host still does not recover, verify whether `vmbr0` is still bridged to the old interface in `/etc/network/interfaces`. A healthy dongle that is not yet the bridge member will not carry management traffic until the bridge config is updated.

Useful follow-up checks on the host:

```bash
journalctl -k | egrep -i 'usb|asix|r8152|cdc_ncm|e1000e|link|carrier'
journalctl -u network-watch.service -u network-watch.timer
systemctl list-timers network-watch.timer
ip route show default
bridge link
UPLINK=$(ip route show default | awk '/default/ {print $5; exit}'); if [ -d "/sys/class/net/$UPLINK/brif" ]; then UPLINK=$(basename "$(find "/sys/class/net/$UPLINK/brif" -mindepth 1 -maxdepth 1 | head -n1)"); fi; ethtool "$UPLINK"
ip link set dev "$UPLINK" promisc on
```

## Local tool and fixture failures

| Symptom | Recovery boundary |
| --- | --- |
| PostgreSQL shuts down immediately after initial readiness | The image uses a temporary socket-only initdb server. Require `pg_isready -h 127.0.0.1` in the fixture before migrations. |
| PostgreSQL container healthy, published port unreachable | Use `WORLDBUILDER_TEST_TRANSPORT=exec` and the owned loopback bridge. A transport failure is not passing acceptance and never justifies a LAN DSN. |
| Docker exec exits 0 before completion | Use fixture-scoped exec-ID polling, terminal status and postconditions. Keep exact labels/names and original Docker CLI; prevent shim recursion. |
| Controller timeout | Inspect events/download progress, bounded deadlines and ownership cleanup. Ready Pod is not proof of webhook/Service routing readiness. See [fixture rules](../docs/testing.md). |
| Browser fails before tests | Install pinned Chromium libraries/fonts; prove standalone render/click first. Keep fixture identities local and inspect traces before attributing it to SQL. |
| Go/Helm cache permission failure | Use writable temporary `GOCACHE`, `GOMODCACHE`, `GOPATH` and `HELM_CACHE_HOME`; never disable checksum/TLS verification. |
| Missing CRD schema / empty helm show crds | Render `helm template --include-crds`; fail on missing custom schemas. Preserve source CRDs, including legal bare `=` enums. |
| Bootstrap render needs registry secret | Use an offline synthetic `--set-file imagePullSecret.dockerconfigjson=...`; JSON braces in `--set-string` can parse as Helm list syntax. Never deploy fixture credentials. |
| Empty principal list passes required | Validate `empty` and wildcard refusal explicitly; Helm required alone is insufficient for security-sensitive collections. |
| TFLint installer returns 404 | Remove duplicate installer; configured Terraform devcontainer feature owns TFLint. Rebuild the full feature-enabled container. |
| Old mc download returns 410 | Devcontainer uses checksum-pinned `weed`; explicit legacy MinIO automation still needs separately installed `mc`. This is not a completed migration. |
| Feature downloads fail behind trusted inspection | Host Node may need `NODE_EXTRA_CA_CERTS` for the existing trusted public CA as well as Docker's build trust. Never disable verification. |
| Backup seed rejects retired roles / misses tables | Update current Author-role seed and all-table physical inventory together; keep fast seed preflight and post-restore ledger/sequence denials. |
| World picker locator matches Reopen world | Use exact combobox accessible name, wait for navigation attachment and preserve API-browser traces separately. |
| Storage archive refuses chronology | Keep mixed listings and opaque versions; use explicit `minio-v2` for observed source encoding/cache markers. Never guess tie order or strip markers sent back to the provider. |
| Multipart migration interrupted | Preserve intent/journal and inspect possible committed versions and unknown upload IDs. Never retry into the populated target or guess an abort. Use a fresh destination after reconciliation. |
| Reference coverage refuses completion | Resolve export-bound context from frozen evidence; never rewrite audit identity or treat processingVersion as S3 version. See [storage](../docs/storage.md). |

An MCP HTTP proxy timeout must return useful nonempty error content; inspect the
upstream error and transport deadline instead of interpreting an empty tool result
as successful work. Do not run the unfinished `scripts/pg-update.sh` placeholder;
use the explicit numbered migration files and owner/grant procedure.
