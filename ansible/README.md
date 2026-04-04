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

The API-management groups default to `localhost` because those playbooks run from your laptop/CI/management node and call MinIO APIs.

The external host bootstrap group connects to the Pi over SSH and installs the MinIO server binary plus a `systemd` service.
The Minecraft VM group connects over SSH and configures the in-guest Minecraft service and systemd unit.

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

Apply MinIO buckets, policies, users, and lifecycle rules against `svartalfheim`:

```bash
make ansible-minio-state
```

Configure the dedicated Minecraft VM:

```bash
make ansible-minecraft-vm
```

This target intentionally disables the MinIO secret-refresh helper because the Minecraft VM playbook does not require the external MinIO admin credentials.

The Minecraft VM playbook expects `MINECRAFT_CURSEFORGE_API_KEY` to be available from `ansible/.env` or your shell environment so `itzg/minecraft-server` can bootstrap the ATM10 Sky pack.

Managed Minecraft VM state:

- installs `docker.io`
- creates `/srv/minecraft/data` and `/srv/minecraft/backups`
- renders `/etc/minecraft/minecraft.env`
- manages a `minecraft.service` systemd unit that runs `itzg/minecraft-server:java21`
- exposes the game directly on TCP `25565`

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
- `static-site-site1-upload`

Users:

- `velero`
- `cnpg`
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
