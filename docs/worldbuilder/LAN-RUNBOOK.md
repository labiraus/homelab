# Worldbuilder — controlled LAN validation and activation

The initial implementation was offline. The user subsequently authorised CI publication from `main` and continued LAN recovery; live checks, restore rehearsal and staged progress are recorded in [implementation status](IMPLEMENTATION-STATUS.md). The default-disabled code can be delivered through existing CI/Flux before LAN return; schema installation, secret provisioning, feature activation and data cutover remain separate controlled steps. Do not treat `dev` as an isolated environment: both `main` and `dev` publish the watched `uat` OCI series.

## 1. Read-only discovery when LAN access returns

The [2026-10-10 public probe](PUBLIC-VALIDATION.md) was blocked by Zscaler with a certificate hostname mismatch. Validate normal public HTTPS, DNS/NAT routing and the installed Gateway certificate before using public application credentials; the probe did not establish application or cluster health.

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

These commands describe the controlled migration procedure; consult implementation status for the environment in which each stage has been verified. Use a private `PGPASSFILE` and verified local psql connection variables populated from the approved operator workflow. `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER` below must describe the intended isolated validation database first, and the independent migration owner. Do not source or paste credentials into this document.

```bash
# Capture a restore-capable logical backup to ignored local storage; also require healthy CNPG WAL/base backups.
mkdir -p backups
pg_dump -Fc -f backups/before-worldbuilder.dump
# Explicit migration; no application/Helm hook runs this.
psql -X -v ON_ERROR_STOP=1 -f sql/worldbuilder/001_up.sql
psql -X -v ON_ERROR_STOP=1 -f sql/worldbuilder/002_up.sql
psql -X -v ON_ERROR_STOP=1 -f sql/worldbuilder/003_up.sql
psql -X -v ON_ERROR_STOP=1 -f sql/worldbuilder/004_up.sql
psql -X -v ON_ERROR_STOP=1 -f sql/worldbuilder/005_up.sql
psql -X -v ON_ERROR_STOP=1 -f sql/worldbuilder/006_up.sql
psql -X -v ON_ERROR_STOP=1 -f sql/worldbuilder/007_up.sql
psql -X -v ON_ERROR_STOP=1 -f sql/worldbuilder/008_up.sql
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

Review the rendered diff before publishing. The operator's requested superuser exception permits their default single-vote workflow; ordinary independent-review acceptance needs two recognised authenticated identities. World membership alone does not register a login identity. Activate external only and prove world creation, owner membership, draft, permitted review, atomic publication, history and cross-world denial. Verify the mesh DENY policy blocks processor/MCP direct world calls and spoofed headers; observe proxy sanitisation. Then activate relay/indexing and verify no pending backlog, durable retries, consumer acknowledgement, duplicate/out-of-order event handling and fresh-index rebuilding. Keep PostgreSQL canonical. Compare scoped search results/citations at current and historical revisions with PostgreSQL; test index lag, outage and rebuild without losing canonical results. Confirm partial-evidence counts are displayed on a world above 100 records, and measure search latency under the existing 5,000-record cap.

Optional MCP activation is a separate stage after canon acceptance. Observe the actual MCP service-account principal and every authenticating edge principal for both Streamable HTTP and legacy SSE, including certificate and OAuth identity/header sanitisation. Verify callers cannot inject `X-Forwarded-Client-Cert`, `X-Auth-Request-Email`, `X-Forwarded-Email` or basic-auth usernames through either edge. Require active `auth.users` identity and current world membership. Keep the existing browser principal distinct from MCP; processor remains excluded from both canon interfaces.

Provision an operator-generated cryptographically random key (at least 32 bytes) as `WORLDBUILDER_MCP_KEY` in a secret mounted only by external/MCP. Do not reuse AI, OAuth or database secrets. Prepare these Flux values using observed principals, not guessed names:

```yaml
external:
  values:
    worldbuilder:
      mcp:
        enabled: true
        delegationSecret: worldbuilder-mcp
        principal: "<observed exact MCP service-account principal>"
mcp:
  values:
    worldbuilder:
      enabled: true
      delegationSecret: worldbuilder-mcp
      trustedEdgePrincipals:
        - "<observed authenticating edge principal>"
```

Render the merged configuration and inspect both DENY policies before source publication. Exercise tools/list and one scoped read on both transports; confirm inactive/nonmember actors fail, draft retries return the same draft, canon remains unchanged, and owner assertions cannot invoke review/publication/administration/private sessions. Confirm unsigned, modified, expired and wrong-audience assertions fail, direct `/api/worlds` access from MCP/processor is denied, and clocks differ by no more than the five-second future allowance. Disabling both delegation flags and rotating the key revokes access without changing canon or document tooling. These are prepared LAN tests, not observed results.

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

For AI, first verify provider/model pricing, maximum bounded input/output charges and the actual gateway route. Set explicit `external.values.worldbuilder.aiBudget` fields: `dailyRequests`, `concurrent`, positive `requestMicros`/`dailyMicros`, uppercase three-letter `currency` and a meaningful `gatewayRoute` label. Derive a conservative per-attempt allowance from observed route limits/tariffs; do not treat test fixture amounts or allocated ledger amounts as provider invoices. Confirm multiple external replicas share daily/concurrency limits, failed calls remain allocated, and abandoned leases cease blocking after 60 seconds without refunding that day. Then set `external.values.worldbuilder.aiSecret` and `processor.values.worldbuilder.aiSecret` to the verified shared secret name, publish that separately reviewed stage, and validate `AI_GATEWAY_BASE_URL` and model JSON capability. Run authorised `make vllm-gateway-smoke`, grounded unknown/citation/adversarial tests, usage limits and privacy checks. Test session reload/restart, concurrent turns, provisional-state isolation and story-outcome review/publish/wiki flow. Confirm actual backup restore and outbox/OpenSearch recovery with fixture worlds before broader use.

Run the small live semantic gate with synthetic data after building processor:

```bash
npm run build --prefix apps/processor
# Set a verified private gateway URL (or its temporary loopback port-forward) and the observed model.
AI_GATEWAY_BASE_URL=http://127.0.0.1:18087/v1 AI_CHAT_MODEL='<observed model>' \
  node scripts/worldbuilder/check-live-ai.mjs
```

This exercises the actual application prompt, response schema and validators against known/unknown facts, hostile evidence and explicit historical/timeline pins. It is a limited live quality gate, not broad correctness or capacity proof. Unlike offline fixtures, it invokes the configured model; never aim it at an unreviewed paid route. Preserve per-case latency/model/usage evidence and keep AI disabled if an unknown fact is invented.

Read-only progress SQL through local psql:

```sql
SELECT version, applied_at FROM worldbuilder.schema_version;
SELECT id, name, revision FROM worldbuilder.world ORDER BY created_at;
SELECT count(*) AS pending, min(created_at) AS oldest FROM worldbuilder.outbox WHERE delivered_at IS NULL;
SELECT status, count(*) FROM worldbuilder.change_set GROUP BY status;
SELECT action, count(*) FROM worldbuilder.audit GROUP BY action;
SELECT status, currency, count(*), sum(allocated_micros) AS allocated_micros
FROM worldbuilder.inference_attempt GROUP BY status, currency;
SELECT count(*) AS unresolved_expired_attempts FROM worldbuilder.inference_attempt
WHERE status='reserved' AND lease_until<now();
```

The external chart rejects missing or wildcard browser principals, including wildcard service accounts and trust domains. Run `python3 scripts/worldbuilder/check-manifests.py` before publication; a successful exact-principal render still requires the live identity and denial checks above.

## 4. Reversibility and stop conditions

- If identity/header isolation, schema grants, backup health or storage differ from the plan, stop before activation.
- App rollback: disable the Worldbuilder flags through reviewed Flux values and publish that change; keep schema/data/secrets for recovery. Older enabled binaries reject newer schema versions; keep their feature flag off.
- Canon rollback: create a compensating proposal from a historical snapshot, satisfy the current review policy (including the explicit superuser exception) and publish a new revision. Never UPDATE/DELETE revision history.
- Projection rollback: restore the previous image if needed; recreate only the derived test index under approval and rebuild from PostgreSQL. Never use search as recovery authority.
- Database restore: restore to a new isolated database, compare revisions/audit/story/turn counts and checksums, then make an explicitly approved connection cutover. Do not run `001_down.sql` on populated production canon; it intentionally refuses.
- Cassandra/manifests/PVs/credentials are untouched. Removal requires a separate inventory/export/recovery/reconciliation approval even after Worldbuilder acceptance.

No irreversible cleanup, live schema change or resource deletion is bundled with source publication.

## Schema-v4 compatibility and lifecycle validation

Before enabling current external, verify max schema version 8 and dedicated grants including `record_identity` and restricted inference-ledger writes. Migration 002 must refuse historical same-ID/different-kind conflicts; inventory them and obtain a reviewed reconciliation plan rather than changing immutable history. Validate reviewed name/default-timeline settings, membership removal, owner transfer (exactly one owner), archive write refusal and retained history. Repeat an administration request and confirm its original version/result without another change. Old v1 snapshots remain intact; introduce their first settings record through review.

Rollback keeps Worldbuilder disabled and data intact. Earlier binaries reject schema 8 when enabled; do not roll back to them with the feature enabled. All eight down migrations refuse populated worlds; 003 also refuses any inference attempts and 008 retains populated account authority/audit. Restore drills include all 19 Worldbuilder tables and the shared auth registry.

Validate private definition reuse at a historical pin after canon advances, owner isolation (including world administrators), saved summary/choice reload and exact turn retries after later turns. Check that generated candidates retain sessionVersion and stale generation is rejected before inference. Exercise narrative-to-premise creation and verify its original timeline/revision are preserved. Compare summaries against full stored turn history; summaries are provisional, not canonical evidence. Existing sessions must retain pre-migration narration/state with empty initial memory.

## Durable generation activation and recovery gate

Apply explicit migrations `005_up.sql`, `006_up.sql`, `007_up.sql` and `008_up.sql` after 001–004, refresh dedicated grants and verify max version 8 before current external activation. Back up and restore all 19 Worldbuilder tables plus the shared auth registry, including generation jobs/outbox and inference ledger; rollback disables flags and preserves populated data. Previous schema-4/5/6 binaries cannot run enabled against schema 8.

Keep `worldbuilder.jobs.enabled=false` on both charts until synchronous AI, identity and budget checks above pass. Provision one dedicated worker secret through the existing secret workflow (key `WORLDBUILDER_JOB_TOKEN`, at least 32 bytes); reference it as `worldbuilder.jobs.workerSecret` on external and orchestrator. Verify external `worldbuilder.jobs.principal` is the actual orchestrator mesh identity, distinct from browser and MCP. Inspect the conditional ingress/egress policies for orchestrator→external TCP 8080 and test forged/missing token, human browser principal and wrong service principal denial. The configurable orchestrator `worldbuilder.jobs.externalAPIURL` must be the private external service root. Activate these flags only as a separately reviewed main-branch values change.

Test a three-part narrative with a worker restart between saved parts, duplicate dispatch, cancellation, revoked member and stale story version. Verify one ledger attempt per executed part and no canonical publication. Kill external after reservation but before checkpoint commit: an expired claim must become `uncertain`, retain any earlier parts and require deliberate new-request action. Kill before reservation: the unstarted part may resume. Compare PostgreSQL job/checkpoint/ledger state with broker pending counts and UI refresh. Observe actual three-replica JetStream quorum and seven-day retention.

The [disposable three-node broker drill](BROKER-RECOVERY.md) provides offline leader/quorum/restart evidence. At activation, inspect the actual generation stream's subject, file storage, three replicas and seven-day limits retention, plus its durable consumer's three replicas. Startup refuses incompatible configuration; preserve pending work and obtain a backup before deliberate repair. The local process drill does not establish Longhorn or machine power-loss durability.

If dispatch was lost after broker retention or a broker restore, first stop the generation worker through reviewed configuration and inspect queued/running jobs, outbox IDs and per-part ledger records read-only. Preserve `uncertain` and terminal jobs. For a demonstrably unstarted queued job, an operator may separately approve replay of its existing metadata event with its original event ID, or reset only that event's delivered marker for relay. This is a controlled mutation, not an automatic recovery command; do not clear ledger reservations, reset completed steps or overwrite private requests. Re-enable the worker and observe convergence before further replay. Source CI cannot establish these LAN observations.

For read-only acceptance and incident baselines, follow [observability](OBSERVABILITY.md): verify existing external scrapes, inspect aggregate SQL diagnostics and review/import the dashboard only after datasource identity is known. Establish representative latency/error/backlog thresholds on LAN before enabling alerts. HTTP worker acknowledgements alone do not prove successful provider output; compare durable job/ledger state.

The initial remote baseline was verified through increment 15; the continuing [offline work audit](OFFLINE-WORK.md) tracks remaining source-only features and deeper recovery checks. The processor S3 adapter has removed the old MinIO transitive findings. Before activation, reproduce [offline acceptance](OFFLINE-ACCEPTANCE.md), compare exact installed CRD versions with the recorded schema snapshots, and retain the read-only discovery evidence above. The real-browser fixture does not replace validation of actual OAuth identities, forwarded-header sanitisation or mesh principals.

For current schema 8, refresh both canon and separate auth grants and include reviews, superuser grants and account audit in the full restore drill. Validate a two-reviewer proposal with real distinct human principals: one vote must not publish; removing/recreating or changing a reviewer’s role must require a fresh review. Lowering a policy must satisfy the previous higher count. Preserve old review/audit records and published approval identities. See [review-policy acceptance](REVIEW-POLICY.md).

## Superuser activation

The user explicitly requested superuser registration and self-approval. After rehearsing migrations 001–008 and restoring all 19 canon tables plus `auth.users`, apply `auth-grants.sql` separately as the existing auth owner/operator. Grant the verified registered operator account through a parameterised insert into `worldbuilder.superuser`; keep account-specific provisioning out of migrations. Verify ordinary-user denial, one-vote quorum behavior, self-approval provenance and revocation/regrant refusal. Superuser activation replaces the two-person requirement for that account's default single-vote workflow; ordinary independent-review acceptance remains separate. AI/MCP identities receive no new capability.

The independent migration owner needs CREATE on the intended database for the explicit migration session, even when the owned schema already exists (`CREATE SCHEMA IF NOT EXISTS` checks that privilege). Grant it as the database operator, run the migration session, and revoke database CREATE afterward. Do not give the migration owner the legacy app role.

Provisioning on CNPG uses its [declarative role manager](https://cloudnative-pg.io/docs/1.25/declarative_role_management/), keeping PostgreSQL superuser login disabled. The PostgreSQL chart has opt-in `worldbuilderRoles.enabled` and three dedicated basic-auth Secret references. All roles are NOINHERIT, non-superuser, without database/role creation or inherited memberships. `migrationLogin` opens only the independent owner login for the explicit migration session; close it after grants and verify new owner connections are refused. External and relay retain their separate scoped logins. Schema installation and account-class assignment are still explicit operator SQL, never startup hooks.

The LAN bootstrap enables the three CNPG-managed role identities after their private Secrets are provisioned. This does not install schema or enable applications. The owner remains NOLOGIN until the explicit migration session.
