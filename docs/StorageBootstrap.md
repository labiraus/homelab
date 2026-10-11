# Storage Bootstrap

The target is external SeaweedFS on `svartalfheim`. Read [the migration plan](SeaweedFSMigration.md) before applying host changes or publishing charts. Bootstrap provisions a new destination alongside the existing source; it does not migrate data or stop MinIO.

## SeaweedFS bootstrap

Put all ten nonempty credentials in local `ansible/.env` (do not commit them):

```text
SEAWEED_ADMIN_ACCESS_KEY=...
SEAWEED_ADMIN_SECRET_KEY=...
SEAWEED_DOCUMENTS_ACCESS_KEY=...
SEAWEED_DOCUMENTS_SECRET_KEY=...
SEAWEED_CNPG_ACCESS_KEY=...
SEAWEED_CNPG_SECRET_KEY=...
SEAWEED_VELERO_ACCESS_KEY=...
SEAWEED_VELERO_SECRET_KEY=...
SEAWEED_SITE1_ACCESS_KEY=...
SEAWEED_SITE1_SECRET_KEY=...
```

Use separate access keys and secrets for every identity. Check the mounted disk, native metadata capacity and Samba subtree access described in the migration plan, then run:

```bash
make ansible-seaweedfs-host
# Equivalent entry point:
make bootstrap-svartalfheim-storage
```

The playbook uses `ansible/inventory/hosts.ini`, installs the pinned/checksummed binary, creates `/etc/seaweedfs/s3.json` with mode 0600, installs the persistent filer configuration and mount-dependent systemd service, creates four buckets and enables `documents` versioning. It leaves lifecycle expiry disabled. It does not change Samba or the source MinIO service. SeaweedFS uses private port 8333; master, filer and volume listeners bind loopback. Administer internal services via SSH.

`seaweedfs-s3-proxy.socket` binds only the Pi's private address and forwards S3 HTTP to loopback using `systemd-socket-proxyd`. This keeps SeaweedFS's internal S3 gRPC port on loopback as well; binding the S3 server directly to the LAN would expose that port too.

New Kubernetes credentials are rendered to ignored `ansible/out/seaweedfs/k8s-secrets.yaml` with mode 0600. Apply that file only in the migration cutover sequence. The new credentials use different Secret names so source credentials remain available for rollback.

Reapply with the same local credential values; changing them rotates destination identities and requires coordinating the corresponding Kubernetes Secrets and consumers. The generic Ansible wrapper no longer fetches MinIO credentials by default. Do not use `make refresh-ansible-secrets` to populate SeaweedFS credentials.

Local checks from a Linux devcontainer:

```bash
# In a disposable test container, install the socket-proxy test utilities:
sudo apt-get update && sudo apt-get install -y systemd
ANSIBLE_ROLES_PATH="$PWD/ansible/roles" ansible-playbook \
  -i ansible/inventory/hosts.ini ansible/playbooks/seaweedfs-external-pi-host.yml --syntax-check
python3 -m venv /tmp/seaweedfs-test-venv
/tmp/seaweedfs-test-venv/bin/pip install boto3 PyYAML Jinja2
/tmp/seaweedfs-test-venv/bin/python scripts/seaweedfs-smoke.py
```

The smoke test requires the `systemd` package's `systemd-socket-activate` and `systemd-socket-proxyd` utilities. It starts its own `weed` process and S3 socket proxy on loopback with temporary data and synthetic credentials, tests bucket isolation and version history, and removes the temporary data on completion. Run it in a disposable container with no published ports. It does not verify the Pi's systemd service lifecycle, NTFS or backup restore behavior.

## Legacy MinIO and Samba bootstrap reference

The following describes the old bootstrap flow, retained for migration/rollback context. `make bootstrap-svartalfheim-storage` now invokes SeaweedFS; use the explicit `make ansible-minio-host` or `make ansible-minio-state` only when operating the legacy source, with a separately installed `mc` client.

`make ansible-minio-host` reapplies the legacy host playbook and explicitly refreshes existing MinIO root credentials. It also requires the existing Samba password in local `ansible/.env`. The old first-install auto-detection wrapper has been replaced; use the SeaweedFS bootstrap for new storage setup.

## Mount Drift Recovery

If CNPG WAL archiving starts failing with `SlowDownWrite`, check the `svartalfheim` host before changing MinIO users or bucket policy:

```bash
make minio-host-checks
```

Known bad pattern:

- `/dev/sda2` with UUID `C2629529629522E9` is still present
- `/srv/minio` reports `not-mounted`
- MinIO is still configured for `/srv/minio/minio-data`
- host logs show `Storage resources are insufficient for the write operation` or `no online disks found`

That means MinIO has fallen back to the root-filesystem directory instead of the external NTFS disk.

Recovery sequence:

```bash
ssh svartalfheim 'sudo mount /srv/minio'
ssh svartalfheim 'mount | grep "/srv/minio" && df -h /srv/minio'
ssh svartalfheim 'sudo systemctl restart minio'
make minio-host-checks
make document-platform-checks
```

If the mount does not come back cleanly, stop and inspect the host disk state before reapplying Ansible or rotating any MinIO credentials.
