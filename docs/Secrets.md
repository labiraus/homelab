# Secrets

This guide covers local generated secrets and the separate Ansible-only secret flow.

## Secret model

Use three local-only secret classes when onboarding a new machine:

1. SSH keypairs and CA-signed SSH certificates in `.devcontainer/ssh/`
2. Generated local files or env entries rebuilt from trusted cluster sources with repo scripts and `make refresh-*` targets
3. A minimal `.devcontainer/.env` for external-only secrets and operator settings that cannot be derived from the cluster

Keep `.devcontainer/.env` intentionally small. If the cluster can tell you a value, prefer a script that repopulates it locally instead of hand-managing it.

For auth providers that require operator-supplied credentials and cannot be derived from the cluster, prefer creating a Kubernetes Secret manually and referencing that existing Secret from Helm rather than committing credential material into chart values.

## Generated local secrets

When a secret or credential is already exposed by the cluster or control plane, prefer rebuilding local state from the source of truth instead of storing it permanently in `.devcontainer/.env`.

Current repo-native flows:

- `make refresh-kubeconfig`: rebuilds `.devcontainer/kubeconfig` from the control plane over SSH
- `make refresh-postgres-env`: updates `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, and `DB_PASS` in `.devcontainer/.env` for the local Postgres port-forward workflow

Operator access to Postgres is intentionally local-only now. Use `make postgres` to start a temporary `kubectl port-forward` to the CNPG read-write service and open `psql` through `127.0.0.1:15432` instead of relying on a permanently exposed in-cluster TCP listener.

Treat those generated entries as cacheable local state, not hand-maintained secrets.

## Ansible secret handling

Use `ansible/.env` for external-host secrets that Ansible needs but the rest of the repo does not.

- `.devcontainer/.env`: shared shell env, minimal, loaded automatically by the devcontainer shell
- `ansible/.env`: Ansible-only local cache and overrides, loaded only by `scripts/ansible-run-playbook.sh`

Sync the MinIO admin credentials from `svartalfheim`:

```bash
make refresh-ansible-secrets
```

That script SSHes to `svartalfheim`, reads `/etc/default/minio`, and upserts:

- `MINIO_EXTERNAL_ADMIN_ACCESS_KEY`
- `MINIO_EXTERNAL_ADMIN_SECRET_KEY`

into `ansible/.env`.

It preserves any other existing entries in `ansible/.env`.

Current limitation:

- the Samba password and managed MinIO user secrets are not recoverable from `/etc/default/minio`
- keep those as local-only values in `ansible/.env` until they also get a fetch path from their own source of truth

Run the external playbooks through the repo wrapper so both env layers are applied consistently:

```bash
make ansible-minio-host
make ansible-minio-state
```
