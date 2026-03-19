# Homelab

This repository combines four related areas:

- Terraform for provisioning Kubernetes worker VMs on Proxmox
- Helm charts for cluster bootstrap and workloads
- Small application services used inside the cluster
- Ansible for MinIO service state that is managed outside Kubernetes manifests

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
├── ansible/                # MinIO buckets/users/policies management
├── apps/
│   ├── goapi/              # Go HTTP service
│   ├── pythonapi/          # Python Flask HTTP service
│   ├── reactapp/           # Vite/React frontend
│   └── pkg/                # Shared Go modules used by goapi
├── bin/
│   └── tf                  # Terraform wrapper script
├── components/
│   └── kubernetes/         # Terraform component entry point
├── docs/                   # Setup and notes
├── etc/
│   ├── env/                # Environment tfvars
│   └── nodes/              # Per-node tfvars
├── helm/
│   ├── apps/               # App charts
│   ├── bootstrap/          # Flux/bootstrap charts
│   ├── data/               # Stateful platform charts
│   ├── infra/              # Infrastructure charts
│   ├── libraries/          # Shared chart templates
│   └── observability/      # Monitoring and tracing charts
├── modules/
│   └── proxmox-ubuntu-vm/  # Reusable Terraform module
├── scripts/                # Helper scripts
├── sql/                    # SQL bootstrap/auth snippets
└── values/                 # Values files for upstream charts
```

## Main Areas

### Terraform and Proxmox

Terraform provisions Kubernetes worker VMs on Proxmox VE using layered `tfvars` and Terraform Cloud for state.

Key files:

- `bin/tf`
- `components/kubernetes/`
- `modules/proxmox-ubuntu-vm/`
- `etc/env/lab.tfvars`
- `etc/nodes/node*.tfvars`

### Kubernetes and Helm

Helm charts in `helm/` define cluster bootstrap, app deployment, data services, and observability components. The repo currently contains Helm charts and CI for chart validation and packaging. It does not currently contain the `infra/argocd/` application manifests referenced by older documentation.

### Application Code

The app stack under `apps/` is intentionally small:

- `apps/goapi`: Go service exposing readiness/liveness, metrics, `/hello`, and `/go/benchmarking`
- `apps/pythonapi`: Flask service exposing readiness/liveness, `/hello`, and `/python/benchmarking`
- `apps/reactapp`: Vite frontend used to hit the Go and Python routes from one UI
- `apps/pkg/*`: shared Go packages for HTTP server startup, logging, metrics, and integrations

### Ansible

`ansible/` manages MinIO state such as buckets, users, policies, lifecycle rules, and optional replication. Kubernetes remains Helm-managed; MinIO service state is handled separately.

## Devcontainer

The devcontainer is the preferred local working environment.

Mounted repo-managed files:

- `.devcontainer/.bashrc` -> `/home/vscode/.bashrc.devcontainer`
- `.devcontainer/.env` -> `/home/vscode/.env`
- `.devcontainer/hosts` -> `/etc/hosts`
- `.devcontainer/ssh/` -> `/home/vscode/.ssh`
- `.devcontainer/kubeconfig` -> `/home/vscode/.kube/config`
- `plugin-cache/` -> `/home/vscode/terraform-plugin-cache`

Included tooling:

- Docker outside Docker
- Terraform
- Flux
- kubectl and Helm
- k9s
- cilium CLI
- skaffold
- Node.js 24

Customize the container by editing `.devcontainer/.env`, `.devcontainer/.bashrc`, `.devcontainer/hosts`, and `.devcontainer/ssh/config`, then rebuild or reopen the devcontainer if needed.

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
- `component`: currently `kubernetes`
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
10. Removes generated `cloud.auto.tf` on exit

Makefile shortcuts:

```bash
make plan COMPONENT=kubernetes LAYER=node1
make apply COMPONENT=kubernetes LAYER=node1
make destroy COMPONENT=kubernetes LAYER=node1
```

Useful helper targets:

- `make refresh-join-token`
- `make refresh-postgres-env`
- `make postgres`

## Kubernetes Worker Bootstrap

The Terraform cloud-init data installs and configures:

- `containerd` with `SystemdCgroup=true`
- Kubernetes packages from `pkgs.k8s.io`
- swap disable
- optional `kubeadm join`

Join is idempotent and only runs if `/etc/kubernetes/kubelet.conf` does not already exist.

## CI and Build Workflows

GitHub Actions currently handle:

- `apps/goapi` tests and image build
- `apps/pythonapi` tests and image build
- `apps/reactapp` tests and image build
- Helm chart discovery, templating tests, and GHCR packaging

Important note: some workflow files may lag behind the runtime choices in the repo. For example, the React app is Vite-based and the devcontainer uses Node 24, so treat workflow definitions as something to verify rather than assume are fully current.

## Ansible Workflow

Run playbooks from `ansible/` to manage MinIO state:

```bash
cd ansible
ansible-playbook -i inventory/hosts.ini playbooks/minio-incluster.yml
```

Generated Kubernetes Secret manifests are written under `ansible/out/` and can be applied manually.

## Setup Notes

See `docs/Setup.md` for workstation setup, SSH certificate setup, kubeconfig bootstrapping, and Proxmox host notes.

## Security Notes

- Do not commit real secrets
- Keep cluster credentials and kubeconfig material out of git
- `kubeadm_join_command` remains sensitive and can appear in Terraform state if embedded in cloud-init
- Prefer local secret overrides or external secret stores for sensitive data

## Documentation Maintenance

When the repo structure, workflows, or conventions change, keep these docs aligned in the same task:

- `README.md`
- the nearest relevant `AGENTS.md`
- `.codex/REPO_MAP.md`
- `.codex/REPO_PLAN.md`
