# Node Classification

Use node labels to make workload placement explicit for game servers and future LLM workloads.

## Current label scheme

- `node-performance=high|standard`
- `node-gpu=passthrough|none`
- `node-llm=gpu|none`
- `node-llm-class=igpu|consumer-gpu|high-vram-gpu|none`
- `node-llm-vram=shared|8gb|12gb|16gb|24gb|48gb|none`

Current intended values:

- `jotunheim`: `node-performance=high`, `node-gpu=passthrough`, `node-llm=none`, `node-llm-class=igpu`, `node-llm-vram=shared`
- `alfheim`: `node-performance=standard`, `node-gpu=passthrough`, `node-llm=none`, `node-llm-class=igpu`, `node-llm-vram=shared`
- `helheim`: `node-performance=standard`, `node-gpu=passthrough`, `node-llm=gpu`, `node-llm-class=consumer-gpu`, `node-llm-vram=8gb`
- `niflheim`: `node-performance=standard`, `node-gpu=passthrough`, `node-llm=none`, `node-llm-class=igpu`, `node-llm-vram=shared`
- `midgard`: manual bare-metal GPU worker, intentionally intermittent; when powered on, classify from the actual exposed accelerator rather than assuming it matches the Intel iGPU workers

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

## Current Proxmox GPU state

The current Proxmox worker hosts do have onboard Intel GPUs:

- `proxmox-node1`: Intel Iris Xe Graphics `[8086:46a8]`
- `proxmox-node2`: Intel Iris Xe Graphics `[8086:46a8]`
- `proxmox-node3`: Intel UHD Graphics 630 `[8086:3e9b]` plus NVIDIA GeForce RTX 2070 Mobile `[10de:1f10]`
- `proxmox-node4`: Intel Iris Xe Graphics `[8086:9a49]`

At the time of writing:

- `proxmox-node1`, `proxmox-node2`, and `proxmox-node4` are the current Intel-iGPU worker hosts
- `proxmox-node3` is a Dell G7 7590 on Proxmox VE `9.1.1` with `15 GiB` RAM, a `476.9 GiB` NVMe, a `931.5 GiB` SATA disk, an Intel iGPU, and an RTX 2070 Mobile dGPU
- `proxmox-node3` now boots with `intel_iommu=on iommu=pt`, loads the VFIO modules, and exposes IOMMU groups for both the Intel iGPU and the NVIDIA device stack
- the Intel iGPU is isolated in IOMMU group `0`
- the NVIDIA GPU, audio, USB, and UCSI functions are grouped together in IOMMU group `2`
- the current VM module still models only one VM disk, so the secondary `931.5 GiB` SATA disk on `proxmox-node3` remains host-visible storage until it is added to Proxmox as a datastore and then modeled separately
- the Intel iGPU workers currently expose passthrough at the PCI level but still do not expose a render node such as `/dev/dri/renderD128`

That means passthrough is working at the PCI level on the existing Intel workers, but the correct current inference label is still `node-llm=none` for those iGPU-backed nodes. Relative GPU preference should currently treat them as `node-llm-class=igpu` with `node-llm-vram=shared`. `helheim` on `proxmox-node3` is the planned consumer-GPU worker and is intended to carry `node-llm=gpu`, `node-llm-class=consumer-gpu`, and `node-llm-vram=8gb` once the RTX-backed guest is built and verified end to end.

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

For changes to an existing worker, VM rebuilds, and post-Terraform recovery, use [WorkerRedeploy.md](/workspaces/homelab/docs/WorkerRedeploy.md).
