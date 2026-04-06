# Minecraft VM

Minecraft runs outside Kubernetes on a dedicated Ubuntu VM on `proxmox-node1`.

## Deployment path

- Terraform component: `components/minecraft-vm`
- primary layer: `minecraft-node1`
- VM name: `nidavellir`
- Proxmox host: `proxmox-node1`
- LAN IP: `192.168.8.126`
- direct player exposure: router port-forward to TCP `25565`

## Current defaults

- `cpu_cores = 6`
- `memory_mb = 14336`
- `disk_size_gb = 120`
- Minecraft heap remains `MEMORY=10G` and `INIT_MEMORY=2G`

## Service management

- provision the VM with `make plan COMPONENT=minecraft-vm LAYER=minecraft-node1`
- apply the VM with `make apply COMPONENT=minecraft-vm LAYER=minecraft-node1`
- configure Minecraft in-guest with `make ansible-minecraft-vm`
- the game runs as `minecraft.service` and exposes `25565/tcp` directly from the guest

Useful checks:

```bash
ENV=lab bin/tf plan minecraft-vm minecraft-node1
ssh nidavellir 'systemctl status --no-pager minecraft'
ssh nidavellir 'docker ps --filter name=minecraft'
```
