# Development

Use the feature-enabled devcontainer as the supported local tool environment.
[Secrets](secrets.md) covers ignored credentials; [infrastructure](infrastructure.md)
covers host provisioning and [testing](testing.md) covers acceptance fixtures.

## Local setup

Install a container runtime, copy `.devcontainer/.env.example` to the ignored
`.devcontainer/.env`, populate only required local settings, and rebuild the
Dev Container with its configured features. Keep SSH identities under
`.devcontainer/ssh/` and kubeconfig at `.devcontainer/kubeconfig`.

```bash
cp .devcontainer/.env.example .devcontainer/.env
```

Do not overwrite an existing local environment file. `make refresh-kubeconfig`
retrieves the control-plane config over SSH; `make refresh-ssh` renews the local
certificate; `make refresh-postgres-env` refreshes local PostgreSQL settings.
Use `make postgres` for an interactive localhost-forwarded `psql` session.
Never reuse that production connection for disposable tests.

## SSH workstation identity

Create a passphrase-protected user CA only for a new trust deployment. For an
existing estate, use its existing CA or the staged [recovery procedure](recovery.md#ssh-trust-recovery).
Keep the CA key and passphrase private and backed up independently.

```bash
ssh-keygen -t ed25519 -f ~/.ssh/ssh_user_ca -C homelab-user-ca
ssh-keygen -t ed25519 -f .devcontainer/ssh/id_ed25519 -C homelab-login
make refresh-ssh
ssh-add .devcontainer/ssh/id_ed25519
```

The certificate accompanies the private login key automatically. Defaults are
identity `oliver@homelab`, principals `oliver,ubuntu,root`, and validity `+30d`.
Override `SSH_CERT_ID`, `SSH_CERT_PRINCIPALS`, `SSH_CA_KEY`, `SSH_KEY_PUB` and
`SSH_CERT_VALIDITY` as appropriate. Inspect validity with
`ssh-keygen -L -f .devcontainer/ssh/id_ed25519-cert.pub` before diagnosing a
public-key failure as a host outage. Public user-CA trust is separate from SSH
host-key identity and Kubernetes PKI.

## Terraform Workflow

`bin/tf` creates a temporary `cloud.auto.tf` and targets workspaces named:

`<ENV>-<COMPONENT>-<PRIMARY_LAYER>`

Example workspace names:

- `lab-kubernetes-node1`
- `lab-kubernetes-node2`

Command format:

```bash
bin/tf <action> <component> <primary_layer> [overlay_layer ...] [--no-refresh] [-- <extra terraform args>]
```

- `action`: `plan`, `apply`, `destroy`, `unlock`, `import`, `state-rm`, `state-list`
- `component`: an existing directory under `components/`, including `kubernetes`, `minecraft-vm` or `inference-bedrock`
- `primary_layer`: first node layer such as `node1`
- overlay layers: loaded from `etc/<overlay>.tfvars` or `etc/overlays/<overlay>.tfvars`

The wrapper script:

1. Resolves the repo root
2. Loads environment and node tfvars in order
3. Verifies Terraform Cloud credentials
4. Sets `TF_IN_AUTOMATION=1`
5. Uses `plugin-cache/` for providers
6. Uses `.tfdata/<ENV>/<component>/<workspace>/` for `TF_DATA_DIR`
7. Generates `components/<component>/cloud.auto.tf`
8. Runs `terraform init -upgrade`
9. Runs `terraform <action> -input=false -lock-timeout=5m`
10. Passes through any extra Terraform args only when provided after `--`
11. Removes generated `cloud.auto.tf` on exit

The wrapper auto-approves `apply` and `destroy` by default. `plan` does not get `-auto-approve`.

If you need to pass additional Terraform flags, append them after `--`:

```bash
ENV=lab bin/tf apply kubernetes node1 -- -refresh=false
ENV=lab bin/tf plan minecraft-vm minecraft-node1
ENV=lab bin/tf destroy kubernetes node1 -- -target=module.kubernetes_vm
```

Makefile shortcuts:

```bash
make plan COMPONENT=kubernetes LAYER=node1
make apply COMPONENT=kubernetes LAYER=node1
make destroy COMPONENT=kubernetes LAYER=node1
make plan COMPONENT=minecraft-vm LAYER=minecraft-node1
```

Useful helper targets:

- `make refresh-kubeconfig`
- `make refresh-postgres-env`
- `make refresh-ansible-secrets`
- `make bootstrap-svartalfheim-storage`
- `make ansible-minecraft-vm`
- `make ansible-kubernetes-worker LIMIT=helheim`
- `make postgres`

`make refresh-postgres-env` refreshes the local `.devcontainer/.env` entries used for operator access to CNPG Postgres. `make postgres` starts a temporary `kubectl port-forward` to `svc/app-db-rw` in the `data` namespace and opens `psql` through that local tunnel instead of relying on a permanently exposed cluster port.

## CI and Build Workflows

GitHub Actions currently handle:

- `apps/external` tests and image build
- `apps/mcp` tests and image build
- `apps/orchestrator` tests and image build
- `apps/processor` tests and image build
- `apps/ui` tests and image build
- `apps/inference` tests and image build
- `helm/infra/vllm` chart packaging for the pinned upstream vLLM runtime image
- Helm chart discovery, templating tests, and GHCR packaging

GHCR publication uses `scripts/build/merge-helm-values.sh` so empty overlays retain chart defaults. `bash scripts/build/test-helm-values.sh` checks merge/refusal cases and compares a packaged Prometheus chart with its source render (requires Helm and Mike Farah's yq v4).

After the vLLM and Envoy AI Gateway releases deploy, use `make vllm-gateway-smoke` to validate the live OpenAI-compatible route used by processor-owned assistant workflows.

The separate `helm/infra/embeddings` chart supplies CPU embeddings through the same private gateway. [Document search recovery](documents.md) distinguishes runtime/model/ingest verification from healthy Pods and pipeline creation.

Use the checked-in workflow definitions and lockfiles as the source of toolchain and gate configuration; a historical runtime note is not an installed-version observation.

## Ansible Workflow

Provision the new SeaweedFS service and buckets with `make ansible-seaweedfs-host` (also `make bootstrap-svartalfheim-storage`); see [storage bootstrap](storage.md) for credentials and [migration](storage.md) for cutover.

The following MinIO playbooks remain legacy migration/rollback tooling and require a separately supplied `mc` client:

```bash
make ansible-minio-host
make ansible-minio-state
```

Run post-provision worker bootstrap for a Terraform-managed Kubernetes node:

```bash
make ansible-kubernetes-worker LIMIT=helheim
```

These targets use `scripts/ansible-run-playbook.sh`, which loads `.devcontainer/.env` and then local `ansible/.env`. Only the explicit legacy MinIO targets refresh MinIO admin credentials from `svartalfheim` over SSH.

Generated Kubernetes Secret manifests are written under ignored `ansible/out/`; install them only at the reviewed activation/cutover stage.

Published private endpoints use wired `http://192.168.8.177:8333` for the SeaweedFS target and port 9000 for the MinIO source; host DNS can resolve an egress-disallowed Wi-Fi address. The attached NTFS drive remains mounted at `/srv/minio`. SeaweedFS uses its own volume directory and native-filesystem metadata; the existing Samba share and MinIO source data require the migration precautions documented in [storage](storage.md).

## Application development

There is no root `package.json`, `go.mod` or `go.work`. Run Go tests in the
individual module directory and npm commands in `apps/ui` or `apps/processor`.
Use the committed lockfiles with `npm ci`. Follow each app's README for its
required environment, health endpoints and build commands.

UI browser fixtures consume the completed production build. Finish `npm run build`
before starting them; never rebuild `dist` while a harness is serving it.
Workspace/PWA fixtures and disposable real API/database journeys are different
acceptance tiers; see [testing](testing.md).

## Tooling and documentation discipline

Check configured devcontainer features before adding a binary installer: the
Terraform feature already installs TFLint. Direct downloads must use pinned official
release assets and architecture-specific checksums. Verify changes with a complete
feature-enabled Dev Containers build, not only a Dockerfile build.

Discover paths with `rg --files`. Read current anchors before multi-file edits;
wait for any background generator/formatter before editing its output. Avoid
placeholder patches and duplicate headings. Normalize only touched CRLF lines.
For scripts invoked by CI, check Git's recorded mode with `git ls-files --stage`;
local executable permissions on Windows-backed workspaces are insufficient. Use
an explicit interpreter in CI as well.

Documentation belongs in the existing canonical topic, with current guidance
replaced in place. Keep dated publication/deployment evidence in [status](status.md)
and recurring operator symptoms in [troubleshooting](../runbooks/troubleshooting.md).
