# Secrets

Keep credential values outside tracked source, logs and shared reports. This guide
owns local custody and cluster credential names; topic guides own activation order.

This guide covers local generated secrets and the separate Ansible-only secret flow.

## Secret model

Use three local-only secret classes when onboarding a new machine:

1. SSH keypairs and CA-signed SSH certificates in `.devcontainer/ssh/`
2. Generated local files or env entries rebuilt from trusted cluster sources with repo scripts and `make refresh-*` targets
3. A minimal `.devcontainer/.env` for external-only secrets and operator settings that cannot be derived from the cluster

Keep `.devcontainer/.env` intentionally small. If the cluster can tell you a value, prefer a script that repopulates it locally instead of hand-managing it.

For auth providers that require operator-supplied credentials and cannot be derived from the cluster, prefer creating a Kubernetes Secret manually and referencing that existing Secret from Helm rather than committing credential material into chart values.

Cluster credential names are listed below.

## Generated local secrets

When a secret or credential is already exposed by the cluster or control plane, prefer rebuilding local state from the source of truth instead of storing it permanently in `.devcontainer/.env`.

Current repo-native flows:

- `make refresh-kubeconfig`: rebuilds `.devcontainer/kubeconfig` from the control plane over SSH
- `make refresh-ssh`: re-signs the local SSH login key in `.devcontainer/ssh/` with the workstation SSH CA
- `make refresh-postgres-env`: updates `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, and `DB_PASS` in `.devcontainer/.env` for the local Postgres port-forward workflow

Operator access to Postgres is intentionally local-only now. Use `make postgres` to start a temporary `kubectl port-forward` to the CNPG read-write service and open `psql` through `127.0.0.1:15432` instead of relying on a permanently exposed in-cluster TCP listener.

Treat those generated entries as cacheable local state, not hand-maintained secrets.

The SSH CA private key is different: it cannot be regenerated from a host's public trust. Keep its passphrase in a password manager and back up the encrypted key securely. During the 2026-10-11 rotation, the replacement encrypted key was installed in `.devcontainer/ssh/ssh_user_ca`; the generated passphrase and rollback copies were retained under ignored `.local/ssh-ca-recovery-20261011/` (directory mode `0700`, secret files `0600`). The passphrase file is `ca-passphrase`. Move that passphrase into the password manager and verify the key backup before removing recovery copies. Never paste their contents into logs, chat, or Git. The local recovery directory is temporary custody, not an independent backup.

## Ansible secret handling

Use `ansible/.env` for external-host secrets that Ansible needs but the rest of the repo does not.

- `.devcontainer/.env`: shared shell env, minimal, loaded automatically by the devcontainer shell
- `ansible/.env`: Ansible-only local cache and overrides, loaded only by `scripts/ansible-run-playbook.sh`

For the SeaweedFS target, supply per-workload `SEAWEED_*` credentials in local `ansible/.env` as described in [Storage Bootstrap](storage.md). The Ansible wrapper defaults to no MinIO credential fetch. Generated destination Kubernetes Secrets stay under ignored `ansible/out/seaweedfs/`; apply only during the [migration cutover](storage.md).

For the legacy source only, sync the MinIO admin credentials from `svartalfheim`:

```bash
make refresh-ansible-secrets
```

That script SSHes to `svartalfheim`, reads `/etc/default/minio`, and upserts:

- `MINIO_EXTERNAL_ADMIN_ACCESS_KEY`
- `MINIO_EXTERNAL_ADMIN_SECRET_KEY`

into `ansible/.env`.

It preserves any other existing entries in `ansible/.env`.

Legacy source limitation:

- the Samba password and managed MinIO user secrets are not recoverable from `/etc/default/minio`
- keep those as local-only values in `ansible/.env` until they also get a fetch path from their own source of truth

That includes app-facing MinIO users such as the `documents` bucket credentials used by the `external` browser API and the Labiraus MCP server.

Run the external playbooks through the repo wrapper so both env layers are applied consistently:

```bash
make ansible-minio-host
make ansible-minio-state
```

## Cluster credential inventory

Use existing operator-managed Secrets rather than embedding values in chart inputs.
Generated manifests remain ignored, with private file permissions. Discover existing
identities before provisioning; a missing local cache is not a reason to replace
working deployed credentials.

| Namespace / Secret | Purpose / required keys |
| --- | --- |
| `flux-system/ghcr-creds` | `kubernetes.io/dockerconfigjson`; private OCI chart pulls |
| `homelab/oauth2-proxy-google` | `OAUTH2_PROXY_CLIENT_ID`, `OAUTH2_PROXY_CLIENT_SECRET`, `OAUTH2_PROXY_COOKIE_SECRET` |
| `ingress/homelab-client-ca` | Public `ca.crt` for optional client certificate validation |
| `homelab/documents-minio-credentials` | Legacy source `accessKey`, `secretKey` |
| `data/cnpg-backup-s3` | Legacy CNPG source storage identity |
| `velero/velero-minio-credentials` | Legacy Velero source identity |
| `homelab/documents-s3-credentials` | Target SeaweedFS documents identity |
| `data/cnpg-seaweedfs-s3` | Target CNPG identity |
| `velero/velero-s3-credentials` | Target Velero identity |
| `homelab/worldbuilder-external` | `WORLDBUILDER_DATABASE_URL`, dedicated non-owner external login |
| `homelab/worldbuilder-relay` | `WORLDBUILDER_RELAY_DATABASE_URL`, dedicated relay login |
| `homelab/worldbuilder-ai` | `WORLDBUILDER_AI_TOKEN`, shared only by external and processor |
| `homelab/worldbuilder-mcp` | `WORLDBUILDER_MCP_KEY`, shared only by external and MCP |

Other credential names are selected by chart configuration: native OAuth's CNPG
password Secret and external `MCP_OAUTH_DATABASE_URL`, durable-job token Secrets,
and the independent inference owner/runtime and client Secrets. Their roles and
activation ordering are specified in [authentication](authentication.md),
[operations](operations.md) and [inference](inference.md). Do not infer actual live
Secret names from examples.

SeaweedFS generation writes `ansible/out/seaweedfs/k8s-secrets.yaml`; legacy MinIO
state writes `ansible/out/k8s-secrets/`. These are outputs, not automatic application
steps. Install target credentials only in the reviewed cutover sequence. Preserve
source credentials for rollback. Never expose either S3 store directly to browsers.

## Rotation

Coordinate external credentials and their Kubernetes consumers. GHCR rotation
affects OCI pulls; Google client/cookie changes can invalidate sessions; client-CA
changes alter accepted certificates. Rotate native OAuth and inference credentials
under their specific protocols. Secret changes do not necessarily reload processes;
verify the relevant rollout and authenticated behavior. Preserve private rollback
material until replacement access is verified.
