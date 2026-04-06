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

If you need to join an Ubuntu machine to the cluster outside the Terraform `cloud-init` flow, use the Ansible playbook for the manual-worker inventory group.

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

If you are using the Terraform worker flow instead, refresh the cached bootstrap token locally first:

```bash
make refresh-join-token
```

That target updates `TF_VAR_kubeadm_join_token` in `.devcontainer/.env` for repo-managed `cloud-init` bootstrap. It is not required for the manual Ansible join path above.


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
