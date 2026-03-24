# Platform Notes

- Kubernetes target: v1.34+
- GitOps controller: Flux
- Deployment packaging: Helm charts under `helm/`, published as OCI artifacts and reconciled by Flux
- Flux bootstrap groups: `flux-bootstrap`, `flux-apps`, `flux-workloads`, `flux-data`, `flux-observability`
- Upstream chart values source of truth: `values/` for upstream charts, plus chart-local `values.yaml` with optional environment overlays such as `values-ghcr.yaml` and `values-ecr.yaml`
- Storage default class: Longhorn
- S3-compatible object storage: MinIO tenant in-cluster
- MinIO state strategy: Ansible playbooks under `ansible/` (no Crossplane/Terraform)
- Secret strategy: generate from Ansible outputs and apply to Kubernetes

## Optional / disabled by default

- Plex is optional and not part of the baseline Flux bootstrap set by default.
- Harvester/KubeVirt resources are documentation-first because most deployments require dedicated hardware and node pools.
- Kafka is intentionally not installed. Add it after baseline stability using Strimzi operator or Redpanda operator.

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

Repo note:

- Keep `helm/bootstrap/flux-bootstrap/values.yaml` aligned with the live Longhorn setting so Flux does not drift the storage threshold back on the next bootstrap upgrade.

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
