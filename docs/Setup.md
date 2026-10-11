# Setup

This guide is only for bringing up and maintaining the Kubernetes environment. Related but separate workflows now live in dedicated docs:

- node labels and worker capability classification: [NodeClassification.md](/workspaces/homelab/docs/NodeClassification.md)
- dedicated Minecraft VM provisioning and in-guest management: [MinecraftVM.md](/workspaces/homelab/docs/MinecraftVM.md)
- external MinIO and Samba host bootstrap on `svartalfheim`: [StorageBootstrap.md](/workspaces/homelab/docs/StorageBootstrap.md)
- SeaweedFS cutover and offline version/client acceptance: [SeaweedFSMigration.md](SeaweedFSMigration.md) and [StorageVersionMigration.md](StorageVersionMigration.md)
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

You will need to re-sign the key every 30 days. Keep the CA passphrase in a password manager and retain a protected backup of the CA private key. In this example `root` is the user for Proxmox, `oliver` is the user set up for `yggdrasil`, and `ubuntu` is the default username used on the Ubuntu machines.

The repo Makefile now includes a helper for that refresh step:

```bash
make refresh-ssh
```

Optional overrides:

- `SSH_CERT_ID`: certificate identity, default `oliver@homelab`
- `SSH_CERT_PRINCIPALS`: SSH principals, default `oliver,ubuntu,root`
- `SSH_CA_KEY`: SSH CA private key path, default `~/.ssh/ssh_user_ca`
- `SSH_KEY_PUB`: login public key to sign, default `.devcontainer/ssh/id_ed25519.pub`
- `SSH_CERT_VALIDITY`: certificate lifetime window, default `+30d`

If a host is reachable but SSH suddenly fails with `Permission denied (publickey,password)`, check whether the login certificate expired before assuming the host-side SSH config changed:

```bash
ssh-keygen -L -f .devcontainer/ssh/id_ed25519-cert.pub
```

If the `Valid` window is in the past, re-sign the existing login key with the same command used during setup, or use the Makefile helper:

```bash
make refresh-ssh
```

### Recovering a lost SSH CA passphrase

The SSH user CA is separate from Kubernetes PKI. A lost SSH CA passphrase does not require recreating VMs, resetting kubeadm, or erasing cluster storage.

1. Preserve the old CA files and login certificate before generating an encrypted replacement CA. Store its new passphrase separately in a password manager.
2. Establish recovery access through an existing authorized login key, a physical/web console, or the Proxmox QEMU guest agent. A working Proxmox API token does not by itself prove host-shell access. With guest-agent execution, poll `exec-status` for terminal success and verify the resulting files.
3. Add the new **public** CA alongside existing trust. Terraform-managed Ubuntu guests use a `cert-authority,principals="ubuntu"` entry in `/home/ubuntu/.ssh/authorized_keys`. Manually configured hosts may instead use `TrustedUserCAKeys`; inspect the effective `sshd -T` configuration. Never copy the private CA key or its passphrase to a host.
4. Sign the existing login public key, then verify a fresh connection actually accepts the new `ED25519-CERT` identity and that required sudo access works. Keep existing trust until every intended host has been verified.
5. Install the replacement workstation CA at `~/.ssh/ssh_user_ca` (the devcontainer mounts `.devcontainer/ssh/` there), its public key beside it, and the new login certificate at `.devcontainer/ssh/id_ed25519-cert.pub`. Update the public CA entries in `etc/nodes/*.tfvars` for future provisioning. Do not apply Terraform merely to rotate live SSH trust: cloud-init changes can have VM lifecycle consequences.
6. After all hosts and recovery paths work, retire the old public CA trust explicitly. A changed SSH **host** key is a separate identity check; verify it through a trusted console or guest agent before changing `known_hosts`.

For Proxmox, inspect `/root/.ssh/authorized_keys` and its link to the cluster-managed `/etc/pve/priv/authorized_keys`. Adding a root-scoped CA entry to the shared file can restore all cluster hosts without restarting their workloads. Preserve unrelated entries and back up the file before editing it.

In `authorized_keys`, the line must begin with `cert-authority,principals="root"` (or the intended guest user). A bare `ssh-ed25519 ...` CA key is only a direct login-key entry and does not authorize certificates signed by that CA. This differs from a `TrustedUserCAKeys` file, which contains bare CA public keys. If a console command is copied through a UI that removes line breaks, use a single shell line with explicit separators and verify the installed entry afterward.

For a console where commands must be typed by hand, a temporary private-LAN HTTP endpoint can transfer the **public CA only**. Before installing it, compare `ssh-keygen -lf` against the exact fingerprint supplied through the trusted operator session; HTTP transport alone does not authenticate the key. Preserve the existing trust file and append the verified public key. Never serve private keys, passphrases, scripts to execute, or a directory listing. Time-limit the endpoint and stop it after verified SSH recovery.

Current recovery progress is recorded in the [troubleshooting runbook](../runbooks/troubleshooting.md#ssh-ca-recovery-2026-10-11).

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

For `helheim` host heat management on `proxmox-node3`, use the helper script in this repo:

```bash
./scripts/helheim-performance.sh down
./scripts/helheim-performance.sh up
```

`down` reduces host power draw by preferring the `low-power` platform profile when available, falling back to a non-quiet profile such as `balanced`, setting the CPU governor to `powersave`, and disabling turbo. Avoid using the `quiet` platform profile for this host because it can suppress fan response instead of only reducing heat generation. `up` restores the host to `performance` mode for heavier GPU or VM work.

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

## Worldbuilder migration

The initial Worldbuilder source baseline is verified and published; the broader [offline work audit](worldbuilder/OFFLINE-WORK.md) remains active, including timeline inheritance, vector discovery and remaining disposable platform/mesh drills. App and Helm publication require disposable API-server validation, real Prometheus/Grafana provisioning and [Prometheus controller reconciliation/recovery](worldbuilder/CONTROLLER-ACCEPTANCE.md). The platform gate now also includes [CNPG physical backup, WAL/PITR recovery and admission failure/recovery](worldbuilder/BACKUP-ACCEPTANCE.md), using synthetic data and external SeaweedFS (increment 29). It also includes [Velero file/resource recovery](worldbuilder/VELERO-ACCEPTANCE.md), with source loss, bad-credential refusal and controller replacement; increment 30 passes all platform gates and image/chart publication. Increment 31 adds Prometheus TLS admission, semantic and certificate refusal, and unavailable-operator recovery to the controller gate; platform acceptance and full external/orchestrator image/chart publication pass. Increment 32 prepares explicit [Prometheus persistence and TSDB recovery](worldbuilder/TSDB-RECOVERY.md), with local and hosted recovery checks, Prometheus chart publication and app publication verified. Opt-in [observability configuration](worldbuilder/OBSERVABILITY.md) is prepared and published; installed identities, selectors and datasource settings still need LAN verification. Reproduce browser, API-server, observability and load checks with [offline acceptance](worldbuilder/OFFLINE-ACCEPTANCE.md). Detailed implementation and CI evidence are tracked in [implementation status](worldbuilder/IMPLEMENTATION-STATUS.md). Use the [LAN runbook](worldbuilder/LAN-RUNBOOK.md) for discovery and controlled activation. Code publication on main is authorised; production migrations and feature activation require the separate LAN gate. Do not use the unfinished `scripts/pg-update.sh` placeholder for this migration.

Public Endpoint Validation runs automatically after successful app/Helm publication and checks HTTPS, MCP discovery and anonymous browser auth from GitHub's runner, with bounded retries and sanitized artifacts. A passing public check does not establish Flux reconciliation or authenticated feature acceptance. See [public connectivity diagnosis](worldbuilder/PUBLIC-VALIDATION.md); run locally with `python3 scripts/validation/public-endpoints.py --output /tmp/public-endpoints.json`.
