# Minecraft Troubleshooting

Use this file as the Codex memory for recurring issues and fixes specific to the Flux-managed Minecraft workload in `helm/workloads/minecraft/`.

## Current Deployment Shape

- namespace: `minecraft`
- chart: `helm/workloads/minecraft`
- image: `itzg/minecraft-server`
- mod loader: NeoForge
- modpack delivery: CurseForge via `MOD_PLATFORM=AUTO_CURSEFORGE`
- persistent data: PVC `minecraft-minecraft`
- current storage target: node-local PV on `jotunheim` at `/var/lib/minecraft-local`

## First Checks

```bash
kubectl get helmrelease -n flux-system minecraft
kubectl get pod,pvc,svc,cronjob -n minecraft
kubectl get pv minecraft-local-pv
kubectl logs -n minecraft deploy/minecraft --tail=200
kubectl describe pod -n minecraft <pod-name>
```

## Local PV Tradeoff For Minecraft

What we changed on April 4, 2026:

- the Minecraft chart switched from Longhorn-backed storage to a dedicated local PersistentVolume
- the local PV is pinned to node `jotunheim`
- the world path is `/var/lib/minecraft-local`

Why:

- the workload showed intermittent multi-second tick stalls even on LAN
- `spark` tick monitor showed real server ticks lasting roughly `1.4s`, `6.4s`, and `7.7s`
- GC pauses during the same window were only tens of milliseconds, which did not explain the freezes
- because the main architectural difference versus the known-good server was Kubernetes plus Longhorn-backed storage, local disk became the primary A/B test

Operator guidance:

- treat this as a latency optimization tradeoff, not a pure upgrade
- local PV removes Longhorn replication overhead from the hot world path
- local PV also removes automatic cross-node failover for this workload
- preserve off-cluster backups before changing or deleting the local PV path
- if the pod remains `Pending`, verify that `/var/lib/minecraft-local` exists on `jotunheim`

Useful checks:

```bash
kubectl get pv minecraft-local-pv
kubectl describe pv minecraft-local-pv
kubectl get pvc -n minecraft minecraft-minecraft -o yaml
kubectl get pod -n minecraft -o wide
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

## Stuck Backup Pod Causes Minecraft PVC Multi-Attach And Salvage Loops

Symptom:

- Minecraft pods alternate between `Pending`, `FailedAttachVolume`, and short-lived starts
- events show a `minecraft-world-backup` pod or cronjob holding the same RWO claim
- Longhorn shows the Minecraft volume as `degraded`, `unknown`, or repeatedly detaching/reattaching
- deleting the Minecraft pod alone does not recover the workload

What we observed on April 3, 2026:

- a stuck `minecraft-world-backup` job in namespace `minecraft` grabbed the `minecraft-minecraft` PVC
- the backup pod also had an injected Istio sidecar, so the job stayed active longer than expected
- Minecraft then hit repeated `Multi-Attach` errors against the same claim
- Longhorn auto-salvaged the volume and left it in a bad state with an orphaned `VolumeAttachment`
- after that, Minecraft could start briefly and then get killed, or remain pending against the broken attachment path

Operator guidance:

- treat this as a storage-attachment failure first, not a modpack or JVM failure
- inspect namespace events for `minecraft-world-backup`, `FailedAttachVolume`, and `BackOff`
- inspect the Longhorn volume for `AutoSalvaged`, `Remount`, `detached`, or `robustness: unknown`
- do not drop the Minecraft PVC by default; assume the world data must be preserved
- before any destructive reset, require human intervention to export the world data from the existing PVC:

```bash
./scripts/mc-debug.sh up
./scripts/mc-debug.sh status
./scripts/mc-debug.sh port-forward
```

- then use FileZilla or another FTP client to pull the relevant world files from the debug deployment before continuing
- after the export is verified, restore normal deployment ownership with:

```bash
./scripts/mc-debug.sh down
```

- only after that backup/export step is complete should a destructive reset be considered for this instance:

```bash
kubectl scale deployment -n minecraft minecraft --replicas=0
kubectl delete pod -n minecraft <pending-or-stuck-minecraft-pod> --ignore-not-found=true
kubectl delete pvc -n minecraft minecraft-minecraft
kubectl get volumeattachment | grep minecraft
kubectl delete volumeattachment <stale-volumeattachment-name>
kubectl apply -f <fresh-minecraft-pvc-manifest>
kubectl scale deployment -n minecraft minecraft --replicas=1
```

- if the PVC deletion finishes but the replacement pod stays unscheduled with no events, deleting that one fresh pending pod can clear the final post-reset limbo and let the ReplicaSet recreate it cleanly
- if the world data has not yet been exported, stop before deleting the PVC and inspect the backup job definition, Longhorn volume state, and `mc-debug` access path first

Current interpretation:

- the immediate crash path on April 3, 2026 was downstream of a stuck backup job plus Longhorn salvage churn on the shared RWO volume
- once the old PVC, orphaned `VolumeAttachment`, and broken Longhorn volume were removed, a fresh PVC provisioned successfully and the Minecraft pod resumed normal startup
- future recovery guidance for this repo is to preserve and export world data first, then wipe only with explicit human confirmation

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
- consider a wrapper script or image-level change if helper CLI timeouts need to be set explicitly during modpack bootstrap

Memory:

- `mc-image-helper install-neoforge --help` shows `--http-response-timeout` and `--tls-handshake-timeout` as supported CLI flags.
- `MC_IMAGE_HELPER_OPTS` is not the right hook for those flags in this image. On March 24, 2026, setting `MC_IMAGE_HELPER_OPTS="--http-response-timeout=PT5M --tls-handshake-timeout=PT1M"` caused Java startup to fail with `Unrecognized option`.

## NeoForge Server Fails Loading Datapacks With `Java heap space`

Symptom:

- server gets through bootstrap and mod loading, then fails during datapack load
- logs include:

```text
Failed to load datapacks, can't proceed with server load
java.lang.OutOfMemoryError: Java heap space
```

What this means:

- this is a JVM heap sizing problem, not a Flux, PVC, or Longhorn problem
- on March 24, 2026, ATM10 Sky on NeoForge 1.21.1 exhausted the default heap during datapack/resource enumeration
- warnings immediately before the crash such as `Initial datapack load took ...` are consistent with a very heavy pack approaching heap exhaustion

Repo mitigation:

- `helm/workloads/minecraft/values.yaml` sets explicit heap defaults:

```yaml
server:
  memory:
    max: 10G
    init: 2G
```

- `helm/workloads/minecraft/templates/deployment.yaml` passes those through as:

```yaml
- name: MEMORY
  value: "10G"
- name: INIT_MEMORY
  value: "2G"
```

- the chart also reserves node capacity for the pod:

```yaml
server:
  resources:
    requests:
      cpu: "4"
      memory: 12Gi
```

Operator guidance:

- if Minecraft fails with `Failed to load datapacks` plus `OutOfMemoryError`, increase heap before trying datapack safe mode
- if the pack changes significantly, revisit `server.memory.max`
- keep node capacity in mind when raising heap further
- if players can authenticate and join but then time out under heavy modpack load, check whether the pod is still `BestEffort`; reserve CPU and memory before chasing network causes

## Players Join But Disconnect A Few Minutes Later

Symptom:

- the player authenticates successfully
- the server logs `joined the game`
- the pod stays `Running`
- a minute or two later the session drops

What we observed on March 24, 2026:

- after fixing bootstrap/OOM/resource issues, Minecraft still had intermittent post-login disconnects
- the pod had an injected `istio-proxy` sidecar because the repo defaults namespaces to Istio injection
- the service and `TCPRoute` were valid, but raw Minecraft traffic still had an unnecessary extra proxy hop through Istio Gateway/Envoy
- upstream Minecraft-on-Kubernetes examples commonly expose the server with a direct `Service` of type `LoadBalancer` instead of a Gateway-managed TCP route
- Gateway API `TCPRoute` remains a more specialized path for raw TCP workloads, so using it by default added complexity without helping this game-server case

Repo mitigation:

- `helm/workloads/minecraft/templates/deployment.yaml` now sets:

```yaml
metadata:
  annotations:
    sidecar.istio.io/inject: "false"
```

- Minecraft now defaults to a direct `LoadBalancer` `Service`:

```yaml
service:
  type: LoadBalancer
```

- the chart keeps Gateway API support as an opt-in path instead of the default:

```yaml
route:
  enabled: false
```

Operator guidance:

- for this workload, prefer running Minecraft without an Istio sidecar unless you need mesh features specifically
- prefer a direct `LoadBalancer` `Service` for player traffic on this repo's cluster rather than routing Minecraft through Istio Gateway API
- prefer `externalTrafficPolicy: Local` on the Minecraft `LoadBalancer` service so LAN traffic stays on the node that actually hosts the single Minecraft pod instead of crossing an extra kube-proxy hop
- if you intentionally keep Minecraft on Gateway API, isolate it onto its own dedicated Gateway instance rather than sharing the generic internal gateway
- if you re-enable `route.enabled`, also re-enable `minecraftGateway.listeners` under `helm/bootstrap/flux-bootstrap/values.yaml` so the listener exists again
- if disconnects continue after the sidecar is removed, investigate the client path and modpack/server behavior separately from Kubernetes routing

What this helps with:

- if the Minecraft pod stays `Running`, server logs only show normal disconnects or `Timed out`, and the client sees `Connection reset by peer`, the extra cross-node forwarding path is a likely suspect
- `externalTrafficPolicy: Local` is a better default for this single-replica, long-lived TCP workload because it removes one forwarding hop from the session path

## Useful Commands

```bash
kubectl get helmrelease -n flux-system minecraft -o yaml
kubectl get svc -n minecraft minecraft -o wide
kubectl get pod -n minecraft -o wide
kubectl logs -n minecraft deploy/minecraft --tail=300
kubectl describe pod -n minecraft <pod-name>
kubectl get pvc -n minecraft
kubectl get volumes.longhorn.io -n longhorn-system
kubectl get nodes.longhorn.io -n longhorn-system -o wide
```
