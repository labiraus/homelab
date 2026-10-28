# Recovery

Begin with [read-only discovery](operations.md#read-only-discovery) and verify the
recovery target, backup identity and current ownership. Procedures here are not
claims of current health. Keep diagnostic artifacts private and record dated
results/remaining cleanup in [status](status.md).

Use `set -euo pipefail` for dependent shell steps or run each step separately and
inspect success. A failed render, missing Python dependency, API error or incomplete
restore must stop subsequent mutation. Use the prepared virtualenv where required.

## SSH trust recovery

The SSH user CA is separate from Kubernetes PKI. A lost SSH CA passphrase does not require recreating VMs, resetting kubeadm, or erasing cluster storage.

1. Preserve the old CA files and login certificate before generating an encrypted replacement CA. Store its new passphrase separately in a password manager.
2. Establish recovery access through an existing authorized login key, a physical/web console, or the Proxmox QEMU guest agent. A working Proxmox API token does not by itself prove host-shell access. With guest-agent execution, poll `exec-status` for terminal success and verify the resulting files.
3. Add the new **public** CA alongside existing trust. Terraform-managed Ubuntu guests use a `cert-authority,principals="ubuntu"` entry in `/home/ubuntu/.ssh/authorized_keys`. Manually configured hosts may instead use `TrustedUserCAKeys`; inspect the effective `sshd -T` configuration. Never copy the private CA key or its passphrase to a host.
4. Sign the existing login public key, then verify a fresh connection actually accepts the new `ED25519-CERT` identity and that required sudo access works. Keep existing trust until every intended host has been verified.
5. Install the replacement workstation CA at `~/.ssh/ssh_user_ca` (the devcontainer mounts `.devcontainer/ssh/` there), its public key beside it, and the new login certificate at `.devcontainer/ssh/id_ed25519-cert.pub`. Update the public CA entries in `etc/nodes/*.tfvars` for future provisioning. Do not apply Terraform merely to rotate live SSH trust: cloud-init changes can have VM lifecycle consequences.
6. After all hosts and recovery paths work, retire the old public CA trust explicitly. A changed SSH **host** key is a separate identity check; verify it through a trusted console or guest agent before changing `known_hosts`.

For Proxmox, inspect `/root/.ssh/authorized_keys` and its link to the cluster-managed `/etc/pve/priv/authorized_keys`. Adding a root-scoped CA entry to the shared file can restore all cluster hosts without restarting their workloads. Preserve unrelated entries and back up the file before editing it.

In `authorized_keys`, the line must begin with `cert-authority,principals="root"` (or the intended guest user). A bare `ssh-ed25519 ...` CA key is only a direct login-key entry and does not authorize certificates signed by that CA. This differs from a `TrustedUserCAKeys` file, which contains bare CA public keys. If a console command is copied through a UI that removes line breaks, use a single shell line with explicit separators and verify the installed entry afterward.

For a console where commands must be typed by hand, a temporary private-LAN HTTP endpoint can transfer the **public CA only**. Before installing it, compare `ssh-keygen -lf` against the exact fingerprint supplied through the trusted operator session; HTTP transport alone does not authenticate the key. Preserve the existing trust file and append the verified public key. Never serve private keys, passphrases, scripts to execute, or a directory listing. Time-limit the endpoint and stop it after verified SSH recovery.

Dated recovery evidence is recorded in [status](status.md).

## Physical control-plane recovery

`yggdrasil` is a physical Chromebook, outside Proxmox VM lifecycle. Read its
[filesystem and trust facts](infrastructure.md#physical-host-setup-facts). Ping or
accepted TCP without an SSH banner is not userspace health. Inspect physical console,
kernel logs, power, disk/mounts and memory before choosing a repair. An expired SSH
certificate, lost user-CA passphrase and Kubernetes PKI fault have different paths.
Do not rebuild the control plane or reset kubeadm to solve an unconfirmed SSH fault.

After host repair, independently validate SSH/sudo, kubelet/containerd/API, storage
and application-data integrity. The 11 October pre-outage metrics cannot rule out
later OOM/disk trouble; the 12 October API rediscovery did not diagnose root cause.

## Retention Defaults

- MinIO `documents` bucket: versioning enabled, current-object expiration disabled.
- Worldbuilder canon revisions, change sets, review/audit records, story turns/outcomes and outbox: preserve indefinitely; never expire unpublished events or pinned revisions.
- Postgres `rag.documents`: retained as the workflow source of truth.
- Legacy Postgres `rag.chunks` and `rag.embeddings`: retained until an explicit migration cleanup policy exists.
- OpenSearch RAG indexes: retained until an explicit reindexing or cleanup policy exists.
- `rag.document_lifecycle_events`: retained indefinitely as processing audit history.
- `rag.document_change_audits`: retained indefinitely as create/edit/revert audit history.
- assistant conversations, messages, memories, tool calls, file proposals, and proposal decisions: retained indefinitely and scoped by authenticated email at query time.
- NATS JetStream processing jobs and lifecycle notifications: operational transport only, not the durable retention layer.

## Cleanup Gate

Before enabling destructive lifecycle cleanup, update this file and the relevant chart/app docs with:

- the records or objects affected
- the retention duration
- the backup source that can restore them
- the restore procedure tested
- the user or operator behavior after cleanup
- the metrics or logs that prove cleanup is working

## Recovery Priorities

Recover in this order:

1. PostgreSQL app database including `worldbuilder`, `auth`, `rag`, `assistant` and native OAuth, plus the independent inference database/roles where enabled.
2. The verified active private S3 documents store, including all referenced versions; preserve both source and destination histories during migration.
3. Generated Kubernetes secrets used by app, MinIO, CNPG backup, and Flux registry access.
4. Flux app releases from the published OCI charts.
5. Derived chunks and embeddings through controlled reprocessing when needed.

Worldbuilder derived state can be rebuilt from PostgreSQL revision snapshots; document derived state can be rebuilt from raw S3 objects, but audit history and user-approved assistant state live in Postgres and should be treated as primary data.

### Database Pod stranded on an unavailable worker

Check the CNPG Cluster, the Pod's deletion timestamp/node, the `app-db-rw`
EndpointSlice readiness and the actual Longhorn engine/replica states. A
Terminating Pod can retain stale `1/1` readiness while its endpoint is not serving.
Confirm local `psql` transport through the normal Service forward; failure to reach
the old kubelet is transport failure, not a database query result.

Do not force-delete a database Pod or detach its volume merely because the node is
NotReady or SSH times out. Establish that the old machine is powered off or fenced
before allowing a replacement writer. Kubernetes' [non-graceful shutdown
procedure](https://kubernetes.io/docs/concepts/cluster-administration/node-shutdown/)
requires this before applying the `node.kubernetes.io/out-of-service` NoExecute
taint. Observe replacement scheduling, volume attachment, PostgreSQL recovery and
ready endpoints before checking registered-user lookup and authenticated app reads.
Remove the temporary taint only after verifying the old node's recovery. Retain
PVCs, replicas and existing backup sources throughout; a restore is a separate
recovery path, not the default response to a stranded Pod.

## Backup Baseline

Postgres recovery rehearsal assumes the repo-managed CNPG chart keeps both of these paths alive:

- WAL archiving to the verified active external store (source defaults use MinIO and `cnpg-backup-s3`; target uses the explicit SeaweedFS overlay)
- a `ScheduledBackup` resource for the `app-db` cluster so base backups exist alongside WAL files

Check that baseline before treating Postgres recovery as covered:

```bash
kubectl -n data get cluster app-db -o yaml
kubectl -n data get scheduledbackup
kubectl -n data get backups.postgresql.cnpg.io
```

The repo-native shortcut for this baseline is:

```bash
make document-platform-checks
```

Do not count this baseline as healthy unless:

- `status.conditions` on `app-db` shows `type=ContinuousArchiving`, `status=True`
- at least one `ScheduledBackup` exists for `app-db`
- the current WAL archive logs do not show repeated `SlowDownWrite` / `PutObject` failures against `svartalfheim`

## Verification

After recovery:

- run `documents.inventory.list` for a known prefix
- run `documents.history.list` for a known document
- run `documents.search` for a known gold query
- run `make ragas-chunking-eval` against the private gold cases
- run `make document-platform-checks`
- run `make vllm-gateway-smoke`
- open the Assistant tab and verify conversations, memories, proposals, and audit rows load for the authenticated user

## Database and identity restore

Restore to isolated storage with actual matching base-backup/WAL coverage. Compare
all populated tables, immutable history, grants/roles/ACLs/functions, triggers and
sequences; a desired restore resource or matching current-record projection is not
acceptance. Current Worldbuilder has 22 tables, including established-account ledger
and shared conversations, alongside the shared auth registry. Update the fixture
seed and physical table inventory whenever the schema evolves.

For an isolated CNPG rehearsal under Cilium, keep default-deny ingress and narrow
egress to DNS, the verified backup store and the Kubernetes API. An `ipBlock`
matching the control-plane address may not match Cilium's reserved API identity;
use a namespace-scoped `CiliumNetworkPolicy` with `toEntities: [kube-apiserver]`
and the observed API ports. Do not remove isolation to fix restore-job API timeouts.
See [Cilium API-server policy](https://docs.cilium.io/en/latest/security/policy/layer3/).

During a schema-8 to schema-10 rehearsal, first compare the restored database
exactly with the frozen source. Migration 009 converts legacy memberships to
Authors, which invokes the existing review-authority rotation trigger. Verify
that only converted memberships receive new authority IDs; preserve original
review rows and immutable revisions byte-for-byte. Compare all other old fields
and sequences, validate new policy/accounting defaults and dedicated-role denials,
then rerun the migrations/grants and require identical state. Apply auth grants
separately as the existing auth owner, close migration-owner login afterwards,
and restore every paused writer/release/scaler. Keep a post-upgrade backup.

Revoke restored native OAuth token families and pending grants before exposing a
restored authorization database; retain registered client identities and require
new consent. Ordinary process replacement must not reset credentials. Restore the
independent inference accounting database with cloud disabled and all unresolved
reservations retained. Never replace an already-used month's ledger with a fresh one.

The physical fixture proves latest and named PITR only after removing owned source
Pods/PVCs, checking credential refusal, comparing full-table/security fingerprints
and exercising restored denials and sequences. Production backups and actual disk/
power-loss behavior require separate evidence; see [testing](testing.md).

SeaweedFS requires independent backups of volume files and filer/master metadata;
a CNPG backup stored on it does not back up the S3 server. Follow the frozen migration
and rollback gates in [storage](storage.md). Velero file recovery requires completed
PodVolumeBackup/Restore, real supported local volumes and source data removal;
production CSI/Longhorn and historical Restic/Kopia compatibility remain separate.

## Worker VM recovery

Use this flow when a Terraform change requires a worker VM to be recreated or materially changed, especially for `bios`, `machine`, disk, memory, or `hostpci` updates.

## Before The Apply

1. Make the Terraform change in the relevant `etc/nodes/<node-layer>.tfvars`.

2. Drain only the node you are changing:

```bash
kubectl drain <node-name> --ignore-daemonsets --delete-emptydir-data --force
```

For Longhorn-backed clusters, expect drain to block on instance-manager PDBs until Longhorn has moved or dropped the remaining replicas. A disposable empty lab can remove a blocking instance-manager PDB/pod, but this is not a data-bearing-cluster procedure. Verify replica evacuation and recoverable data before proceeding.

3. Apply only that node layer:

```bash
ENV=lab bin/tf apply kubernetes <node-layer> --no-refresh
```

Examples:

```bash
ENV=lab bin/tf apply kubernetes node1 --no-refresh
ENV=lab bin/tf apply kubernetes node2 --no-refresh
ENV=lab bin/tf apply kubernetes node4 --no-refresh
```

4. Join or rejoin the rebuilt worker through Ansible:

```bash
make ansible-kubernetes-worker LIMIT=<node-name>
```

## If The VM Is Recreated

A recreated worker may come back as a fresh Ubuntu machine before Kubernetes is healthy again. The most common states are:

- `cloud-init` still running
- `kubelet` missing `/var/lib/kubelet/config.yaml`
- the old Kubernetes `Node` object still exists and shows `NotReady`

Check the guest first:

```bash
ssh <node-name> 'cloud-init status --long || true; systemctl is-active kubelet containerd cloud-final || true'
```

If `cloud-init` is still `running`, wait for it to finish before intervening. It is responsible for:

- installing `containerd`
- installing `kubelet`, `kubeadm`, and `kubectl`
- writing `/etc/default/kubelet`
- enabling `qemu-guest-agent`

If `cloud-init` completed but `kubelet` is missing its config or never joined, run the Ansible worker bootstrap for that node:

```bash
make ansible-kubernetes-worker LIMIT=<node-name>
```

If a stale node object from the old VM still exists, delete it first:

```bash
kubectl delete node <node-name>
```

If you already joined once and then deleted the stale node object, do a full reset and rerun the worker bootstrap so kubelet can register a fresh node identity:

```bash
ssh <node-name> 'sudo kubeadm reset -f'
make ansible-kubernetes-worker LIMIT=<node-name>
```

## If The Rebuilt Node Is Stuck `NotReady`

Check the node condition:

```bash
kubectl describe node <node-name>
```

Common case: `KubeletNotReady` with `cni plugin not initialized`.

Check the node-local networking pods:

```bash
kubectl get pods -n kube-system -o wide | egrep 'cilium|kube-proxy'
```

If the rebuilt node has stale Cilium or kube-proxy pods from before the VM recreation, delete the node-local pods and let the DaemonSets recreate them:

```bash
kubectl delete pod -n kube-system <pod-name> --grace-period=0 --force
```

If Cilium is stuck pulling or waiting on the image, pre-pull it on the node and then recycle the pod once:

```bash
ssh <node-name> 'sudo ctr -n k8s.io images pull <exact-image-from-current-daemonset>'
kubectl delete pod -n kube-system <cilium-pod> --grace-period=0 --force
```

If Cilium shows node-authorizer errors after a node object was deleted and recreated, stop trying to recover it piecemeal and re-bootstrap the worker cleanly:

```bash
ssh <node-name> 'sudo kubeadm reset -f'
make ansible-kubernetes-worker LIMIT=<node-name>
```

## If Terraform Says The VM Started But Kubernetes Is Still Stale

Sometimes Terraform loses the task stream even though Proxmox applied the VM change. Verify directly on the Proxmox host:

```bash
ssh proxmox-nodeX 'qm status <vmid>'
ssh proxmox-nodeX 'qm config <vmid>'
```

If the VM hardware looks correct and the guest is booting, prefer checking `cloud-init` and `kubeadm` state in the guest rather than re-running Terraform immediately.

If Terraform is hanging during refresh before making any live Proxmox change, it is usually safe to interrupt and retry with `--no-refresh` after manually checking the VM state.

## If Terraform Fails With `PCI device mapping not found`

This means the node layer references a Proxmox cluster PCI resource mapping that does not exist yet.

Check the current cluster mappings:

```bash
ssh proxmox-nodeX 'pvesh get /cluster/mapping/pci --output-format json-pretty'
```

For `node3`, the current worker tfvars expect these mappings:

```bash
ssh proxmox-node3 'pvesh create /cluster/mapping/pci --id node3-intel-igpu --map node=proxmox-node3,path=0000:00:02.0,id=8086:3e9b,subsystem-id=1028:08eb,iommugroup=0'
ssh proxmox-node3 'pvesh create /cluster/mapping/pci --id node3-rtx2070-gpu --map node=proxmox-node3,path=0000:01:00.0,id=10de:1f10,subsystem-id=1028:08eb,iommugroup=2'
ssh proxmox-node3 'pvesh create /cluster/mapping/pci --id node3-rtx2070-audio --map node=proxmox-node3,path=0000:01:00.1,id=10de:10f9,subsystem-id=1028:0000,iommugroup=2'
ssh proxmox-node3 'pvesh create /cluster/mapping/pci --id node3-rtx2070-usb --map node=proxmox-node3,path=0000:01:00.2,id=10de:1ada,subsystem-id=1028:0000,iommugroup=2'
ssh proxmox-node3 'pvesh create /cluster/mapping/pci --id node3-rtx2070-ucsi --map node=proxmox-node3,path=0000:01:00.3,id=10de:1adb,subsystem-id=1028:0000,iommugroup=2'
```

After the mappings exist, rerun the node apply:

```bash
ENV=lab bin/tf apply kubernetes node3 --no-refresh
```

Then rerun the Ansible worker bootstrap for that node:

```bash
make ansible-kubernetes-worker LIMIT=helheim
```

## If Terraform Or Proxmox Leaves A Broken VM Definition

An empty `qm config` is not sufficient permission to destroy a VM. Compare task
history, Terraform state, guest/disk ownership and recoverable data first. Preserve
the config and identify all attached volumes. Only for a confirmed disposable or
separately approved broken definition, unlock/stop/remove that exact VM and recreate
its node layer. Execute each step separately and inspect success; never hide a
failed stop/destroy with `|| true` and continue into apply.

## Sizing Guidance

Leave headroom on the Proxmox host for the host OS and QEMU overhead. Do not size a worker VM to nearly 100% of the host RAM.

For example, `proxmox-node2` has about `15 GiB` of RAM total, so `memory_mb = 15032` was too aggressive and caused QEMU to be killed by the host OOM killer during startup. Reducing the worker to `12288` MB left enough headroom for the host and allowed the VM to boot.

Likewise, `jotunheim` on `proxmox-node1` was too large at `memory_mb = 30064` on a host with about `31 GiB` usable RAM, especially with Intel iGPU passthrough enabled. That combination led to the host OOM killer terminating QEMU during VM startup. Reducing `jotunheim` to `24576` MB was temporarily workable while it was the only large guest on that host.

Current reality on `proxmox-node1` is tighter because it also runs the dedicated Minecraft VM `nidavellir` at `14336` MB. With both guests sharing the same 32 GiB laptop host, `jotunheim = 24576` MB plus `nidavellir = 14336` MB is too large and can again trigger host OOM kills during autostart. The current safer split is to keep `nidavellir` at `14336` MB for Minecraft and size `jotunheim` to `12288` MB so the host still has several GiB left for Proxmox and passthrough overhead.

`proxmox-node4` is much tighter at about `7.5 GiB` total RAM. `niflheim = 5368` MB left too little headroom for Proxmox, corosync, and QEMU overhead and was OOM-killed by the host on June 28, 2026. Keep `niflheim` at `4096` MB unless the host memory budget changes materially.

## Finish The Maintenance

After the node is back and `Ready`:

```bash
kubectl get nodes -o wide
kubectl uncordon <node-name>
```

Then re-check Longhorn separately:

```bash
kubectl get nodes.longhorn.io -n longhorn-system -o wide
kubectl get pods -n longhorn-system -o wide
```

Kubernetes node recovery and Longhorn node recovery are related but not identical. A worker can be `Ready` before Longhorn has fully recreated its per-node components.

If Longhorn still does not recover after the VM redeploy, check for disk identity drift:

```bash
kubectl get nodes.longhorn.io -n longhorn-system -o yaml
ssh <node-name> 'sudo cat /var/lib/longhorn/longhorn-disk.cfg'
```

Common post-redeploy failure mode:

- the Longhorn node object still expects the old disk UUID
- the rebuilt VM generated a fresh `/var/lib/longhorn/longhorn-disk.cfg`
- Longhorn marks the disk `DiskFilesystemChanged` / `DiskNotReady`
- affected disks show `storageAvailable: 0` and `storageMaximum: 0`
- new workload PVCs stay `Pending` even though the Kubernetes node itself is `Ready`

After verifying that the disk belongs to the rebuilt node and preserving its metadata, restore the expected `diskUUID` in `/var/lib/longhorn/longhorn-disk.cfg` on the rebuilt node, then restart the `longhorn-manager` pods so Longhorn rescans the repaired disk metadata.

## Bootstrap token discovery

A worker missing `/var/lib/kubelet/config.yaml` may never have completed join.
If Ansible reports missing `jws-kubeconfig-<token-id>` in `kube-public/cluster-info`,
the control plane created a token but has not signed discovery for it. Inspect
controller-manager/bootstrap-signing health and the ConfigMap before retrying;
repeating `kubeadm join` cannot fix a missing signature. Require the exact token's
JWS entry before rerunning `make ansible-kubernetes-worker LIMIT=<node-name>`.
Keep token values private. This is a control-plane discovery fault, not grounds
for resetting the physical control plane.

## Node-local CNI and Longhorn recovery

For `ContainerCreating` with `istio-cni ... Unauthorized`, inspect Pod events,
node concentration and actual Istio CNI DaemonSets. Remove a node-local CNI file
only after proving it is stale, isolating/draining that node and verifying volume
coverage. Back up the exact `/etc/cni/net.d/` files before moving them out, then
restart that node's containerd/kubelet and recreate only its Cilium Pod. Require
node/network and workload readiness before uncordoning. Do not remove configuration
from an active CNI deployment or weaken network/authentication policy.

For Longhorn scheduling refusal, inspect disk `Schedulable`, `DiskPressure`,
`DiskFilesystemChanged` and expected UUID, not just free bytes:

```bash
kubectl -n longhorn-system get nodes.longhorn.io -o yaml
kubectl -n longhorn-system get settings.longhorn.io \
  storage-over-provisioning-percentage storage-minimal-available-percentage default-replica-count -o yaml
```

Adjust capacity/replicas only against the verified physical budget. Keep reviewed
settings in the bootstrap chart so Flux cannot revert them; do not copy a historical
over-provisioning percentage. Keep the Longhorn chart pinned and its Helm timeout
long enough for bounded manager/CSI/engine downloads across active nodes. Only
new, confirmed empty faulted volumes are candidates for recreation; data-bearing
volumes require their own recovery. Disk UUID repair is described above.

## Flux reconciliation recovery

Inspect the controller Deployment's node selector before restarting it. A hard
`kubernetes.io/hostname` pin to an unavailable worker can leave every replacement
Pending. Preserve the current Deployment locally, identify the selector's owner,
and remove a diagnosed stale hostname restriction while retaining OS/security
constraints. The repo's `scripts/kube-init.sh` uses ordinary `flux install` without
a hostname pin. Verify the replacement's readiness and renewed leader lease;
restoring the controller can immediately resume already-published upgrades.

Inspect source/helm controller logs, OCI and HelmRelease dependencies, and compare
`status.observedGeneration` with `metadata.generation`. An old Ready condition
with lagging generation does not prove current reconciliation. Confirm actual
publication/digest and effective parent values before restarting controllers.
After a diagnosed controller repair, wait for rollout readiness, then reconcile
dependencies, source and release in that order, with bounded waits:

```bash
set -euo pipefail
flux reconcile helmrelease <dependency> -n <discovered-namespace> --timeout=5m
flux reconcile source oci <source> -n <discovered-namespace> --timeout=3m
flux reconcile helmrelease <release> -n <discovered-namespace> --with-source --timeout=5m
```

Require the intended attempted/installed revision and application behavior afterward.
Reconciliation of an old published artifact cannot deploy a local-only correction.

## Broker and generation recovery

Run `bash scripts/worldbuilder/test-jetstream.sh` on Linux with Go 1.25.6. The
harness builds NATS Server **2.11.9** using the public Go checksum database and
starts three loopback-only processes with separate temporary file stores. It
does not load operator environment files, accept a broker URL, or contact the
cluster. Initial dependency downloads require internet access. Processes and
stores are removed after the test; Linux parent-death signals also kill brokers
if the test process is terminated.

The orchestrator CI test job runs this drill before image/chart publication.
Ordinary `go test` skips it unless the harness supplies its server binary. A
skipped test alone is not recovery acceptance.

## Exercised behavior

- The real startup helper creates a three-replica, file-backed, seven-day
  limits-retention generation stream and a three-replica durable consumer.
- A metadata-only job dispatch reaches the actual worker HTTP adapter and a
  loopback fixture representing an incomplete checkpoint. The stream leader is
  killed before acknowledgement. The surviving majority elects a leader,
  retains message-ID deduplication and redelivers the original job reference.
- The adapter resumes that reference; the completed fixture response is
  acknowledged synchronously. Stopping a second server prevents a new publish
  from receiving a successful acknowledgement.
- The uncertain publication is retried with its original message ID after
  quorum recovery. The test verifies one stored copy rather than assuming that
  a timeout proves non-delivery.
- All three processes are killed and restarted using their existing stores.
  Original message bytes, sequence identities, deduplication and durable
  acknowledgement state survive. Completed work is not redelivered.
- Retention drift is refused without rewriting the existing stream; an
  existing single-replica consumer is refused. Unit cases also cover wrong
  subjects, memory storage, one replica and incompatible retention policies.

The consumer redelivery timer is shortened from the production two minutes to
one second after checking the production configuration. The drill follows the
[NATS quorum model](https://docs.nats.io/running-a-nats-service/configuration/clustering/jetstream_clustering)
and [acknowledgement semantics](https://docs.nats.io/using-nats/developer/develop_jetstream/model_deep_dive).

## Production gates retained

This is protocol and process-recovery evidence for the pinned test server.
It does not establish the installed NATS version, mesh connectivity, node or
Longhorn durability under power loss, seven days of elapsed retention, or live
provider behavior. The job runner here is a deterministic HTTP fixture;
PostgreSQL checkpoint, allowance and uncertain-provider recovery are exercised
separately by `test-postgres.sh`.

On activation, existing stream configuration must match its subjects, three
replicas, file storage and seven-day limits retention. Existing consumers must
match the requested three replicas and acknowledgement settings. Startup
refuses incompatible resources instead of silently changing their history.
Inspect and back up the actual stream/consumer under the
[LAN runbook](operations.md) before any deliberate repair. Do not delete broker
state to clear a startup error or automatically replay uncertain provider work.

## Persistent metric history

### Prepared storage settings

Select these inputs only after observing the installed stack, storage class, available capacity, ingestion rate and intended retention:

```yaml
stack:
  chartVersion: '92.3.0' # Tested source baseline; verify the installed target before selecting.
prometheusStorage:
  enabled: true
  storageClassName: REPLACE_WITH_OBSERVED_CLASS
  size: REPLACE_WITH_CAPACITY
  retention: REPLACE_WITH_RETENTION
```

`size` accepts positive whole `Mi`, `Gi` or `Ti` quantities; `retention` accepts positive whole `h`, `d` or `w` durations. The example deliberately fails validation until actual inputs replace its placeholders. The wrapper requires an exact chart version and an explicit storage class. It renders a ReadWriteOnce claim and keeps the admin API disabled. Base, GHCR and ECR values retain their existing defaults while disabled; enabling dashboard provisioning and storage together preserves both configurations.

The [Prometheus storage guidance](https://prometheus.io/docs/prometheus/latest/storage/) describes local durability and consistent snapshots. Size for ingestion, WAL/head data and compaction headroom; a retention duration is not a disk-space guarantee. The [operator storage contract](https://prometheus-operator.dev/docs/platform/storage/) explains PVC provisioning. Changing storage for an existing installation needs a reviewed migration and backup; publishing this opt-in does not move existing data or select a real storage class.

### Disposable acceptance

Use the Linux/Docker/Helm/Python prerequisites and source-render commands in [controller acceptance](testing.md) and [observability acceptance](operations.md). The API-server render pass now emits `storage-upstream-values.yaml` under its `renders/observability` directory. Run with a new output directory:

```bash
python3 scripts/worldbuilder/test-tsdb-unit.py
python3 scripts/worldbuilder/test-tsdb.py \
  "$artifacts/tsdb" "$artifacts/renders/observability" "$artifacts/metrics.txt"
```

The mandatory `tsdb-restore` platform job runs alongside the existing controller, CNPG, Velero and runtime-observability jobs before callers publish images/charts. It uses the same pinned kind/Kubernetes and kube-prometheus-stack inputs and the same fixture ownership, exec-status transport and teardown checks. Only synthetic metrics, public renders, hashes and logs are uploaded; kubeconfig and private certificates stay outside artifacts.

The fixture uses the source-rendered claim and retention settings with kind's `standard` storage class. It adds the local-volume annotation and enables the admin API only inside its owned cluster. It then requires:

- Actual Go counter and histogram samples plus a phase marker, followed by stopped producers and a quiescent, fixed-time raw sample matrix.
- A bound PVC/PV with verified claim identity, live WAL files and no pre-existing compacted blocks. A new Prometheus Pod UID on the same PVC must replay WAL and return the exact original labels, timestamps and values.
- A real [snapshot including head samples](https://prometheus.io/docs/prometheus/latest/querying/api/#snapshot), with complete block metadata, indexes, chunks and SHA-256 file hashes. Unsafe paths, links, missing data, unexpected live-WAL layout, changed content and oversized archives are refused before extraction.
- Positive evidence of samples written after the snapshot, then removal of the source workload, PVC, PV and its actual data path.
- Restore to a different volume, byte/hash comparison before startup, the exact same Prometheus image digest, no scrape jobs and no admin API. Restored history must match exactly and omit later samples.
- An intentionally corrupted index rejected by Prometheus itself, followed by recovery from the verified snapshot on a fresh PVC.
- Successful owned-cluster teardown after all assertions. A failed check or failed cleanup cannot pass acceptance.

Read `tsdb-result.json`, `snapshot-manifest.json`, `source-wal.log`, `corrupt-index.log`, runtime/cleanup logs and the final process/workflow conclusion together. The archive parser's 32 MiB compressed / 64 MiB expanded limits are deliberate bounds for synthetic fixtures, not production backup limits or a general restore utility.

The restored Service route can lag API-server proxy readiness. Retry only initial curl connection refusal (exit 7) for at most 45 seconds, then require the explicit disabled-admin-API response. DNS, timeout, TLS, malformed responses or an enabled API fail; connection failure is never isolation acceptance. Dated CI evidence is in [status](status.md).

### Remaining live gates

This verifies the prepared storage contract and recovery mechanism with synthetic data. It does not prove installed versions, Longhorn replication, physical node/disk failure, realistic ingestion/capacity, backup scheduling/transport/custody, production inventory or cross-version compatibility. Admin snapshot access remains disabled in published settings; any production backup procedure needs its own authenticated, restricted access and external backup retention plan. The [LAN runbook](operations.md) remains authoritative for discovery and controlled activation.
