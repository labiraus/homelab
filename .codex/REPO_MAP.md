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
  - `mcp/`: public Go MCP service; see `apps/mcp/AGENTS.md` for transport/client-compatibility rules
  - `orchestrator/`: internal Go document service
  - `processor/`: internal TypeScript NATS JetStream worker
  - `ui/`: Vite frontend
  - `pkg/`: shared Go modules
- `ansible/`
  - external service state management, including the Minecraft VM
- `docs/`
  - `Setup.md`: Kubernetes setup path for local machine, Proxmox worker prep, SSH, kubeconfig, and worker network stability
  - `Secrets.md`: generated local secrets and Ansible-only secret handling
  - `StorageBootstrap.md`: external MinIO and Samba bootstrap on `svartalfheim`
  - `EnvoyAIGateway.md`: Flux infra path and scope for Envoy Gateway plus Envoy AI Gateway
  - `MinecraftVM.md`: dedicated Minecraft VM provisioning and operations
  - `NodeClassification.md`: Kubernetes worker label policy and GPU/LLM classification
  - `Auth.md`: current auth model across browser OIDC and certificate-based identity
  - `MCPPrompts.md`: current MCP prompt catalog
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
- Infra charts: `helm/infra/`
- Bootstrap child infra chart: `helm/bootstrap/flux-infra/`
- Browser edge auth chart: `helm/infra/oauth2-proxy/`
- Bootstrap charts: `helm/bootstrap/`
- Envoy AI Gateway infra chart: `helm/infra/envoy-ai-gateway/`
- Stateful/data charts: `helm/data/`
- Misc workload charts: `helm/workloads/`
- Shared chart templates: `helm/libraries/commonapi/`
- Async worker chart templates: `helm/libraries/commonscaled/`
- Upstream override values: `values/`

### Go Service Work

- Public API entry point: `apps/external/main.go`
- MCP entry point: `apps/mcp/main.go`
- Orchestrator entry point: `apps/orchestrator/main.go`
- Orchestrator request, scan, and async contract shapes: `apps/orchestrator/documents.go` and `apps/orchestrator/scan.go`
- Browser-facing document inventory API: `apps/external/inventory.go`
- Browser-facing document history API: `apps/external/history.go`
- Browser-facing document control proxy API: `apps/external/control.go`
- Shared HTTP server and probes: `apps/pkg/api/api.go`
- Logging and readiness wiring: `apps/pkg/base/base.go`
- Metrics: `apps/pkg/prometheusutil/prometheus.go`
- Local deterministic embeddings: `apps/pkg/embeddingutil/embedding.go`
- NATS JetStream helpers: `apps/pkg/natsutil/nats.go`
- S3 / MinIO helpers: `apps/pkg/minioutil/minio.go`
- Postgres helpers: `apps/pkg/postgresutil/postgres.go`

Current behavior note:

- `orchestrator` is the intended control-plane service for reconciliation and task dispatch.
- `processor` is the intended stateless data-plane worker.
- operator access to the CNPG Postgres cluster is through `make postgres`, which opens a temporary local `kubectl port-forward` rather than relying on a standing TCP ingress path

### Frontend Work

- Entry point: `apps/ui/src/main.jsx`
- Main UI: `apps/ui/src/App.jsx`
- Tests: `apps/ui/src/App.test.jsx`
- Tooling and scripts: `apps/ui/package.json`
- Vite config: `apps/ui/vite.config.js`
- Container build: `apps/ui/dockerfile`

Current behavior note:

- The frontend calls `/api/users/count` for the overview check.
- Authenticated document workflows call `/api/documents/inventory`, `/api/documents/search`, `/api/documents/history`, `/api/documents/context`, `/api/documents/curation`, `/api/documents/edit-text`, `/api/documents/reprocess`, `/api/documents/scan-bucket`, `/api/documents/events`, `/api/documents/tree`, `/api/documents/object`, and `/api/documents/upload`.
- The Inventory tab calls `/api/documents/inventory` to inspect Postgres-backed reconciliation and processing state.
- The Inventory tab can call `/api/documents/scan-bucket` to ask orchestrator to reconcile the current prefix filter, then refresh inventory rows.
- The Inventory tab can call `/api/documents/history` for a selected inventory row to inspect durable lifecycle events.
- The Inventory tab can call `/api/documents/curation` for a selected inventory row to update curated metadata through the orchestrator control plane.
- The Inventory tab can load raw source text through `/api/documents/object` and save guarded text edits through `/api/documents/edit-text` for text inventory rows with source object keys.
- The Inventory tab can call `/api/documents/reprocess` for text inventory rows with source object keys, then load version-specific lifecycle history for the queued version.
- The Search tab also calls `/api/documents/context` to assemble cited context blocks from the current query and filters.
- The Search tab can select a result for document actions, updating curated metadata, performing guarded text edits, and queueing reprocessing through `external` proxy routes backed by `orchestrator`.
- Reprocess and guarded edit actions automatically load `/api/documents/history` for the queued processing version returned by `orchestrator`.
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
- NATS JetStream plus KEDA are the async execution layer, not the durable state store.
- `external` exposes semantic document search for the UI.
- `mcp` exposes document inventory and semantic search for agents.
- `external` and `mcp` expose durable document lifecycle history from `rag.document_lifecycle_events`.
- `ui` renders durable lifecycle history for retrieved documents by calling `/api/documents/history`.
- `ui` renders cited context blocks by calling `/api/documents/context` from the Search tab.
- `ui` exposes metadata curation, guarded text editing, reprocess, and scan actions by calling `/api/documents/curation`, `/api/documents/edit-text`, `/api/documents/reprocess`, and `/api/documents/scan-bucket`; `external` proxies those requests to `orchestrator`.

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
- Terraform worker bootstrap playbook: `ansible/playbooks/kubernetes-terraform-node.yml`

Current behavior note:

- The repo treats the external Raspberry Pi `svartalfheim` as the authoritative MinIO service and attached-drive file-share host; Helm no longer deploys an in-cluster MinIO tenant.
- Minecraft is managed outside Kubernetes on the dedicated VM `nidavellir` through Terraform plus Ansible.
- `midgard` can be prepared and joined as a manually managed Ubuntu worker through the Ansible playbook `ansible/playbooks/kubernetes-manual-node.yml`.
- Terraform-managed worker VMs are provisioned through `bin/tf` and then joined to Kubernetes through the Ansible playbook `ansible/playbooks/kubernetes-terraform-node.yml`.
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
