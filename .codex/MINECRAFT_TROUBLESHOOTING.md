# Minecraft Troubleshooting

Use this file as the Codex memory for recurring issues and fixes specific to the dedicated Minecraft VM on `nidavellir`.

## Current Deployment Shape

- Proxmox host: `proxmox-node1`
- Terraform component: `components/minecraft-vm`
- Terraform layer: `minecraft-node1`
- guest hostname: `nidavellir`
- guest IP: `192.168.8.126`
- service manager: `systemd`
- runtime: Docker container `itzg/minecraft-server:java21`
- active-profile model: one running container at a time, switched by repo-managed symlinks
- port: `25565/tcp`
- mod loader: NeoForge
- modpack delivery: CurseForge via `MOD_PLATFORM=AUTO_CURSEFORGE`

## First Checks

```bash
ssh nidavellir 'systemctl status --no-pager minecraft'
ssh nidavellir 'docker ps --filter name=minecraft'
ssh nidavellir 'docker logs --tail=200 minecraft'
ssh nidavellir 'ss -ltnp | grep 25565'
ssh nidavellir 'cat /etc/minecraft/active-server'
ssh nidavellir 'readlink -f /srv/minecraft/data'
```

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
- `atm11`:
  - `CF_SLUG=all-the-mods-11`
  - `CF_FILENAME_MATCHER=0.0.4`
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
