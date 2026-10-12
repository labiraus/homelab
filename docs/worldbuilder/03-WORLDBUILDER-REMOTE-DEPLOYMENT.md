# Worldbuilder AI — Remote preparation and LAN validation runbook

## Important distinction

Use the [current acceptance matrix](ACCEPTANCE-MATRIX.md) and [recovery handoff](../RecoveryHandoff-2026-10-11.md) before selecting a stage. The original remote-only phase was followed by authorized LAN recovery, production schema 8 and verified activation checkpoints for core Worldbuilder, AI, MCP and durable jobs. A later physical Pi API/SSH outage now blocks new cluster acceptance; it does not erase those checkpoints. Preserve existing activation, managed identities and data rather than resetting them to initial defaults.

While cluster access is unavailable, continue source changes, disposable tests and artifact publication. The user explicitly authorizes tested commits/pushes to `main`; do not request that permission again. Green source/CI checks never establish live readiness. When access returns, first rediscover current state, then continue remaining stages within the existing authorization. Concrete procedures and remaining gates are in [LAN-RUNBOOK.md](LAN-RUNBOOK.md).

## Phase R — remote preparation (execute now)

**Allowed**
- Repository audit, documentation reconciliation, application refactors, new database schema and migration code, unit/component tests.
- Local ephemeral PostgreSQL/OpenSearch/NATS tests **only if resources and container runtime are available**; otherwise record unavailable checks.
- Go/TS compile, lint, tests, code generation, local test fixtures, SQL migration verification.
- `helm lint`, `helm template` with every applicable environment values overlay, `kustomize build`, schema checks, policy/static security scans, manifest diffs, and Flux-specific local validation where tooling is installed.
- Produce environment-specific values and manifests for review (without credentials), planned ordering and dry-run commands for the LAN stage.
- Prepare a rollout checklist, backup checklist, test queries and rollback path.

**Boundaries for live operations**
- Do not mutate an unidentified or inaccessible cluster, infer safety from a rendered manifest, or bypass the normal GitOps workflow with ad hoc Helm replacement. Use existing scoped authorization and current evidence for recovery stages; destructive Cassandra/data retirement remains separately approved.
- Publish tested changes through the authorized main pipeline while preserving verified active configuration. New optional stages, including canon semantic retrieval, retain their own disabled defaults and activation gates. Source delivery alone does not authorize data deletion, credential rotation or storage cutover.
- Claim live behavior only from actual observations, with its checkpoint and scope. Prior success does not prove present readiness during the outage.

## Remote verification checklist

| Check | Remote result to record |
|---|---|
| Repo inventory and impact map | Verified paths, symbols, services, dependencies |
| Build/test/lint for Go/TS/UI | Commands, pass/fail/skip, logs |
| Migration tests against disposable PostgreSQL | Commands and schema version, or blocked reason |
| Helm chart lint and rendered manifests | Environment overlays checked, warnings, saved diffs |
| Kustomize/Flux object validation | All relevant dev/stable/release overlays, or unavailable tooling |
| Secret references, env vars, ports, storage claims, probes | Checked against existing values and policies |
| Database backup/restore evidence | Named source/checkpoint, actual isolated restore comparison and remaining coverage; a plan or desired manifest alone is insufficient |
| Rollback checklist | Compatible versions, database recovery and GitOps revert path |
| GitOps auto-sync safety | Tested main publication is authorized; preserve established activation, gate new stages and distinguish artifacts from observed readiness |

Document actual status as `LOCAL_VERIFIED`, `NOT_RUN`, `BLOCKED_CLUSTER_ACCESS`, or `FAILED`. No `DEPLOYED` status for offline work.

## Phase L — LAN / cluster validation (when access returns)

Live work requires verified access, identity, recovery evidence and authorization for the particular action. Existing session authorization persists; access returning is not a reason to request it again or repeat completed migrations. Before further mutations:

1. Confirm VPN/LAN, kube-context, namespace, cluster identity, RBAC and active GitOps source branch. Run read-only `kubectl cluster-info`, `kubectl get nodes`, `kubectl get ns`, `flux get all -A` (if available), `kubectl get helmreleases -A` and storage/backup inspection. Record differences from assumptions.
2. Confirm backup and restore capability for Cassandra (if needed) and persistent resources. Verify free capacity, StorageClass, PVC/PV binding expectations, PostgreSQL operator or Helm chart availability, secrets and network access. Verify zero unexpected pending/destroy operations.
3. Diff proposed Helm/Flux manifests against live resources. Check image registry availability and whether new tags exist and are signed as required. Review blast radius, monitoring and rollback.
4. PostgreSQL already exists. Rehearse new schema/configuration on explicitly isolated storage first, verify mounts, DNS, policy, credentials and restart persistence, then inspect the installed schema before deciding whether migration is needed. A branch or namespace named `dev` does not establish isolation. Do not remove Cassandra.
5. Roll out domain API and other milestones incrementally behind feature flags. Confirm readiness, error budget, request tracing, authN/authZ, OpenSearch sync, NATS delivery and Envoy inference in that order as relevant. Run smoke and end-to-end tests with seeded fictional world fixtures.
6. Test deliberate failure/recovery and backup restore in an isolated environment. Confirm outbox idempotency and search-index rebuild.
7. Only after a documented migration reconciliation and explicit go/no-go may the Cassandra decommission plan proceed. Make irreversible steps separate PRs with operator review.

## Stop/go matrix

| Situation | Decision |
|---|---|
| Remote work complete, no cluster access | STOP before any live rollout; report prepared files and blocked checks |
| Helm rendering fails or missing values | STOP and fix locally |
| Context/namespace unexpected | STOP and investigate |
| Persistent volume/backup unverified | STOP before data migration or destructive actions |
| PostgreSQL passes isolated dev validation | GO to API integration, not to Cassandra removal |
| App rollback requires incompatible irreversible schema change | STOP, redesign rollout/backwards compatibility |
| Reconciliation/backup/restore and operator approval complete | Consider controlled legacy decommission |

## Handover report format

Record in `docs/worldbuilder/IMPLEMENTATION-STATUS.md`:

- Completed milestones and changed files/commits.
- Exact local commands and results; blocked tests and missing tools.
- Rendered manifest locations and config overlays verified.
- GitOps/auto-sync risk evaluation.
- Unresolved architectural assumptions and high-risk migration issues.
- Exact read-only cluster inspection commands and ordered deployment steps for LAN.
- Stop/go decision and rollback instructions.

Do not estimate or assert deployment success without observing the cluster.
