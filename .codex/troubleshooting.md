# Troubleshooting Learnings

## Terraform And Proxmox

### Worker VM image import on Proxmox

- Worker boot disks must be imported from a Proxmox `import` image, not an `iso`.
- In this repo, the Ubuntu cloud image should be downloaded as `content_type = "import"` and referenced from the VM disk with `import_from`.
- A source like `local:iso/ubuntu-24.04-noble-server-cloudimg-amd64.img` fails for VM creation. Use an importable disk image such as `ubuntu-24.04-noble-server-cloudimg-amd64.qcow2`.

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

## Kubernetes Worker Recovery

### Recreated workers may not auto-join

- A rebuilt worker only auto-joins if a valid `kubeadm_join_command` is available at apply time.
- Refresh the join token before Terraform changes:

```bash
make refresh-join-token
. /home/vscode/.env
```

- If the VM comes up but `/var/lib/kubelet/config.yaml` is missing, `cloud-init` likely finished without a successful `kubeadm join`.

### Correct recovery order after VM recreation

- Check `cloud-init` first. If it is still running, wait before intervening.
- If `cloud-init` finished but the node did not join, run `kubeadm join` manually.
- If the old Kubernetes `Node` object still exists, delete it before trying to register the rebuilt VM.
- If the worker joined once, then the node object was deleted and kubelet is stuck in an identity mismatch, stop trying to patch around it. Run:

```bash
sudo kubeadm reset -f
sudo kubeadm join <api-server>:6443 --token <token> --discovery-token-ca-cert-hash <hash>
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
