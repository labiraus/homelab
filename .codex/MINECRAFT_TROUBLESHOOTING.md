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
  - `CF_FILENAME_MATCHER=0.0.6`
  - `NEOFORGE_VERSION=26.1.2.17-beta`
  - `start_mode=preinstalled_run_script`
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
- Symptom: `minecraft.service` keeps restarting and `docker logs` shows many mods failing with `NoClassDefFoundError: net/neoforged/neoforge/event/level/BlockEvent$BreakEvent`.
- April 21, 2026 finding: ATM11 `0.0.5` on `nidavellir` booted with NeoForge `26.1.2.22-beta`, while `.curseforge-manifest.json` declared `modLoaderId=neoforge-26.1.2.17-beta`. That newer loader broke mods including `occultism`, `lootr`, `refinedstorage`, `pylons`, and several Balm-based mods.
- Extra finding: even with `NEOFORGE_VERSION=26.1.2.17-beta` present in the container environment, the `itzg/minecraft-server` AUTO_CURSEFORGE startup path still reinstalled NeoForge `26.1.2.22-beta`.
- Repo workaround: install NeoForge `26.1.2.17-beta` directly with `mc-image-helper`, then run the generated `/data/run.sh` from the container entrypoint by setting `atm11.start_mode=preinstalled_run_script`.

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

## Public Networking

- player traffic is a direct router port-forward to `nidavellir:25565`
- there is no Kubernetes `Service`, `Gateway`, `TCPRoute`, or MetalLB path in the serving flow

If public connectivity fails:

```bash
ssh nidavellir 'ss -ltnp | grep 25565'
ssh nidavellir 'docker logs --tail=100 minecraft'
```

Then check the router port-forward and DNS/client path separately from the guest.

## Legacy Kubernetes Notes

Minecraft previously ran in Kubernetes under `helm/workloads/minecraft`.
That path was retired after repeated latency-sensitive gameplay issues and node instability on the cluster path.

Legacy K8s-specific notes should be treated as historical context only, not the current source of truth.
