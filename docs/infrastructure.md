# Infrastructure

This is the repository's host inventory and provisioning model, not a live health
report. Inspect actual hosts and [dated status](status.md) before maintenance.
Terraform manages Proxmox VM lifecycle; Ansible configures guests and joins workers;
Helm/Flux manages cluster workloads.

## Hosts

| Host | Address | Role / parent |
| --- | --- | --- |
| `yggdrasil` | `192.168.8.132` | Physical Chromebook control plane; never a Proxmox VM |
| `svartalfheim` | `192.168.8.177` wired, `.202` Wi-Fi | Physical Pi; private external S3 and Samba |
| `midgard` | `192.168.8.205` | Intermittent bare-metal GPU worker |
| `proxmox-node1` | `192.168.8.229` | Proxmox host for `jotunheim` and `nidavellir` |
| `proxmox-node2` | `192.168.8.133` | Proxmox host for `alfheim` |
| `proxmox-node3` | `192.168.8.191` | Proxmox host for `helheim` |
| `proxmox-node4` | `192.168.8.103` | Proxmox host for `niflheim` |
| `jotunheim` | `192.168.8.121` | Terraform worker |
| `alfheim` | `192.168.8.122` | Terraform worker |
| `helheim` | `192.168.8.123` | Terraform NVIDIA worker |
| `niflheim` | `192.168.8.124` | Terraform worker |
| `nidavellir` | `192.168.8.126` | Dedicated Minecraft VM |

`proxmox-node3` is a Dell G7 7590, i7-9750H (6 cores/12 threads), about 15 GiB
RAM, 476.9 GiB NVMe and 931.5 GiB SATA, with UHD 630 and RTX 2070 Mobile graphics.
Inventory and node inputs are under `ansible/inventory/` and `etc/nodes/`.

## Physical host setup facts

Record new durable physical-host facts here rather than in separate agent memory.

- `yggdrasil` root was observed on ext4 `/dev/mmcblk1p2`, UUID
  `4503a76b-f0a0-4337-8c84-32ca0091e510`; boot is vfat `/dev/mmcblk1p1`.
  Ping or a TCP connection without an SSH banner does not establish userspace
  health. Use physical-console/kernel/disk evidence before choosing recovery.
- `yggdrasil` and `svartalfheim` accepted the replacement user CA via
  `/etc/ssh/trusted-user-ca-keys.pub` during the 11 October 2026 recovery.
  Inspect current effective trust before changing it; historical verification
  is not current access evidence.
- `svartalfheim` data is NTFS `/dev/sda2`, UUID `C2629529629522E9`, mounted at
  `/srv/minio`. The 11 October observation was 9.1 TiB total, about 4 TiB used;
  the older 8 TB estimate is superseded. Preserve media and backup directories.
- MinIO source data is `/srv/minio/minio-data`. SeaweedFS uses a separate
  `/srv/minio/seaweedfs/volumes` subtree and native-filesystem metadata under
  `/var/lib/seaweedfs`. Samba must exclude both backend data subtrees.
- A lost mount can leave a writable root-filesystem directory at `/srv/minio`.
  Check the mounted device before restarting storage; never create destination
  directories over an absent mount. See [storage](storage.md).
- SeaweedFS 4.48's internal interfaces, including S3 gRPC, remain loopback-only;
  a systemd proxy exposes only wired private S3 HTTP on port 8333. Wi-Fi DNS
  resolution is not equivalent to the egress-permitted wired address.

## ProxMox

For a laptop, set HandleLidSwitch=ignore in /etc/systemd/logind.conf to prevent sleep on lid close. For a desktop, this is not necessary.

``` bash
hostnamectl set-hostname proxmox-node3
apt update && apt dist-upgrade -y
apt install -y chrony
systemctl enable --now chrony
systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target
nano /etc/systemd/logind.conf
systemctl restart systemd-logind

pvesm set local --content iso,vztmpl,backup,import,snippets
```

Fresh Proxmox installs need to be joined to the existing Proxmox cluster before they will behave like the other homelab hosts. The current cluster name is `homecluster`, and `proxmox-node1` (`192.168.8.229`) is a good seed node to join against.

On the new host:

```bash
pvecm status
pvecm add 192.168.8.229
```

That command will prompt for the root password of the seed cluster node and then pull the corosync configuration onto the new host.

Verify afterward:

```bash
pvecm status
pvecm nodes
```

For a healthy join, `pvecm status` on the new node should report cluster name `homecluster` and show the current members:

- `proxmox-node1` `192.168.8.229`
- `proxmox-node2` `192.168.8.133`
- `proxmox-node3` `192.168.8.191`
- `proxmox-node4` `192.168.8.103`

Do this while the node is still effectively empty. Joining a non-fresh Proxmox host into an existing cluster has extra caveats around local VM and cluster state, but for a newly installed node the normal `pvecm add` flow is the right path.

For GPU passthrough prerequisites on Intel-based Proxmox hosts, add IOMMU and VFIO setup during the host bootstrap:

```bash
if ! grep -q 'intel_iommu=on' /etc/default/grub; then
  sed -i 's/^GRUB_CMDLINE_LINUX_DEFAULT="\(.*\)"/GRUB_CMDLINE_LINUX_DEFAULT="\1 intel_iommu=on iommu=pt"/' /etc/default/grub
fi

for module in vfio vfio_iommu_type1 vfio_pci vfio_virqfd; do
  grep -qxF "$module" /etc/modules || echo "$module" >> /etc/modules
done

update-grub
update-initramfs -u -k all
```

This only stages the host for passthrough. Reboot afterward, then verify:

```bash
dmesg | grep -e DMAR -e IOMMU -e AMD-Vi
find /sys/kernel/iommu_groups -maxdepth 2 -type l
pvesh get /nodes/$(hostname)/hardware/pci --pci-class-blacklist ""
lsmod | grep vfio
```

Do not blacklist `i915` on these hosts as part of the base setup. Each current Proxmox node only has an integrated Intel GPU, and blacklisting the host driver too early can remove local console output before you have verified passthrough on a rebuilt worker VM.

For `helheim` host heat management on `proxmox-node3`, use the helper script in this repo:

```bash
./scripts/helheim-performance.sh down
./scripts/helheim-performance.sh up
```

`down` reduces host power draw by preferring the `low-power` platform profile when available, falling back to a non-quiet profile such as `balanced`, setting the CPU governor to `powersave`, and disabling turbo. Avoid using the `quiet` platform profile for this host because it can suppress fan response instead of only reducing heat generation. `up` restores the host to `performance` mode for heavier GPU or VM work.

## Kubernetes

### Kubernetes Control Plane

Initialize or refresh the devcontainer kubeconfig from `yggdrasil`:

```bash
make refresh-kubeconfig
```

This file contains cluster credentials and private key material. Keep it out of git and treat it as a secret.

The helper script defaults to `yggdrasil:~/.kube/config`. Override with `KUBECONFIG_SOURCE_HOST` or `KUBECONFIG_SOURCE_PATH` in `.devcontainer/.env` when the control-plane source changes.

### Manual Node Join

If you need to join an Ubuntu machine to the cluster outside the Terraform-managed worker flow, use the Ansible playbook for the manual-worker inventory group.

`midgard` is the current manual GPU worker. It is a bare-metal Linux machine, not a Proxmox VM. It is intentionally powered on only when heavier GPU workloads need extra capacity, so it is expected to be absent or offline at times. Treat `midgard` being switched off as normal operating state, not a Kubernetes bug, unless work was explicitly meant to be running there.

Prerequisites:

- SSH is installed and reachable on the node.
- Your login key and certificate are in place using the SSH setup above.
- The node exists in [hosts.ini](../ansible/inventory/hosts.ini) under `kubernetes_manual_node`.
- The control-plane host `yggdrasil` is reachable from the machine running Ansible so the playbook can fetch a fresh join command at runtime.
- The local GPU and its drivers should already be working on `midgard` before you try to use it for GPU workloads.

Run the playbook for `midgard`:

```bash
ANSIBLE_FETCH_MINIO_SECRETS=0 ./scripts/ansible-run-playbook.sh \
  -i ansible/inventory/hosts.ini \
  ansible/playbooks/kubernetes-manual-node.yml \
  --limit midgard
```

That playbook installs the Kubernetes prerequisites on `midgard`, fetches a fresh `kubeadm join` command from `yggdrasil`, and joins the node when it is not already bootstrapped.

For `midgard`, the playbook also enables the node for NVIDIA-backed Kubernetes workloads by installing `nvidia-container-toolkit`, configuring containerd with an `nvidia` runtime, applying the NVIDIA device-plugin manifest, and labeling the node for GPU scheduling.

`midgard` should also stay headless. The manual-node role can disable display-manager services, switch the default boot target to `multi-user.target`, and purge the configured GNOME or Xorg packages so the local desktop does not reclaim the NVIDIA GPU on boot.

If the machine was previously joined and you need to re-bootstrap it cleanly, reset it first:

```bash
ssh midgard 'sudo kubeadm reset -f'
```

Useful checks after the join:

```bash
ssh midgard 'systemctl is-active kubelet containerd'
kubectl get nodes -o wide
```

Verify local GPU availability before scheduling GPU workloads there:

```bash
ssh midgard 'lspci | egrep -i "vga|3d|display|nvidia|amd|intel" || true; echo ---; ls -lah /dev/dri 2>/dev/null || true; echo ---; test -e /dev/dri/renderD128 && echo RENDER=present || echo RENDER=absent; echo ---; command -v nvidia-smi >/dev/null && nvidia-smi || true'
```

If the machine is being added as a manually managed Ubuntu worker rather than a Terraform-managed node, apply the expected node labels after it joins using [infrastructure.md](infrastructure.md). For `midgard`, that means at least `node-gpu=passthrough` when the node can see its real local GPU, and `node-llm=gpu` only after the node exposes a usable render or compute interface.

When you are finished with a burst of GPU work on `midgard`, drain and power it off cleanly:

```bash
kubectl drain midgard --ignore-daemonsets --delete-emptydir-data --force
kubectl cordon midgard
ssh midgard 'sudo poweroff'
```

If `midgard` is intentionally offline afterward, leave it that way. A missing or powered-off `midgard` is expected when no heavy GPU work is queued.

### Terraform-Managed Worker Join

Terraform now builds the worker VM and installs the base Kubernetes packages, but the actual `kubeadm join` step is handled afterward through Ansible rather than through Terraform cloud-init.

GPU-oriented Terraform workers can also declare node-specific cloud-init distro packages in their tfvars. `helheim` now uses that path to install its initial NVIDIA driver package set during first boot, while the follow-up Ansible worker playbook remains the authoritative place that enforces the chosen driver family plus the running-kernel and `-generic` NVIDIA module packages.

After `ENV=lab bin/tf apply kubernetes <node-layer>`, run the worker bootstrap playbook for that node:

```bash
make ansible-kubernetes-worker LIMIT=helheim
```

Equivalent direct wrapper command:

```bash
ANSIBLE_FETCH_MINIO_SECRETS=0 ./scripts/ansible-run-playbook.sh \
  -i ansible/inventory/hosts.ini \
  ansible/playbooks/kubernetes-terraform-node.yml \
  --limit helheim
```

This is the preferred path for Terraform-managed workers such as `jotunheim`, `alfheim`, `helheim`, and `niflheim` because it fetches a fresh join command from `yggdrasil` at runtime instead of baking cluster bootstrap material into Terraform state.

If you want to roll the same Ansible-side change across every Terraform-managed worker, omit `LIMIT`:

```bash
make ansible-kubernetes-worker
```

## Current label scheme

- `node-performance=high|standard`
- `node-gpu=passthrough|none`
- `node-llm=gpu|none`
- `node-llm-class=igpu|consumer-gpu|high-vram-gpu|none`
- `node-llm-vram=shared|8gb|12gb|16gb|24gb|48gb|none`

Current intended values:

- `yggdrasil`: physical Chromebook control plane; keep tainted `node-role.kubernetes.io/control-plane:NoSchedule` and do not schedule normal workloads there
- `jotunheim`: `node-performance=high`, `node-gpu=passthrough`, `node-llm=none`, `node-llm-class=igpu`, `node-llm-vram=shared`
- `alfheim`: `node-performance=standard`, `node-gpu=passthrough`, `node-llm=none`, `node-llm-class=igpu`, `node-llm-vram=shared`
- `helheim`: `node-performance=standard`, `node-gpu=passthrough`, `node-llm=gpu`, `node-llm-class=consumer-gpu`, `node-llm-vram=8gb`
- `niflheim`: `node-performance=standard`, `node-gpu=passthrough`, `node-llm=none`, `node-llm-class=igpu`, `node-llm-vram=shared`
- `midgard`: manual bare-metal GPU worker, intentionally intermittent; when powered on, classify from the actual exposed accelerator rather than assuming it matches the Intel iGPU workers

## Control-plane node

`yggdrasil` is the physical Chromebook dedicated to Kubernetes control-plane duties. Diagnose actual CPU, memory, power and disk pressure before selecting hardware changes; it is not a resizable Proxmox VM.

Working rule for this repo:

- keep the `node-role.kubernetes.io/control-plane:NoSchedule` taint on `yggdrasil`
- expect only static control-plane pods plus required node plumbing there: `etcd`, `kube-apiserver`, `kube-controller-manager`, `kube-scheduler`, `kubelet`, `containerd`, `kube-proxy`, and Cilium node agents
- keep avoidable cluster services off the control plane; CoreDNS and the Cilium operator should use node affinity that excludes control-plane nodes
- keep MetalLB speakers off the control plane by setting upstream chart value `speaker.tolerateMaster=false`
- run `yggdrasil` as a headless server profile; desktop/audio/printing/bluetooth/snap desktop integration services should stay disabled unless there is a specific maintenance reason

## Intermittent GPU workers

`midgard` is not part of the always-on baseline worker pool. It is a bare-metal Linux machine that is expected to be powered on only for heavier GPU jobs and to stay offline the rest of the time.

Working rule for this repo:

- do not treat a powered-off or missing `midgard` as a bug by itself
- only expect `midgard` to be `Ready` when there is active work that needs its GPU capacity
- when `midgard` returns, verify both Kubernetes readiness and GPU visibility before scheduling GPU-bound workloads there

## How to check CPU performance

For Terraform-managed nodes, classify performance from the Proxmox host CPU rather than only the guest's virtual CPU string.

Check the Proxmox host CPU:

```bash
ssh proxmox-node1 'lscpu | egrep "Model name|CPU max MHz|Vendor ID|Socket|Core|Thread"'
ssh proxmox-node2 'lscpu | egrep "Model name|CPU max MHz|Vendor ID|Socket|Core|Thread"'
ssh proxmox-node3 'lscpu | egrep "Model name|CPU max MHz|Vendor ID|Socket|Core|Thread"'
ssh proxmox-node4 'lscpu | egrep "Model name|CPU max MHz|Vendor ID|Socket|Core|Thread"'
```

Check the current guest-reported clock:

```bash
ssh jotunheim 'grep -m1 "cpu MHz" /proc/cpuinfo'
ssh alfheim 'grep -m1 "cpu MHz" /proc/cpuinfo'
ssh helheim 'grep -m1 "cpu MHz" /proc/cpuinfo'
ssh niflheim 'grep -m1 "cpu MHz" /proc/cpuinfo'
```

Working rule for this repo:

- label a node `node-performance=high` when it is clearly better than the rest of the worker pool for latency-sensitive workloads, usually because it has the strongest host CPU in the cluster
- label a node `node-performance=standard` for general-purpose worker nodes
- do not treat a node as under-provisioned unless the VM has fewer vCPUs than the host exposes; a smaller host can still be correctly assigned its full complement

At the time of writing:

- `proxmox-node1` / `jotunheim` is the strongest worker host: 12th Gen Intel Core i7-1265U, max 4.8 GHz
- `proxmox-node2` / `alfheim` is standard: 12th Gen Intel Core i5-1245U, max 4.4 GHz
- `proxmox-node3` / `helheim` is GPU-oriented but still standard for CPU-only placement: 9th Gen Intel Core i7-9750H, 6 cores / 12 threads, max 4.5 GHz
- `proxmox-node4` / `niflheim` is standard: 11th Gen Intel Core i5-1155G7, max 4.5 GHz

## How to check LLM/GPU capability

Check inside the Kubernetes node guest first:

```bash
ssh jotunheim 'lspci | egrep -i "vga|3d|display|nvidia|amd|intel" || true; echo ---; ls -lah /dev/dri 2>/dev/null || true; echo ---; test -e /dev/dri/renderD128 && echo RENDER=present || echo RENDER=absent; echo ---; command -v nvidia-smi >/dev/null && nvidia-smi || true'
ssh alfheim 'lspci | egrep -i "vga|3d|display|nvidia|amd|intel" || true; echo ---; ls -lah /dev/dri 2>/dev/null || true; echo ---; test -e /dev/dri/renderD128 && echo RENDER=present || echo RENDER=absent; echo ---; command -v nvidia-smi >/dev/null && nvidia-smi || true'
ssh helheim 'lspci | egrep -i "vga|3d|display|nvidia|amd|intel" || true; echo ---; ls -lah /dev/dri 2>/dev/null || true; echo ---; test -e /dev/dri/renderD128 && echo RENDER=present || echo RENDER=absent; echo ---; command -v nvidia-smi >/dev/null && nvidia-smi || true'
ssh niflheim 'lspci | egrep -i "vga|3d|display|nvidia|amd|intel" || true; echo ---; ls -lah /dev/dri 2>/dev/null || true; echo ---; test -e /dev/dri/renderD128 && echo RENDER=present || echo RENDER=absent; echo ---; command -v nvidia-smi >/dev/null && nvidia-smi || true'
```

Classify a node as `node-gpu=passthrough` when the node can see a real physical GPU in `lspci`, even if the user-space compute stack is not ready yet. For the Proxmox workers that means a passed-through GPU in the guest; for bare-metal `midgard` it means the locally attached GPU is visible.

Use `node-llm-class` to express relative scheduling preference for inference:

- `igpu`: integrated GPU or otherwise low-end/shared-memory accelerator
- `consumer-gpu`: discrete desktop GPU such as a GeForce card
- `high-vram-gpu`: larger-memory accelerator that should be preferred for bigger models
- `none`: no meaningful accelerator present

Use `node-llm-vram` to express the memory tier that an inference workload can rely on:

- `shared` for iGPUs borrowing system memory
- explicit sizes such as `8gb`, `12gb`, `16gb`, `24gb`, or `48gb` for discrete cards
- `none` when there is no useful accelerator

Classify a node as `node-llm=gpu` only when the node has an actually usable accelerator for inference, for example:

- an NVIDIA GPU visible to `nvidia-smi`
- an Intel or AMD GPU with usable render devices under `/dev/dri`

Do not count the default emulated QEMU VGA device as GPU-capable for LLM work.

At the time of writing, the current worker nodes do see their passed-through Intel GPUs in `lspci`, but they still expose only `card0` under `/dev/dri` and do not expose a render node such as `/dev/dri/renderD128`. That means the correct current labels are:

- `node-gpu=passthrough`
- `node-llm=none`
- `node-llm-class=igpu`
- `node-llm-vram=shared`

## Proxmox GPU passthrough checklist

Use this when adding a new Proxmox-backed worker or rebuilding one of the existing ones for GPU access.

1. Enable virtualization and IOMMU in firmware.
   On Intel systems that usually means `VT-x` plus `VT-d`.
2. Enable IOMMU on the Proxmox host.
   Add `intel_iommu=on iommu=pt` to the host kernel command line and reboot.
3. Load the VFIO modules on the Proxmox host.
   Add `vfio`, `vfio_iommu_type1`, `vfio_pci`, and `vfio_virqfd` to `/etc/modules`, then regenerate initramfs.
4. Verify passthrough prerequisites after reboot.
   `dmesg | grep -e DMAR -e IOMMU`
   `find /sys/kernel/iommu_groups -maxdepth 2 -type l`
   `pvesh get /nodes/<proxmox-node>/hardware/pci --pci-class-blacklist ""`
5. Make sure the GPU has an isolated IOMMU group.
   If it does not, do not pass it through until group isolation is understood.
6. Switch the VM to a passthrough-friendly shape.
   Use `bios = "ovmf"`, `machine = "q35"`, and an `efi_disk`.
7. Attach the GPU to the VM with a Proxmox PCI resource mapping.
   This is the safest fit for this repo because the Terraform provider is using an API token; direct PCI IDs require root username/password in the provider.
   For the current Intel iGPU workers, use a shared mapping named `intel-igpu`:
   `pvesh create /cluster/mapping/pci --id intel-igpu --map node=proxmox-node1,path=0000:00:02.0,id=8086:46a8 --map node=proxmox-node2,path=0000:00:02.0,id=8086:46a8 --map node=proxmox-node4,path=0000:00:02.0,id=8086:9a49`
   For `proxmox-node3`, create dedicated mappings for the Intel and NVIDIA functions:
   `pvesh create /cluster/mapping/pci --id node3-intel-igpu --map node=proxmox-node3,path=0000:00:02.0,id=8086:3e9b`
   `pvesh create /cluster/mapping/pci --id node3-rtx2070-gpu --map node=proxmox-node3,path=0000:01:00.0,id=10de:1f10`
   `pvesh create /cluster/mapping/pci --id node3-rtx2070-audio --map node=proxmox-node3,path=0000:01:00.1,id=10de:10f9`
   `pvesh create /cluster/mapping/pci --id node3-rtx2070-usb --map node=proxmox-node3,path=0000:01:00.2,id=10de:1ada`
   `pvesh create /cluster/mapping/pci --id node3-rtx2070-ucsi --map node=proxmox-node3,path=0000:01:00.3,id=10de:1adb`
8. Rebuild or update the worker VM, then verify the guest sees a real GPU.
   For Intel or AMD, check `/dev/dri`.
   For NVIDIA, check `nvidia-smi`.
9. Only after the guest has a usable accelerator, apply `node-llm=gpu`.

Important caveat for these hosts:

- the current Intel-iGPU worker hosts rely on their integrated GPU as the primary host display adapter
- passing through the only GPU can remove local console output from the Proxmox host
- `proxmox-node3` is less constrained here because it also has a discrete RTX 2070 Mobile, but the full NVIDIA function group still needs to be assigned deliberately and verified in-guest

Treat passthrough on the Proxmox worker hosts as feasible-but-experimental until each target node is converted and verified end to end.

## Terraform support for passthrough-capable Proxmox workers

The VM module now supports the pieces needed for passthrough-capable workers:

- `vm.bios`
- `vm.machine`
- `vm.efi_disk`
- `vm.hostpci_devices`

Recommended pattern for this repo:

- create a cluster-wide PCI resource mapping in Proxmox
- reference that mapping from `vm.hostpci_devices`
- keep the actual worker classification in `kubelet_node_labels`

Example shape for a future GPU-capable worker tfvars:

```hcl
vm = {
  cpu_cores    = 12
  memory_mb    = 32768
  disk_size_gb = 280
  ssh_username = "ubuntu"
  bios         = "ovmf"
  machine      = "q35"
  efi_disk = {
    datastore_id = "local-lvm"
    type         = "4m"
  }
  hostpci_devices = [
    {
      device  = "hostpci0"
      mapping = "intel-igpu"
      pcie    = true
    }
  ]
}

kubelet_node_labels = {
  "topology.kubernetes.io/zone" = "lab-a"
  "node-performance"           = "high"
  "node-gpu"                   = "passthrough"
  "node-llm"                   = "gpu"
  "node-llm-class"             = "consumer-gpu"
  "node-llm-vram"              = "12gb"
}
```

## How to apply labels

For Terraform-managed nodes, keep labels in the node tfvars under `kubelet_node_labels` so they are applied at join time.

For manually added Ubuntu nodes, apply labels after the node joins:

```bash
kubectl label node <node-name> node-performance=standard --overwrite
kubectl label node <node-name> node-gpu=none --overwrite
kubectl label node <node-name> node-llm=none --overwrite
kubectl label node <node-name> node-llm-class=none --overwrite
kubectl label node <node-name> node-llm-vram=none --overwrite
```

For the intermittent GPU worker `midgard`, use this sequence after it joins and after you confirm the node can see its local physical GPU:

```bash
kubectl label node midgard node-performance=standard --overwrite
kubectl label node midgard node-gpu=passthrough --overwrite
```

The Ansible manual-node playbook now applies these labels automatically for `midgard` and also applies the NVIDIA device-plugin selector label used by the repo-managed DaemonSet.

Only promote `midgard` to `node-llm=gpu` after the node has a usable inference interface:

```bash
kubectl label node midgard node-llm=gpu --overwrite
kubectl label node midgard node-llm-class=<consumer-gpu|high-vram-gpu> --overwrite
kubectl label node midgard node-llm-vram=<8gb|12gb|16gb|24gb|48gb> --overwrite
```

If `midgard` is up but the node cannot expose a usable render or compute interface yet, keep it honest:

```bash
kubectl label node midgard node-llm=none --overwrite
kubectl label node midgard node-llm-class=none --overwrite
kubectl label node midgard node-llm-vram=none --overwrite
```

If a future node has the strongest CPU in the worker pool:

```bash
kubectl label node <node-name> node-performance=high --overwrite
```

If a future node has a usable inference GPU:

```bash
kubectl label node <node-name> node-gpu=passthrough --overwrite
kubectl label node <node-name> node-llm=gpu --overwrite
kubectl label node <node-name> node-llm-class=consumer-gpu --overwrite
kubectl label node <node-name> node-llm-vram=12gb --overwrite
```

For changes to an existing worker, VM rebuilds, and post-Terraform recovery, use [recovery.md](recovery.md).

## Host bootstrap notes

Laptop hosts disable lid-triggered sleep through `HandleLidSwitch=ignore` and
mask suspend/hibernate targets. Install SSH and validate sudoers on new Ubuntu
hosts before automation. Install only the verified public user CA in the effective
trust path; never copy an entire `authorized_keys` file as a CA bundle.

Chromebook firmware replacement requires removing hardware write protection and
using the hardware vendor's supported firmware procedure; the former unpinned
remote-script execution example is not a repository-managed installer.

### Dell BIOS maintenance

The retained setup record identifies `proxmox-node1` as a Dell Latitude 5430.
Confirm model, current firmware and console access before planning maintenance;
preserve backups and stop workloads as appropriate. Prefer the Linux `fwupd` path
after a rebuild. On that host install `fwupd`, `fwupd-amd64-signed` and `udisks2`,
then verify `findmnt /boot/efi` and `fwupdtool esp-list`.

The recorded ESP was `nvme0n1p2`, UUID `E683-2C33`, mounted as
`/boot/efi vfat defaults 0 1`. Reverify the current disk/UUID before restoring an
fstab entry. Missing/running `udisks2` must be checked if fwupd cannot detect the ESP.

On the verified host, execute maintenance steps separately and inspect results:

```bash
systemctl restart udisks2 fwupd
fwupdmgr refresh --force
fwupdmgr get-updates
fwupdmgr update -y
fwupdmgr get-history
ls /boot/efi/EFI/proxmox/fw
```

Require the intended System Firmware update to report `Needs reboot` with a
`fwupd-*.cap` capsule before the planned host reboot. Let the firmware updater
finish, then verify `hostnamectl` and `dmidecode -t bios`. The April 2026 record
staged 1.32.1 to 1.36.0; those are historical versions, not a current upgrade target.
