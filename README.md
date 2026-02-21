# homelab

## Chromeboook

ssh oliver@192.168.8.132 - yggdrasil

### Initial Setup

``` bash
cd ~
curl -LO https://mrchromebox.tech/firmware-util.sh
sudo bash firmware-util.sh
```

## Proxmox cluster

### Nodes

root@192.168.8.229:8006 - proxmox-node1
root@192.168.8.133:8006 - proxmox-node2
root@192.168.8.231:8006 - proxmox-node3
root@192.168.8.103:8006 - proxmox-node4

### Initial setup

``` bash
hostnamectl set-hostname proxmox-nodex
apt update && apt dist-upgrade -y
apt install -y chrony
systemctl enable --now chrony
systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target
sudo nano /etc/systemd/logind.conf
systemctl restart systemd-logind
```

ssh root@192.168.8.229
ssh root@192.168.8.133
ssh root@192.168.8.231
ssh root@192.168.8.103

- Add the following lines to the /etc/hosts file on each node:

`nano /etc/hosts`

``` text
192.168.8.229 proxmox-node1
192.168.8.133 proxmox-node2
192.168.8.231 proxmox-node3
192.168.8.103 proxmox-node4
192.168.8.132 yggdrasil
```

`systemctl restart pveproxy pvedaemon`

- Create cluster on the first node

`pvecm create homecluster`

- Join the other nodes to the cluster

`pvecm add 192.168.8.229`
