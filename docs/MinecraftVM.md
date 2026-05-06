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
- shared default image: `itzg/minecraft-server:java21`
- `atm11` overrides to `itzg/minecraft-server:java25` because its current NeoForge server bootstrap requires Java 25
- `atm11` also pins `NEOFORGE_VERSION=26.1.2.30-beta`
- `atm11` mirrors that pin into `CF_MOD_LOADER_VERSION` so the CurseForge installer refreshes the server with the same NeoForge build instead of resolving a newer one
- `atm11` starts from the installed `/data/run.sh` script instead of re-running the image's AUTO_CURSEFORGE NeoForge bootstrap on every restart, so the guest keeps using the repo-selected NeoForge build
- when a preinstalled AUTO_CURSEFORGE profile drifts, Ansible reruns the image once with `SETUP_ONLY=true` to refresh `run.sh`, `.curseforge-manifest.json`, and the installed NeoForge loader before restarting the service
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

If `docker ps` or `docker logs` returns a permission error for `/var/run/docker.sock`, rerun `make ansible-minecraft-vm` so the `ubuntu` SSH user is in the `docker` group, then reconnect your SSH session. `sudo docker ...` works as an immediate fallback.

## Updating a managed server version

When updating a managed profile such as `atm11`, take the server offline first so the world data is consistent, archive the profile data, copy that archive to the external MinIO host `svartalfheim`, then bump the pinned modpack version in Ansible and reapply the role.

Example: update `atm11` from `0.0.5` to `0.0.13`.

1. Stop the active Minecraft service.

```bash
ssh nidavellir 'sudo systemctl stop minecraft'
```

2. Package the current profile data into a compressed tarball on the VM.

```bash
ssh nidavellir '\
  ts=$(date -u +%Y%m%dT%H%M%SZ) && \
  mkdir -p /srv/minecraft/archives && \
  tar -C /srv/minecraft/servers/atm11 -czf /srv/minecraft/archives/atm11-${ts}.tar.gz data && \
  ls -lh /srv/minecraft/archives/atm11-${ts}.tar.gz'
```

3. Copy the archive to the MinIO host for safekeeping.

```bash
ssh nidavellir '\
  latest=$(ls -t /srv/minecraft/archives/atm11-*.tar.gz | head -n1) && \
  scp "$latest" svartalfheim:/srv/minio/backups/minecraft/'
```

4. Update the pinned profile version in [minecraft_vm.yml](/workspaces/homelab/ansible/inventory/group_vars/minecraft_vm.yml) by changing `atm11.runtime_env.CF_FILENAME_MATCHER` from `0.0.5` to `0.0.13`, keep `atm11.runtime_env.NEOFORGE_VERSION` aligned with the loader version the ATM11 release was tested against, and preserve `atm11.start_mode=preinstalled_run_script` unless the upstream AUTO_CURSEFORGE NeoForge mismatch is known to be fixed.

5. Reapply the VM configuration so the container restarts on the new game version.

```bash
make ansible-minecraft-vm
```

6. Verify the service is healthy again.

```bash
ssh nidavellir 'systemctl status --no-pager minecraft'
ssh nidavellir 'docker logs --tail 100 minecraft'
```

Notes:

- Archive the profile-specific directory under `/srv/minecraft/servers/<profile>/data`, not the `/srv/minecraft/data` symlink, so the backup remains tied to the intended server profile.
- The repo remains authoritative for the active profile and pinned version. Any local in-guest changes will be overwritten the next time `make ansible-minecraft-vm` runs.
- If `/srv/minio/backups/minecraft/` does not exist yet on `svartalfheim`, create it once before the first copy with `ssh svartalfheim 'mkdir -p /srv/minio/backups/minecraft'`.
