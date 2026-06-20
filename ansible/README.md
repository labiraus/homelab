# Ansible External Service Management

This directory manages external services out-of-band from Kubernetes:

- MinIO server bootstrap on `svartalfheim`
- buckets
- users/access keys
- policies
- lifecycle rules
- optional replication/mirroring skeleton
- Samba network sharing for the attached `svartalfheim` hard drive
- dedicated Minecraft VM service configuration on `nidavellir`

Kubernetes remains Helm/Flux-managed. Storage-host services are Ansible-managed.
The authoritative MinIO service and attached-drive network share for this repo both live on the external Raspberry Pi host `svartalfheim`.
Minecraft now runs on its own Proxmox VM and is managed through Ansible rather than Helm.

## Prerequisites

- `ansible-core` 2.15+
- MinIO client `mc` on the machine running playbooks
- Optional: `amazon.aws` collection for future S3 module adoption
- SSH access with sudo for external host bootstrap playbooks

Install collection (optional):

```bash
ansible-galaxy collection install amazon.aws
```

## Inventory and targets

- External Pi API-management group: `minio_external_pi`
- External Pi host bootstrap group: `minio_external_pi_host`
- Dedicated Minecraft VM group: `minecraft_vm`
- Manual Ubuntu Kubernetes node group: `kubernetes_manual_node`
- Terraform-managed Ubuntu Kubernetes worker group: `kubernetes_terraform_node`

The API-management groups default to `localhost` because those playbooks run from your laptop/CI/management node and call MinIO APIs.

The external host bootstrap group connects to the Pi over SSH and installs the MinIO server binary plus a `systemd` service.
The Minecraft VM group connects over SSH and configures the in-guest Minecraft service and systemd unit.
The Kubernetes node groups connect over SSH, install the worker prerequisites on Ubuntu, and join the node with a fresh token fetched from `yggdrasil` at runtime.

## Secure credentials

Keep Ansible secret material local-only and split it by scope:

- shared operator settings or credentials used across the repo belong in `.devcontainer/.env`
- Ansible-only external-service secrets belong in `ansible/.env`
- cluster-derived credentials should be rebuilt from the cluster when possible rather than copied by hand into env files

Sync the MinIO admin credentials from `svartalfheim` into `ansible/.env`:

```bash
make refresh-ansible-secrets
```

This SSHes to `svartalfheim`, reads `/etc/default/minio`, and upserts:

- `MINIO_EXTERNAL_ADMIN_ACCESS_KEY`
- `MINIO_EXTERNAL_ADMIN_SECRET_KEY`

The script preserves any other existing entries in `ansible/.env`.

Current limitation:

- `SVARTALFHEIM_SAMBA_PASSWORD` is not recoverable from `/etc/default/minio`
- managed MinIO user access keys and secret keys are not recoverable from the live MinIO server after creation
- keep those remaining Ansible-only secrets in local `ansible/.env` until they gain their own fetch workflow

Set admin credentials for the `svartalfheim` MinIO endpoint:

```bash
export MINIO_EXTERNAL_ADMIN_ACCESS_KEY='...'
export MINIO_EXTERNAL_ADMIN_SECRET_KEY='...'
```

For the external Pi bootstrap playbook, these same environment variables are used as the MinIO root credentials written into the service environment file on the host.

Set managed user credentials (do not commit):

```bash
export MINIO_VELERO_ACCESS_KEY='...'
export MINIO_VELERO_SECRET_KEY='...'
export MINIO_CNPG_ACCESS_KEY='...'
export MINIO_CNPG_SECRET_KEY='...'
export MINIO_DOCUMENTS_ACCESS_KEY='...'
export MINIO_DOCUMENTS_SECRET_KEY='...'
export MINIO_SITE1_UPLOADER_ACCESS_KEY='...'
export MINIO_SITE1_UPLOADER_SECRET_KEY='...'
```

Set the Samba password for the `oliver` network-share account:

```bash
export SVARTALFHEIM_SAMBA_PASSWORD='...'
```

Use Ansible Vault or CI secret store for production automation.
For local interactive runs in this repo, prefer `ansible/.env` plus the wrapper scripts below over exporting secrets into every shell.

## Run playbooks

Bootstrap the external Raspberry Pi host services:

```bash
make bootstrap-svartalfheim-storage
```

This installs the MinIO binary from `https://dl.min.io/server/minio/release/linux-arm64/minio`, creates a `systemd` service, stores objects under `/srv/minio/minio-data` by default, installs Samba, and exposes the attached drive as the `storage` share backed by `/srv/minio`.

The bootstrap wrapper automatically handles first install versus reapply:

- first install: if `/etc/default/minio` is absent on `svartalfheim`, it uses the bootstrap values already present in `ansible/.env`
- later runs: if `/etc/default/minio` exists, it refreshes `MINIO_EXTERNAL_ADMIN_ACCESS_KEY` and `MINIO_EXTERNAL_ADMIN_SECRET_KEY` from the host before running the playbook

On `svartalfheim`, both MinIO and the Samba share currently run through user `oliver` because the attached NTFS disk is mounted with ownership mapped to that account. If the disk is later migrated to a Linux-native filesystem, consider switching MinIO and file-sharing ownership to dedicated service users.

Apply MinIO buckets, policies, users, lifecycle rules, and requested bucket versioning against `svartalfheim`:

```bash
make ansible-minio-state
```

The `documents` bucket is versioned so assistant-approved edits and reverts have raw-object rollback points. Its current-object lifecycle remains disabled until a noncurrent-version retention policy is chosen.

Configure the dedicated Minecraft VM:

```bash
make ansible-minecraft-vm
```

This target intentionally disables the MinIO secret-refresh helper because the Minecraft VM playbook does not require the external MinIO admin credentials.

Prepare and join the manual Ubuntu worker `midgard`:

```bash
ANSIBLE_FETCH_MINIO_SECRETS=0 ./scripts/ansible-run-playbook.sh \
  -i ansible/inventory/hosts.ini \
  ansible/playbooks/kubernetes-manual-node.yml \
  --limit midgard
```

This playbook:

- installs `containerd` and the Kubernetes packages from `pkgs.k8s.io`
- disables swap and applies the required kernel modules and sysctls
- can optionally switch a manual worker into a headless boot profile by disabling display-manager services, setting the default target to `multi-user.target`, and purging configured desktop packages
- enables `containerd` and `kubelet`
- fetches a fresh `kubeadm join` command from `yggdrasil`
- joins the node only when `/etc/kubernetes/kubelet.conf` does not already exist
- can optionally configure NVIDIA GPU support for Kubernetes on selected manual nodes, including the guest driver, `nvidia-ctk` containerd runtime wiring, and the device-plugin/runtime manifests

Operational note for `midgard`:

- it is an intermittent worker intended for heavier GPU jobs, not an always-on baseline node
- if it is powered off when no GPU work is queued, that is expected and should not be treated as a bug
- it should boot headless rather than into a local GNOME session so the NVIDIA GPU stays available for worker use
- before scheduling GPU workloads there, verify the node can actually see its local GPU with `lspci`, `/dev/dri`, or `nvidia-smi`
- the `midgard` host vars now enable NVIDIA support in this playbook, which installs the guest NVIDIA driver plus `nvidia-container-toolkit`, configures the `nvidia` containerd runtime through `nvidia-ctk`, labels the node for GPU scheduling, and applies the NVIDIA device-plugin DaemonSet plus `RuntimeClass`

Prepare and join a Terraform-managed Ubuntu worker after `bin/tf apply` has rebuilt or created the VM:

```bash
make ansible-kubernetes-worker LIMIT=helheim
```

This target runs:

```bash
ANSIBLE_FETCH_MINIO_SECRETS=0 ./scripts/ansible-run-playbook.sh \
  -i ansible/inventory/hosts.ini \
  ansible/playbooks/kubernetes-terraform-node.yml \
  --limit helheim
```

This worker playbook:

- reconnects to the Terraform-created VM over SSH
- ensures the Kubernetes prerequisites and services are in place
- fetches a fresh `kubeadm join` command from `yggdrasil`
- checks that the control plane has signed the fresh bootstrap token into `kube-public/cluster-info` before attempting `kubeadm join`
- joins the node only when `/etc/kubernetes/kubelet.conf` does not already exist
- reapplies the repo's per-node labels for all Terraform-managed workers through host vars
- can enable NVIDIA runtime support on selected Terraform-managed GPU workers such as `helheim`
- can pin an explicit NVIDIA driver family and matching kernel-module package prefix through host vars; `helheim` uses that path to install `nvidia-driver-580-open` plus both the running-kernel and `-generic` `linux-modules-nvidia-580-open` packages
- only keeps `node-llm=gpu` style labels when `nvidia-smi` is actually usable in the guest

If you omit `LIMIT`, the target runs against the full `kubernetes_terraform_node` inventory group so you can roll package or runtime changes across every repo-managed worker:

```bash
make ansible-kubernetes-worker
```

The Minecraft VM playbook expects these values to be available from `ansible/.env` or your shell environment:

- `MINECRAFT_CURSEFORGE_API_KEY` so `itzg/minecraft-server` can bootstrap the managed modpack profiles
- `MINECRAFT_RCON_PASSWORD` so Ansible can enable authenticated server-command execution through RCON

Managed Minecraft VM state:

- installs `docker.io`
- adds the `ubuntu` operator user to the `docker` group for direct container inspection over SSH
- creates per-profile data under `/srv/minecraft/servers/<profile>/data`
- creates per-profile backups under `/srv/minecraft/servers/<profile>/backups`
- makes `/srv/minecraft` writable by the `ubuntu` login user for direct SFTP uploads
- renders per-profile container env and runtime metadata under `/etc/minecraft/servers`
- keeps `/etc/minecraft/minecraft.env`, `/etc/minecraft/runtime.env`, `/srv/minecraft/data`, and `/srv/minecraft/backups` pointed at the active profile
- enforces selected `server.properties` values such as `enable-rcon=true`, `sync-chunk-writes=false`, and `spawn-protection=0` for the active `atm11` profile
- manages a `minecraft.service` systemd unit that runs the active profile's image
- keeps the shared image default at `itzg/minecraft-server:java21`, while `atm11` overrides to `itzg/minecraft-server:java25` because current NeoForge server builds require Java 25 there
- pins `atm11` to `NEOFORGE_VERSION=26.1.2.76`, mirrors that into `CF_MOD_LOADER_VERSION` for AUTO_CURSEFORGE installs, and starts it via the already-installed `/data/run.sh` script so the VM keeps using the repo-selected NeoForge build
- refreshes preinstalled AUTO_CURSEFORGE profile files when the generated `run.sh` or `.curseforge-manifest.json` drift away from the repo-pinned modpack or NeoForge version
- runs a loader-only NeoForge setup for profiles with `loader_setup_version` after the CurseForge refresh, which is needed when a server pack keeps its original loader installer despite `CF_MOD_LOADER_VERSION`
- manages `/data/user_jvm_args.txt` for preinstalled profiles from the repo `MEMORY` and `INIT_MEMORY` values, because the direct `/data/run.sh` path does not consume the container image's normal memory environment handling
- restores ownership of drifted preinstalled profile data before the one-shot refresh so the setup container can rewrite existing CurseForge override paths
- retries the one-shot CurseForge refresh to ride through transient ForgeCDN DNS or download failures
- supports per-profile `remove_mod_globs` before installing `extra_mods`; `atm11` currently uses `extra_mods` only for the server-side Connectivity login-timeout mitigation
- installs `minecraft-switch` on the VM so operators can swap profiles locally without editing repo vars
- exposes the game directly on TCP `25565`

Current seeded profiles:

- `atm11` is the repo-authoritative active profile and is pinned with `CF_SLUG=all-the-mods-11`, `CF_FILENAME_MATCHER=0.1.1`, `NEOFORGE_VERSION=26.1.2.76`, and `start_mode=preinstalled_run_script`
- `atm10_tts` preserves the older ATM10 To The Sky world with `CF_SLUG=all-the-mods-10-sky` and `CF_FILENAME_MATCHER=2.0.2`

The first multi-profile rollout migrates the old single-server `/srv/minecraft/data` and `/srv/minecraft/backups` directories into the `atm10_tts` profile before repointing the active links to `atm11`.

For world restores or other bulk file copies, prefer direct SFTP to the VM rather than port-forwarding through Kubernetes. The Minecraft VM has its own LAN IP and the playbook makes `/srv/minecraft` writable by `ubuntu`, so you can upload straight to:

- `/srv/minecraft/data` for the currently active server data
- `/srv/minecraft/backups` for the currently active server backups
- `/srv/minecraft/servers/<profile>/data` for a specific active or inactive profile
- `/srv/minecraft/servers/<profile>/backups` for a specific profile's staged restore archives

Example:

```bash
sftp ubuntu@nidavellir
put -r ./world /srv/minecraft/data/world
```

Switch profiles directly on the VM:

```bash
ssh nidavellir 'sudo minecraft-switch atm10_tts'
ssh nidavellir 'sudo minecraft-switch atm11'
```

The repo remains authoritative. Rerunning `make ansible-minecraft-vm` reapplies the configured `minecraft_vm_active_server`, which currently switches the active profile back to `atm11`.

After the playbook adds or refreshes Docker group membership for `ubuntu`, start a new SSH session before relying on plain `docker ps` or `docker logs`. Until then, use `sudo docker ...`.

For large world replacements, stop the service first so files are consistent:

```bash
ssh nidavellir 'sudo systemctl stop minecraft'
sftp ubuntu@nidavellir
ssh nidavellir 'sudo systemctl start minecraft'
```

To roll a managed modpack profile forward, stop the service, archive the profile data, copy that tarball to `svartalfheim`, then bump the profile's `CF_FILENAME_MATCHER` in `ansible/inventory/group_vars/minecraft_vm.yml` and rerun `make ansible-minecraft-vm`.

Example for `atm11` moving from `0.0.5` to `0.0.23`:

```bash
ssh nidavellir 'sudo systemctl stop minecraft'
ssh nidavellir '\
  ts=$(date -u +%Y%m%dT%H%M%SZ) && \
  mkdir -p /srv/minecraft/archives && \
  tar -C /srv/minecraft/servers/atm11 -czf /srv/minecraft/archives/atm11-${ts}.tar.gz data'
ssh svartalfheim 'mkdir -p /srv/minio/backups/minecraft'
ssh nidavellir '\
  latest=$(ls -t /srv/minecraft/archives/atm11-*.tar.gz | head -n1) && \
  scp "$latest" svartalfheim:/srv/minio/backups/minecraft/'
make ansible-minecraft-vm
ssh nidavellir 'systemctl status --no-pager minecraft'
ssh nidavellir 'docker exec minecraft rcon-cli list'
ssh nidavellir 'docker exec minecraft rcon-cli "say maintenance complete"'
```

Use `docker exec minecraft rcon-cli ...` for Minecraft server commands such as `list`, `say maintenance complete`, or `stop`. Commands do not need a leading slash.

The make targets call `scripts/ansible-run-playbook.sh`, which sources:

1. `.devcontainer/.env` for shared repo-level operator env
2. `scripts/ansible-fetch-secrets.sh` to refresh MinIO admin credentials from `svartalfheim` unless `ANSIBLE_FETCH_MINIO_SECRETS=0`
3. `ansible/.env` for Ansible-only local secret overrides or cached values

You can also run the wrapper directly for other playbooks:

```bash
./scripts/ansible-run-playbook.sh -i ansible/inventory/hosts.ini ansible/playbooks/minio-external-pi.yml
```

The external Pi API endpoint defaults to `http://svartalfheim:9000`. Override `minio_external_endpoint` when DNS or TLS is introduced.

The attached hard drive is currently mounted from NTFS partition UUID `C2629529629522E9` at `/srv/minio`. The Samba share intentionally exposes `/srv/minio` while hiding `/srv/minio/minio-data` from normal share access so object-storage internals do not show up in the file share.

## Expected managed state

Buckets:

- `velero`
- `cnpg-backups`
- `documents`
- `static-sites-site1`

Policies:

- `velero-rw`
- `cnpg-rw`
- `documents-rw`
- `static-site-site1-upload`

Users:

- `velero`
- `cnpg`
- `documents`
- `static-site-uploader-site1`

Lifecycle rules:

- backup buckets expire objects by configured retention days (`30`/`90` by default)
- static site lifecycle disabled by default

## Outputs

Generated artifacts are written to `ansible/out/`:

- policy JSON (`out/policies/`)
- lifecycle JSON (`out/lifecycle/`)
- user credential files (`out/credentials/`)
- Kubernetes Secret manifests (`out/k8s-secrets/`)

Apply generated Kubernetes secrets manually:

```bash
kubectl apply -f ansible/out/k8s-secrets/
```

## Key rotation

1. Rotate key material in secret manager / CI variables.
2. Re-run the relevant playbook.
3. Re-apply generated Kubernetes secrets.
4. Restart workloads if they do not hot-reload credentials.

## Replication / mirroring skeleton

`roles/minio_replication` provides a safe default skeleton using `mc mirror`.

- default: `enabled=false`
- optional dry run: `dry_run=true`
- source/target aliases support external-to-external mirroring scenarios when a second target is introduced

Enable via `group_vars` override when ready.
