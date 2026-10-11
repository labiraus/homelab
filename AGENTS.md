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
- `apps/mcp/`
- `components/`
- `helm/`
- `sql/`

## Working Rules

- Worldbuilder AI is the application target; follow `docs/worldbuilder/` and its verified implementation status. The Labiraus MCP server (`homelab-mcp`) remains a first-class deployed integration surface.
- The Worldbuilder target initially assumed separate engine/infra repositories, Cassandra and Fastify. This checkout is a monorepo with existing PostgreSQL and Node HTTP; audit actual code before acting on illustrative paths. No Cassandra resource removal is authorised by its absence from this checkout.
- When evaluating the deployed project, prefer using the Labiraus MCP server together with other repo-native tools such as local `psql`, Helm/Flux inspection, and app logs so MCP-discovered behavior is checked against the rest of the deployed system.
- Prefer small, targeted changes that fit the current structure of the repo.
- Discover existing file paths with `rg --files` before targeted reads; do not infer app, chart or migration-document paths from naming conventions.
- When changing behavior, config, workflows, or layout, update the relevant human docs in the same task.
- Do not commit real secrets, kubeconfig material, private keys, or local-only environment files.
- Prefer repo-native workflows before inventing new ones.
- SeaweedFS is the target external S3 service on `svartalfheim`; follow `docs/SeaweedFSMigration.md`. Existing MinIO automation and data are migration/rollback sources, not the target architecture. Do not remove source data or treat source configuration changes as a completed migration.
- Keep SeaweedFS on the private network. Cluster workloads use its private S3 endpoint; browser-facing document delivery remains brokered through authenticated `mcp.labiraus.com` routes. Master, filer and volume interfaces stay on loopback for this single-host deployment.
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

## Devcontainer tool installation

Before adding a Dockerfile tool installer, check whether a configured devcontainer feature already installs it (the Terraform feature includes TFLint). Prefer that single source. For direct binary downloads, use pinned official release assets with architecture-specific checksum verification; avoid floating installer URLs piped into a shell. Validate changes with a complete feature-enabled Dev Containers build, since fixing one expired download can expose another.

## Patch application

For multi-file patches, read exact current headings/anchors first and omit placeholder or no-op updates; one stale anchor rejects the entire patch.

## CI script execution

On Windows-backed workspaces, local executable permissions do not prove Git stores an executable file mode. For scripts invoked directly by CI, verify `git ls-files --stage` reports `100755`; use an explicit interpreter in CI as well.

## Disposable database test transport

Docker-outside-Docker published ports can become unreachable despite a healthy container. Use the Worldbuilder harness with `WORLDBUILDER_TEST_TRANSPORT=exec` for its loopback-only Docker-exec bridge when needed; do not switch these offline tests to a LAN/production DSN. A failed transport preflight is not a passing integration test.
