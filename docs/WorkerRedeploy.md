# Worker Redeploy

Use this flow when a Terraform change requires a worker VM to be recreated or materially changed, especially for `bios`, `machine`, disk, memory, or `hostpci` updates.

## Before The Apply

1. Make the Terraform change in the relevant `etc/nodes/nodeX.tfvars`.

2. Drain only the node you are changing:

```bash
kubectl drain <node-name> --ignore-daemonsets --delete-emptydir-data --force
```

For Longhorn-backed clusters, expect drain to block on instance-manager PDBs until Longhorn has moved or dropped the remaining replicas. In a disposable lab, it can be necessary to remove the blocking instance-manager PDB and pod manually.

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
ssh <node-name> 'sudo ctr -n k8s.io images pull quay.io/cilium/cilium:v1.19.1'
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

If `qm config <vmid>` is empty or the config file exists but contains no usable VM config, remove the broken definition and recreate the node cleanly:

```bash
ssh proxmox-nodeX 'qm unlock <vmid> || true; qm stop <vmid> || true; qm destroy <vmid> --purge 1 || true'
ENV=lab bin/tf apply kubernetes <node-layer> --no-refresh
```

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
ssh -o StrictHostKeyChecking=no <node-name> 'sudo cat /var/lib/longhorn/longhorn-disk.cfg'
```

Common post-redeploy failure mode:

- the Longhorn node object still expects the old disk UUID
- the rebuilt VM generated a fresh `/var/lib/longhorn/longhorn-disk.cfg`
- Longhorn marks the disk `DiskFilesystemChanged` / `DiskNotReady`
- affected disks show `storageAvailable: 0` and `storageMaximum: 0`
- new workload PVCs stay `Pending` even though the Kubernetes node itself is `Ready`

If that happens, restore the expected `diskUUID` in `/var/lib/longhorn/longhorn-disk.cfg` on the rebuilt node, then restart the `longhorn-manager` pods so Longhorn rescans the repaired disk metadata.
