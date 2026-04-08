# Setup

This guide is only for bringing up and maintaining the Kubernetes environment. Related but separate workflows now live in dedicated docs:

- node labels and worker capability classification: [NodeClassification.md](/workspaces/homelab/docs/NodeClassification.md)
- dedicated Minecraft VM provisioning and in-guest management: [MinecraftVM.md](/workspaces/homelab/docs/MinecraftVM.md)
- external MinIO and Samba host bootstrap on `svartalfheim`: [StorageBootstrap.md](/workspaces/homelab/docs/StorageBootstrap.md)
- generated local secrets and Ansible secret handling: [Secrets.md](/workspaces/homelab/docs/Secrets.md)

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

## SSH

Copy the public key to the new node

```bash
ssh-copy-id -o PreferredAuthentications=password -o PubkeyAuthentication=no -o PasswordAuthentication=yes -i ~/.ssh/ssh_user_ca.pub root@192.168.8.191
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

If you are setting up a new Ubuntu machine, create a user for yourself and add it to the sudo group with passwordless permissions:

```bash
sudo usermod -aG sudo oliver
echo 'oliver ALL=(ALL) NOPASSWD:ALL' | sudo tee /etc/sudoers.d/oliver
sudo chmod 440 /etc/sudoers.d/oliver
sudo visudo -cf /etc/sudoers.d/oliver
```

For ubuntu systems install ssh:

```bash
sudo apt update
sudo apt install -y openssh-server
sudo systemctl enable --now ssh
sudo ufw allow OpenSSH
```

Then include the following:

```bash
sudo install -m 644 -D /home/oliver/.ssh/authorized_keys /etc/ssh/trusted-user-ca-keys.pub
sudo sed -i '/^TrustedUserCAKeys /d' /etc/ssh/sshd_config
echo 'TrustedUserCAKeys /etc/ssh/trusted-user-ca-keys.pub' | sudo tee -a /etc/ssh/sshd_config >/dev/null
sudo sshd -t && sudo systemctl reload sshd
```

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
- The node exists in [hosts.ini](/workspaces/homelab/ansible/inventory/hosts.ini) under `kubernetes_manual_node`.
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

If the machine is being added as a manually managed Ubuntu worker rather than a Terraform-managed node, apply the expected node labels after it joins using [NodeClassification.md](/workspaces/homelab/docs/NodeClassification.md). For `midgard`, that means at least `node-gpu=passthrough` when the node can see its real local GPU, and `node-llm=gpu` only after the node exposes a usable render or compute interface.

When you are finished with a burst of GPU work on `midgard`, drain and power it off cleanly:

```bash
kubectl drain midgard --ignore-daemonsets --delete-emptydir-data --force
kubectl cordon midgard
ssh midgard 'sudo poweroff'
```

If `midgard` is intentionally offline afterward, leave it that way. A missing or powered-off `midgard` is expected when no heavy GPU work is queued.

### Terraform-Managed Worker Join

Terraform now builds the worker VM and installs the base Kubernetes packages, but the actual `kubeadm join` step is handled afterward through Ansible rather than through Terraform cloud-init.

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


## Network Instability

Install the watchdog script and `systemd` timer on Proxmox:

```bash
scp scripts/network-watch.sh root@proxmox-node4:/usr/local/sbin/network-watch.sh
scp scripts/network-watch.service root@proxmox-node4:/etc/systemd/system/network-watch.service
scp scripts/network-watch.timer root@proxmox-node4:/etc/systemd/system/network-watch.timer
ssh root@proxmox-node4 'chmod 755 /usr/local/sbin/network-watch.sh && systemctl daemon-reload && systemctl enable --now network-watch.timer && systemctl status --no-pager network-watch.timer'
```

The watchdog derives the management path from the current default route. On a routed bridge such as `vmbr0`, it pings the gateway through the bridge interface and restarts the bridge member that is actually carrying the uplink instead of assuming the physical NIC is always named `nic0`.

On the newer `cdc_ncm` USB dongles used on `proxmox-node1` and `proxmox-node4`, the Linux bridge's implicit promiscuous handling was not enough to keep VM return traffic working reliably. A direct `ip link set dev <uplink> promisc on` on the physical bridge member restored bidirectional traffic from `yggdrasil` to the worker VMs. The watchdog now reapplies explicit promiscuous mode on the detected uplink before testing or bouncing it.

If a Proxmox host is migrated to a new USB Ethernet dongle, stop the watchdog before changing the bridge or hot-plugging multiple candidate uplinks. A stale watchdog that still targets the previous NIC can flap the healthy interface while you are trying to recover the host:

```bash
systemctl stop network-watch.timer network-watch.service
systemctl disable network-watch.timer
```

USB Ethernet adapters can also fail in ways that look like Kubernetes or Flux problems but are really host-level link issues. In the April 8, 2026 outage, a new ASIX `AX88179` dongle was detected by Linux and exposed as `enx...`, but it was not the active bridge member yet, and one host showed intermittent USB registration failures while the old Realtek uplink was still the live management path. When diagnosing a Proxmox node that vanished from the cluster, confirm the host uplink first:

```bash
ip -br link
ip -br addr
ip route
bridge link
ethtool <candidate-uplink>
journalctl -k -b | egrep -i 'usb|asix|r8152|cdc_ncm|link|carrier'
```

If a newly attached USB dongle reports `LOWER_UP` / `Link detected: yes` but the host still does not recover, verify whether `vmbr0` is still bridged to the old interface in `/etc/network/interfaces`. A healthy dongle that is not yet the bridge member will not carry management traffic until the bridge config is updated.

Useful follow-up checks on the host:

```bash
journalctl -k | egrep -i 'usb|asix|r8152|cdc_ncm|e1000e|link|carrier'
journalctl -u network-watch.service -u network-watch.timer
systemctl list-timers network-watch.timer
ip route show default
bridge link
UPLINK=$(ip route show default | awk '/default/ {print $5; exit}'); if [ -d "/sys/class/net/$UPLINK/brif" ]; then UPLINK=$(basename "$(find "/sys/class/net/$UPLINK/brif" -mindepth 1 -maxdepth 1 | head -n1)"); fi; ethtool "$UPLINK"
ip link set dev "$UPLINK" promisc on
```

## `proxmox-node1` BIOS Updates

`proxmox-node1` is a Dell Latitude 5430. If the host is wiped and rebuilt, use the Linux `fwupd` path first before resorting to a manual Windows or BIOS menu update.

Install the required packages on the Proxmox host:

```bash
ssh root@proxmox-node1 'apt-get update'
ssh root@proxmox-node1 'DEBIAN_FRONTEND=noninteractive apt-get install -y fwupd fwupd-amd64-signed udisks2'
```

Make sure the EFI system partition is mounted and visible to `fwupd`:

```bash
ssh root@proxmox-node1 'findmnt /boot/efi'
ssh root@proxmox-node1 'fwupdtool esp-list'
```

Expected rebuild note:

- this host uses `/boot/efi` on `nvme0n1p2`
- `fwupd` will complain that no ESP is detected until `udisks2` is installed and running
- `/etc/fstab` should contain `UUID=E683-2C33 /boot/efi vfat defaults 0 1`

Check for updates and stage the BIOS capsule:

```bash
ssh root@proxmox-node1 'systemctl restart udisks2 fwupd'
ssh root@proxmox-node1 'fwupdmgr refresh --force'
ssh root@proxmox-node1 'fwupdmgr get-updates'
ssh root@proxmox-node1 'fwupdmgr update -y'
```

For the successful April 2026 repair attempt, `fwupdmgr` offered:

- `System Firmware`
- current version `1.32.1`
- target version `1.36.0`

Before rebooting, verify that the capsule has been staged:

```bash
ssh root@proxmox-node1 'fwupdmgr get-history | sed -n "/System Firmware/,+20p"'
ssh root@proxmox-node1 'ls /boot/efi/EFI/proxmox/fw'
```

Expected staged state:

- `System Firmware` shows `Needs reboot`
- `/boot/efi/EFI/proxmox/fw/` contains a `fwupd-*.cap` file

Then reboot the host and let the Dell firmware updater finish:

```bash
ssh root@proxmox-node1 'systemctl reboot'
```

After the host returns, confirm the update:

```bash
ssh root@proxmox-node1 'hostnamectl | rg "Firmware Version"'
ssh root@proxmox-node1 'dmidecode -t bios | sed -n "1,20p"'
```
