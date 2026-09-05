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
- default active server profile: `ftb_skies_2_aero`
- available managed server profiles: `atm10_aeronautics`, `ftb_skies_2_aero`, `atm11`, `atm10_tts`
- shared default image: `itzg/minecraft-server:java21`
- `ftb_skies_2_aero` pins FTB pack ID `134`, version ID `100490` (Aero `1.9.1`), and uses the image's FTBA bootstrap path on Java 21
- `atm10_aeronautics` pins the CurseForge pack `all-the-mods-10-aeronautics` at `0.4.1` (Minecraft 1.21.1 / NeoForge) and uses the image's AUTO_CURSEFORGE bootstrap path on Java 21
- first-time modpack installation is allowed up to one hour to create `server.properties`, since a modpack installer can download thousands of individual pack files
- `atm11` overrides to `itzg/minecraft-server:java25` because its current NeoForge server bootstrap requires Java 25
- `atm11` also pins `NEOFORGE_VERSION=26.1.2.93`
- `atm11` mirrors that pin into `CF_MOD_LOADER_VERSION` so the CurseForge installer refreshes the server with the same NeoForge build instead of resolving a newer one
- `atm11` starts from the installed `/data/run.sh` script instead of re-running the image's AUTO_CURSEFORGE NeoForge bootstrap on every restart, so the guest keeps using the repo-selected NeoForge build
- when a preinstalled AUTO_CURSEFORGE profile drifts, Ansible reruns the image once with `SETUP_ONLY=true` to refresh `run.sh`, `.curseforge-manifest.json`, and the installed NeoForge loader before restarting the service
- `atm11` then runs a loader-only NeoForge setup with `loader_setup_version=26.1.2`, because the ATM11 server pack can keep its original NeoForge installer even when AUTO_CURSEFORGE accepts a newer `CF_MOD_LOADER_VERSION`
- preinstalled profiles get `/data/user_jvm_args.txt` managed from the repo `MEMORY` and `INIT_MEMORY` values so direct `/data/run.sh` starts with the intended heap
- before that refresh, Ansible restores the drifted profile data tree to the configured Minecraft upload user so existing override directories remain writable to the setup container
- the drift refresh retries transient CurseForge CDN download and DNS failures before failing the playbook
- `atm11` installs the server-side `connectivity-26.1-7.6.jar` extra mod after any CurseForge refresh to mitigate NeoForge login/configuration timeout and packet-size issues; Ansible also manages `config/connectivity.json` with longer login and in-game disconnect windows
- `atm11` no longer carries the temporary EvilCraft replacement used for ATM11 `0.0.23`; the managed `0.3.0` profile should use the EvilCraft jar bundled by the CurseForge server pack
- `atm11` overrides `spawn-protection=0` so players can build at world spawn after the next service restart
- Minecraft heap is managed as `MEMORY=10G` and `INIT_MEMORY=2G`

## Service management

- provision the VM with `make plan COMPONENT=minecraft-vm LAYER=minecraft-node1`
- apply the VM with `make apply COMPONENT=minecraft-vm LAYER=minecraft-node1`
- configure Minecraft in-guest with `make ansible-minecraft-vm`; this requires `MINECRAFT_CURSEFORGE_API_KEY` and `MINECRAFT_RCON_PASSWORD` in `ansible/.env` or the shell environment
- the game runs as `minecraft.service` and exposes `25565/tcp` directly from the guest
- switch the active profile on-box with `sudo minecraft-switch <server>`; the profile-aware launcher selects the normal image bootstrap or the preinstalled `run.sh` path from that profile's runtime metadata

Local `ansible/.env` example:

```bash
MINECRAFT_CURSEFORGE_API_KEY='replace-with-curseforge-api-key'
MINECRAFT_RCON_PASSWORD='replace-with-rcon-password'
```

The Ansible role renders per-profile files under `/etc/minecraft/servers/` and keeps these active links in sync with the repo-selected profile:

- `/etc/minecraft/minecraft.env`
- `/etc/minecraft/runtime.env`
- `/srv/minecraft/data`
- `/srv/minecraft/backups`

The original multi-profile rollout migrated the old single-server `/srv/minecraft/data` and `/srv/minecraft/backups` directories into `atm10_tts`. Current rollouts preserve that historical profile and point the active links to the repo-selected profile.

Useful checks:

```bash
ENV=lab bin/tf plan minecraft-vm minecraft-node1
ssh nidavellir 'systemctl status --no-pager minecraft'
ssh nidavellir 'docker ps --filter name=minecraft'
ssh nidavellir 'cat /etc/minecraft/active-server'
ssh nidavellir 'readlink -f /srv/minecraft/data'
```

Switch between managed profiles with:

```bash
ssh nidavellir 'sudo minecraft-switch atm11'
ssh nidavellir 'sudo minecraft-switch atm10_aeronautics'
ssh nidavellir 'sudo minecraft-switch ftb_skies_2_aero'
```

The switch helper gracefully restarts the single `minecraft.service`; all profiles remain isolated
under `/srv/minecraft/servers/<profile>`, and only one container binds TCP `25565` at a time. Rerunning
the Ansible playbook restores the repo-authoritative `ftb_skies_2_aero` selection.

Before the first Aero deployment, preserve a cold copy of the ATM11 world:

```bash
ssh nidavellir 'sudo docker exec minecraft rcon-cli "save-all flush"'
ssh nidavellir 'sudo systemctl stop minecraft'
ssh nidavellir '\
  ts=$(date -u +%Y%m%dT%H%M%SZ) && \
  sudo tar -C /srv/minecraft/servers/atm11/data -czf /srv/minecraft/archives/atm11-world-${ts}.tar.gz world && \
  sudo tar -tzf /srv/minecraft/archives/atm11-world-${ts}.tar.gz >/dev/null && \
  df -h /srv/minecraft'
```

Do not reuse or synchronize the ATM11 data directory for Aero. If Aero initialization fails after
the role has installed the launcher and switch helper, restore ATM11 with
`sudo minecraft-switch atm11`.

If `docker ps` or `docker logs` returns a permission error for `/var/run/docker.sock`, rerun `make ansible-minecraft-vm` so the `ubuntu` SSH user is in the `docker` group, then reconnect your SSH session. `sudo docker ...` works as an immediate fallback.

If `docker logs minecraft` reports `No such container`, inspect `sudo journalctl -u minecraft -n 200 --no-pager`. The systemd unit starts Docker with `--rm`, so a container that fails during startup is removed immediately and its failure remains in the unit journal.

## Updating a managed server version

When updating a managed profile such as `atm11`, take the server offline first so the world data is consistent, archive the profile data, copy that archive to the external MinIO host `svartalfheim`, then bump the pinned modpack version in Ansible and reapply the role.

Example: update `atm11` from `0.0.5` to `0.0.23`.

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

4. Update the pinned profile version in [minecraft_vm.yml](/workspaces/homelab/ansible/inventory/group_vars/minecraft_vm.yml) by changing `atm11.runtime_env.CF_FILENAME_MATCHER` from `0.0.5` to `0.0.23`, keep `atm11.runtime_env.NEOFORGE_VERSION` aligned with the loader version the ATM11 release was tested against, and preserve `atm11.start_mode=preinstalled_run_script` unless the upstream AUTO_CURSEFORGE NeoForge mismatch is known to be fixed. Revisit any temporary `CF_EXCLUDE_MODS`, `remove_mod_globs`, or `extra_mods` overrides when the new pack bundles the fixed mod itself.

5. Reapply the VM configuration so the container restarts on the new game version.

```bash
make ansible-minecraft-vm
```

6. Verify the service is healthy again.

```bash
ssh nidavellir 'systemctl status --no-pager minecraft'
ssh nidavellir 'docker logs -f --tail 100 minecraft'
```

7. Run Minecraft server commands through RCON.

```bash
ssh -t nidavellir 'docker exec -it minecraft rcon-cli'
```

Use Minecraft server commands without a leading slash. The Ansible role enables RCON in `server.properties` and stores the RCON password in the root-only container env file at `/etc/minecraft/minecraft.env`.

Notes:

- Archive the profile-specific directory under `/srv/minecraft/servers/<profile>/data`, not the `/srv/minecraft/data` symlink, so the backup remains tied to the intended server profile.
- The repo remains authoritative for the active profile and pinned version. Any local in-guest changes will be overwritten the next time `make ansible-minecraft-vm` runs.
- If `/srv/minio/backups/minecraft/` does not exist yet on `svartalfheim`, create it once before the first copy with `ssh svartalfheim 'mkdir -p /srv/minio/backups/minecraft'`.
