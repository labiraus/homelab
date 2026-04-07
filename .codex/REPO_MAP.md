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
- `docs/`
  - `Setup.md`: Kubernetes setup path for local machine, Proxmox worker prep, SSH, kubeconfig, and worker network stability
  - `Secrets.md`: generated local secrets and Ansible-only secret handling
  - `StorageBootstrap.md`: external MinIO and Samba bootstrap on `svartalfheim`
  - `MinecraftVM.md`: dedicated Minecraft VM provisioning and operations
  - `NodeClassification.md`: Kubernetes worker label policy and GPU/LLM classification
  - `Auth.md`: current auth model across browser OIDC and certificate-based identity
  - `GoogleOIDCSetup.md`: Google-side OIDC setup and redirect URI guidance
  - `OAuth2ProxyGoogle.md`: current chosen `oauth2-proxy + Google` browser-auth path
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
  - SQL snippets and bootstrap scripts, including the `rag` schema foundation
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
- Browser edge auth chart: `helm/apps/oauth2-proxy/`
- Bootstrap charts: `helm/bootstrap/`
- Stateful/data charts: `helm/data/`
- Misc workload charts: `helm/workloads/`
- Shared chart templates: `helm/libraries/commonapi/`
- Async worker chart templates: `helm/libraries/commonscaled/`
- Upstream override values: `values/`

### Go Service Work

- Public API entry point: `apps/external/main.go`
- MCP entry point: `apps/mcp/main.go`
- Orchestrator entry point: `apps/orchestrator/main.go`
- Orchestrator request and Kafka contract shapes: `apps/orchestrator/documents.go`
- Shared HTTP server and probes: `apps/pkg/api/api.go`
- Logging and readiness wiring: `apps/pkg/base/base.go`
- Metrics: `apps/pkg/prometheusutil/prometheus.go`
- Kafka helpers: `apps/pkg/kafkautil/kafka.go`
- S3 / MinIO helpers: `apps/pkg/s3util/s3.go`
- Postgres helpers: `apps/pkg/postgresutil/postgres.go`

Current behavior note:

- `orchestrator` is the intended control-plane service for reconciliation and task dispatch.
- `processor` is the intended stateless data-plane worker.

### Frontend Work

- Entry point: `apps/ui/src/main.jsx`
- Main UI: `apps/ui/src/App.jsx`
- Tests: `apps/ui/src/App.test.jsx`
- Tooling and scripts: `apps/ui/package.json`
- Vite config: `apps/ui/vite.config.js`
- Container build: `apps/ui/dockerfile`

Current behavior note:

- The frontend calls `/api/users/count`.
- Browser login is currently expected to be challenged at the edge through `oauth2-proxy`, not implemented directly in React.
- This app is Vite-based, not Create React App.

### Document Platform And RAG Work

- Working architecture and phase plan: `.codex/REPO_PLAN.md`
- Initial document/control-plane schema: `sql/rag/schema.pgsql`
- Async ingestion design note: `docs/async-ingestion.md`
- High-level RAG and retrieval direction: `docs/RAG.md`

Current behavior note:

- MinIO is the canonical raw object store.
- Postgres is the source of truth for metadata, state, chunks, and embeddings.
- Kafka plus KEDA are the async execution layer, not the durable state store.

### MinIO and Ansible

- Start with `ansible/README.md`
- Supporting operator docs: `docs/Secrets.md` and `docs/StorageBootstrap.md`
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
- `midgard` can be prepared and joined as a manually managed Ubuntu worker through the Ansible playbook `ansible/playbooks/kubernetes-manual-node.yml`.
- Shared operator env belongs in `.devcontainer/.env`; Ansible-only external secrets belong in ignored `ansible/.env`.
- MinIO admin credentials are refreshed from `svartalfheim:/etc/default/minio` rather than copied from a tracked example file.

### Kubernetes Setup And Worker Policy

- Core Kubernetes setup guide: `docs/Setup.md`
- Worker rebuild and recovery: `docs/WorkerRedeploy.md`
- Worker label and capability policy: `docs/NodeClassification.md`

### CI and Release Behavior

- App workflows: `.github/workflows/app-*.yml`
- Helm workflow: `.github/workflows/helm-all.yml`

## Known Repo Realities

- The root `README.md` is the main human overview and should stay aligned with the real directory structure.
- The root `AGENTS.md` is the main repo-wide agent policy; use the nearest directory-level `AGENTS.md` for more specific conventions.
- The current GitOps path is Flux plus `helm/`; keep docs and changes aligned to that delivery model.
- If Codex changes behavior, entry points, or layout, update both the human docs and this file in the same task.
