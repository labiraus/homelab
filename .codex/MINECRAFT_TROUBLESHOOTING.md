# Minecraft Troubleshooting

Use this file as the Codex memory for recurring issues and fixes specific to the Flux-managed Minecraft workload in `helm/workloads/minecraft/`.

## Current Deployment Shape

- namespace: `minecraft`
- chart: `helm/workloads/minecraft`
- image: `itzg/minecraft-server`
- mod loader: NeoForge
- modpack delivery: CurseForge via `MOD_PLATFORM=AUTO_CURSEFORGE`
- persistent data: PVC `minecraft-minecraft`

## First Checks

```bash
kubectl get helmrelease -n flux-system minecraft
kubectl get pod,pvc,svc,cronjob -n minecraft
kubectl logs -n minecraft deploy/minecraft --tail=200
kubectl describe pod -n minecraft <pod-name>
```

## Flux Reconcile Fails On `Recreate`

Symptom:

- `HelmRelease` fails with `spec.strategy.rollingUpdate: Forbidden`

Cause:

- the live `Deployment` still has `strategy.type=RollingUpdate`
- the chart switched to `strategy.type=Recreate`
- Kubernetes rejects the patch unless `rollingUpdate` is explicitly cleared

Repo fix:

- [deployment.yaml](/workspaces/homelab/helm/workloads/minecraft/templates/deployment.yaml) sets:

```yaml
strategy:
  type: Recreate
  rollingUpdate: null
```

If the release is already wedged, stale Helm release state in `flux-system` can keep retries failing even after the chart is fixed. In that case, clear the stuck release state and let Flux reinstall cleanly.

## Longhorn Disk UUID Mismatch After Worker Redeploy

Symptom:

- Minecraft PVC stays `Pending`
- Longhorn volume shows replica scheduling failures
- Longhorn node disk status shows `DiskFilesystemChanged` / `DiskNotReady`
- disk status shows `storageAvailable: 0` and `storageMaximum: 0`

Likely cause in this repo:

- worker VMs were redeployed
- the rebuilt VM generated a fresh `/var/lib/longhorn/longhorn-disk.cfg`
- Longhorn still expected the old disk UUID for `/var/lib/longhorn`

Check:

```bash
kubectl get nodes.longhorn.io -n longhorn-system -o yaml
ssh -o StrictHostKeyChecking=no <node-name> 'sudo cat /var/lib/longhorn/longhorn-disk.cfg'
```

Recovery:

```bash
# restore the diskUUID Longhorn already expects into the node-local file
ssh -o StrictHostKeyChecking=no <node-name> 'sudo tee /var/lib/longhorn/longhorn-disk.cfg'

# then restart managers so they rescan
kubectl delete pod -n longhorn-system -l app=longhorn-manager
```

If a brand-new Minecraft PVC/Longhorn volume was created while all disks were unavailable, it can remain faulted even after the disk repair. In that case, delete the empty Minecraft pod/PVC/Longhorn volume and let Flux recreate them on healthy disks.

Memory:

- On March 22-24, 2026, worker VM redeploys regenerated node-local `longhorn-disk.cfg` files with new `diskUUID` values.
- Longhorn then rejected the rebuilt disks until the expected UUIDs were restored.

## NeoForge Or CurseForge Bootstrap Times Out

Symptom:

- container logs show NeoForge/CurseForge bootstrap failures during startup
- the error often looks like a timeout downloading Mojang metadata or `server.jar`
- example:

```text
java.net.SocketTimeoutException: Read timed out
Downloading minecraft server failed, invalid checksum.
[init] [ERROR] Failed to auto-install CurseForge modpack
```

What this means:

- this is happening inside the `itzg/minecraft-server` bootstrap flow, not in Flux or Kubernetes scheduling
- on a fresh PVC, the container has to download the CurseForge pack, NeoForge installer assets, Mojang metadata, and the Mojang `server.jar`
- if one of those upstream downloads stalls long enough, the container exits and Kubernetes restarts it

What we observed on March 24, 2026:

- the Minecraft pod failed once during NeoForge bootstrap while downloading Mojang assets
- after the restart, the workload reused the partially populated `/data` volume
- logs then showed many `Mod file ... already exists` lines, the NeoForge installer ran again, and the pod eventually reached `Running`
- the `HelmRelease` also became `Ready`

Current interpretation:

- this looks like a transient upstream network/read-timeout during first bootstrap, not a persistent config error in the chart
- because `/data` is persistent, a restart usually resumes from a much warmer state

Operator guidance:

- if the first bootstrap fails once with a Mojang or NeoForge read timeout, check whether the next restart is making progress before changing chart config
- if logs show `Mod file ... already exists`, the persistent volume is preserving the earlier downloads and the retry is likely worthwhile
- treat repeated failures across multiple restarts as an upstream fetch reliability issue, not a storage or Flux issue

Possible mitigations if this becomes frequent:

- keep the PVC intact so retries reuse downloaded artifacts
- avoid deleting the Minecraft PVC during troubleshooting unless storage itself is the problem
- consider pre-seeding the server artifacts or modpack contents into `/data` if first-boot network reliability continues to be poor
- expose image-helper timeout controls in the chart through `MC_IMAGE_HELPER_OPTS` if repeated read timeouts continue to happen during modpack bootstrap

Repo mitigation:

- [values.yaml](/workspaces/homelab/helm/workloads/minecraft/values.yaml) now sets `server.bootstrap.imageHelperOpts`
- [deployment.yaml](/workspaces/homelab/helm/workloads/minecraft/templates/deployment.yaml) exports that as `MC_IMAGE_HELPER_OPTS`
- current default: `--http-response-timeout=PT5M --tls-handshake-timeout=PT1M`

Memory:

- `mc-image-helper` consumes `MC_IMAGE_HELPER_OPTS`; a probe with an invalid option caused the helper JVM startup to fail immediately, confirming that the env var is honored by the image.

## Useful Commands

```bash
kubectl get helmrelease -n flux-system minecraft -o yaml
kubectl get pod -n minecraft -o wide
kubectl logs -n minecraft deploy/minecraft --tail=300
kubectl describe pod -n minecraft <pod-name>
kubectl get pvc -n minecraft
kubectl get volumes.longhorn.io -n longhorn-system
kubectl get nodes.longhorn.io -n longhorn-system -o wide
```
