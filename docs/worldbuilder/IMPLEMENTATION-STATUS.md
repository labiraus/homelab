# Worldbuilder implementation status

Execution started 2026-10-10. **Initial slice verified locally; further offline increments in progress; main-branch CI publication subsequently authorised; no live cluster observations.**

## Verified inventory (M0)

Only `/workspaces/homelab` is available; no separate `engine`/`infra` checkout is present. This monorepo contains both application and infrastructure code. No Cassandra client, schema, credentials or chart was found by case-insensitive source search outside the target docs. This is not evidence of absence from the live estate. No BFF, OpenAPI generator or application Kustomize overlays were found.

| Component | Verified source / entry point | Gap to target |
|---|---|---|
| Public API | `apps/external/main.go`, `db.go`, `assistant.go`, `search.go` | Document routes and processor proxy; canon domain absent at audit |
| MCP | `apps/mcp/main.go`, `documents.go`, `external_proxy.go`, `transport.go` | Existing document tools; no world tools; preserve compatibility |
| AI/worker | `apps/processor/src/index.ts`, `assistant_api.ts`, `assistant_llm.ts` | Node HTTP (not Fastify); LangGraph assistant chat and NATS worker; no canon grounding |
| Orchestrator | `apps/orchestrator/main.go`, `db.go`, `documents.go`, `scan.go` | Document reconciliation and JetStream publishing; no transactional canon outbox |
| UI | `apps/ui/src/App.jsx`, `main.jsx`, `vite.config.js` | React/Vite document, inventory, search, assistant views |
| PostgreSQL | `apps/pkg/postgresutil/postgres.go` (`Begin`, `GetPool`); `sql/auth.pgsql`, `sql/rag/schema.pgsql`, `sql/assistant/schema.pgsql` | pgx already present; dedicated canon schema/roles required |
| Retrieval | `apps/processor/src/opensearch.ts`, `apps/pkg/opensearchrag/opensearchrag.go` | Rebuildable document index; no world/timeline/revision projection |
| Messaging | `apps/pkg/documentevents/events.go`, `apps/processor/src/config.ts` | `documents.ingest`, stream `documents`; `documents.events.>`, stream `document-events` |
| Inference | `helm/infra/vllm/templates/aigateway.yaml`, `helm/infra/envoy-ai-gateway/`, `docs/EnvoyAIGateway.md` | Gateway configured in source; model/embeddings compatibility needs LAN observation |
| Platform | `components/{kubernetes,minecraft-vm}/`, `helm/{apps,bootstrap,data,infra}/`, `ansible/` | Preserve hardware/network/auth; no Terraform change needed |
| Recovery | `helm/data/postgres/templates/{cluster,scheduledbackup}.yaml`, `docs/RetentionAndRecovery.md` | Existing CNPG app-db, Longhorn 10Gi, external MinIO WAL/base backups; restore health unverified |

## GitOps safety

`.github/workflows/app-*.yml` and `helm-all.yml` publish on `main` and `dev`. Flux bootstrap charts create OCIRepository/HelmRelease resources watching `uat` semver tags at one-minute intervals. Both branches are deployment triggers, not isolated environments. Initially created `feat/worldbuilder-offline-foundation` under the no-push instruction. The user then explicitly requested main-branch CI deployment, superseding that instruction for source delivery. Switched to `main`; planned publication keeps Worldbuilder runtime flags disabled. No direct cluster mutations, production SQL, feature activation or Cassandra removal are authorised. Git credential-path dry-run push succeeded; GitHub CLI has no login. Public GitHub Actions API is readable for delivery observation.

Pre-existing edits were preserved; by the main-branch switch they had been included in the existing `84adfb9` base commit, not in our migration changes: `.devcontainer/Dockerfile`, `.devcontainer/devcontainer.json`, `AGENTS.md`, `README.md`, `docs/Setup.md`, `runbooks/troubleshooting.md`, untracked `.devcontainer/.dockerignore`, `.devcontainer/AGENTS.md`.

## Dependency-ordered implementation map

1. M0: reconcile product docs and preserve prior plan as history.
2. M1: additive versioned canon migration, dedicated runtime credentials, disposable migration/restore harness; reuse pgx and CNPG.
3. M2/M3: world membership, typed projections and immutable snapshots; deterministic validation, separate human reviewer, atomic publication and transactional outbox.
4. M4: world/wiki/editor/review UI on existing external API.
5. M5: authorised revision-scoped evidence and tested LangGraph grounding; durable projection work after outbox foundation.
6. M6/M7: persistent pinned story sessions, atomic/idempotent turns, outcomes routed to ordinary review.
7. M8: prepare discovery/export/recovery and controlled rollout; no decommission offline.

## Milestone checkpoints

| Milestone | Source implementation and result | Remaining gate |
|---|---|---|
| M0 | LOCAL_VERIFIED: audited actual monorepo, doc dispositions, archive and active plan reconciliation | Live estate inventory remains unknown |
| M1 | LOCAL_VERIFIED: dedicated schema/roles, explicit migrations, existing pgx/CNPG reuse, disposable up/up/down/up and restore tests | BLOCKED_CLUSTER_ACCESS: independent migration owner, credentials, storage, CNPG recovery |
| M2 | LOCAL_VERIFIED initial domain: world/member, typed entity schemas, facts/relationships/events/timelines/rules/sources, relational projections and immutable snapshots, fictional ticks and lexical wiki queries | Reviewed world settings, owner administration/archive/transfer and stable deleted-ID kinds implemented in schema v2; temporal cardinality/exclusivity/chronology exceptions implemented; optional timeline inheritance/calendar presentation remain |
| M3 | LOCAL_VERIFIED: independent approval, revalidation, per-world locks, atomic projection/revision/audit/outbox, stale revision/revoked review checks, retries and compensating revision | Explicit reconciliation, revision diffs and compensating restore implemented in the next increment; live multi-user/header/mesh trust validation and richer review policies remain |
| M4 | LOCAL_VERIFIED initial React Worlds tab: wiki/history, editor staging, before/after preview, membership, review/publish | History/diff paging, remaining list continuation and complete multi-field editing implemented; visual/multi-user acceptance on deployed host remains |
| M5 | LOCAL_VERIFIED: bounded LangGraph workflows, external-scoped evidence and resolvable citations, candidate-only outputs, transactional outbox relay, NATS duplicate handling, OpenSearch external-version indexing/rebuild | BLOCKED_CLUSTER_ACCESS: Envoy/model JSON compatibility and live quality; canon vector/embedding search, scoped MCP delegation and distributed cost quotas are not implemented |
| M6 | LOCAL_VERIFIED: persistent pinned sessions, private provisional typed state, atomic narration/turn/state, optimistic concurrency and idempotency; generated candidates explicitly saved | Rich story definitions/scenario authoring, persisted summaries and durable long-running generation jobs remain follow-ups |
| M7 | LOCAL_VERIFIED initial outcome -> draft -> review -> publish -> wiki path; advanced canon conflicts now have original/current/proposed comparison and explicit fresh-draft reconciliation | Optional graph/chronology visualisations remain follow-ups; projection end-to-end cluster observation pending |
| M8 | Prepared inventory/export, staged rollout, backup/recovery and compensating rollback runbook | BLOCKED_CLUSTER_ACCESS; no Cassandra removal or cutover claimed |

This is a tested incremental slice across M1–M7, **not completion of every acceptance criterion in M1–M8**. The unimplemented items above are explicit follow-ups rather than claims of deployed functionality.

## Changed components

- `apps/external/worldbuilder/` and `worldbuilder.go`: domain, database transaction engine, HTTP routes, reasoning boundary, story persistence and integration tests; `main.go` registers only when enabled.
- `sql/worldbuilder/`: schema versions 1–2, guarded empty rollback, dedicated grants and operator notes.
- `apps/orchestrator/worldbuilder.go`: optional relay/index consumer; `worldbuilder_test.go` covers retry, projection, rebuild and real disposable transport/index integration.
- `apps/processor/src/worldbuilder.ts`: bounded token-authenticated LangGraph endpoint and tests; no canonical DB access or model-selected tools.
- `apps/ui/src/Worldbuilder.jsx` and tests: world/wiki/editor/review/story/AI candidates within the existing auth shell.
- App Helm templates/values: disabled flags and private secret references, browser-principal DENY policy and optional indexing egress. No CNPG/PVC/Terraform changes.
- `.github/workflows/app-*.yml`: main tests gate publication, correct Go working directories, TS/UI builds, disposable database test in external CI; outdated action runtimes updated to setup-go/setup-node v6 and cache v5.
- `scripts/worldbuilder/`: reproducible local database, chart and Flux schema checks.
- README, agent guidance, app READMEs, RAG diagram, setup/recovery docs and `.codex` plan/map reconciled; old plan archived. See DOCS-AUDIT.md.

## Initial-slice verification log — 2026-10-10

Local tools: Go 1.27.2, Node 24.21.0, Helm 4.3.0. CI keeps declared Go 1.25.6; all five application CI test gates, including the external PostgreSQL integration harness, have passed on their configured runners. All PostgreSQL/NATS/OpenSearch writes below targeted newly created disposable Docker services, never the homelab cluster. Go integration tests normally skip without explicit disposable DSNs; the database harness runs them with real PostgreSQL.

Use `GOCACHE=/tmp/worldbuilder-go-cache GOMODCACHE=/tmp/worldbuilder-go-mod GOPATH=/tmp/worldbuilder-gopath` in the restricted workspace.

| Exact command/context | Result |
|---|---|
| `(cd apps/external && go test -race ./...)` with cache variables | LOCAL_VERIFIED; root package passes, domain unit tests pass; explicit DB integration is separately run below |
| `(cd apps/orchestrator && go test -race ./...)` | LOCAL_VERIFIED; normal unit tests pass; explicit service tests separately run below |
| `(cd apps/mcp && go test -race ./...)` | LOCAL_VERIFIED; race suite passed |
| `for module in apps/pkg/*/go.mod; do (cd "${module%/go.mod}" && go test ./...); done` with cache variables | LOCAL_VERIFIED; all 14 shared modules compiled/tested (modules without tests reported as such), no cluster endpoints supplied |
| `scripts/worldbuilder/test-postgres.sh`; final Docker-outside-Docker repeat with `WORLDBUILDER_TEST_PORT=25434` | LOCAL_VERIFIED: PostgreSQL 16.13 up/up/down/up; denied legacy schema access/relay canon writes; publication/history/world isolation/temporal/chronology/story/outcome/concurrency/failure/compensation tests; outbox retry; populated-down refusal; pg_dump/pg_restore revision checksum match. Dedicated external runtime-role publication, immutable-history denial and authoritative AI scope/base-revision regression also passed |
| `(cd apps/processor && npm ci --cache /tmp/worldbuilder-npm-cache --ignore-scripts && npm run build && npm test)` | LOCAL_VERIFIED compile + 30 tests (six new Worldbuilder cases); npm advisories remain |
| `(cd apps/ui && npm ci --cache /tmp/worldbuilder-npm-cache-ui --ignore-scripts && npm run build && npm test)` | LOCAL_VERIFIED production build + 22 tests (three new Worldbuilder cases); npm advisories remain |
| `python3 scripts/worldbuilder/check-manifests.py` | LOCAL_VERIFIED: 26 base/GHCR/ECR combinations across five app charts, postgres, flux-apps/data/infra, vllm and Envoy AI Gateway; Helm lint/template plus local `kubectl kustomize` builds; explicit enabled renders and missing-principal refusal |
| `flux install --export > /tmp/worldbuilder-flux-schemas.yaml` then `/tmp/worldbuilder-python/bin/python scripts/worldbuilder/validate-flux.py /tmp/worldbuilder-flux-schemas.yaml /tmp/worldbuilder-manifests-tf796e6g` | LOCAL_VERIFIED: 76 rendered Flux resources validated against installed Flux CRD schemas. Other custom-resource schemas (CNPG/Istio/AI Gateway) are not covered by this check |
| `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7 -shellcheck= .github/workflows/app-{external,orchestrator,mcp,processor,ui}.yml` with cache variables | LOCAL_VERIFIED after updating obsolete action runtimes; shellcheck explicitly disabled |
| `git diff --check` and relative Markdown link scan of changed product docs | LOCAL_VERIFIED, no whitespace errors/broken product links |
| `GIT_TERMINAL_PROMPT=0 git push --dry-run origin main` | LOCAL_VERIFIED credential path, no remote changes from dry run |

Disposable live transport/index test:

```bash
# Services created only for this local test, with no persistent host mounts:
docker run -d --name worldbuilder-offline-postgres -e POSTGRES_PASSWORD=offline-test-only -e POSTGRES_DB=worldbuilder_test -p 127.0.0.1:25432:5432 postgres:16.13
docker exec -i worldbuilder-offline-postgres psql -U postgres -d worldbuilder_test -v ON_ERROR_STOP=1 < sql/worldbuilder/001_up.sql
docker run -d --name worldbuilder-offline-nats -p 127.0.0.1:24222:4222 nats:2.11.9 -js
docker run -d --name worldbuilder-offline-search --memory=2g -e discovery.type=single-node -e DISABLE_SECURITY_PLUGIN=true -e DISABLE_INSTALL_DEMO_CONFIG=true -e OPENSEARCH_JAVA_OPTS='-Xms512m -Xmx512m' -p 127.0.0.1:29200:9200 opensearchproject/opensearch:3.7.0
# From apps/orchestrator, with the cache variables above:
WORLDBUILDER_TEST_DATABASE_URL='postgres://postgres:offline-test-only@host.docker.internal:25432/worldbuilder_test?sslmode=disable' \
WORLDBUILDER_TEST_NATS_URL='nats://host.docker.internal:24222' \
WORLDBUILDER_TEST_OPENSEARCH_URL='http://host.docker.internal:29200' \
go test -race -count=1 -v -run TestCanonLiveTransportAndIndex .
```

**LOCAL_VERIFIED**: real JetStream acknowledgement/dedup, durable pull/ack, OpenSearch index creation/projection, older event rejection preserving revision 2, and snapshot rebuild passed. One-node disposable services do not prove three-node cluster quorum, security configuration or cluster networking. Disposable containers are removed after validation.

Local artifacts: `/tmp/worldbuilder-postgres-final.log`, `/tmp/worldbuilder-postgres-roles.log`, `/tmp/worldbuilder-postgres-checked.log`, `/tmp/worldbuilder-shared-go.log`, `/tmp/worldbuilder-manifests-tf796e6g/`. These are local diagnostics, not committed artifacts and may disappear after the session; durable outcomes are recorded here. Render checks use missing-secret placeholders only offline.

## Unresolved issues and activation gates

**BLOCKED_CLUSTER_ACCESS**: live proxy/header/mTLS trust, dedicated migration-owner/runtime privileges, production migration, CNPG storage/WAL/base backup/restore health, external MinIO reachability, registry/signature observation, Envoy model support/quality, live NATS/OpenSearch topology, rollout readiness and network policies. Follow the exact read-only sequence and separately authorised stages in [LAN-RUNBOOK.md](LAN-RUNBOOK.md).

**NOT_RUN**: browser automation against real oauth2-proxy, real-model adversarial/grounding evaluation, full non-Flux CRD schema validation, production-scale load/latency/cost tests. Initial UI and context caps, per-world serialisation and snapshot rewrites are documented in [API.md](API.md).

**Known retained gaps**: npm advisories (latest check: processor 4 moderate; UI 0; see [dependency validation](DEPENDENCIES.md)), legacy RAGAS harness still reads old pgvector rows, unfinished `scripts/pg-update.sh` remains unused, no Cassandra estate inventory outside this checkout. Dependency upgrades are explicitly recorded below; no destructive data cleanup was performed.

## Initial-slice publication record

Source commit `422f3c3` was pushed to `main`; all five app pipelines and Helm discovery started. Processor CI `38061475297` completed successfully and published through the existing artifact pipeline. Helm discovery `38061475275` succeeded; separate Helm app publishing was correctly skipped because changed app source already triggers the paired image/chart pipelines.

External CI `38061475231` passed Go unit tests but its database step failed with exit 126: the script was stored as mode 100644 despite local executable access on the Windows-backed workspace. The `needs: test` gate prevented external publication. Corrective commit `0219456` sets tracked executable mode and explicit Bash invocation. Follow-up [external run 38061652923](https://github.com/labiraus/homelab/actions/runs/38061652923) completed successfully: Go/unit tests, disposable PostgreSQL migration/integrity/recovery checks, image build/signing/push and paired Helm chart publication all passed.

All five application delivery pipelines are now successful. External published from corrective commit `0219456`; the four unchanged applications published from source commit `422f3c3`:

- [Processor 38061475297](https://github.com/labiraus/homelab/actions/runs/38061475297)
- [UI 38061475254](https://github.com/labiraus/homelab/actions/runs/38061475254)
- [Orchestrator 38061475364](https://github.com/labiraus/homelab/actions/runs/38061475364)
- [MCP 38061475285](https://github.com/labiraus/homelab/actions/runs/38061475285)

The repository has `core.filemode=false`; Git executable mode is now explicit. No disposable Worldbuilder containers remain.

Feature activation remains off by default. No live cluster deployment/ready state has been observed.

Initial-slice delivery checkpoint: both source and CI correction commits are on origin/main. This documentation-only handover update does not trigger app/chart publication. Working-tree cleanliness is checked after its commit. No cluster context was contacted; actual Flux reconciliation, pod readiness and Worldbuilder activation remain **BLOCKED_CLUSTER_ACCESS**.

## Increment 2 — history, reconciliation and compensation

Implemented source: `apps/external/worldbuilder/history.go` plus HTTP routes and real-database tests; `apps/ui/src/WorldHistory.jsx` and component tests; manual draft revision retention in `Worldbuilder.jsx`. No migration/config/manifest changes; schema version 1 and disabled activation flags remain compatible.

- Membership-scoped revision metadata and paginated complete before/after diffs.
- Proposal and private story three-way comparison, explicit conflict choices, fresh deterministic validation, fresh independent approval and unchanged story pin.
- Audited lineage and exact retry binding; compensating restore produces ordinary reviewed revisions.
- UI history/diff paging, reconciliation and restore drafts. Manual staged drafts preserve the original revision and before-state across refresh.

Validation completed locally for this increment (2026-10-10):

- `GOCACHE=/tmp/worldbuilder-go-cache GOMODCACHE=/tmp/worldbuilder-go-mod GOPATH=/tmp/worldbuilder-gopath go test -race ./...` from `apps/external`: PASS.
- `WORLDBUILDER_TEST_PORT=25434 GOPATH=/tmp/worldbuilder-gopath bash scripts/worldbuilder/test-postgres.sh`: PASS migration repeat/rollback, role isolation, all external domain/integration tests (four new database history/reconciliation scenarios plus classification unit test), orchestrator outbox tests, populated-down refusal and restore checksum. Log: `/tmp/worldbuilder-history-final.log`; disposable container removed.
- `npm run build && npm test` from `apps/ui`: PASS production build and 26 tests, including three history/reconciliation components and manual draft revision regression.

No manifest or configuration changed, so the previously passed render/schema checks remain applicable. Existing CI success above applies only to the initial slice until this increment's publication is observed. Remaining offline requirements remain active; see [acceptance matrix](ACCEPTANCE-MATRIX.md).

## Increment 3 — complete structured editing and list navigation

Added `WorldRecordEditor.jsx`: existing-record prefill preserving full fields/relations/attributes, multiple schema predicates and relationship declarations, self-referencing types, typed fact values and life-bound rule inputs. Existing ID/kind stay fixed; duplicate field names and lossy fictional integers are rejected. Editing retains the record's inspected revision. Canon `kind` filtering is validated before pagination. Reference catalogs load independently of wiki search at one pinned revision.

Added load-more controls for worlds, canon, proposals, sessions and turns. Canon paging retains inspected revision/applied filters; links to records outside the loaded page resolve through the authenticated scoped API. Mutable lists deduplicate by ID, with refresh needed for concurrent insertions. No schema, runtime configuration or Helm changes.

Checks (2026-10-10): `npm run build && npm test` from `apps/ui` PASS with 32 tests, including five structured-editor cases and pinned paging/catalog regression. `WORLDBUILDER_TEST_PORT=25434 GOPATH=/tmp/worldbuilder-gopath bash scripts/worldbuilder/test-postgres.sh` PASS, including kind-filter pagination/isolation and all publication/story/recovery tests; log `/tmp/worldbuilder-editor-postgres.log`. Disposable services removed.

Increment 2 publication: commit `4eea01b` is on origin/main. [UI run 38062683593](https://github.com/labiraus/homelab/actions/runs/38062683593) succeeded. [External run 38062683575](https://github.com/labiraus/homelab/actions/runs/38062683575) was still running at this checkpoint; no success assumed. Increment 3 has passed local checks and is ready for source publication. A targeted repeat `npm test -- --run src/WorldRecordEditor.test.jsx` also passed all five cases after preserving valid empty attribute values.

## Increment 4 — governed settings, owner lifecycle and stable identity

Added explicit `002_up.sql`/`002_down.sql`; current external requires schema v2, with startup migration still prohibited. New world creation includes reviewed `world_settings`; subsequent name/default-timeline/settings updates use ordinary proposal/approval/publication and preserve historical snapshots. Version-1 worlds receive no synthetic history edits. Owner administration lists/revokes members, transfers ownership atomically, archives/reopens with expected administration version and exact audited retries. Archive blocks canon/story/inference writes while retaining reads. An immutable identity registry rejects a different record kind after deletion and is backfilled from every historical snapshot.

UI adds owner administration, archived-state handling, default-timeline opening and structured settings editing. No Helm values, credentials or production database changed; runtime remains disabled. Migration/rollback order and v1-binary incompatibility are recorded in the SQL notes and LAN runbook.

LOCAL_VERIFIED (2026-10-10): `(cd apps/external && go test -race ./...)` with the documented cache variables passed; `(cd apps/ui && npm run build && npm test)` passed production build and 33 tests. `WORLDBUILDER_TEST_PORT=25434 GOPATH=/tmp/worldbuilder-gopath bash scripts/worldbuilder/test-postgres.sh` passed empty migration reruns/down/up, populated v1 upgrade with deleted identity recovery, refusal of conflicting historical identity, role isolation, settings publication, archive/revocation/transfer/retry/concurrent administration checks and existing canon/story/outbox suites. The pg_dump/restore checksum now covers all 12 canon/lifecycle/story tables, not only revision snapshots. Current log: `/tmp/worldbuilder-lifecycle-verified.log`; disposable services removed. `bash -n scripts/worldbuilder/test-postgres.sh` and `git diff --check` passed. No manifests changed; earlier render results remain applicable.

Delivery observation: both [history external 38062683575](https://github.com/labiraus/homelab/actions/runs/38062683575) and [history UI 38062683593](https://github.com/labiraus/homelab/actions/runs/38062683593) succeeded for `4eea01b`. Both [editor external 38063044676](https://github.com/labiraus/homelab/actions/runs/38063044676) and [editor UI 38063044642](https://github.com/labiraus/homelab/actions/runs/38063044642) succeeded for `ea0ab1c`. No cluster readiness observation is implied.

## Increment 5 — dependency remediation and publication gates

Updated UI to Vite 7.3.7 / Vitest 4.1.11 and compatible lockfile fixes; retained processor MinIO 8 with compatible dependency updates. UI `npm run build && npm test` PASS (33 tests); processor same command PASS (30 tests). Final npm audit: UI zero; processor four moderate and zero high/critical. A forced SDK downgrade or incompatible ESM override was not applied. [Dependency validation](DEPENDENCIES.md) records commands, upstream advisory links and the remaining compatibility issue. Added high/critical audit gates to UI/processor CI after their build/tests, before publishing.

Lifecycle source is published as `c20ac2d`; [UI run 38063580450](https://github.com/labiraus/homelab/actions/runs/38063580450) succeeded. [External lifecycle run 38063580318](https://github.com/labiraus/homelab/actions/runs/38063580318) also succeeded, including the schema-v2 disposable upgrade/integrity/recovery checks and image/chart publication.

Increment-5 workflow checks: actionlint 1.7.7 passed for UI/processor workflows (shellcheck explicitly disabled); `npm audit --audit-level=high` passed in both apps; changed-document relative links and `git diff --check` passed.

## Increment 6 — deterministic cardinality, exclusivity and chronology exceptions

Added `rules.go` and domain tests. Versioned rules can constrain simultaneous relationship counts by declared predicate, subject type, timeline and fictional interval; boolean facts can be mutually exclusive. Life-bound rules accept explicit true canonical exception facts at the event time and optional type/timeline/interval scope. Temporal sweeps preserve half-open intervals, including signed 64-bit endpoints without overflow. Numeric birth/death values must be exact safe integers rather than silently rounded floats. The structured editor exposes these fields and preserves unrelated rule attributes.

Local checks: external `go test -race ./...` passed with documented caches; UI build and 34 tests passed. Disposable grouped rule/canon publication and recovery checks passed with `WORLDBUILDER_TEST_TRANSPORT=exec GOPATH=/tmp/worldbuilder-gopath bash scripts/worldbuilder/test-postgres.sh`. This includes all domain/integration tests, both migration lifecycle checks, role isolation and all-table restore checksums. No schema migration or deployment configuration changed.

The first two increment-6 database attempts failed before Go tests because Docker-outside-Docker host forwarding on ports 25434 and 25433 became unreachable, despite healthy disposable PostgreSQL containers. Added `postgres-exec-bridge.sh` and a loopback-only socat fallback using `docker exec` into the exact generated test container. Forced exec transport then passed the full suite; log `/tmp/worldbuilder-rules-bridge.log`. Both failed attempts cleaned up their containers; successful relay/container cleanup also completed. `bash -n` and `git diff --check` passed. This is local test transport, with no cluster access or production credentials.

Dependency commit `ff2744d` also completed publication successfully: [UI 38063961709](https://github.com/labiraus/homelab/actions/runs/38063961709) and [processor 38063961761](https://github.com/labiraus/homelab/actions/runs/38063961761), including the new high-severity audit gates.

## Increment 7 — scoped discovery and bounded grounded evidence

Added external `search.go` and `context.go`: membership-first OpenSearch candidate IDs, PostgreSQL resolution and complete bounded fallback, temporal visibility, pinned historical reads and stable canonical pagination. Large worlds now receive selected evidence with visible reference closures, mandatory policy/settings/story dependencies and explicit partial-coverage metadata. The processor prompt and UI preserve that limitation; citation and candidate validation remain mandatory. Wiki queries use the new scoped search route. No schema/Helm/config changes; existing external OpenSearch credentials/path are reused and activation remains off.

LOCAL_VERIFIED (2026-10-10): external `go test -race ./...` passed, including `TestDiscoveryAgainstDisposableOpenSearch` against a fresh OpenSearch 3.7.0 service through a loopback Docker-exec relay. The real engine accepted the nested query/source extraction and filtered foreign worlds, future revisions and other timelines. The test refuses to reuse an existing index and removes only its newly created index. `WORLDBUILDER_TEST_TRANSPORT=exec GOPATH=/tmp/worldbuilder-gopath bash scripts/worldbuilder/test-postgres.sh` passed the full database/recovery suite, including membership-before-discovery, expired facts, and selected-evidence reasoning over 120 background entities; log `/tmp/worldbuilder-search-postgres.log`. Processor build/31 tests and UI build/35 tests passed, including partial-coverage and scoped-search regressions. `git diff --check` passed.

Index candidate hints do not yet accelerate the complete canonical scan; vector retrieval and quality evaluation remain separate work. Partial evidence cannot establish global absence or full consistency. Required policy groups exceeding context fail explicitly. Rules increment `9e17a7b` has successful [external CI 38064535801](https://github.com/labiraus/homelab/actions/runs/38064535801) and [UI CI 38064536100](https://github.com/labiraus/homelab/actions/runs/38064536100). No cluster state was inspected.
