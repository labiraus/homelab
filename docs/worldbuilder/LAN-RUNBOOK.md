# Worldbuilder — controlled LAN validation and activation

**No live checks or mutations were executed during implementation.** The user subsequently authorised CI publication from `main`. The default-disabled code can be delivered through existing CI/Flux before LAN return; schema installation, secret provisioning, feature activation and data cutover remain separate controlled steps. Do not treat `dev` as an isolated environment: both `main` and `dev` publish the watched `uat` OCI series.

## 1. Read-only discovery when LAN access returns

Run these in order, record the context and results in IMPLEMENTATION-STATUS.md, and stop on unexpected identity/environment. Commands below do not alter cluster resources.

```bash
kubectl config current-context
kubectl cluster-info
kubectl get nodes -o wide
kubectl get namespaces
flux get all -A
kubectl get ocirepositories.source.toolkit.fluxcd.io -A
kubectl get helmreleases.helm.toolkit.fluxcd.io -A
kubectl -n flux-system get helmrelease external orchestrator processor ui postgres -o yaml
kubectl -n flux-system get ocirepository external orchestrator processor ui postgres -o yaml
kubectl -n data get cluster app-db -o yaml
kubectl -n data get scheduledbackup,backup
kubectl get storageclass
kubectl get pvc -A
kubectl get pv
kubectl -n homelab get deployment,serviceaccount,networkpolicy
kubectl -n homelab get peerauthentication,authorizationpolicy
kubectl -n homelab get deployment homelab-oauth2-proxy -o jsonpath='{.spec.template.spec.serviceAccountName}'
kubectl get statefulset -A
kubectl get deployments -A
kubectl -n nats get pods,services
kubectl -n data get pods -l cnpg.io/cluster=app-db
kubectl -n homelab logs deployment/homelab-external --tail=100
kubectl -n homelab logs deployment/homelab-orchestrator --tail=100
kubectl -n homelab logs deployment/homelab-processor --tail=100
```

Discover possible Cassandra outside this checkout: inspect stateful workloads/PVCs, related image names, Flux sources and known external hosts. List secret **names/metadata only**, never dump secret values into logs. If no Cassandra is found, record the evidence and inventory boundary; do not fabricate an export or delete unused-looking volumes. If found, inventory keyspaces/tables/clients/TTL/tombstones and owners, take consistent snapshots plus schema exports, checksum/count a repeatable transform/import, restore separately, and require a decommission-specific approval before any retirement.

Check Flux source revision/image tags against the CI run and signed image/chart artifacts. CI uses existing signing/publication actions; confirm signatures and registry access rather than assuming they succeeded. Do not run `make vllm-gateway-smoke` yet: it creates a temporary pod and belongs to the authorised test phase, not read-only discovery.

## 2. Backup and isolation gate — before mutation

Require `ContinuousArchiving=True`, recent successful CNPG base backup plus continuous WAL coverage, healthy MinIO private-network writes, bound PVCs and capacity. Existing `SlowDownWrite` history means a configured backup is not proof of recoverability. Restore CNPG into an isolated test environment first using the installed operator version's recovery process; do not overwrite `app-db`.

Use the repo-native local `psql` path. `make refresh-postgres-env` refreshes ignored local settings; `make postgres` opens the temporary data/app-db-rw tunnel for an interactive read-only inventory. Inspect `current_user`, schema owners, role memberships and `has_schema_privilege` there. Do not run new SQL through the shared legacy app login. Provision an independent migration owner and nonprivileged NOINHERIT logins `worldbuilder_external` and `worldbuilder_relay` with operator-managed passwords. Processor/MCP/shared app credentials must not inherit these roles. Keep production credentials out of terminal arguments/history and generated docs.

Before production, run the offline harness again and validate the same changes on an explicitly isolated database accessible through local psql. The schema migration has no destructive legacy operations and no CNPG bootstrap/PVC changes.

## 3. Separately authorised staged changes

These commands are **prepared, not executed**. Use a private `PGPASSFILE` and verified local psql connection variables populated from the approved operator workflow. `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER` below must describe the intended isolated validation database first, and the independent migration owner. Do not source or paste credentials into this document.

```bash
# Capture a restore-capable logical backup to ignored local storage; also require healthy CNPG WAL/base backups.
mkdir -p backups
pg_dump -Fc -f backups/before-worldbuilder.dump
# Explicit migration; no application/Helm hook runs this.
psql -X -v ON_ERROR_STOP=1 -f sql/worldbuilder/001_up.sql
psql -X -v ON_ERROR_STOP=1 -f sql/worldbuilder/grants.sql
psql -X -v ON_ERROR_STOP=1 -c 'TABLE worldbuilder.schema_version;'
psql -X -v ON_ERROR_STOP=1 -c "SELECT has_schema_privilege('app','worldbuilder','USAGE');"
# Expect false for legacy app. Inspect individual role grants and attempt denial probes in the isolated DB.
```

Create `homelab/worldbuilder-external` containing only `WORLDBUILDER_DATABASE_URL` for the dedicated login, and `homelab/worldbuilder-relay` containing only `WORLDBUILDER_RELAY_DATABASE_URL`. Use existing secret-generation/operator conventions with private local files. Optionally create `homelab/worldbuilder-ai` with `WORLDBUILDER_AI_TOKEN`, shared only by external and processor. No secret value is included in source.

Prepare reviewed Flux values in `helm/bootstrap/flux-apps/values.yaml` (and verify the GHCR overlay does not override them), preserving existing values:

```yaml
external:
  values:
    worldbuilder:
      enabled: true
      databaseSecret: worldbuilder-external
      # Substitute the OBSERVED mesh principal; this example is not verified live.
      browserPrincipal: cluster.local/ns/homelab/sa/homelab-oauth2-proxy
      aiSecret: "" # Enable only after deterministic canon acceptance.
orchestrator:
  values:
    worldbuilder:
      enabled: true
      databaseSecret: worldbuilder-relay
      openSearchURL: http://labiraus-rag-data.data.svc.cluster.local:9200
      rebuildOnStart: false
```

Review the rendered diff before publishing. First ensure at least two recognised authenticated identities exist through the approved `auth.users` operator workflow; world membership alone does not register a login identity. Activate external only and prove world creation, owner membership, editor draft, independent review, atomic publication, history and cross-world denial. Verify the mesh DENY policy blocks processor/MCP direct world calls and spoofed headers; observe proxy sanitisation. Then activate relay/indexing and verify no pending backlog, durable retries, consumer acknowledgement, duplicate/out-of-order event handling and fresh-index rebuilding. Keep PostgreSQL canonical.

Publish each approved configuration stage through the existing pipeline, not ad hoc Helm upgrades. Once LAN validation and the stage's approval are recorded:

```bash
python3 scripts/worldbuilder/check-manifests.py
# Review the exact staged files/diff before each stage.
git diff --check
git diff -- helm/bootstrap/flux-apps/values.yaml
# Commit only the reviewed stage, then trigger existing CI/Flux.
git add helm/bootstrap/flux-apps/values.yaml
git commit -m "Enable validated Worldbuilder stage"
git push origin main
# Observe reconciliation; no forced reconciliation is required.
flux get helmreleases -A
kubectl -n homelab rollout status deployment/homelab-external --timeout=180s
kubectl -n homelab rollout status deployment/homelab-orchestrator --timeout=180s
```

For AI, next set `external.values.worldbuilder.aiSecret` and `processor.values.worldbuilder.aiSecret` to the verified shared secret name, publish that separately reviewed stage, and validate `AI_GATEWAY_BASE_URL` and model JSON capability. Run authorised `make vllm-gateway-smoke`, grounded unknown/citation/adversarial tests, usage limits and privacy checks. Test session reload/restart, concurrent turns, provisional-state isolation and story-outcome review/publish/wiki flow. Confirm actual backup restore and outbox/OpenSearch recovery with fixture worlds before broader use.

Read-only progress SQL through local psql:

```sql
SELECT version, applied_at FROM worldbuilder.schema_version;
SELECT id, name, revision FROM worldbuilder.world ORDER BY created_at;
SELECT count(*) AS pending, min(created_at) AS oldest FROM worldbuilder.outbox WHERE delivered_at IS NULL;
SELECT status, count(*) FROM worldbuilder.change_set GROUP BY status;
SELECT action, count(*) FROM worldbuilder.audit GROUP BY action;
```

## 4. Reversibility and stop conditions

- If identity/header isolation, schema grants, backup health or storage differ from the plan, stop before activation.
- App rollback: disable the Worldbuilder flags through reviewed Flux values and publish that change; keep schema/data/secrets for recovery. Prior app versions ignore the additive schema.
- Canon rollback: create a compensating proposal from a historical snapshot, obtain a different reviewer and publish a new revision. Never UPDATE/DELETE revision history.
- Projection rollback: restore the previous image if needed; recreate only the derived test index under approval and rebuild from PostgreSQL. Never use search as recovery authority.
- Database restore: restore to a new isolated database, compare revisions/audit/story/turn counts and checksums, then make an explicitly approved connection cutover. Do not run `001_down.sql` on populated production canon; it intentionally refuses.
- Cassandra/manifests/PVs/credentials are untouched. Removal requires a separate inventory/export/recovery/reconciliation approval even after Worldbuilder acceptance.

No irreversible cleanup, live schema change or resource deletion is bundled with source publication.
