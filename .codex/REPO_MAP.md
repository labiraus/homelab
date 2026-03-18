# Repo Map For Codex

Use this file as the first fast pass when you need to locate the relevant code in this repo.

## Top-Level Map

- `.codex/`
  - `AGENTS.md`: persistent repo-specific rules for Codex
  - `REPO_MAP.md`: this summary
- `.devcontainer/`
  - Devcontainer config, mounted kubeconfig, SSH config, env file
- `apps/`
  - `goapi/`: Go service
  - `pythonapi/`: Flask service
  - `reactapp/`: Vite frontend
  - `pkg/`: shared Go modules
- `ansible/`
  - MinIO service state management
- `bin/`
  - `tf`: Terraform wrapper used by the Makefile
- `components/`
  - Terraform entry points
- `modules/`
  - reusable Terraform modules
- `helm/`
  - installable charts and chart libraries
- `scripts/`
  - operational helper scripts
- `sql/`
  - SQL snippets and bootstrap scripts
- `values/`
  - values files for upstream charts

## Where To Look By Task

### Terraform and VM Provisioning

- Start with `bin/tf`
- Then check `Makefile`
- Terraform component entry: `components/kubernetes/`
- Reusable VM module: `modules/proxmox-ubuntu-vm/`
- Environment data: `etc/env/` and `etc/nodes/`

### Kubernetes App Deployment

- App charts: `helm/apps/`
- Bootstrap charts: `helm/bootstrap/`
- Stateful/data charts: `helm/data/`
- Shared chart templates: `helm/libraries/commonapi/`
- Upstream override values: `values/`

### Go Service Work

- Entry point: `apps/goapi/main.go`
- Old outbound client helper: `apps/goapi/client.go`
- Local tests: `apps/goapi/client_test.go`
- Shared HTTP server and probes: `apps/pkg/api/api.go`
- Logging and readiness wiring: `apps/pkg/base/base.go`
- Metrics: `apps/pkg/prometheusutil/prometheus.go`
- In-cluster Kubernetes access: `apps/pkg/kubernetesutil/kubernetes.go`

Current behavior note:

- `apps/goapi/main.go` handles `/hello` directly and no longer uses `GetUser()` from `client.go`.

### Python Service Work

- Entry point: `apps/pythonapi/app.py`
- Response type: `apps/pythonapi/UserResponse.py`
- Tests: `apps/pythonapi/tests/app_test.py`
- Container dependencies: `apps/pythonapi/requirements.txt`

Current behavior note:

- `/hello` still calls `USER_API_URL`, defaulting to `http://userapi/user`.

### Frontend Work

- Entry point: `apps/reactapp/src/main.jsx`
- Main UI: `apps/reactapp/src/App.jsx`
- Tests: `apps/reactapp/src/App.test.jsx`
- Tooling and scripts: `apps/reactapp/package.json`
- Vite config: `apps/reactapp/vite.config.js`
- Container build: `apps/reactapp/dockerfile`

Current behavior note:

- The frontend calls `/go/hello` and `/python/hello`.
- This app is Vite-based, not Create React App.

### MinIO and Ansible

- Start with `ansible/README.md`
- Inventory: `ansible/inventory/`
- Playbooks: `ansible/playbooks/`
- Roles: `ansible/roles/`

### CI and Release Behavior

- App workflows: `.github/workflows/app-*.yml`
- Helm workflow: `.github/workflows/helm-all.yml`

## Known Repo Realities

- The root `README.md` is the main human overview and should stay aligned with the real directory structure.
- Older assumptions about `infra/argocd` and `charts/` are stale; the repo uses `helm/`.
- If Codex changes behavior, entry points, or layout, update both the human docs and this file in the same task.
