# Repo Map For Codex

Use this file as the first fast pass when you need to locate the relevant code in this repo.

## Top-Level Map

- `.codex/`
  - `AGENTS.md`: reference index for Codex docs in this directory
  - `PI_NOTES.md`: durable setup notes for Raspberry Pi hosts that do not yet have dedicated repo code
  - `REPO_MAP.md`: this summary
  - `REPO_PLAN.md`: repo target state, priorities, and gaps
  - `MINECRAFT_TROUBLESHOOTING.md`: recurring Minecraft-specific failures, causes, and fixes
- `AGENTS.md`: root repository operating rules
- `.devcontainer/`
  - Devcontainer config, mounted kubeconfig, SSH config, env file
- `apps/`
  - `external/`: public Go service
  - `mcp/`: public Go MCP service
  - `orchestrator/`: internal Go document service
  - `processor/`: internal TypeScript Kafka worker
  - `ui/`: Vite frontend
  - `pkg/`: shared Go modules
- `ansible/`
  - external service state management, including the Minecraft VM
- `bin/`
  - `tf`: Terraform wrapper used by the Makefile
- `components/`
  - Terraform entry points
- `modules/`
  - reusable Terraform modules
- `helm/`
  - installable charts and chart libraries
  - `workloads/`: misc Flux-managed workloads outside the core platform
- `scripts/`
  - operational helper scripts such as kubeconfig refresh
- `sql/`
  - SQL snippets and bootstrap scripts
- `values/`
  - values files for upstream charts

## Where To Look By Task

### Terraform and VM Provisioning

- Start with `bin/tf`
- Then check `Makefile`
- Terraform component entries: `components/kubernetes/`, `components/minecraft-vm/`
- Reusable VM module: `modules/proxmox-ubuntu-vm/`
- Environment data: `etc/env/` and `etc/nodes/`

Current behavior note:

- the authoritative Minecraft deployment path is the dedicated VM layer `minecraft-node1` under `components/minecraft-vm/`, not Helm

### Kubernetes App Deployment

- App charts: `helm/apps/`
- Bootstrap charts: `helm/bootstrap/`
- Stateful/data charts: `helm/data/`
- Misc workload charts: `helm/workloads/`
- Shared chart templates: `helm/libraries/commonapi/`
- Upstream override values: `values/`

### Go Service Work

- Public API entry point: `apps/external/main.go`
- MCP entry point: `apps/mcp/main.go`
- Orchestrator entry point: `apps/orchestrator/main.go`
- Shared HTTP server and probes: `apps/pkg/api/api.go`
- Logging and readiness wiring: `apps/pkg/base/base.go`
- Metrics: `apps/pkg/prometheusutil/prometheus.go`
- Kafka helpers: `apps/pkg/kafkautil/kafka.go`

### Frontend Work

- Entry point: `apps/ui/src/main.jsx`
- Main UI: `apps/ui/src/App.jsx`
- Tests: `apps/ui/src/App.test.jsx`
- Tooling and scripts: `apps/ui/package.json`
- Vite config: `apps/ui/vite.config.js`
- Container build: `apps/ui/dockerfile`

Current behavior note:

- The frontend calls `/api/users/count`.
- This app is Vite-based, not Create React App.

### MinIO and Ansible

- Start with `ansible/README.md`
- Inventory: `ansible/inventory/`
- Playbooks: `ansible/playbooks/`
- Roles: `ansible/roles/`
- Secret sync: `scripts/ansible-fetch-secrets.sh`
- Wrapper: `scripts/ansible-run-playbook.sh`
- Host bootstrap: `scripts/bootstrap-svartalfheim-storage.sh`
- Minecraft VM playbook: `ansible/playbooks/minecraft-vm.yml`

Current behavior note:

- The repo treats the external Raspberry Pi `svartalfheim` as the authoritative MinIO service and attached-drive file-share host; Helm no longer deploys an in-cluster MinIO tenant.
- Minecraft is managed outside Kubernetes on the dedicated VM `nidavellir` through Terraform plus Ansible.
- Shared operator env belongs in `.devcontainer/.env`; Ansible-only external secrets belong in ignored `ansible/.env`.
- MinIO admin credentials are refreshed from `svartalfheim:/etc/default/minio` rather than copied from a tracked example file.

### CI and Release Behavior

- App workflows: `.github/workflows/app-*.yml`
- Helm workflow: `.github/workflows/helm-all.yml`

## Known Repo Realities

- The root `README.md` is the main human overview and should stay aligned with the real directory structure.
- The root `AGENTS.md` is the main repo-wide agent policy; use the nearest directory-level `AGENTS.md` for more specific conventions.
- The current GitOps path is Flux plus `helm/`; keep docs and changes aligned to that delivery model.
- If Codex changes behavior, entry points, or layout, update both the human docs and this file in the same task.
