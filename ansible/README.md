# Ansible External Storage Host Management

This directory manages external storage-host services out-of-band from Kubernetes:

- MinIO server bootstrap on `svartalfheim`
- buckets
- users/access keys
- policies
- lifecycle rules
- optional replication/mirroring skeleton
- Samba network sharing for the attached `svartalfheim` hard drive

Kubernetes remains Helm/Flux-managed. Storage-host services are Ansible-managed.
The authoritative MinIO service and attached-drive network share for this repo both live on the external Raspberry Pi host `svartalfheim`.

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

The API-management groups default to `localhost` because those playbooks run from your laptop/CI/management node and call MinIO APIs.

The external host bootstrap group connects to the Pi over SSH and installs the MinIO server binary plus a `systemd` service.

## Secure credentials

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
export MINIO_MINECRAFT_ACCESS_KEY='...'
export MINIO_MINECRAFT_SECRET_KEY='...'
export MINIO_SITE1_UPLOADER_ACCESS_KEY='...'
export MINIO_SITE1_UPLOADER_SECRET_KEY='...'
```

Set the Samba password for the `oliver` network-share account:

```bash
export SVARTALFHEIM_SAMBA_PASSWORD='...'
```

Use Ansible Vault or CI secret store for production automation.

## Run playbooks

Bootstrap the external Raspberry Pi host services:

```bash
cd ansible
ansible-playbook -i inventory/hosts.ini playbooks/minio-external-pi-host.yml
```

This installs the MinIO binary from `https://dl.min.io/server/minio/release/linux-arm64/minio`, creates a `systemd` service, stores objects under `/srv/minio/minio-data` by default, installs Samba, and exposes the attached drive as the `storage` share backed by `/srv/minio`.

On `svartalfheim`, both MinIO and the Samba share currently run through user `oliver` because the attached NTFS disk is mounted with ownership mapped to that account. If the disk is later migrated to a Linux-native filesystem, consider switching MinIO and file-sharing ownership to dedicated service users.

Apply MinIO buckets, policies, users, and lifecycle rules against `svartalfheim`:

```bash
cd ansible
ansible-playbook -i inventory/hosts.ini playbooks/minio-external-pi.yml
```

The external Pi API endpoint defaults to `http://svartalfheim:9000`. Override `minio_external_endpoint` when DNS or TLS is introduced.

The attached hard drive is currently mounted from NTFS partition UUID `C2629529629522E9` at `/srv/minio`. The Samba share intentionally exposes `/srv/minio` while hiding `/srv/minio/minio-data` from normal share access so object-storage internals do not show up in the file share.

## Expected managed state

Buckets:

- `velero`
- `cnpg-backups`
- `minecraft-backups`
- `documents`
- `static-sites-site1`

Policies:

- `velero-rw`
- `cnpg-rw`
- `minecraft-rw`
- `static-site-site1-upload`

Users:

- `velero`
- `cnpg`
- `minecraft`
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
