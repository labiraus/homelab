# Storage Bootstrap

This guide covers first install and reapply workflows for the external MinIO and Samba host on `svartalfheim`.

## MinIO and Samba bootstrap

Use the storage bootstrap script when bringing up `svartalfheim` for the first time or when reapplying the host service configuration:

```bash
make bootstrap-svartalfheim-storage
```

That wrapper:

- requires `ansible/.env` to contain `MINIO_EXTERNAL_ADMIN_ACCESS_KEY`, `MINIO_EXTERNAL_ADMIN_SECRET_KEY`, and `SVARTALFHEIM_SAMBA_PASSWORD`
- checks `svartalfheim` for `/etc/default/minio`
- skips the MinIO credential fetch on first install when MinIO is not present yet
- re-enables the fetch automatically on later runs once MinIO is installed
- runs the Ansible host playbook that installs MinIO and Samba

First-install flow:

1. Create `ansible/.env` with the three bootstrap values.
2. Run `make bootstrap-svartalfheim-storage`.
3. After the install succeeds, `make refresh-ansible-secrets` can pull the MinIO admin credentials back from `svartalfheim`.

Reapply flow after MinIO already exists:

1. Keep `ansible/.env` with the Samba password and any other local-only values.
2. Run `make bootstrap-svartalfheim-storage`.
3. The script will detect `/etc/default/minio` and refresh the MinIO admin credentials automatically before the playbook runs.

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
