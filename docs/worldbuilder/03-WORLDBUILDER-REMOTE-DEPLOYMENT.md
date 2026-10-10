# Worldbuilder AI — Remote preparation and LAN validation runbook

## Important distinction

You currently have no access to the Kubernetes cluster. Work now must be **offline/remote preparatory**: source changes, unit/integration tests with local dependencies if available, chart/schema validation and generated manifest inspection. Do not confuse green local tests with successful deployment. When LAN access is restored, run the cluster verification and controlled activation steps. The user subsequently authorised main-branch CI publication during this session; this supersedes the no-push instruction for source delivery only. Database migration and feature activation remain gated. Concrete commands are in [LAN-RUNBOOK.md](LAN-RUNBOOK.md).

## Phase R — remote preparation (execute now)

**Allowed**
- Repository audit, documentation reconciliation, application refactors, new database schema and migration code, unit/component tests.
- Local ephemeral PostgreSQL/OpenSearch/NATS tests **only if resources and container runtime are available**; otherwise record unavailable checks.
- Go/TS compile, lint, tests, code generation, local test fixtures, SQL migration verification.
- `helm lint`, `helm template` with every applicable environment values overlay, `kustomize build`, schema checks, policy/static security scans, manifest diffs, and Flux-specific local validation where tooling is installed.
- Produce environment-specific values and manifests for review (without credentials), planned ordering and dry-run commands for the LAN stage.
- Prepare a rollout checklist, backup checklist, test queries and rollback path.

**Forbidden until verified cluster access and approval**
- `kubectl apply`, `helm upgrade/install/uninstall`, `flux reconcile`, destructive Terraform apply, scale-down or replacement of Cassandra, database/PVC deletion, credential rotation, production schema changes against live DB.
- Unapproved GitOps publication. For this execution the user has explicitly authorised main-branch CI publication; publish only after offline checks, with Worldbuilder flags disabled. Never equate artifact publication with cluster verification.
- Any claim that pods, volumes, database connections, inference, ingress or external integrations are working in the cluster.

## Remote verification checklist

| Check | Remote result to record |
|---|---|
| Repo inventory and impact map | Verified paths, symbols, services, dependencies |
| Build/test/lint for Go/TS/UI | Commands, pass/fail/skip, logs |
| Migration tests against disposable PostgreSQL | Commands and schema version, or blocked reason |
| Helm chart lint and rendered manifests | Environment overlays checked, warnings, saved diffs |
| Kustomize/Flux object validation | All relevant dev/stable/release overlays, or unavailable tooling |
| Secret references, env vars, ports, storage claims, probes | Checked against existing values and policies |
| Database backup/restore plan | Named steps and responsible storage location, **not yet tested on cluster** |
| Rollback checklist | Compatible versions, database recovery and GitOps revert path |
| GitOps auto-sync safety | Confirm that feature branch/PR will not roll to live workloads |

Document actual status as `LOCAL_VERIFIED`, `NOT_RUN`, `BLOCKED_CLUSTER_ACCESS`, or `FAILED`. No `DEPLOYED` status for offline work.

## Phase L — LAN / cluster validation (when access returns)

This is a **separate human-authorised execution gate**, not an automatic continuation of remote work. Before any mutating operations:

1. Confirm VPN/LAN, kube-context, namespace, cluster identity, RBAC and active GitOps source branch. Run read-only `kubectl cluster-info`, `kubectl get nodes`, `kubectl get ns`, `flux get all -A` (if available), `kubectl get helmreleases -A` and storage/backup inspection. Record differences from assumptions.
2. Confirm backup and restore capability for Cassandra (if needed) and persistent resources. Verify free capacity, StorageClass, PVC/PV binding expectations, PostgreSQL operator or Helm chart availability, secrets and network access. Verify zero unexpected pending/destroy operations.
3. Diff proposed Helm/Flux manifests against live resources. Check image registry availability and whether new tags exist and are signed as required. Review blast radius, monitoring and rollback.
4. Deploy PostgreSQL in a **nonproduction environment first**, preferably dev namespace/data namespace according to existing conventions. Verify storage mount, DNS, NetworkPolicy, credentials, application connections, migrations and restart persistence. Do not remove Cassandra.
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
