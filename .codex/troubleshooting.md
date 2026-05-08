# Troubleshooting Learnings

## Terraform And Proxmox

### Worker VM image import on Proxmox

- Worker boot disks must be imported from a Proxmox `import` image, not an `iso`.
- In this repo, the Ubuntu cloud image should be downloaded as `content_type = "import"` and referenced from the VM disk with `import_from`.
- A source like `local:iso/ubuntu-24.04-noble-server-cloudimg-amd64.img` fails for VM creation. Use an importable disk image such as `ubuntu-24.04-noble-server-cloudimg-amd64.qcow2`.
- If Proxmox already has `ubuntu-24.04-noble-server-cloudimg-amd64.qcow2` in the datastore, set `download_ubuntu_image = false` in the relevant node layer so Terraform reuses the existing import image instead of failing with `refusing to override existing file`.

### Safe interpretation of Terraform hangs

- If Terraform is hanging during provider refresh and Proxmox shows no live mutation, it is usually safe to interrupt and retry with `--no-refresh` after manually checking `qm status` and `qm config`.
- Do not assume every hang is harmless. Once Terraform reaches create or destroy, verify the live Proxmox VM state before retrying.

### Broken Proxmox VM definitions

- A failed create can leave an empty or partial Proxmox VM config behind.
- If `qm config <vmid>` is empty or unusable, clean it up with:

```bash
qm unlock <vmid> || true
qm stop <vmid> || true
qm destroy <vmid> --purge 1 || true
```

- After cleanup, recreate the node from Terraform rather than trying to salvage the partial VM.

### Proxmox host memory headroom matters

- Do not size a worker VM to nearly 100% of the Proxmox host RAM.
- `alfheim` failed to boot at `memory_mb = 15032` on a host with about `15 GiB` total memory because QEMU was killed by the host OOM killer.
- Leaving host headroom fixed it; `12288` MB was stable.

### Proxmox USB uplinks can fail below Kubernetes

- If several worker nodes suddenly become `NotReady` at once, check the Proxmox host uplinks before debugging Cilium, Flux, or the workloads.
- On April 8, 2026, the real failure mode was host-side USB Ethernet instability plus bridge drift, not a cluster-wide Kubernetes regression.
- A new USB dongle can be detected successfully and still not be the live management path if `/etc/network/interfaces` keeps `vmbr0` bridged to the old NIC.
- The quickest host checks are:

```bash
ip -br link
ip -br addr
ip route
bridge link
ethtool <candidate-uplink>
journalctl -k -b | egrep -i 'usb|asix|r8152|cdc_ncm|link|carrier'
```

- If the host uses a bridge such as `vmbr0`, reason about two interfaces separately:
  - the routed interface that holds the IP and default route
  - the bridge member that is the actual physical uplink
- Do not assume the physical uplink is always named `nic0`. USB adapters often appear as `enx...` names and can change across hosts and recovery attempts.
- The `network-watch.sh` watchdog should derive the active uplink from the current default route. A watchdog hard-coded to the wrong NIC can bounce the healthy path and make host recovery worse.
- On April 8, 2026, `proxmox-node1` and `proxmox-node4` still dropped VM return traffic even after the new `cdc_ncm` dongles were bridged correctly. The durable fix was to put the physical uplink into explicit user-requested promiscuous mode:

```bash
ip link set dev <uplink> promisc on
```

- The key symptom was asymmetric traffic:
  - the guest could open SYNs toward `192.168.8.132:6443`
  - `yggdrasil` showed many `SYN-RECV` sockets from the guest IPs
  - but `yggdrasil` could not ping or SSH the guests until promisc was enabled directly on the USB uplink
- Keep this behavior in mind for `cdc_ncm` bridge uplinks. Bridge membership alone may not be enough; the physical USB NIC may need explicit promisc reapplied after link bounces or reboot.

## Kubernetes Worker Recovery

### Flux can be green in git but red in-cluster when the published OCI chart is bad

- Flux in this repo deploys published OCI chart artifacts, not the local chart files in the workspace.
- If `helm template` is clean locally but a `HelmRelease` is still failing in-cluster, inspect the paired `OCIRepository` before assuming the workspace still contains the bug.
- On April 9, 2026, `external`, `mcp`, and `ui` were stuck because Flux had already fetched newer bad OCI chart tags even though the local templates rendered cleanly again.
- The practical live workaround was to patch the `OCIRepository.spec.ref` from semver tracking to an exact known-good `tag`, then reconcile the source and `HelmRelease`.
- `oauth2-proxy` was a separate packaging bug: the upstream image tags use a leading `v`, so `quay.io/oauth2-proxy/oauth2-proxy:7.12.0` failed with `ErrImagePull` while `v7.12.0` was the valid tag shape.
- On April 12, 2026, `processor` was red even though the workspace had already switched to NATS JetStream because the live `OCIRepository` was still pinned to an older pre-NATS chart digest. The tell was a `ScaledObject` that did not match the current chart values, while the repo had already moved the chart to:

```yaml
type: nats-jetstream
natsServerMonitoringEndpoint: nats-nats-headless.nats.svc.cluster.local:8222
```

- If that mismatch appears again, the fix is not in the current workspace templates; it is to publish or repoint Flux at the newer processor chart artifact so KEDA uses the NATS JetStream scaler.
- On May 7, 2026, `orchestrator` and `processor` were crash looping because their live child `HelmRelease` and `OCIRepository` objects retained stale Kafka-era fields even though the parent `flux-apps` chart rendered `values: {}` and semver-tracking OCI sources. The live repair was to replace each child `spec.values` with `{}`, remove `spec.ref.digest` from the paired app `OCIRepository`, then annotate both the source and release with a fresh `reconcile.fluxcd.io/requestedAt` value.
- If `processor` starts but logs `column does not have dimensions` while creating `rag_embeddings_vector_cosine_idx`, check `rag.embeddings.vector`. pgvector HNSW indexes require a fixed dimension such as `vector(384)`, not an unconstrained `vector` column.

### Parent-owned CRDs reduce child release drift

- For upstream stacks that bundle both controllers and CRDs, prefer Flux-owned CRD sources in the wrapper chart over letting the child controller release claim CRDs directly.
- In this repo, the intended ownership split for Envoy Gateway and Envoy AI Gateway is:
  - `flux-bootstrap` owns Gateway API CRDs
  - the `envoy-ai-gateway` wrapper owns Envoy Gateway CRDs through a `GitRepository` + `Kustomization`
  - the `envoy-ai-gateway` wrapper owns Envoy AI Gateway CRDs through an `OCIRepository` + `Kustomization`
  - the child controller `HelmRelease` resources install with CRD management disabled

### Control-plane instability can be an external API client problem

- If Flux rollouts stop progressing, `kubectl` begins timing out on `yggdrasil`, and controller pods start losing leader election, do not assume the failing app is the root cause.
- On April 9, 2026, the actual bottleneck was the control-plane host `yggdrasil` itself:
  - local `https://127.0.0.1:6443/livez` and `readyz` checks were slow or timing out
  - `readyz` reported only `etcd` failing when it did answer
  - `kube-apiserver`, `etcd`, `kube-controller-manager`, and `kube-scheduler` were all CPU-hot on the host
  - `vmstat` showed CPU saturation with little or no I/O wait
- A high-connection external watcher can be enough to push the single control-plane host over the edge. In that incident, `192.168.8.140` (`UK-CVMSCC4.lan`) held about 62 concurrent connections to the API server and was the busiest external client by far.
- Before restarting workloads, check the control plane directly from `yggdrasil`:

```bash
curl -k -m 10 -s -o /dev/null -w "%{http_code} %{time_connect} %{time_starttransfer} %{time_total}\n" https://127.0.0.1:6443/livez
curl -k -m 15 https://127.0.0.1:6443/readyz?verbose
uptime
vmstat 1 5
ss -ant | grep -c ':6443'
ss -ant | grep -c ':2379'
ss -ant | grep ':6443' | awk '{print $5}' | sed 's/.*ffff://; s/]//; s/\[//' | cut -d: -f1 | sort | uniq -c | sort -nr | head
```

- If one external machine dominates the API connections, stop or reboot that watcher first before deciding the cluster needs a node reboot.

### Istio ext-auth drift can look like generic RBAC denies

- The current browser-facing path no longer relies on Istio `CUSTOM` auth for `ui` and `external`. `oauth2-proxy` now owns `/` and `/api` as a reverse proxy, while `mcp` keeps its own route space.
- If `mcp.labiraus.com` starts returning a plain `RBAC: access denied` instead of redirecting browsers to Google, inspect the Istio `oauth2-proxy` extension provider before debugging the app routes.
- In the current Flux bootstrap flow, the live top-level `flux-system/istio` HelmRelease is seeded from `helm/bootstrap/flux-bootstrap/values.yaml`. Keeping `helm/bootstrap/istio/values.yaml` correct is not enough if the bootstrap release values omit the same `meshConfig` entries.
- In this repo, the provider must target the rendered service name `homelab-oauth2-proxy.homelab.svc.cluster.local`, not the unprefixed chart name.
- The provider must use the Service port `80`, not the container port `4180`.
- The provider must send checks to `/oauth2/auth`. Hitting `/`, `/oauth2/start`, or another browser endpoint produces the wrong behavior for Envoy external authorization.
- If browsers authenticate successfully but then land on a plain `Authenticated` page, the auth check is returning the user to `/oauth2/auth` instead of the original site URL. Set an explicit `X-Auth-Request-Redirect` in the Istio extension provider so oauth2-proxy sends the browser back to `https://mcp.labiraus.com/`.
- As a defense in depth on the public edge, keep `/oauth2/start` and `/oauth2/callback` routable to `oauth2-proxy`, but redirect browser hits on `/oauth2` and `/oauth2/auth` back to `/` so the raw auth-check endpoint is never the visible destination.
- If browser login still loops or strands users after Google auth, prefer debugging the `oauth2-proxy` reverse-proxy upstreams for `/` and `/api` before reintroducing Istio `CUSTOM` auth on `ui` or `external`.
- If `oauth2-proxy` returns `503 upstream connect error or disconnect/reset before headers` for `/`, check whether the `ui` upstream is using a mesh-native service port. In this repo, routing `oauth2-proxy` to `homelab-ui` on service port `8000` caused Envoy to use `PassthroughCluster`, which skipped mTLS origination and was rejected by the UI's `STRICT` `PeerAuthentication`. Using the UI service on port `80` fixes that path.
- If the `oauth2-proxy` sidecar still shows `PassthroughCluster` for `homelab-ui` even after moving to service port `80`, inspect the outbound route table for port `80`. In this repo, the sidecar had the correct `homelab-ui` cluster and route, but `oauth2-proxy` was preserving `Host: mcp.labiraus.com` upstream. On Istio's shared `0.0.0.0:80` listener, that host fell through to the `allow_any` virtual host and was routed to `PassthroughCluster`. Setting `OAUTH2_PROXY_PASS_HOST_HEADER=false` fixed the match by letting the upstream service host drive routing.
- `oauth2-proxy` itself should show this split when checked directly:
  - `/oauth2/start` returns a `302` redirect to Google
  - `/oauth2/auth` returns `401 Unauthorized` when no session cookie is present
- If `mcp` discovery looks broken even after auth is fixed, make sure the shared host route also publishes:
  - `/.well-known/mcp.json`
  - `/.well-known/oauth-protected-resource`
- If Codex/RMCP can `initialize` against `/mcp` but then reports that the transport closed while sending `notifications/initialized`, check that Streamable HTTP one-way messages return `202 Accepted` with an empty body. A JSON-RPC ack body for notifications can be treated as an unexpected response and close the client transport.
- For the same symptom, compare `POST /mcp` request responses against OpenAI Docs MCP: Labiraus should return JSON-RPC request responses as finite SSE `event: message` envelopes with `Content-Type: text/event-stream`, not as bare `application/json`.
- For the same symptom, also check `GET /mcp` with `Accept: text/event-stream`: the stream should open with an SSE comment rather than a synthetic empty `data:` event, because strict clients may parse that empty event as an invalid JSON-RPC server message and some proxies will not flush headers until at least one body byte is written.
- Also verify that sessionless startup fallback still works: `notifications/initialized`, response-only messages, one-way batches, and stateless requests such as `resources/list` should not require `MCP-Session-Id`; subscription and stream operations should still require a session.
- If MCP connects but `homelab://mcp/postgres/...` resources return `Postgres backend is unavailable`, first compare the live `homelab/mcp-data-connections` secret keys and the `homelab-mcp` deployment `envFrom`. MCP expects `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_DATABASE`, `POSTGRES_USER`, and `POSTGRES_PASSWORD`; those should come from the CNPG `data/app-db-bootstrap` secret and point at `app-db-rw.data.svc.cluster.local`.
- For the same symptom, confirm both sides of the NetworkPolicy path: the MCP policy must allow egress to namespace `data`, pods labeled `cnpg.io/cluster: app-db`, on TCP `5432`, and the Postgres `data/app-db` policy must allow ingress from pods labeled `app.kubernetes.io/instance: homelab-mcp`. A local chart fix is not live until the app chart is published and Flux reconciles it.
- If `kubectl config set-context --current --namespace=homelab` fails on `/home/vscode/.kube/config.lock`, check ownership of the parent directory, not only the config file. In this devcontainer the config file can be writable while `/home/vscode/.kube` is root-owned; `sudo chown vscode:vscode /home/vscode/.kube` fixes locking. Avoid `sudo kubectl config ...` for this, because root usually has no current context.

### Recreated workers now need post-provision Ansible join

- A rebuilt Terraform-managed worker no longer joins the cluster from Terraform cloud-init.
- The expected post-apply step is:

```bash
make ansible-kubernetes-worker LIMIT=<node-name>
```

- If the VM comes up but `/var/lib/kubelet/config.yaml` is missing, that now usually means the Ansible join step has not run yet or did not complete.
- If `kubeadm` hangs early while polling `kube-public/cluster-info`, check whether the control plane is signing bootstrap tokens into the ConfigMap. A missing JWS signature for the new token ID blocks join before kubelet writes its config.

### Correct recovery order after VM recreation

- Check `cloud-init` first. If it is still running, wait before intervening.
- If `cloud-init` finished but the node did not join, run `make ansible-kubernetes-worker LIMIT=<node-name>`.
- If that Ansible run now fails early with a message about `kube-public/cluster-info` missing `jws-kubeconfig-<token-id>`, the control plane is creating bootstrap tokens but not signing them into the discovery ConfigMap yet. Fix that control-plane issue first; repeating `kubeadm join` on the worker will not help until the JWS entry exists.
- If the old Kubernetes `Node` object still exists, delete it before trying to register the rebuilt VM.
- If the worker joined once, then the node object was deleted and kubelet is stuck in an identity mismatch, stop trying to patch around it. Run:

```bash
sudo kubeadm reset -f
make ansible-kubernetes-worker LIMIT=<node-name>
```

### Cilium recovery after node recreation

- A recreated worker can stay `NotReady` with `KubeletNotReady: cni plugin not initialized` because stale Cilium or kube-proxy DaemonSet pods are still associated with the old node lifecycle.
- Deleting the node-local Cilium and kube-proxy pods is often enough to let the DaemonSets reconcile.
- If the Cilium image is the bottleneck, pre-pull it on the guest with:

```bash
sudo ctr -n k8s.io images pull quay.io/cilium/cilium:v1.19.1
```

- If Cilium reports node-authorizer errors after a node object was deleted and recreated, the durable fix is a full `kubeadm reset` and rejoin.

## Longhorn Maintenance

### Drain behavior is not the same as Longhorn eviction

- `kubectl drain` alone does not guarantee Longhorn will move replicas off a node.
- Longhorn can block drain with instance-manager PDBs even after a node is cordoned.
- `replica-auto-balance = least-effort` and a maintenance-friendly `node-drain-policy` help, but they do not make every drain non-blocking.

### Disposable lab shortcut

- In a disposable lab with no important data, it can be faster to delete the blocking Longhorn instance-manager PDB and pod than to wait for perfect eviction behavior.
- Do not treat that as the default approach for a data-bearing cluster.

## GPU Passthrough Classification

### Passthrough is not the same as LLM-ready

- A worker can have successful PCI passthrough and still not be ready for inference workloads.
- In this repo, distinguish:
  - `node-gpu=passthrough`: guest sees the physical GPU in `lspci`
  - `node-llm=gpu`: guest has a usable compute/render interface such as `nvidia-smi` or `/dev/dri/renderD128`

### Current Intel iGPU state

- After the rebuilds, all three workers see their Intel iGPU at PCI address `01:00.0`.
- The guests currently expose only `/dev/dri/card0` and no render node such as `/dev/dri/renderD128`.
- That is enough to label them `node-gpu=passthrough`, but not enough to label them `node-llm=gpu`.
