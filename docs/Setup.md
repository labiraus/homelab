# Setup

## Secret model

Use three local-only secret classes when onboarding a new machine:

1. SSH keypairs and CA-signed SSH certificates in `.devcontainer/ssh/`
2. Generated local files or env entries rebuilt from trusted cluster sources with repo scripts and `make refresh-*` targets
3. A minimal `.devcontainer/.env` for external-only secrets and operator settings that cannot be derived from the cluster

Keep `.devcontainer/.env` intentionally small. If the cluster can tell you a value, prefer a script that repopulates it locally instead of hand-managing it.

## Local

Install containerisation tool (Docker desktop, Rancher desktop, etc) and rebuild devcontainer.

Initialize ignored local env files from the tracked examples:

```bash
cp .devcontainer/.env.example .devcontainer/.env
```

`ansible/.env` is generated or updated locally when needed. You do not need a checked-in example file for it.

Create an SSH CA

``` bash
ssh-keygen -t ed25519 -f ~/.ssh/ssh_user_ca -C "homelab-user-ca"
```

Create your login keypair and certificate for this machine. Keep these local-only files under `.devcontainer/ssh/`.

``` bash
ssh-keygen -t ed25519 -f .devcontainer/ssh/id_ed25519 -C "homelab-login"
ssh-keygen -s ~/.ssh/ssh_user_ca \
  -I "oliver@homelab" \
  -n oliver,ubuntu,root \
  -V +30d \
  .devcontainer/ssh/id_ed25519.pub

ssh-add .devcontainer/ssh/id_ed25519
```

`ssh-add` only needs the private key. OpenSSH will use the matching certificate (`.devcontainer/ssh/id_ed25519-cert.pub`) automatically.

You will need to run the second `ssh-keygen` command every 30 days to re-sign the key, or remove the timeout. In this example root is the user for proxmox, oliver is the user I set up for yggdrasil and ubuntu is the default username that will be used on all ubuntu machines.

## Chromebook

Before reimaging a chromebook with ubuntu from a usb drive you need to remove hardwork write protections, and then update the firmware:

``` bash
cd ~
curl -LO https://mrchromebox.tech/firmware-util.sh
sudo bash firmware-util.sh
```

## ProxMox

``` bash
hostnamectl set-hostname proxmox-nodeX
apt update && apt dist-upgrade -y
apt install -y chrony
systemctl enable --now chrony
systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target
sudo nano /etc/systemd/logind.conf
systemctl restart systemd-logind

pvesm set local --content iso,vztmpl,backup,import,snippets
```

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

## SSH

Copy the public key to the new node

```bash
ssh-copy-id -o PreferredAuthentications=password -o PubkeyAuthentication=no -o PasswordAuthentication=yes -i ~/.ssh/ssh_user_ca.pub root@192.168.8.X
```

### ProMox

```bash
install -m 644 -D /root/.ssh/authorized_keys /etc/ssh/trusted-user-ca-keys.pub
sed -i '/^TrustedUserCAKeys /d' /etc/ssh/sshd_config
echo 'TrustedUserCAKeys /etc/ssh/trusted-user-ca-keys.pub' | tee -a /etc/ssh/sshd_config >/dev/null
sshd -t && systemctl reload sshd
```

To turn off EEE (network powersave)
```bash
ethtool --set-eee nic0 eee off
```

### Ubuntu

For ubuntu systems include the following:

```bash
sudo install -m 644 -D /home/oliver/.ssh/authorized_keys /etc/ssh/trusted-user-ca-keys.pub
sudo sed -i '/^TrustedUserCAKeys /d' /etc/ssh/sshd_config
echo 'TrustedUserCAKeys /etc/ssh/trusted-user-ca-keys.pub' | sudo tee -a /etc/ssh/sshd_config >/dev/null
sudo sshd -t && sudo systemctl reload sshd
```

For proxmox use:

## Kubernetes Control Plane

Initialize or refresh the devcontainer kubeconfig from `yggdrasil`:

```bash
make refresh-kubeconfig
```

This file contains cluster credentials and private key material. Keep it out of git and treat it as a secret.

The helper script defaults to `yggdrasil:~/.kube/config`. Override with `KUBECONFIG_SOURCE_HOST` or `KUBECONFIG_SOURCE_PATH` in `.devcontainer/.env` when the control-plane source changes.

## Generated local secrets

When a secret or credential is already exposed by the cluster or control plane, prefer rebuilding local state from the source of truth instead of storing it permanently in `.devcontainer/.env`.

Current repo-native flows:

- `make refresh-kubeconfig`: rebuilds `.devcontainer/kubeconfig` from the control plane over SSH
- `make refresh-join-token`: updates `TF_VAR_kubeadm_join_token` in `.devcontainer/.env`
- `make refresh-postgres-env`: updates `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, and `DB_PASS` in `.devcontainer/.env`

Treat those generated entries as cacheable local state, not hand-maintained secrets.

## Ansible secret handling

Use `ansible/.env` for external-host secrets that Ansible needs but the rest of the repo does not.

- `.devcontainer/.env`: shared shell env, minimal, loaded automatically by the devcontainer shell
- `ansible/.env`: Ansible-only local cache and overrides, loaded only by `scripts/ansible-run-playbook.sh`

Sync the MinIO admin credentials from `svartalfheim`:

```bash
make refresh-ansible-secrets
```

That script SSHes to `svartalfheim`, reads `/etc/default/minio`, and upserts:

- `MINIO_EXTERNAL_ADMIN_ACCESS_KEY`
- `MINIO_EXTERNAL_ADMIN_SECRET_KEY`

into `ansible/.env`.

It preserves any other existing entries in `ansible/.env`.

Current limitation:

- the Samba password and managed MinIO user secrets are not recoverable from `/etc/default/minio`
- keep those as local-only values in `ansible/.env` until they also get a fetch path from their own source of truth

Run the external playbooks through the repo wrapper so both env layers are applied consistently:

```bash
make ansible-minio-host
make ansible-minio-state
```

## MinIO and Samba bootstrap

Use the storage bootstrap script when bringing up `svartalfheim` for the first time or when reapplying the host service configuration:

```bash
make bootstrap-svartalfheim-storage
```

That wrapper:

- requires `ansible/.env` to contain `MINIO_EXTERNAL_ADMIN_ACCESS_KEY`, `MINIO_EXTERNAL_ADMIN_SECRET_KEY`, and `SVARTALFHEIM_SAMBA_PASSWORD`
- checks `svartalfheim` for `/etc/default/minio`
- skips the MinIO credential fetch on first install when MinIO is not present yet
- re-enables the fetch automatically on later runs once MinIO is installed
- runs the Ansible host playbook that installs MinIO and Samba

First-install flow:

1. Create `ansible/.env` with the three bootstrap values.
2. Run `make bootstrap-svartalfheim-storage`.
3. After the install succeeds, `make refresh-ansible-secrets` can pull the MinIO admin credentials back from `svartalfheim`.

Reapply flow after MinIO already exists:

1. Keep `ansible/.env` with the Samba password and any other local-only values.
2. Run `make bootstrap-svartalfheim-storage`.
3. The script will detect `/etc/default/minio` and refresh the MinIO admin credentials automatically before the playbook runs.

## Minecraft networking

Expose the Minecraft server with a direct Kubernetes `Service` of type `LoadBalancer` by default.

This repo keeps Gateway API support optional for Minecraft, but long-lived game TCP sessions have been more reliable when they bypass the extra Istio Gateway/TCPRoute hop entirely.

Current default chart behavior:

- `helm/workloads/minecraft` exposes port `25565` through `service.type=LoadBalancer`
- `service.externalTrafficPolicy=Local` keeps player TCP sessions on the node that is actually running the Minecraft pod
- `route.enabled=false` keeps the Gateway API path disabled unless you explicitly opt back into it
- `sidecar.istio.io/inject: "false"` keeps the Minecraft pod out of the mesh proxy path
- the optional FTP debug deployment now creates its own auth secret (`mc-debug-ftp-auth` by default) if one does not already exist

If you opt back into Gateway API for Minecraft, also restore the `minecraftGateway.listeners` entry in `helm/bootstrap/flux-bootstrap/values.yaml` so the dedicated TCP listener is created again.

Useful checks:

```bash
kubectl get svc -n minecraft minecraft -o wide
kubectl describe svc -n minecraft minecraft
kubectl get endpointslice -n minecraft -l kubernetes.io/service-name=minecraft
```

## Node Classification

Use node labels to make workload placement explicit for game servers and future LLM workloads.

Current label scheme:

- `node-performance=high|standard`
- `node-gpu=passthrough|none`
- `node-llm=gpu|none`
- `node-llm-class=igpu|consumer-gpu|high-vram-gpu|none`
- `node-llm-vram=shared|8gb|12gb|16gb|24gb|48gb|none`

Current intended values:

- `jotunheim`: `node-performance=high`, `node-gpu=passthrough`, `node-llm=none`, `node-llm-class=igpu`, `node-llm-vram=shared`
- `alfheim`: `node-performance=standard`, `node-gpu=passthrough`, `node-llm=none`, `node-llm-class=igpu`, `node-llm-vram=shared`
- `niflheim`: `node-performance=standard`, `node-gpu=passthrough`, `node-llm=none`, `node-llm-class=igpu`, `node-llm-vram=shared`

### How to check CPU performance

For Terraform-managed nodes, classify performance from the Proxmox host CPU rather than only the guest's virtual CPU string.

Check the Proxmox host CPU:

```bash
ssh proxmox-node1 'lscpu | egrep "Model name|CPU max MHz|Vendor ID|Socket|Core|Thread"'
ssh proxmox-node2 'lscpu | egrep "Model name|CPU max MHz|Vendor ID|Socket|Core|Thread"'
ssh proxmox-node4 'lscpu | egrep "Model name|CPU max MHz|Vendor ID|Socket|Core|Thread"'
```

Check the current guest-reported clock:

```bash
ssh jotunheim 'grep -m1 "cpu MHz" /proc/cpuinfo'
ssh alfheim 'grep -m1 "cpu MHz" /proc/cpuinfo'
ssh niflheim 'grep -m1 "cpu MHz" /proc/cpuinfo'
```

Working rule for this repo:

- label a node `node-performance=high` when it is clearly better than the rest of the worker pool for latency-sensitive workloads, usually because it has the strongest host CPU in the cluster
- label a node `node-performance=standard` for general-purpose worker nodes
- do not treat a node as under-provisioned unless the VM has fewer vCPUs than the host exposes; a smaller host can still be correctly assigned its full complement

At the time of writing:

- `proxmox-node1` / `jotunheim` is the strongest worker host: 12th Gen Intel Core i7-1265U, max 4.8 GHz
- `proxmox-node2` / `alfheim` is standard: 12th Gen Intel Core i5-1245U, max 4.4 GHz
- `proxmox-node4` / `niflheim` is standard: 11th Gen Intel Core i5-1155G7, max 4.5 GHz

### How to check LLM/GPU capability

Check inside the Kubernetes node guest first:

```bash
ssh jotunheim 'lspci | egrep -i "vga|3d|display|nvidia|amd|intel" || true; echo ---; ls -lah /dev/dri 2>/dev/null || true; echo ---; test -e /dev/dri/renderD128 && echo RENDER=present || echo RENDER=absent; echo ---; command -v nvidia-smi >/dev/null && nvidia-smi || true'
ssh alfheim 'lspci | egrep -i "vga|3d|display|nvidia|amd|intel" || true; echo ---; ls -lah /dev/dri 2>/dev/null || true; echo ---; test -e /dev/dri/renderD128 && echo RENDER=present || echo RENDER=absent; echo ---; command -v nvidia-smi >/dev/null && nvidia-smi || true'
ssh niflheim 'lspci | egrep -i "vga|3d|display|nvidia|amd|intel" || true; echo ---; ls -lah /dev/dri 2>/dev/null || true; echo ---; test -e /dev/dri/renderD128 && echo RENDER=present || echo RENDER=absent; echo ---; command -v nvidia-smi >/dev/null && nvidia-smi || true'
```

Classify a node as `node-gpu=passthrough` when the Kubernetes guest can see the passed-through physical GPU in `lspci`, even if the user-space compute stack is not ready yet.

Use `node-llm-class` to express relative scheduling preference for inference:

- `igpu`: integrated GPU or otherwise low-end/shared-memory accelerator
- `consumer-gpu`: discrete desktop GPU such as a GeForce card
- `high-vram-gpu`: larger-memory accelerator that should be preferred for bigger models
- `none`: no meaningful accelerator present

Use `node-llm-vram` to express the memory tier that an inference workload can rely on:

- `shared` for iGPUs borrowing system memory
- explicit sizes such as `8gb`, `12gb`, `16gb`, `24gb`, or `48gb` for discrete cards
- `none` when there is no useful accelerator

Classify a node as `node-llm=gpu` only when the Kubernetes guest has an actually usable accelerator for inference, for example:

- a passed-through NVIDIA GPU visible to `nvidia-smi`
- a passed-through Intel or AMD GPU with usable render devices under `/dev/dri`

Do not count the default emulated QEMU VGA device as GPU-capable for LLM work.

At the time of writing, the current worker nodes do see their passed-through Intel GPUs in `lspci`, but they still expose only `card0` under `/dev/dri` and do not expose a render node such as `/dev/dri/renderD128`. That means the correct current labels are:

- `node-gpu=passthrough`
- `node-llm=none`
- `node-llm-class=igpu`
- `node-llm-vram=shared`

### Current Proxmox GPU state

The current Proxmox worker hosts do have onboard Intel GPUs:

- `proxmox-node1`: Intel Iris Xe Graphics `[8086:46a8]`
- `proxmox-node2`: Intel Iris Xe Graphics `[8086:46a8]`
- `proxmox-node4`: Intel Iris Xe Graphics `[8086:9a49]`

At the time of writing:

- all three hosts are on Proxmox VE `9.1.0`
- all three hosts now boot with `intel_iommu=on iommu=pt`
- all three hosts now load the VFIO modules and expose IOMMU groups
- the cluster-wide PCI mapping `intel-igpu` is defined in Proxmox for the three worker hosts
- the running worker VMs now see the physical Intel GPU at `01:00.0`
- the guests do not yet expose a render node such as `/dev/dri/renderD128`

That means passthrough is working at the PCI level, but the correct current inference label is still `node-llm=none` for every worker. Relative GPU preference should currently treat them as `node-llm-class=igpu` with `node-llm-vram=shared`, and future GeForce-backed workers should be labeled into `consumer-gpu` or `high-vram-gpu` with an explicit VRAM tier.

### Proxmox GPU passthrough checklist

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
8. Rebuild or update the worker VM, then verify the guest sees a real GPU.
   For Intel or AMD, check `/dev/dri`.
   For NVIDIA, check `nvidia-smi`.
9. Only after the guest has a usable accelerator, apply `node-llm=gpu`.

Important caveat for these hosts:

- each current Proxmox worker appears to rely on its integrated GPU as the primary host display adapter
- passing through the only GPU can remove local console output from the Proxmox host
- because these are iGPUs rather than discrete cards, success is more hardware-sensitive than on a server with a dedicated GPU

Treat passthrough on the current three hosts as feasible-but-experimental until one node is converted and verified end to end.

### Terraform support for passthrough-capable Proxmox workers

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

### How to apply labels

For Terraform-managed nodes, keep labels in the node tfvars under `kubelet_node_labels` so they are applied at join time.

For manually added Ubuntu nodes, apply labels after the node joins:

```bash
kubectl label node <node-name> node-performance=standard --overwrite
kubectl label node <node-name> node-gpu=none --overwrite
kubectl label node <node-name> node-llm=none --overwrite
kubectl label node <node-name> node-llm-class=none --overwrite
kubectl label node <node-name> node-llm-vram=none --overwrite
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

## Network Instability

Install the watchdog script and `systemd` timer on Proxmox:

```bash
scp scripts/network-watch.sh root@proxmox-node4:/usr/local/sbin/network-watch.sh
scp scripts/network-watch.service root@proxmox-node4:/etc/systemd/system/network-watch.service
scp scripts/network-watch.timer root@proxmox-node4:/etc/systemd/system/network-watch.timer
ssh root@proxmox-node4 'chmod 755 /usr/local/sbin/network-watch.sh && systemctl daemon-reload && systemctl enable --now network-watch.timer && systemctl status --no-pager network-watch.timer'
```

Useful follow-up checks on the host:

```bash
journalctl -k -g e1000e
journalctl -u network-watch.service -u network-watch.timer
systemctl list-timers network-watch.timer
ethtool nic0
ethtool --show-eee nic0
```
