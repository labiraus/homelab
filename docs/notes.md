# Platform Notes

- Kubernetes target: v1.34+
- GitOps controller: Flux
- Deployment packaging: Helm charts under `helm/`, published as OCI artifacts and reconciled by Flux
- Flux bootstrap groups: `flux-bootstrap`, `flux-infra`, `flux-apps`, `flux-workloads`, `flux-data`, `flux-observability`
- Upstream chart values source of truth: `values/` for upstream charts, plus chart-local `values.yaml` with optional environment overlays such as `values-ghcr.yaml` and `values-ecr.yaml`
- Storage default class: Longhorn
- S3-compatible object storage: external MinIO on `svartalfheim`
- MinIO state strategy: Ansible playbooks under `ansible/` (no Crossplane/Terraform)
- Secret strategy: generate from Ansible outputs and apply to Kubernetes

## Optional / disabled by default

- Plex is optional and not part of the baseline Flux bootstrap set by default.
- Harvester/KubeVirt resources are documentation-first because most deployments require dedicated hardware and node pools.
- NATS JetStream is available through the Flux-managed upstream Helm chart; treat it as async execution infrastructure rather than a source of truth.

## Troubleshooting

### Broken `istio-cni` on a node (pods stuck in `ContainerCreating`)

If pods are stuck in `ContainerCreating` after a node reboot/offline event, check for stale Istio CNI config.

Detect:

```bash
# Look for sandbox/CNI errors on stuck pods
kubectl -n flux-system describe pod <pod-name> | grep -A3 FailedCreatePodSandBox

# Typical error:
# plugin type="istio-cni" name="istio-cni" failed (add): Unauthorized

# See if failures are concentrated on one node
kubectl get pods -A -o wide | grep ContainerCreating

# Confirm Istio CNI is not actually running
kubectl get ds -A | grep istio-cni
kubectl get pods -A | grep istio-cni
```

Fix (example bad node: `niflheim`):

```bash
# 1) Isolate and evacuate the node
kubectl cordon niflheim
kubectl drain niflheim --ignore-daemonsets --delete-emptydir-data --force

# 2) On the node (SSH), remove stale Istio CNI config and restart runtimes
sudo mkdir -p /root/cni-backup-$(date +%F-%H%M)
sudo mv /etc/cni/net.d/*istio* /root/cni-backup-$(date +%F-%H%M)/ 2>/dev/null || true
sudo systemctl restart containerd kubelet

# 3) Recreate Cilium pod on that node
kubectl -n kube-system get pod -o wide | grep niflheim | grep cilium
kubectl -n kube-system delete pod <cilium-pod-on-niflheim>

# 4) Return node to service
kubectl uncordon niflheim
```

Verify recovery:

```bash
kubectl get pods -A -o wide | grep niflheim
kubectl -n flux-system get pods
flux reconcile source oci flux-bootstrap -n flux-system --timeout=5m
```

### Stuck Flux components (`reconcile` hangs or revisions do not advance)

Use this when `flux reconcile ...` hangs, a `HelmRelease` stays on an older revision, or dependencies remain "not ready" after underlying pods are healthy.

Detect:

```bash
# 1) Controller health
kubectl -n flux-system get pods -o wide

# 2) Current status for source + release
flux get source oci -n flux-system
flux get helmrelease -n flux-system

# 3) Check dependency chain and stale status
kubectl -n flux-system get helmrelease <name> -o yaml
# Look for:
# - dependency '<ns>/<name>' is not ready
# - status.conditions Ready=True but observedGeneration < metadata.generation

# 4) Look for controller/runtime errors
kubectl -n flux-system logs deploy/source-controller --since=30m --tail=300
kubectl -n flux-system logs deploy/helm-controller --since=30m --tail=300
```

Common signs and meaning:

- `dependency '<ns>/<dep>' is not ready`: reconcile is blocked by `dependsOn`.
- `connect: connection refused` to `source-controller...`: source-controller pod/service is unhealthy.
- `observedGeneration` behind `metadata.generation`: stale controller state; status has not caught up.

Recovery order:

```bash
# 1) Ensure source-controller and helm-controller are healthy
kubectl -n flux-system rollout restart deploy/source-controller deploy/helm-controller
kubectl -n flux-system rollout status deploy/source-controller --timeout=180s
kubectl -n flux-system rollout status deploy/helm-controller --timeout=180s

# 2) Reconcile dependency first
flux reconcile helmrelease <dependency-name> -n flux-system --timeout=5m

# 3) Refresh source for the target release
flux reconcile source oci <release-name> -n flux-system --timeout=3m

# 4) Reconcile target release with source
flux reconcile helmrelease <release-name> -n flux-system --with-source --timeout=5m
```

Verify:

```bash
kubectl -n flux-system get helmrelease <release-name> -o jsonpath='{.status.lastAttemptedRevision}{"\n"}'
flux get helmrelease <release-name> -n flux-system
```

### App releases time out with `Deployment ... status: 'InProgress'` after NATS-dependent startup changes

Use this when app `HelmRelease` objects like `external`, `mcp`, `orchestrator`, or `processor` stop reconciling even though their `OCIRepository` is healthy.

Detect:

```bash
# 1) Confirm Flux fetched the chart but the rollout stalled
flux get source oci -n flux-system
flux get helmrelease -n flux-system
kubectl describe helmrelease -n flux-system <release-name>

# 2) Check the newest pod revision
kubectl -n homelab get deploy,pod | grep <release-name>
kubectl -n homelab logs deploy/homelab-<release-name> --all-pods --previous --tail=100

# 3) Check the actual NATS service name
kubectl -n nats get svc
```

Common signs:

- `OCIRepository` is `Ready=True`, but the `HelmRelease` times out waiting on the Deployment.
- container logs show `nats.Connect: dial tcp: lookup nats.nats.svc.cluster.local ... no such host`
- container logs show `flushing nats connection: nats: context requires a deadline`
- the live broker service is `nats-nats` rather than `nats`

Recovery:

```bash
# Update app chart values to use the real in-cluster DNS name
nats://nats-nats.nats.svc.cluster.local:4222

# For processor monitoring, also use:
nats-nats-headless.nats.svc.cluster.local:8222

# Then publish the chart and let Flux retry, or reconcile manually after publication
flux reconcile source oci <release-name> -n flux-system --timeout=3m
flux reconcile helmrelease <release-name> -n flux-system --with-source --timeout=10m
```

### MCP HTTP proxy tools return empty error content after a timeout

Use this when `documents.scanBucket`, `documents.reprocess`, `documents.curation.update`, or `documents.editText` appears in the live MCP manifest but `tools/call` returns `isError: true` with an empty text body.

Common signs:

- direct calls to `homelab-orchestrator` work
- MCP inventory/search tools still work because they use direct Postgres execution
- orchestrator-backed MCP tools wait until the MCP HTTP client times out

Check the MCP NetworkPolicy. Kubernetes NetworkPolicy egress is evaluated against the selected destination pod and port, so an HTTP call to service port `80` can still require egress to the orchestrator pod `targetPort` `8080`.

Recovery:

```bash
kubectl -n homelab get svc,endpoints homelab-orchestrator
kubectl -n homelab get networkpolicy homelab-mcp -o yaml
```

The `homelab-mcp` NetworkPolicy should allow egress to pods labeled `app.kubernetes.io/instance=homelab-orchestrator` and `app.kubernetes.io/name=orchestrator` on TCP `8080`.

### Helm upgrade fails after switching a workload to `Recreate`

Use this when a `HelmRelease` starts failing with an error like `spec.strategy.rollingUpdate: Forbidden` after a chart changes a Deployment from `RollingUpdate` to `Recreate`.

Detect:

```bash
kubectl describe helmrelease -n flux-system <release-name>
kubectl get deployment -n <workload-namespace> <deployment-name> -o yaml
```

Common signs:

- Helm reports `Deployment.apps "<name>" is invalid: spec.strategy.rollingUpdate: Forbidden: may not be specified when strategy type is 'Recreate'`.
- The live Deployment still shows `strategy.type: RollingUpdate` with a populated `rollingUpdate` block.
- Other resources from the same release can be missing because the upgrade never completed, for example a PVC that the pods need to schedule.

Recovery:

```bash
# In the chart, explicitly clear rollingUpdate when using Recreate
strategy:
  type: Recreate
  rollingUpdate: null

# Then publish the chart and let Flux retry, or reconcile manually
flux reconcile source oci <release-name> -n flux-system --timeout=3m
flux reconcile helmrelease <release-name> -n flux-system --with-source --timeout=10m
```

### Longhorn PVC stuck `Pending` for new workloads

Use this when a new workload never installs and the `HelmRelease` times out waiting on a `Deployment`, while its PVC stays `Pending`.

Detect:

```bash
# 1) Check the blocked release
kubectl describe helmrelease -n flux-system <release-name>

# 2) Check the workload namespace
kubectl get deploy,pod,pvc -n <workload-namespace>
kubectl describe pvc -n <workload-namespace> <claim-name>

# 3) Check Longhorn node schedulability
kubectl get nodes.longhorn.io -n longhorn-system -o yaml
kubectl get settings.longhorn.io -n longhorn-system \
  storage-over-provisioning-percentage \
  storage-minimal-available-percentage \
  default-replica-count -o yaml
```

Common sign:

- Longhorn node disk status shows `type: Schedulable` with `status: "False"` and `reason: DiskPressure`, even though `storageAvailable` is still non-zero. In that state, new PVCs can remain `Pending` with no useful PVC events.
- After worker VM redeploys, Longhorn node disks can instead show `reason: DiskFilesystemChanged` with messages like `record diskUUID doesn't match the one on the disk`, plus `storageAvailable: 0` and `storageMaximum: 0`. In that state, new PVCs also remain `Pending` because Longhorn no longer trusts the rebuilt VM's `/var/lib/longhorn` disk identity.

Recovery:

```bash
# Raise the over-provisioning threshold enough for the next volume to schedule
kubectl patch settings.longhorn.io -n longhorn-system \
  storage-over-provisioning-percentage \
  --type=merge \
  -p '{"value":"135"}'

# Then let Flux retry, or reconcile manually
flux reconcile helmrelease <release-name> -n flux-system --with-source --timeout=10m
```

If the failure mode is `DiskFilesystemChanged` after worker redeploys:

```bash
# 1) Compare the disk UUID Longhorn expects with the rebuilt node's local file
kubectl get nodes.longhorn.io -n longhorn-system <node-name> -o yaml
ssh -o StrictHostKeyChecking=no <node-name> 'sudo cat /var/lib/longhorn/longhorn-disk.cfg'

# 2) If the rebuilt VM generated a new UUID, restore the UUID Longhorn already knows
#    in /var/lib/longhorn/longhorn-disk.cfg on that node, then restart longhorn-manager
kubectl delete pod -n longhorn-system -l app=longhorn-manager

# 3) Re-check Longhorn disk readiness and recreate any brand-new faulted PVC/volume if needed
kubectl get nodes.longhorn.io -n longhorn-system -o yaml
kubectl get volumes.longhorn.io -n longhorn-system
```

Repo note:

- Keep the upstream Longhorn chart pinned in `helm/bootstrap/flux-bootstrap/values.yaml`; bump it intentionally rather than letting Flux follow every latest chart release.
- Keep the Longhorn HelmRelease timeout explicit and generous; chart upgrades pull manager, CSI, engine, and instance-manager images across every active storage node, and a 10 minute Helm timeout can fail even when the cluster is recovering normally.
- Keep `helm/bootstrap/flux-bootstrap/values.yaml` aligned with the live Longhorn setting so Flux does not drift the storage threshold back on the next bootstrap upgrade.
- Treat `DiskFilesystemChanged` as a likely post-redeploy VM identity problem before treating it as generic storage exhaustion. In this repo, the March 22, 2026 worker redeploys regenerated `/var/lib/longhorn/longhorn-disk.cfg` with new `diskUUID` values and made Longhorn reject the rebuilt node disks until the expected UUIDs were restored.

### Control plane lease timeouts during reconciliation

Use this when controllers repeatedly restart or lose leader election with API requests timing out against the local apiserver, for example `kube-controller-manager` or `kube-scheduler` logs showing `failed to renew lease` / `context deadline exceeded`.

Detect:

```bash
kubectl get pods -n kube-system -o wide
kubectl get --raw='/readyz?verbose'
kubectl logs -n kube-system kube-controller-manager-yggdrasil --previous --tail=120
kubectl logs -n kube-system etcd-yggdrasil --since=30m --tail=120
kubectl exec -n kube-system etcd-yggdrasil -- etcdctl \
  --command-timeout=30s \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  endpoint status --write-out=table
```

Recovery:

```bash
kubectl exec -n kube-system etcd-yggdrasil -- etcdctl \
  --command-timeout=30s \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  defrag
```

After defrag, re-check `endpoint health` and wait for the static control-plane pods to clear their CrashLoopBackOff backoff window. On May 6, 2026, etcd defrag reduced `yggdrasil` etcd DB size from 60 MB to 25 MB and endpoint health latency from about 1.15s to about 265ms, which allowed controller reconciliation to resume.

If you edit static pod manifests on `yggdrasil`, do not leave backup files under `/etc/kubernetes/manifests`. Kubelet treats that directory as live input, so a `*.yaml.bak` copy with the same pod name can race the intended manifest and make kubelet revert to the old static-pod hash. Move backups to a sibling directory such as `/etc/kubernetes/manifest-backups/<date>/`.

### Worker GPU passthrough current state

After the recent worker rebuilds, all three Kubernetes worker guests can now see a passed-through Intel iGPU in `lspci`:

- `jotunheim`: Alder Lake Iris Xe `[8086:46a8]`
- `alfheim`: Alder Lake Iris Xe `[8086:46a8]`
- `niflheim`: Tiger Lake Iris Xe `[8086:9a49]`

But all three currently expose only `/dev/dri/card0` and do not expose a render node such as `/dev/dri/renderD128`. Treat that as:

- `node-gpu=passthrough`
- `node-llm=none`
- `node-llm-class=igpu`
- `node-llm-vram=shared`

Operationally, that means PCI passthrough is working, but the guests are not yet ready for LLM workloads that need a usable compute/render device.

For future discrete GPUs, use the relative labels to steer inference workloads:

- `node-llm-class=consumer-gpu` for GeForce-class cards
- `node-llm-class=high-vram-gpu` for cards that should be preferred for larger models
- `node-llm-vram=<tier>` such as `12gb`, `16gb`, `24gb`, or `48gb`
