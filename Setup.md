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
ssh-add ~/.ssh/id_ed25519-cert.pub
```

You will need to run the second command every 30 days to re-sign the key, or remove the timout. In this example root is the user for proxmox, oliver is the user I set up for yggdrasil and ubuntu is the default username that will be used on all ubuntu machines.

## Chromebook

Before reimaging a chromebook with ubuntu from a usb drive you need to remove hardwork write protections, and then update the firmware:

``` bash
cd ~
curl -LO https://mrchromebox.tech/firmware-util.sh
sudo bash firmware-util.sh
```

## ProxMox

Copy the public key to the new node

```bash
ssh-copy-id -i ~/.ssh/ssh_user_ca.pub root@192.168.8.X
```

``` bash
hostnamectl set-hostname proxmox-nodeX
apt update && apt dist-upgrade -y
apt install -y chrony
systemctl enable --now chrony
systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target
sudo nano /etc/systemd/logind.conf
systemctl restart systemd-logind

pvesm set local --content iso,vztmpl,backup,import,snippets,images
```

For ubuntu systems include the following:

```bash
sudo install -m 644 -D /home/oliver/.ssh/authorized_keys /etc/ssh/trusted-user-ca-keys.pub
sudo sed -i '/^TrustedUserCAKeys /d' /etc/ssh/sshd_config
echo 'TrustedUserCAKeys /etc/ssh/trusted-user-ca-keys.pub' | sudo tee -a /etc/ssh/sshd_config >/dev/null
sudo sshd -t && sudo systemctl reload sshd
```

For proxmox use:

```bash
install -m 644 -D /root/.ssh/authorized_keys /etc/ssh/trusted-user-ca-keys.pub
sed -i '/^TrustedUserCAKeys /d' /etc/ssh/sshd_config
echo 'TrustedUserCAKeys /etc/ssh/trusted-user-ca-keys.pub' | tee -a /etc/ssh/sshd_config >/dev/null
sshd -t && systemctl reload sshd
```

## Kubernetes Control Plane

Initialize the devcontainer kubeconfig from `yggdrasil`:

```bash
# Run from the repository root
set -euo pipefail

mkdir -p .devcontainer
ssh yggdrasil 'cat ~/.kube/config' > .devcontainer/kubeconfig
chmod 600 .devcontainer/kubeconfig

# Quick sanity check
grep -E 'current-context:|server:' .devcontainer/kubeconfig
```

This file contains cluster credentials and private key material. Keep it out of git and treat it as a secret.
