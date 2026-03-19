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
