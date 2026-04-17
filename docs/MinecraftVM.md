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
- default active server profile: `atm11`
- available managed server profiles: `atm11`, `atm10_tts`
- Minecraft heap remains `MEMORY=10G` and `INIT_MEMORY=2G`

## Service management

- provision the VM with `make plan COMPONENT=minecraft-vm LAYER=minecraft-node1`
- apply the VM with `make apply COMPONENT=minecraft-vm LAYER=minecraft-node1`
- configure Minecraft in-guest with `make ansible-minecraft-vm`
- the game runs as `minecraft.service` and exposes `25565/tcp` directly from the guest
- switch the active profile on-box with `sudo minecraft-switch <server>`

The Ansible role renders per-profile files under `/etc/minecraft/servers/` and keeps these active links in sync with the repo-selected profile:

- `/etc/minecraft/minecraft.env`
- `/etc/minecraft/runtime.env`
- `/srv/minecraft/data`
- `/srv/minecraft/backups`

On the first rollout, the old single-server `/srv/minecraft/data` and `/srv/minecraft/backups` directories are migrated into the `atm10_tts` profile before the active links are repointed to `atm11`.

Useful checks:

```bash
ENV=lab bin/tf plan minecraft-vm minecraft-node1
ssh nidavellir 'systemctl status --no-pager minecraft'
ssh nidavellir 'docker ps --filter name=minecraft'
ssh nidavellir 'cat /etc/minecraft/active-server'
ssh nidavellir 'readlink -f /srv/minecraft/data'
```
