## Scope And Precedence

Use this file as the root operating policy for work in this repository.

When multiple agent docs exist, apply them in this order:

1. the most specific directory-level `AGENTS.md`
2. this root `AGENTS.md`
3. reference material under `.codex/`

Use the `.codex` docs as reference context:

- `.codex/REPO_MAP.md`: fast navigation map for the codebase
- `.codex/REPO_PLAN.md`: current target state, priorities, and active gaps
- `.codex/PI_NOTES.md`: durable setup notes for Raspberry Pi hosts that do not yet have dedicated repo code

Directory-level `AGENTS.md` files currently exist for:

- `ansible/`
- `apps/`
- `components/`
- `helm/`
- `sql/`

## Working Rules

- Treat the Labiraus MCP server (`homelab-mcp` in local Codex config) as the primary subject of this repo and its deployed surface area, not as a sidecar utility.
- When evaluating the deployed project, prefer using the Labiraus MCP server together with other repo-native tools such as local `psql`, Helm/Flux inspection, and app logs so MCP-discovered behavior is checked against the rest of the deployed system.
- Prefer small, targeted changes that fit the current structure of the repo.
- When changing behavior, config, workflows, or layout, update the relevant human docs in the same task.
- Do not commit real secrets, kubeconfig material, private keys, or local-only environment files.
- Prefer repo-native workflows before inventing new ones.
- Treat MinIO as an external service hosted on the Raspberry Pi `svartalfheim`; do not reintroduce an in-cluster MinIO deployment unless repo guidance is explicitly updated.
- Do not assume the external MinIO host should be publicly reachable from the internet. Cluster workloads should talk to MinIO directly on the private/repo-managed network path, and any browser-facing or pre-signed-style document delivery should be brokered through an authenticated `mcp.labiraus.com` endpoint rather than exposing MinIO itself.
- When a session reveals durable setup facts about Raspberry Pi hosts that do not yet have dedicated repo code, record them in `.codex/PI_NOTES.md`.

For Postgres troubleshooting in this repo, prefer the devcontainer `psql` workflow over `kubectl exec` into the database pod:

- use `make postgres` when an interactive session is appropriate
- use the connection details populated by `make refresh-postgres-env` / `.devcontainer/.env` for scripted `psql` checks
- only fall back to in-cluster execution if the local `psql` path is unavailable or explicitly requested

## Documentation Upkeep

When Codex changes code, config, workflows, or repository structure, consider whether any of these need updates:

- `README.md`
- app-level READMEs under `apps/`
- setup or operator docs under `docs/`
- `.codex/REPO_MAP.md` if the navigation map changed
- `.codex/REPO_PLAN.md` if priorities, direction, or assumptions changed
- the nearest directory-level `AGENTS.md` if a local rule changed

## Self-Learning Loop

When a problem is fixed, capture the learning close to where it matters:

- update `runbooks/troubleshooting.md` if the issue is operator-facing
- update the nearest `AGENTS.md` if the fix should change future agent behavior
- update `.codex/REPO_PLAN.md` if the fix changes current priorities or closes a known gap

If the same issue or class of issue happens more than once, add a permanent rule to the nearest relevant `AGENTS.md` rather than relying on memory.

If Codex makes an incorrect assumption about this repo, correct that assumption in the nearest relevant `AGENTS.md` so future sessions start from the updated rule.
