# Minecraft

Minecraft runs in the dedicated `nidavellir` VM, managed through Terraform and
Ansible. It is separate from Worldbuilder. Use the pinned inventory/role inputs
as configuration authority; dated rollout observations are not present health.

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
- `ftb_skies_2_aero` pins FTB pack ID `134`, version ID `100497` (Aero `1.10.0`), and uses the image's FTBA bootstrap path on Java 21
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

For `ftb_skies_2_aero`, update `FTB_MODPACK_VERSION_ID` rather than a CurseForge filename
matcher. Verify the release name in the live [FTB pack API](https://api.feed-the-beast.com/v1/modpacks/public/modpack/134)
and [version API](https://api.feed-the-beast.com/v1/modpacks/public/modpack/134/100497): Aero
`1.10.0` is version `100497`, with Minecraft `1.21.1`, NeoForge `21.1.248`, and Java 21.
Indexed server-download pages may show a stale release label beside a newer version ID.
After applying, verify `/srv/minecraft/data/.ftb-installed`, the installed manifest, startup
completion in the journal, and an authenticated RCON response; an active systemd service alone
does not mean the modpack has finished installing or the world has loaded.

If SSH to `svartalfheim` is unavailable, verify a cold archive on the VM and an independent copy
on operator storage before upgrading. Record that the normal backup-host copy remains outstanding.

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

4. Update the pinned profile version in [minecraft_vm.yml](../ansible/inventory/group_vars/minecraft_vm.yml) by changing `atm11.runtime_env.CF_FILENAME_MATCHER` from `0.0.5` to `0.0.23`, keep `atm11.runtime_env.NEOFORGE_VERSION` aligned with the loader version the ATM11 release was tested against, and preserve `atm11.start_mode=preinstalled_run_script` unless the upstream AUTO_CURSEFORGE NeoForge mismatch is known to be fixed. Revisit any temporary `CF_EXCLUDE_MODS`, `remove_mod_globs`, or `extra_mods` overrides when the new pack bundles the fixed mod itself.

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

## First Checks

```bash
ssh nidavellir 'systemctl status --no-pager minecraft'
ssh nidavellir 'sudo docker ps --filter name=minecraft'
ssh nidavellir 'sudo docker logs --tail=200 minecraft'
ssh nidavellir 'ss -ltnp | grep 25565'
ssh nidavellir 'sudo cat /etc/minecraft/active-server'
ssh nidavellir 'readlink -f /srv/minecraft/data'
```

If plain `docker ...` fails with `permission denied` on `/var/run/docker.sock`, the SSH user session does not currently have Docker group access. The repo fix is to rerun `make ansible-minecraft-vm`, which appends `ubuntu` to the `docker` group on `nidavellir`, then reconnect SSH. `sudo docker ...` remains a valid immediate check.

## Startup Failure Notes

- Symptom: `minecraft.service` keeps restarting and `journalctl -u minecraft` shows `UnsupportedClassVersionError` for `net.neoforged.fml.startup.Server`.
- Meaning: the selected container image is running an older Java major version than the downloaded NeoForge server bootstrap needs.
- April 17, 2026 finding: `atm11` failed under `itzg/minecraft-server:java21` because the downloaded NeoForge server classes were compiled for class-file version `69` while Java 21 only supports up to `65`.
- Repo fix: pin `atm11` to `itzg/minecraft-server:java25` and rerun `make ansible-minecraft-vm`.
- Symptom: `minecraft.service` rejects players with `Incompatible client! Please use NeoForge 26.1.2.17-beta` after the managed client pack and repo config have moved on.
- April 24, 2026 update: the repo-authoritative ATM11 pin is now `NEOFORGE_VERSION=26.1.2.29-beta`.
- April 24, 2026 finding: `start_mode=preinstalled_run_script` can leave `/data/run.sh` and `.curseforge-manifest.json` pinned to an older NeoForge install even after `/etc/minecraft/minecraft.env` is updated.
- April 24, 2026 extra finding: `itzg/minecraft-server` AUTO_CURSEFORGE uses `CF_MOD_LOADER_VERSION` when refreshing a CurseForge pack; `NEOFORGE_VERSION` alone does not constrain the mod-loader reinstall path.
- Repo fix: have the Ansible role inspect the installed prebuilt files and rerun the image with `SETUP_ONLY=true`, `CF_FORCE_SYNCHRONIZE=true`, and `CF_FORCE_REINSTALL_MODLOADER=true` whenever the generated profile files drift from the repo pin.
- Repo fix: mirror the pinned NeoForge value into `CF_MOD_LOADER_VERSION` for AUTO_CURSEFORGE profiles so the refreshed install and generated `run.sh` stay on the same loader version the repo expects.
- Note: `atm11.start_mode=preinstalled_run_script` remains intentional so restarts keep using the repo-pinned NeoForge version instead of re-resolving the loader during AUTO_CURSEFORGE bootstrap.
- May 15, 2026 finding: the refresh container runs as uid/gid `1000`, so existing root-owned override paths can fail with `AccessDeniedException`, as seen with `/data/./local/kubejs`.
- Repo fix: before rerunning the one-shot CurseForge refresh for a drifted preinstalled profile, recursively restore the profile data tree to the configured Minecraft upload user/group.
- May 15, 2026 follow-up: after the ownership fix, a refresh can still fail on transient DNS for `mediafilez.forgecdn.net` via the LAN resolver (`192.168.8.1:53`), so the one-shot CurseForge refresh is retried before the playbook gives up.
- June 6, 2026 finding: ATM11 `0.0.23` can crash the server when a player kills an entity with `java.lang.NoSuchMethodError: LivingDamageEvent$Post.getNewDamage()` from `evilcraft-26.1.2-neoforge-1.2.96-949.jar`.
- Meaning: the bundled EvilCraft build calls a NeoForge `LivingDamageEvent.Post` method that is not present in the runtime API. EvilCraft `26.1.2-1.2.97` changed the heal-from-damage enchantment effect to use `getHealthDamage()` and updated to NeoForge `26.1.2.75`.
- Repo fix: pin `atm11` to `NEOFORGE_VERSION=26.1.2.75`, set `CF_EXCLUDE_MODS=evilcraft`, run a loader-only NeoForge setup with `loader_setup_version=26.1.2`, remove the bad bundled `evilcraft-26.1.2-neoforge-1.2.96-*` jar before extra mod installation, and install `evilcraft-26.1.2-neoforge-1.2.97.jar` from Modrinth with a checksum. Remove this local replacement once a newer ATM11 release bundles the fixed EvilCraft version.
- June 6, 2026 follow-up: the AUTO_CURSEFORGE refresh logged that it was overriding the mod loader from `26.1.2.71` to `26.1.2.75`, but still ran the pack's `26.1.2.71` NeoForge installer. A separate loader-only `TYPE=NEOFORGE VERSION=26.1.2 NEOFORGE_VERSION=26.1.2.75 SETUP_ONLY=true` run updated `/data/run.sh` and installed the `26.1.2.75` libraries.
- June 10, 2026 follow-up: keep the removal glob narrow, such as `evilcraft-26.1.2-neoforge-1.2.96-*.jar`, so Ansible does not delete the replacement `evilcraft-26.1.2-neoforge-1.2.97.jar` and restart Minecraft on every playbook run.
- June 13, 2026 update: remove the temporary EvilCraft replacement for ATM11 `0.3.0`. The repo no longer sets `CF_EXCLUDE_MODS=evilcraft`, no longer removes `evilcraft-26.1.2-neoforge-1.2.96-*`, and no longer installs `evilcraft-26.1.2-neoforge-1.2.97.jar`; the server should use the EvilCraft jar bundled by the ATM11 `0.3.0` CurseForge server pack.
- June 13, 2026 finding: player login still failed after the EvilCraft override was removed. Server logs showed repeated SecurityCraft manual packet stack traces: `Recipe#assemble unexpectedly returned null for type crafting` from `net.geforcemods.securitycraft.items.SCManualItem.safeAssemble`, followed by client-side connection resets.
- June 13, 2026 failed attempt: replacing ATM11's `[26.1.2] SecurityCraft v1.10.1-beta3.jar` with Modrinth's stable `[1.21.1] SecurityCraft v1.10.1.jar` did not work. The stable jar declares `minecraft` dependency range `[1.21.1,1.22)`, while this ATM11 NeoForge server presents `minecraft` as `26.1.2`, so the server refuses to start. Do not use that jar as the replacement for this pack.
- June 10, 2026 finding: ATM11 `0.3.0` crashed while loading `kaisyn:village/birch_forest_romanian/streets/crossroad_04` with `java.lang.OutOfMemoryError: Java heap space`.
- Meaning: in `start_mode=preinstalled_run_script`, `minecraft.service` launches `/data/run.sh` directly. That script reads `/data/user_jvm_args.txt`, and the live file still contained NeoForge's default `-Xmx1G -Xms1G`, so the repo `MEMORY=10G` value was not reaching Java at runtime.
- Repo fix: manage `/data/user_jvm_args.txt` for preinstalled profiles from `runtime_env.MEMORY` and `runtime_env.INIT_MEMORY`, then restart `minecraft.service`.
A connect timeout with no new server logs can be stale public DNS even when the
VM and router forwarding are healthy. Compare current `mc.labiraus.com` resolution
with observed WAN egress from both the workstation and guest. Test TCP 25565 only
against the newly verified address; never reuse a historical WAN address.

```bash
dig +short mc.labiraus.com
curl -4 https://api.ipify.org
ssh nidavellir 'curl -4 https://api.ipify.org'
```

## Legacy Kubernetes Notes

Minecraft previously ran in a now-retired Kubernetes workload chart (available in Git history).
That path was retired after repeated latency-sensitive gameplay issues and node instability on the cluster path.

Legacy K8s-specific notes should be treated as historical context only, not the current source of truth.
