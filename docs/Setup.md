# Setup

## Local

Install containerisation tool (Docker desktop, Rancher desktop, etc) and rebuild devcontainer

Create an SSH CA

``` bash
ssh-keygen -t ed25519 -f ~/.ssh/ssh_user_ca -C "homelab-user-ca"
```

Create your login certificate which will add it to the ./.devcontainer/ssh folder

``` bash
ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519 -C "homelab-login"
ssh-keygen -s .devcontainer/ssh/ssh_user_ca \
  -I "oliver@homelab" \
  -n oliver,ubuntu,root \
  -V +30d \
  .devcontainer/ssh/id_ed25519.pub

ssh-add ~/.ssh/id_ed25519
```

`ssh-add` only needs the private key. OpenSSH will use the matching certificate (`~/.ssh/id_ed25519-cert.pub`) automatically.

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

## SSH

Copy the public key to the new node

```bash
ssh-copy-id -i ~/.ssh/ssh_user_ca.pub root@192.168.8.X
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

Initialize the devcontainer kubeconfig from `yggdrasil`:

```bash
ssh yggdrasil 'cat ~/.kube/config' > .devcontainer/kubeconfig
git update-index --skip-worktree .devcontainer/kubeconfig
```

This file contains cluster credentials and private key material. Keep it out of git and treat it as a secret.

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
