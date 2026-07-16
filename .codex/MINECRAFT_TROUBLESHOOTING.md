# Minecraft Troubleshooting

Use this file as the Codex memory for recurring issues and fixes specific to the dedicated Minecraft VM on `nidavellir`.

## Current Deployment Shape

- Proxmox host: `proxmox-node1`
- Terraform component: `components/minecraft-vm`
- Terraform layer: `minecraft-node1`
- guest hostname: `nidavellir`
- guest IP: `192.168.8.126`
- service manager: `systemd`
- runtime: active profile image managed by Ansible symlinks
- active-profile model: one running container at a time, switched by repo-managed symlinks
- port: `25565/tcp`
- mod loader: NeoForge
- modpack delivery: CurseForge via `MOD_PLATFORM=AUTO_CURSEFORGE`

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

## Provisioning And Config

```bash
make plan COMPONENT=minecraft-vm LAYER=minecraft-node1
make apply COMPONENT=minecraft-vm LAYER=minecraft-node1
make ansible-minecraft-vm
```

The Terraform layer provisions the VM.
The Ansible playbook installs Docker, renders per-profile files under `/etc/minecraft/servers`, keeps active symlinks at `/etc/minecraft/minecraft.env`, `/etc/minecraft/runtime.env`, `/srv/minecraft/data`, and `/srv/minecraft/backups`, and manages `minecraft.service`.

## Runtime Defaults

- active profile: `atm11`
- alternate preserved profile: `atm10_tts`
- shared defaults:
  - `MEMORY=10G`
  - `INIT_MEMORY=2G`
  - `TYPE=NEOFORGE`
  - `VERSION=1.21.1`
  - `MOD_PLATFORM=AUTO_CURSEFORGE`
- shared image default: `itzg/minecraft-server:java21`
- `atm11`:
  - `image=itzg/minecraft-server:java25`
  - `CF_SLUG=all-the-mods-11`
  - `CF_FILENAME_MATCHER=0.2.1`
  - `NEOFORGE_VERSION=26.1.2.76`
  - `loader_setup_version=26.1.2`
  - `start_mode=preinstalled_run_script`
  - extra server-side login mitigation: `connectivity-26.1-7.6.jar`
- `atm10_tts`:
  - `CF_SLUG=all-the-mods-10-sky`
  - `CF_FILENAME_MATCHER=2.0.2`

`MINECRAFT_CURSEFORGE_API_KEY` must be present in `ansible/.env` or the operator environment before running the Ansible playbook.

Switch profiles locally:

```bash
ssh nidavellir 'sudo minecraft-switch atm10_tts'
ssh nidavellir 'sudo minecraft-switch atm11'
```

The next `make ansible-minecraft-vm` reapplies the repo-selected active profile.

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
- June 13, 2026 update: remove the temporary EvilCraft replacement for ATM11 `0.2.1`. The repo no longer sets `CF_EXCLUDE_MODS=evilcraft`, no longer removes `evilcraft-26.1.2-neoforge-1.2.96-*`, and no longer installs `evilcraft-26.1.2-neoforge-1.2.97.jar`; the server should use the EvilCraft jar bundled by the ATM11 `0.2.1` CurseForge server pack.
- June 13, 2026 finding: player login still failed after the EvilCraft override was removed. Server logs showed repeated SecurityCraft manual packet stack traces: `Recipe#assemble unexpectedly returned null for type crafting` from `net.geforcemods.securitycraft.items.SCManualItem.safeAssemble`, followed by client-side connection resets.
- June 13, 2026 failed attempt: replacing ATM11's `[26.1.2] SecurityCraft v1.10.1-beta3.jar` with Modrinth's stable `[1.21.1] SecurityCraft v1.10.1.jar` did not work. The stable jar declares `minecraft` dependency range `[1.21.1,1.22)`, while this ATM11 NeoForge server presents `minecraft` as `26.1.2`, so the server refuses to start. Do not use that jar as the replacement for this pack.
- June 10, 2026 finding: ATM11 `0.2.1` crashed while loading `kaisyn:village/birch_forest_romanian/streets/crossroad_04` with `java.lang.OutOfMemoryError: Java heap space`.
- Meaning: in `start_mode=preinstalled_run_script`, `minecraft.service` launches `/data/run.sh` directly. That script reads `/data/user_jvm_args.txt`, and the live file still contained NeoForge's default `-Xmx1G -Xms1G`, so the repo `MEMORY=10G` value was not reaching Java at runtime.
- Repo fix: manage `/data/user_jvm_args.txt` for preinstalled profiles from `runtime_env.MEMORY` and `runtime_env.INIT_MEMORY`, then restart `minecraft.service`.
- June 17, 2026 finding: ATM11 `0.2.1` failed during FML startup because `fastsuite`, `cristellib`, and `gateways` require NeoForge `26.1.2.76` or newer while the repo still pinned `26.1.2.75`.
- Repo fix: bump `atm11.runtime_env.NEOFORGE_VERSION` to `26.1.2.76` and rerun `make ansible-minecraft-vm` so the loader-only setup refreshes `/data/run.sh` and installed NeoForge libraries.

## Lag Notes

- Symptom seen on the dedicated VM: brief 1-3 second delays on interactions such as opening chests, even after moving Minecraft off Kubernetes.
- Observed fix: set `sync-chunk-writes=false` in `/srv/minecraft/data/server.properties` and restart `minecraft.service`.
- Why this matters: with synchronous chunk writes enabled, the server thread can pause waiting for chunk/world writes to complete, which looks like gameplay lag even when VM CPU, RAM, and disk space are healthy.
- Current repo note: this appears to have been the main cause of the remaining lag spikes on `nidavellir`.

Apply on the VM:

```bash
ssh nidavellir 'sudo systemctl stop minecraft'
ssh nidavellir 'sudo sed -i "s/^sync-chunk-writes=.*/sync-chunk-writes=false/" /srv/minecraft/data/server.properties'
ssh nidavellir 'sudo systemctl start minecraft'
```

## Login Timeout Notes

- Symptom: players authenticate successfully, then the server logs `lost connection: Timed out` during `ServerConfigurationPacketListenerImpl` or `Took too long to log in`.
- May 28, 2026 finding: ATM11 had `pause-when-empty-seconds=60`, and the server logged `Server empty for 60 seconds, pausing` immediately before repeated login timeouts. A paused modded server may not complete the login/configuration phase for the first connecting player.
- Repo fix: manage `pause-when-empty-seconds=-1` alongside `sync-chunk-writes=false` in the Ansible-rendered `server.properties` overrides, then restart `minecraft.service`.
- May 28, 2026 follow-up: disabling empty-server pause was not enough. The remaining failure showed the player authenticating, NeoForge injecting `GenericPacketSplitter`, then `ServerConfigurationPacketListenerImpl` timing out after roughly 30 seconds with no server resource pressure.
- Repo mitigation: install `connectivity-26.1-7.6.jar` as an `atm11.extra_mods` entry. Connectivity explicitly targets login timeouts, packet-size errors, and payload issues, and its project page says the mod is not required on both sides.
- May 30, 2026 follow-up: players could sometimes join, then disconnect around the in-game 60-second timeout window. Manage `config/connectivity.json` with `logintimeout=240`, `disconnectTimeout=180`, and `debugPrintMessages=true` so slow modded handshakes have more room and future failures produce better packet diagnostics.

Apply on the VM:

```bash
ssh nidavellir 'sudo systemctl stop minecraft'
ssh nidavellir 'sudo sed -i "s/^pause-when-empty-seconds=.*/pause-when-empty-seconds=-1/" /srv/minecraft/data/server.properties'
ssh nidavellir 'curl -fL -o /tmp/connectivity-26.1-7.6.jar https://mediafilez.forgecdn.net/files/7866/655/connectivity-26.1-7.6.jar && sudo install -o ubuntu -g ubuntu -m 0644 /tmp/connectivity-26.1-7.6.jar /srv/minecraft/data/mods/connectivity-26.1-7.6.jar'
ssh nidavellir 'sudo systemctl start minecraft'
```

## Public Networking

- player traffic is a direct router port-forward to `nidavellir:25565`
- there is no Kubernetes `Service`, `Gateway`, `TCPRoute`, or MetalLB path in the serving flow

If public connectivity fails:

```bash
ssh nidavellir 'ss -ltnp | grep 25565'
ssh nidavellir 'docker logs --tail=100 minecraft'
```

Then check the router port-forward and DNS/client path separately from the guest.

- June 17, 2026 finding: a connect timeout with no new Minecraft logs can be caused by stale public DNS even when the VM and router forward are healthy.
- Observed example on June 17, 2026: `mc.labiraus.com` still resolved to `86.130.183.215`, while the live WAN IPv4 reported by both the devcontainer and `nidavellir` was `81.146.38.8`. Direct TCP connect to `81.146.38.8:25565` succeeded, but `mc.labiraus.com:25565` timed out.
- Quick check:

```bash
dig +short mc.labiraus.com
curl -4 https://api.ipify.org
ssh nidavellir 'curl -4 https://api.ipify.org'
nc -vz -w 5 81.146.38.8 25565
```

## Legacy Kubernetes Notes

Minecraft previously ran in Kubernetes under `helm/workloads/minecraft`.
That path was retired after repeated latency-sensitive gameplay issues and node instability on the cluster path.

Legacy K8s-specific notes should be treated as historical context only, not the current source of truth.
