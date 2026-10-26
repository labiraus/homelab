# Repository operating policy

## Scope and precedence

Apply the most specific directory-level `AGENTS.md`, then this root policy.
[Documentation](docs/README.md) is reference context, not additional authorization.
Discover applicable instruction files with `rg --files --hidden -g AGENTS.md`.

## Working rules

- Worldbuilder is the application target; Labiraus MCP remains a first-class integration. Read [architecture](docs/architecture.md), the relevant topic guide and [status](docs/status.md). Inspect actual code before acting on illustrative architecture; no Cassandra removal is authorized by its absence here.
- Before frontend changes, read [experience and journeys](docs/worldbuilder/experience.md) and `apps/ui/AGENTS.md`. Preserve stable result IDs, original scope and actual persistence/authority boundaries.
- Prefer small changes and repo-native workflows. Discover paths with `rg --files` and read its output before selecting files, templates or workflows.
- When evaluating deployment, use Labiraus MCP alongside local `psql`, Flux/Helm and app logs. Discover Flux namespaces separately from workloads; app HelmReleases live in `flux-system` and target `homelab`.
- Read [operations](docs/operations.md) before deployment diagnosis. Publication/source labels/digests do not establish signing, reconciliation or authenticated functionality. Public reports use `attempts[].ok`; substantiate cluster/authenticated claims separately.
- For recurring public failures, compare current DNS, observed WAN and private Gateway before changing app/TLS configuration. Address overrides retain hostname/SNI and verified TLS; they are not public-DNS acceptance.
- Before storage work, read [storage](docs/storage.md), including chronology, reference coverage and uncertain-copy rules. SeaweedFS is the private target on `svartalfheim`; MinIO automation/data remain migration/rollback sources. Preserve source data until separately authorized retirement.
- Keep browser document delivery authenticated through `mcp.labiraus.com`. S3 remains private; SeaweedFS master/filer/volume and S3 gRPC remain loopback-only.
- Before host/SSH recovery, read [infrastructure](docs/infrastructure.md) and [recovery](docs/recovery.md). `yggdrasil` is a physical Chromebook, not a Raspberry Pi or Proxmox VM. Ping/TCP without SSH banner is not userspace health. Check certificate validity and available console/key access first; SSH CA loss is not a Kubernetes PKI reset or VM rebuild.
- Record new durable physical-host setup facts in [infrastructure](docs/infrastructure.md). Verify effective trust paths and authorized-key symlinks, staged certificate authentication and changed host-key identity before replacing trust. Keep private keys/passphrases/rollback material local-only.
- For PostgreSQL troubleshooting prefer `make postgres` or local `psql` using refreshed ignored devcontainer settings. Fall back to database-pod execution only if local transport is unavailable or explicitly requested.
- Before disposable database/controller/storage work, read [testing](docs/testing.md) and its transport/ownership safeguards. Never redirect offline fixtures to production. Passing skipped/preflight checks is not integration acceptance.
- When evolving canon, update backup seed and physical restore table inventory alongside the API harness; preserve full-table/security checksums and restored ledger/sequence checks.
- Use fail-fast dependent recovery commands (`set -euo pipefail`) or inspect each step separately. Use the prepared Python environment for PyYAML/boto3 scripts.
- Never commit real secrets, kubeconfig, private keys, local environment files or private fixture artifacts.

## Changes and documentation

- Update the canonical topic when behavior/config/layout changes; keep component READMEs local. [Status](docs/status.md) owns dated evidence and remaining work, [architecture](docs/architecture.md) owns navigation, and [troubleshooting](runbooks/troubleshooting.md) owns operator symptoms.
- Replace stale guidance in place. Keep historical narrative in Git history; do not append corrections above obsolete instructions or create parallel status/memory files.
- Distinguish source, fixture, publication and deployed evidence. Neither an old failure nor a prior success establishes current health; desired restore manifests are not completed restores.
- Record recurring lessons and corrected repository assumptions in the nearest relevant `AGENTS.md`, with detailed procedures in the topic guide. Preserve all safeguards when relocating instructions.
- On case-insensitive workspaces, record case-only renames through an intermediate filename with Git; writing lowercase then deleting the old spelling can delete the same file. Verify exact path case in links.
- Read exact current anchors before patches; avoid placeholders, duplicate headings and no-op updates. Wait for background generators/formatters before editing output. Normalize only touched CRLF lines.
- Before installing devcontainer tools, check existing features (Terraform includes TFLint). Use pinned official downloads with architecture-specific checksum verification and validate the complete feature-enabled build.
- For directly invoked CI scripts, verify Git records `100755` with `git ls-files --stage`; local Windows-backed executable permissions are insufficient. Use an explicit interpreter in CI as well.
