# Homelab

This repository combines five related areas:

- Terraform for provisioning Kubernetes worker VMs and dedicated service VMs on Proxmox
- Helm charts for cluster bootstrap and in-cluster workloads
- Small application services used inside the cluster
- Ansible for external service management, including MinIO on `svartalfheim`
- Ansible for dedicated VM-hosted workloads such as Minecraft and post-provision Kubernetes worker bootstrap

## Repo Guidance For Humans And Agents

The repository includes repo-local guidance for both navigation and working conventions.

- `AGENTS.md`: root operating rules for work in this repo
- `.codex/REPO_MAP.md`: fast map of where code and workflows live
- `.codex/REPO_PLAN.md`: target state, current priorities, and known gaps

More specific `AGENTS.md` files exist under:

- `ansible/`
- `apps/`
- `components/`
- `helm/`
- `sql/`

Use the nearest relevant `AGENTS.md` for local conventions, and update it when a repeated lesson should persist for future work.

## Repository Layout

```text
.
├── .codex/                 # Codex reference docs: repo map and repo plan
├── .devcontainer/          # Preferred local operator environment
├── .github/workflows/      # CI for apps and Helm charts
├── ansible/                # External services and VM-hosted workloads
├── apps/
│   ├── external/           # Public Go HTTP service
│   ├── mcp/                # Public Go MCP service
│   ├── orchestrator/       # Internal Go document service
│   ├── processor/          # Internal TypeScript NATS JetStream worker
│   ├── ui/                 # Vite/React frontend
│   └── pkg/                # Shared Go modules used by the Go services
├── bin/
│   └── tf                  # Terraform wrapper script
├── components/
│   ├── kubernetes/         # Terraform component for Kubernetes worker VMs
│   └── minecraft-vm/       # Terraform component for the dedicated Minecraft VM
├── docs/                   # Setup and notes
├── evals/                  # RAGAS and other evaluation harnesses
├── etc/
│   ├── env/                # Environment tfvars
│   └── nodes/              # Per-node tfvars
├── helm/
│   ├── apps/               # App charts
│   ├── bootstrap/          # Flux/bootstrap charts
│   ├── data/               # Stateful platform charts
│   ├── infra/              # Infrastructure charts
│   ├── libraries/          # Shared chart templates
│   ├── observability/      # Monitoring and tracing charts
│   └── workloads/          # Misc workloads outside the core platform
├── modules/
│   └── proxmox-ubuntu-vm/  # Reusable Terraform module
├── scripts/                # Helper scripts
├── sql/                    # SQL bootstrap/auth snippets
└── values/                 # Values files for upstream charts
```

## Main Areas

### Terraform and Proxmox

Terraform provisions Kubernetes worker VMs and dedicated service VMs on Proxmox VE using layered `tfvars` and Terraform Cloud for state.

Key files:

- `bin/tf`
- `components/kubernetes/`
- `components/minecraft-vm/`
- `modules/proxmox-ubuntu-vm/`
- `etc/env/lab.tfvars`
- `etc/nodes/node*.tfvars`

Important Proxmox token note:

- both the Kubernetes worker flow and the dedicated Minecraft VM flow upload a cloud-init snippet with `proxmox_virtual_environment_file`
- the node tfvars currently target `snippets_datastore_id = "local"`
- the Proxmox API token therefore needs datastore permissions on `/storage/local`, including at least `Datastore.Audit` and `Datastore.AllocateSpace`
- if those permissions are missing, `bin/tf plan kubernetes <node>` can fail before planning completes with a `403` while listing files on the datastore
- the worker VM boot disk is imported from an Ubuntu cloud image stored on Proxmox as `content_type = "import"` rather than `iso`

### Kubernetes and Helm

Helm charts in `helm/` define cluster bootstrap, app deployment, data services, observability components, and miscellaneous in-cluster workloads. The current GitOps flow is Flux-based: charts are packaged as OCI artifacts by GitHub Actions and reconciled in-cluster by Flux from `flux-system`.

Core bootstrap covers cluster-fabric components such as Istio and Gateway API support. Utility controllers such as Envoy AI Gateway live under `helm/infra/` and are reconciled through the regular Flux application layer.

Minecraft is no longer deployed through Helm/Flux. The authoritative Minecraft path is a dedicated Proxmox VM managed through Terraform and Ansible.

### Application Code

The app stack under `apps/` is intentionally small:

- `apps/ui`: public React frontend
- `apps/external`: public Go API for the UI and stable browser-facing integrations
- `apps/mcp`: public Go MCP service for AI-native access
- `apps/orchestrator`: internal Go control-plane service for reconciliation and task dispatch
- `apps/processor`: internal TypeScript NATS JetStream worker for extraction, chunking, embedding, and persistence
- `apps/pkg/*`: shared Go packages for HTTP server startup, logging, metrics, and integrations
- `evals/ragas`: RAGAS-based retrieval/chunking quality checks against live processed chunks

Current document-platform direction:

- MinIO on `svartalfheim` is the canonical raw object store
- Postgres via CNPG plus pgvector is the source of truth for metadata, state, chunks, and embeddings
- document lifecycle history is also persisted in Postgres for processing and reprocessing audits
- browser and MCP surfaces expose retrieval, cited context, lifecycle history, metadata curation, guarded text edits, and reprocess requests while `orchestrator` keeps workflow ownership
- RAGAS chunking evaluation can score current processed chunks against gold retrieval cases under `evals/ragas/`
- NATS JetStream plus KEDA handle asynchronous execution and worker scaling
- Redis is available but not yet a core design dependency
- Mongo is intentionally not part of the active application architecture

### Ansible

`ansible/` manages `svartalfheim` host services such as the MinIO server, MinIO state, and the attached-drive Samba share, plus the dedicated Minecraft VM on `nidavellir` and post-provision bootstrap for repo-managed Kubernetes worker guests. Kubernetes remains Helm-managed for in-cluster services; external service state is handled separately.

## Devcontainer

The devcontainer is the preferred local working environment.

Secret handling is intentionally split into three local-only classes:

- per-machine SSH identities and SSH certificates under `.devcontainer/ssh/`
- generated local files or env entries rebuilt from trusted cluster sources, such as `.devcontainer/kubeconfig` or `make refresh-postgres-env`
- a minimal `.devcontainer/.env` for true external-only secrets and operator settings that cannot be derived from the cluster

For Ansible-specific external secrets, prefer `ansible/.env` over loading everything into the shared shell environment.

Mounted local-only files:

- `.devcontainer/.bashrc` -> `/home/vscode/.bashrc.devcontainer`
- `.devcontainer/.env` -> `/home/vscode/.env`
- `.devcontainer/ssh/` -> `/home/vscode/.ssh`
- `.devcontainer/kubeconfig` -> `/home/vscode/.kube/config`
- `plugin-cache/` -> `/home/vscode/terraform-plugin-cache`

Included tooling:

- Docker outside Docker
- GitHub CLI (`gh`)
- Terraform
- Ansible Core 2.15.x
- MinIO client (`mc`)
- Flux
- kubectl and Helm
- k9s
- cilium CLI
- skaffold
- Node.js 24

Bootstrap new machines from the example files:

- copy `.devcontainer/.env.example` to `.devcontainer/.env`
- mint or copy the local SSH keypair and CA-signed cert into `.devcontainer/ssh/`
- run `make refresh-kubeconfig` and any other `refresh-*` targets needed for cluster-derived credentials
- run `make refresh-ansible-secrets` if you need the external-host playbooks and want to sync MinIO admin credentials from `svartalfheim`

Customize the container by editing local-only files such as `.devcontainer/.env`, `.devcontainer/.bashrc`, `.devcontainer/hosts`, and `.devcontainer/ssh/config`, then rebuild or reopen the devcontainer if needed.

On container start, `.devcontainer/scripts/sync-hosts.sh` merges the lab host mappings from `.devcontainer/hosts` into the container `/etc/hosts` and appends the current container hostname. This keeps the tracked repo file stable while keeping the desktop-lite VNC session healthy.

## Terraform Workflow

`bin/tf` creates a temporary `cloud.auto.tf` and targets workspaces named:

`<ENV>-<COMPONENT>-<PRIMARY_LAYER>`

Example workspace names:

- `lab-kubernetes-node1`
- `lab-kubernetes-node2`

Command format:

```bash
bin/tf <action> <component> <primary_layer> [overlay_layer ...] [-- <extra terraform args>]
```

- `action`: `plan`, `apply`, `destroy`
- `component`: `kubernetes` or `minecraft-vm`
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

## Kubernetes Worker Bootstrap

Terraform cloud-init on worker VMs installs and configures:

- `containerd` with `SystemdCgroup=true`
- Kubernetes packages from `pkgs.k8s.io`
- swap disable
- `qemu-guest-agent`
- node-local `KUBELET_EXTRA_ARGS` such as labels and taints

Cluster join is handled afterward through Ansible, not through Terraform state or cloud-init.

## CI and Build Workflows

GitHub Actions currently handle:

- `apps/external` tests and image build
- `apps/mcp` tests and image build
- `apps/orchestrator` tests and image build
- `apps/processor` tests and image build
- `apps/ui` tests and image build
- Helm chart discovery, templating tests, and GHCR packaging

Important note: some workflow files may lag behind the runtime choices in the repo. For example, the React app is Vite-based and the devcontainer uses Node 24, so treat workflow definitions as something to verify rather than assume are fully current.

## Ansible Workflow

Run playbooks from `ansible/` to manage `svartalfheim` host services and MinIO state:

```bash
make ansible-minio-host
make ansible-minio-state
```

Run post-provision worker bootstrap for a Terraform-managed Kubernetes node:

```bash
make ansible-kubernetes-worker LIMIT=helheim
```

These targets use `scripts/ansible-run-playbook.sh`, which loads shared operator env from `.devcontainer/.env` and then applies Ansible-only overrides from `ansible/.env`.
Before loading `ansible/.env`, the wrapper refreshes the tracked MinIO admin credentials from `svartalfheim` over SSH by default.

Generated Kubernetes Secret manifests are written under `ansible/out/` and can be applied manually.

The authoritative MinIO endpoint is the external Raspberry Pi `svartalfheim` on `http://svartalfheim:9000`. Its attached NTFS drive is mounted at `/srv/minio` and is also exported as the `storage` Samba share. Bucket, user, policy, and host-share changes should go through the Ansible playbooks rather than cluster manifests.

## Setup Notes

See [docs/Setup.md](/workspaces/homelab/docs/Setup.md) for Kubernetes setup, workstation bootstrap, SSH certificate setup, kubeconfig bootstrapping, and Proxmox worker host notes. Related workflows now live in [docs/NodeClassification.md](/workspaces/homelab/docs/NodeClassification.md), [docs/MinecraftVM.md](/workspaces/homelab/docs/MinecraftVM.md), [docs/StorageBootstrap.md](/workspaces/homelab/docs/StorageBootstrap.md), and [docs/Secrets.md](/workspaces/homelab/docs/Secrets.md). For day-two worker changes and recovery after Terraform-driven VM recreation, use [docs/WorkerRedeploy.md](/workspaces/homelab/docs/WorkerRedeploy.md).

## App Authentication

The public UI and `external` API now use an email-based identity model with three runtime auth states:

- certificate auth from Istio-forwarded client certificate identity
- OIDC auth from a trusted upstream email header after login
- no auth when neither identity source is present

The only seeded valid user is `oliver@labiraus.com`, and the UI shows the resolved auth mode, email, validity, and invalid reason from `/api/auth/status`.

For architecture, client-certificate generation, Istio CA trust, and the Google login redirect flow, see [docs/Auth.md](/workspaces/homelab/docs/Auth.md).

## Security Notes

- Do not commit real secrets
- Keep cluster credentials and kubeconfig material out of git
- Prefer script-generated local secret material when the cluster is already the authoritative source
- Keep `.devcontainer/.env` minimal and reserve it for external-only secrets plus non-secret operator settings
- Keep Ansible-only external secrets in `ansible/.env` instead of the shared shell env when they are not needed by other tooling
- Prefer `make refresh-ansible-secrets` for host-sourced MinIO admin credentials instead of copying them by hand
- avoid reintroducing `kubeadm join` commands or bootstrap tokens into Terraform-managed cloud-init
- Prefer local secret overrides or external secret stores for sensitive data

## Documentation Maintenance

When the repo structure, workflows, or conventions change, keep these docs aligned in the same task:

- `README.md`
- the nearest relevant `AGENTS.md`
- `.codex/REPO_MAP.md`
- `.codex/REPO_PLAN.md`
