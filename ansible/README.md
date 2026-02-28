# Ansible MinIO State Management

This directory manages MinIO/S3 service state out-of-band from Kubernetes:

- buckets
- users/access keys
- policies
- lifecycle rules
- optional replication/mirroring skeleton

Kubernetes remains Helm/Argo-managed. MinIO state is Ansible-managed.

## Prerequisites

- `ansible-core` 2.15+
- MinIO client `mc` on the machine running playbooks
- Optional: `amazon.aws` collection for future S3 module adoption

Install collection (optional):

```bash
ansible-galaxy collection install amazon.aws
```

## Inventory and targets

- In-cluster target group: `minio_incluster`
- External Pi target group: `minio_external_pi`

Both groups default to `localhost` because playbooks run from your laptop/CI/management node and call MinIO APIs.

## Secure credentials

Set admin credentials for target endpoint:

```bash
export MINIO_INCLUSTER_ADMIN_ACCESS_KEY='...'
export MINIO_INCLUSTER_ADMIN_SECRET_KEY='...'

export MINIO_EXTERNAL_ADMIN_ACCESS_KEY='...'
export MINIO_EXTERNAL_ADMIN_SECRET_KEY='...'
```

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

Use Ansible Vault or CI secret store for production automation.

## Run playbooks

In-cluster MinIO:

```bash
cd ansible
ansible-playbook -i inventory/hosts.ini playbooks/minio-incluster.yml
```

External Pi MinIO placeholder:

```bash
cd ansible
ansible-playbook -i inventory/hosts.ini playbooks/minio-external-pi.yml
```

## Expected managed state

Buckets:

- `velero`
- `cnpg-backups`
- `minecraft-backups`
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
- source/target aliases support in-cluster -> external PI scenarios

Enable via `group_vars` override when ready.
