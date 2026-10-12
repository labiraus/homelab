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
- Read the discovery output before selecting paths: chart templates are not necessarily named after their resource kinds, and workflow filenames may differ from their display names.
- When changing behavior, config, workflows, or layout, update the relevant human docs in the same task.
- Do not commit real secrets, kubeconfig material, private keys, or local-only environment files.
- Prefer repo-native workflows before inventing new ones.
- SeaweedFS is the target external S3 service on `svartalfheim`; follow `docs/SeaweedFSMigration.md`. Existing MinIO automation and data are migration/rollback sources, not the target architecture. Do not remove source data or treat source configuration changes as a completed migration.
- Keep SeaweedFS on the private network. Cluster workloads use its private S3 endpoint; browser-facing document delivery remains brokered through authenticated `mcp.labiraus.com` routes. Master, filer and volume interfaces stay on loopback for this single-host deployment.
- When a session reveals durable setup facts about Raspberry Pi hosts that do not yet have dedicated repo code, record them in `.codex/PI_NOTES.md`.
- `yggdrasil` is a physical Raspberry Pi control plane, not a Proxmox VM. Read `.codex/PI_NOTES.md` before selecting its recovery path; ping or a TCP connection without an SSH banner does not establish userspace health.

For Postgres troubleshooting in this repo, prefer the devcontainer `psql` workflow over `kubectl exec` into the database pod:

- use `make postgres` when an interactive session is appropriate
- use the connection details populated by `make refresh-postgres-env` / `.devcontainer/.env` for scripted `psql` checks
- only fall back to in-cluster execution if the local `psql` path is unavailable or explicitly requested

For SSH recovery, check certificate validity and available authorized-key or console access before changing infrastructure. The SSH user CA is separate from Kubernetes PKI: a lost signing passphrase calls for a staged trust rotation, not a VM rebuild or kubeadm reset. Verify the effective `sshd` trust paths and actual authorized-key symlink before prescribing a Proxmox shared-file change; API access or a successful file edit does not prove SSH recovery. An `authorized_keys` CA entry requires the `cert-authority` option and appropriate principals; a bare CA public key does not authorize its signed certificates. Provide console commands as a single shell line when the question interface flattens line breaks. Verify new certificate authentication before retiring old trust, and independently verify changed host keys before editing `known_hosts`. Update public CA provisioning inputs without applying Terraform solely to rotate live trust. Follow `docs/Setup.md` and keep CA private keys, passphrases, and rollback copies local-only.

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

For multi-file patches, read exact current headings/anchors first and omit placeholder or no-op updates; one stale anchor rejects the entire patch. When inserting a section before a heading, replace the anchor once rather than retaining it above the new section and duplicating it below. Check changed documents for duplicate headings.

Wait for background formatters or generators to finish before editing their output files, then read the current anchors again. A yielded command may still mutate files even when its tool call has returned.

Reference markdown can contain mixed line endings. If `git diff --check` flags newly added CRLF lines, normalize only the touched lines rather than reformatting unrelated content.

## CI script execution

On Windows-backed workspaces, local executable permissions do not prove Git stores an executable file mode. For scripts invoked directly by CI, verify `git ls-files --stage` reports `100755`; use an explicit interpreter in CI as well.

## Disposable database test transport

Docker-outside-Docker published ports can become unreachable despite a healthy container. Use the Worldbuilder harness with `WORLDBUILDER_TEST_TRANSPORT=exec` for its loopback-only Docker-exec bridge when needed; do not switch these offline tests to a LAN/production DSN. A failed transport preflight is not a passing integration test.

Finite commands in the disposable PostgreSQL fixture also require the labelled, fixture-scoped exec-ID transport in `scripts/worldbuilder/docker-exec.py`; plain Docker attach may return success before `createdb`, migrations or restores finish. Preserve the original streaming CLI only for the loopback database bridge.

## Disposable controller transport

For disposable kind work, set the generated `KUBECONFIG` for creation and deletion; cleanup must not lock or edit the operator's home configuration. The controller harness supplies both explicit arguments and environment settings.

Docker exec attach can terminate early with exit 0 while its command is still running, and archive-copy acknowledgements can lack the expected files in a proxied kind environment. Do not infer command completion from that client status. Use the fixture-scoped exec-ID transport in `scripts/worldbuilder/docker-exec.py`, verify terminal status and output/postconditions, and keep ownership checks and timeout failure intact. Never redirect an offline fixture to production to bypass transport failures.

Use `scripts/worldbuilder/kind-fixture.py` for shared controller/backup lifecycle and loopback mappings. CNPG and Prometheus webhook refusals can appear as ordinary Kubernetes `Invalid` messages without a webhook name or `admission webhook` prefix. Check the tested semantic error (for example, conflicting bootstrap or an invalid PromQL expression) and separately prove unavailable-webhook refusal/recovery. Database fixture fallback is for unavailable local transport, never authentication or SQL failures.

Rendered Helm metadata can contain explicit null annotations. Treat absent/null annotation maps as empty when classifying hook resources; execute certificate-generation and CA-patching Jobs in their intended order before claiming webhook acceptance.

Inspect retained upstream Helm renders for Secret values before uploading fixture artifacts. Cilium renders CA/Hubble private keys inside its YAML; excluding a private directory is insufficient. Apply original resources only inside the owned fixture and retain redacted copies for public evidence.

For disposable controller cold-start timeouts, inspect Pod events and image-download progress before attributing failure to admission, networking or reconciliation. Keep startup waits bounded, retain the failed run and verify owned cleanup before retrying. A partial bootstrap or continuing download is not passing acceptance.

In custom-CNI kind fixtures, CoreDNS exists while nodes await CNI installation. Set its disposable Deployment progress deadline before cold CNI downloads; a longer later rollout wait cannot repair an already exceeded deadline. Still require actual node/DNS readiness, and keep default-networking fixtures unchanged.

Select controller Pods using the Deployment's actual selector when testing replacement or shutdown. Prometheus admission hook Pods share the operator's `app.kubernetes.io/name`; that label alone includes completed certificate Jobs and makes deletion waits or single-Pod assertions incorrect.

NodePort creation does not mean kube-proxy has installed its route: Docker can accept and close an initial connection. Retry bounded preflight connection closure before evaluating database readiness; persistent closure must still fail rather than switch to peer authentication.

A successful API-server Service-proxy read also does not prove the node's ClusterIP route is ready. For the restored Prometheus admin check, retry only initial connection refusal within a bounded interval, then require the actual disabled-API response. Connection failure must never count as successful administrative isolation.

Webhook Service routing can also lag a Ready replacement Pod. After rollout, require a successful real admission request within a bounded readiness interval before continuing; never replace it with a Pod-ready-only assertion or bypass TLS verification.

Istio's initial validator can have `failurePolicy: Ignore` until its CA is patched. Admission readiness must verify matching rules, nonempty trust and `Fail` before the real request. For Gateway certificate-trust refusal, keep the configured SNI and use an unrelated CA: unknown SNI may reset the connection before any certificate is presented, which does not prove client trust validation.

For disposable Velero file recovery, assert the actual PV is supported `local` storage rather than kind's default hostPath, require completed PodVolumeBackup/Restore evidence and remove the source data path before comparison. Exclude the source's generated namespace root-CA ConfigMap with the `velero.io/exclude-from-backup=true` **label**, not an annotation; keep fresh destination trust and fail unexplained restore warnings. Use `scripts/worldbuilder/s3-fixture.py` for owned external storage and the target's scoped identity actions.

## Storage chronology evidence

Treat S3 version IDs as opaque. Retain mixed listing/pagination evidence for timestamp ties, and use `IsLatest` when it proves a unique position: a latest null version is not ambiguous merely because its timestamp ties another version. Refuse null ties only when multiple chronological positions remain possible; never infer order from the provider's placement of null last.

The observed MinIO source uses form-encoded listing keys and v2 cache suffixes in pagination markers. Use the explicit `minio-v2` source-listing dialect and retain it with both scans; do not globally reinterpret plus signs or strip provider markers before sending them back. Check apparent `NoSuchVersion` failures against the decoded key and actual SDK read before declaring source loss. SeaweedFS/default evidence keeps its existing interpretation.

## Disposable TSDB history

A healthy scrape or replacement Pod does not prove historical recovery. Stop producers, wait for quiescent raw samples, pin the comparison time, verify PVC identity and the actual `prometheus-db` mount, and compare exact sample labels/timestamps/values after WAL replay and snapshot restore. Delete only owned fixture volumes, prove the old paths are gone and use a fresh PVC after a corrupt-data refusal. Keep the admin API confined to the disposable source and disabled in published storage settings.

## Recovery command dependencies

Use `set -euo pipefail` in multi-step shell recovery commands, or execute dependent steps separately and inspect each result. A failed render or missing Python dependency must prevent subsequent apply/mutation steps; do not infer success from the final command exit status. Use the prepared Python virtualenv when scripts require PyYAML/boto3.
