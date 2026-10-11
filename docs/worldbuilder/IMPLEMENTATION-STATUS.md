# Worldbuilder implementation status

## Increment 20 — reviewed calendars and full-range date facts

World settings now accept a bounded repeating calendar: custom months, intercalary/leap additions, up to 1000-year patterns, weekdays, exact epoch/year and subday ticks. The structured editor proposes these through ordinary independent review. Canon/search return the requested snapshot calendar even if settings are filtered out; wiki, relationship intervals and chronology display civil dates beside original ticks using BigInt. Staged settings and current metadata cannot change historical presentation. New `tick` facts use canonical signed int64 strings; life-bound checks support the full range and retain legacy safe-number behavior.

Explicit additive migration 006 extends the fact constraint and calendar projection; current external requires schema 6. History, numeric serialization and retry identity remain unchanged. No production migration or activation runs automatically. The offline harness now waits for the final PostgreSQL TCP server instead of racing its temporary initdb socket server.

LOCAL_VERIFIED (2026-10-11): UI build and 56 tests, external full race suite, processor build/53 passing tests (actual-storage test separately CI-gated). Chromium/Go/PostgreSQL acceptance passes reviewed calendar publication, filtered metadata, historical/timeline scope and staged-calendar exclusion, plus existing date/navigation/private-story flows and all 16-table restore checks (`/tmp/homelab-calendar-browser-final.log`). Screenshot inspected. A search fixture initially matched calendar field names; it now selects the event's exact end tick. The final populated upgrade run preserves numeric record/history checksums, refuses unexpected fact-constraint drift atomically and passes all migrations/roles/restore checks (`/tmp/homelab-calendar-upgrade-final.log`). A cross-app audit also found the generation worker still required schema 5; it now requires 6, with a real-database startup probe that stops before broker activation. The full orchestrator race suite, updated migration/worker/restore harness (`/tmp/homelab-calendar-worker-final.log`) and workflow lint pass; orchestrator CI now requires that harness and responds to schema changes. CI publication evidence follows observation.

## Increment 19 — locked dependencies in published Node images

Correction `d5b197c` passed [UI CI/CD 38133862925](https://github.com/labiraus/homelab/actions/runs/38133862925), including complete-image smoke and chart publication. The same corrected smoke script passed locally against the previously published image with a cold pull. Together with processor CI below, published-image dependency parity is now verified for this checkpoint.

[Processor CI/CD 38133562255](https://github.com/labiraus/homelab/actions/runs/38133562255) passed all tests, actual-client storage acceptance, complete-image dependency/runtime smoke checks and chart publication. [UI CI 38133562050](https://github.com/labiraus/homelab/actions/runs/38133562050) passed unit/browser/database acceptance and built the image, then exposed a smoke-harness bug: Docker image-pull progress from stderr was concatenated into the stdout container ID. The harness now preserves separate streams; UI chart publication remains blocked until its corrected check passes.

UI and both processor Docker stages now copy the committed lockfile and use `npm ci`; processor runtime dependencies omit development packages. Docker contexts exclude local dependencies/output/environment files. Hosted release jobs now test the actual images before publishing their charts: processor checks the source lockfile hash, installed package versions/dependency health, compiled imports and exact-date support; UI starts Nginx and serves its index/compiled assets over its own loopback. Both runtime checks disable container networking.

Workflow lint and smoke-script syntax checks pass. Complete local builds were attempted but the Docker worker's Node runtime rejects npm's certificate issuer (`UNABLE_TO_GET_ISSUER_CERT_LOCALLY`), independently reproduced with a direct registry fetch. Processor npm terminated with `Exit handler never called`; the UI retrying build was explicitly cancelled after diagnosing the same trust boundary. No TLS bypass or private workstation CA was added to production images. Hosted CI supplies the required complete-image validation; source/runtime publication evidence is pending observation.

## Increment 18 — scoped relationship graph and chronology

`WorldViews.jsx` adds interactive directed relationship exploration and exact-tick event chronology to the wiki. Views consume only loaded published records at submitted revision/timeline/time/search scope, display partial coverage, and extend with existing pinned pagination. Entity focus and the complete loaded relationship table complement a bounded, labelled SVG diagram; mouse/keyboard links resolve through the same scoped canon navigation. Chronology uses signed BigInt ordering and ASCII ID ties, with explicit half-open endpoints and source/participant links. Staged edits, provisional stories and the editor catalog remain separate.

LOCAL_VERIFIED (2026-10-11): production UI build and 50 component tests pass, followed by focused rechecks for legacy decimal-filter syntax. The real Chromium/Go/disposable PostgreSQL harness passes graph mouse/keyboard navigation, extreme dates, staged-candidate exclusion, historical revision reads, half-open end filtering (including a leading-zero query) and alternate-timeline isolation, plus existing publication/private-story and migration/restore checks (`/tmp/homelab-canon-views-browser-final.log`). A real browser failure on thin SVG paths led to larger hit areas and visible relationship labels; rerun passed without forced clicks. The fixture view screenshot was inspected. Publication evidence follows CI observation; calendar configuration and full-range life-bound fact semantics remain open.

Source `e140275` is published on main. [UI CI/CD 38132984126](https://github.com/labiraus/homelab/actions/runs/38132984126) and [external CI/CD 38132984108](https://github.com/labiraus/homelab/actions/runs/38132984108) both passed all tests and image/chart publication. This closes the scoped graph/chronology item in the offline audit; calendar functionality remains separate.

## Increment 17 — exact signed fictional dates

Record dates now accept equivalent numeric tokens and decimal strings over signed int64 without changing numeric response/history serialization or canonical idempotency. UI and processor parse original date tokens; unsafe values stay strings in editor state, prompts and candidates. MCP preserves numeric tokens before delegated signing and in structured replies, and advertises numeric/string `at` input. Browser date fields use decimal text instead of floating-point number controls. Unknown fields, invalid/overflow dates and unsupported browser source-context support fail visibly.

LOCAL_VERIFIED (2026-10-11): UI build and 45 tests; processor build and 52 existing/new unit tests plus the added real HTTP date test; external and MCP Go suites. Disposable PostgreSQL migration/role/recovery acceptance passed with full-range date publication, numeric/string retry identity, half-open filters, deletion and historical reads. The real production UI/Go/PostgreSQL browser harness passed extreme-date entry, publication, reloading and editing alongside the existing independent-review/private-story acceptance (`/tmp/homelab-exact-ticks-browser.log`). No schema migration, deployment activation or LAN operation was performed. CI publication evidence follows observation.

Calendar configuration/presentation and full-range life-bound fact semantics remain open in [the offline audit](OFFLINE-WORK.md); numeric fact semantics are unchanged by this transport correction.

Follow-up audit found the orchestrator's derived index text also passed through `float64`. It now retains raw JSON field tokens, with a full-range projection regression and the complete orchestrator Go suite passing. This keeps date search terms exact while preserving canonical reconciliation; it changes neither index schema nor authority. Core source `a1e7c92` is published; application CI and projection publication are being observed.

Wiki navigation audit also identified that links to unloaded records omitted the submitted `at` filter. They now preserve exact fictional time with the displayed revision/timeline, independently of unsaved filter inputs; component acceptance covers the minimum signed tick and unsaved edits to all three controls. UI build and all 46 component tests pass.

Core publication `a1e7c92` passed [MCP CI/CD 38131962137](https://github.com/labiraus/homelab/actions/runs/38131962137), [UI CI/CD 38131962242](https://github.com/labiraus/homelab/actions/runs/38131962242), [processor CI/CD 38131962494](https://github.com/labiraus/homelab/actions/runs/38131962494) and [external CI/CD 38131962227](https://github.com/labiraus/homelab/actions/runs/38131962227), including the browser/database and actual-client storage gates. Projection `fa754b8` passed [orchestrator CI/CD 38132038681](https://github.com/labiraus/homelab/actions/runs/38132038681), and navigation `8b3f25a` passed [UI CI/CD 38132256070](https://github.com/labiraus/homelab/actions/runs/38132256070). All corresponding image/chart publications succeeded. Cluster rollout remains unobserved.

Execution started 2026-10-10. **The initial remote baseline was verified through increment 15. The continuing goal includes the broader [offline work audit](OFFLINE-WORK.md), which remains open. Tested main-branch publication is authorised; no live cluster observations. Deployment activation and M8 cutover remain LAN-gated.**

Public read-only probing on 2026-10-10 reached DNS/TLS but was blocked by Zscaler with a hostname-mismatched certificate and HTTP 403 diagnostic page. No verified application response or cluster state was obtained; see [public validation evidence and remaining checks](PUBLIC-VALIDATION.md).

Subsequent user-authorised CI diagnosis added automatic verified public probes and fixed MCP builds advertising `local` by embedding the published image version. MCP CI `38082111065` successfully published image/chart `0.0.0-uat.38082111065`. Independent hosted probes, including its post-publication check, each failed ten times with TLS hostname mismatch (code 62) at `87.117.64.111`; no deployed version or application response was observable. Seven checker tests, MCP Go tests and affected workflow lint pass. Flux state remains unknown because no usable cluster observation connection is configured; continuation needs verified ingress routing or read-only cluster status, not an assumed stopped reconciliation.

Continued CI diagnosis on 2026-10-11 (`38125438853`, source `027921b`) passed eight checker tests and independently captured the origin's Let's Encrypt/YE2 certificate for unrelated `bprj.co.uk` hostnames; HTTP-01 port-80 probing timed out. This establishes a public hostname/certificate mismatch beyond local interception. The configured SSH control plane is LAN-only, and the user confirmed LAN connectivity remains unavailable. Full evidence and the unresolved DNS/NAT versus installed-gateway distinction are in [public validation](PUBLIC-VALIDATION.md).

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

`.github/workflows/app-*.yml` and `helm-all.yml` publish on `main` and `dev`. Flux bootstrap charts create OCIRepository/HelmRelease resources watching `uat` semver tags at one-minute intervals. Both branches are deployment triggers, not isolated environments. Initially created `feat/worldbuilder-offline-foundation` under the no-push instruction. The user then explicitly requested main-branch CI deployment, superseding that instruction for source delivery. Switched to `main`; publication keeps Worldbuilder runtime flags disabled. No direct cluster mutations, production SQL, feature activation or Cassandra removal are authorised. Git credential-path dry-run push succeeded; GitHub CLI has no login. Public GitHub Actions API is readable for delivery observation.

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
| M2 | LOCAL_VERIFIED initial domain: world/member, typed entity schemas, facts/relationships/events/timelines/rules/sources, relational projections and immutable snapshots, fictional ticks and lexical wiki queries | Reviewed world settings, owner administration/archive/transfer and stable deleted-ID kinds implemented in schema v2; temporal cardinality/exclusivity/chronology exceptions implemented; reviewed calendar/date facts are implemented in schema 6; optional timeline inheritance remains |
| M3 | LOCAL_VERIFIED: independent approval, revalidation, per-world locks, atomic projection/revision/audit/outbox, stale revision/revoked review checks, retries and compensating revision | Explicit reconciliation, revision diffs and compensating restore implemented in the next increment; structured advisory findings and visible independent-review policy are implemented; live identity/header/mesh validation remains |
| M4 | LOCAL_VERIFIED initial React Worlds tab: wiki/history, editor staging, before/after preview, membership, review/publish | History/diff paging, remaining list continuation and complete multi-field editing implemented; real Chromium editor/reviewer and private-story flows pass against disposable PostgreSQL; acceptance on the deployed host remains |
| M5 | LOCAL_VERIFIED: bounded LangGraph workflows, external-scoped evidence and resolvable citations, candidate-only outputs, transactional outbox relay, NATS duplicate handling, OpenSearch external-version indexing/rebuild | BLOCKED_CLUSTER_ACCESS: Envoy/model JSON compatibility and live quality; canon vector/embedding search and provider billing verification remain; scoped MCP delegation is implemented in increment 8 and durable per-actor allowances in increment 9 |
| M6 | LOCAL_VERIFIED: persistent pinned sessions, private provisional typed state, atomic narration/turn/state, optimistic concurrency and idempotency; generated candidates explicitly saved | Private definitions/scenario authoring and persisted bounded summary/choices are implemented in increment 10; durable generation jobs are implemented in increment 11; live recovery remains gated |
| M7 | LOCAL_VERIFIED initial outcome -> draft -> review -> publish -> wiki path; advanced canon conflicts now have original/current/proposed comparison and explicit fresh-draft reconciliation | Scoped graph/chronology and reviewed calendar presentation pass offline/browser acceptance; projection end-to-end cluster observation pending |
| M8 | Prepared inventory/export, staged rollout, backup/recovery and compensating rollback runbook | BLOCKED_CLUSTER_ACCESS; no Cassandra removal or cutover claimed |

The required source migration and feasible remote checks are implemented across M1–M8. **Live acceptance, activation and cutover are not complete.** The acceptance matrix separates these LAN gates from optional visualisations and upstream dependency follow-ups. Historical logs below retain the counts and limitations observed at each checkpoint.

## Changed components

- `apps/external/worldbuilder/` and `worldbuilder.go`: domain, database transaction engine, HTTP routes, reasoning boundary, story persistence and integration tests; `main.go` registers only when enabled.
- `sql/worldbuilder/`: schema versions 1–5, guarded empty rollback, dedicated grants and operator notes.
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

## Increment 8 — authenticated MCP reads and draft delegation

Added `pkg/api/delegation.go`, external `worldbuilder/tools.go` and MCP `worldbuilder.go`: short-lived request-bound assertions, active identity/current membership checks, five opt-in tools, and only reads/validated drafts. Owner identity cannot expand the operation allowlist. No database/schema change. New default-disabled external/MCP chart inputs inject only a dedicated key secret and require exact separate mesh principals; MCP protects both current and legacy transports. Existing browser-only publication policy is retained. Shared identity tests now gate both publishing workflows.

LOCAL_VERIFIED (2026-10-10): `go test -race ./...` in `apps/pkg/api`, `apps/external` and `apps/mcp` passed with the documented cache variables. Tests bind audience/path/method/body/lifetime, reject raw identity context and tampering before database access, exercise MCP tools/call dispatch, prove current membership revocation and active-user checks, and deny human operations even for owners. The full disposable PostgreSQL harness passed again with forced exec transport, including exact draft retry and unchanged canon; `/tmp/worldbuilder-mcp-postgres.log`. Existing MCP transport compatibility tests also passed.

`python3 scripts/worldbuilder/check-manifests.py` passed 26 base/overlay lint/template/Kustomize combinations, ordinary activation gates and delegation activation/missing/wildcard principal checks. Initial negative testing caught Helm `required` accepting an empty list; explicit `empty` rejection fixed it and is recorded in Helm guidance. Final render directory: `/tmp/worldbuilder-manifests-jfx_lb93`. Actionlint 1.7.7 passed external/MCP workflows with shellcheck explicitly disabled. No live identities, secrets or policies were changed.

Search increment `a22ccea` completed [external CI 38065157506](https://github.com/labiraus/homelab/actions/runs/38065157506), [UI CI 38065157467](https://github.com/labiraus/homelab/actions/runs/38065157467) and [processor CI 38065157461](https://github.com/labiraus/homelab/actions/runs/38065157461) successfully. Delegation remains disabled pending LAN authentication/mesh/clock validation.

Increment-8 schema/doc checks: 76 rendered Flux objects passed local installed-CRD validation; other custom resources remain outside that validator. Worldbuilder document relative links passed. `git diff --check` passed after preserving the MCP manifest’s existing mixed-line-ending convention on the changed line.

## Increment 9 — durable inference allowances and provenance

Added explicit migration 003 and restricted ledger grants; current external now requires schema v3. A separate two-connection pool commits per-actor reservations under PostgreSQL advisory locking before provider work, retaining allocations through failures/cancellations/rollbacks. UTC daily request/amount and active-lease limits span replicas and worlds. Expired leases release concurrency only. Runtime cannot rewrite allocation/identity/time or delete attempts. The API/audit/logs carry trusted request correlation, configured gateway route, measured latency, model/usage and allocated allowance; UI generation details distinguish this from an invoice. Processor forwards correlation and preserves a gateway-reported model. Actual provider identity/tariffs remain LAN gates. AI activation now refuses absent monetary allowance configuration.

Offline validation (2026-10-10): external race suite passed; full disposable schema 001/002/003 repeat/empty rollback, populated upgrade, runtime role, domain/story/outbox and 13-table dump/restore suite passed. New tests cover simultaneous reservations through separate instances, fresh-instance daily enforcement, failed inference surviving canon rollback, expired leases without refunds, currency mismatch and denied allocation rewrites. Final database log `/tmp/worldbuilder-budget-postgres-final.log`. The first database attempt caught a stale test assertion expecting schema 2; updated to 3 and reran the complete suite successfully. UI production build and 35 tests passed, including visible allowance/provenance details.

All 26 chart/overlay checks passed, plus AI missing-allowance refusal and explicitly configured test allowance render; artifacts `/tmp/worldbuilder-manifests-oongnjjm`. All 76 Flux objects passed installed-schema validation; other custom-resource schemas remain outside that validator. Shell syntax and whitespace checks passed. No production migration or configuration was applied. The ledger stores hashes rather than prompt/evidence content and does not automatically retry uncertain provider work; durable story jobs remain upcoming work.

MCP increment `6564820` has successful [external 38071175883](https://github.com/labiraus/homelab/actions/runs/38071175883), [MCP 38071175878](https://github.com/labiraus/homelab/actions/runs/38071175878), [orchestrator 38071175862](https://github.com/labiraus/homelab/actions/runs/38071175862) and [Helm 38071175847](https://github.com/labiraus/homelab/actions/runs/38071175847) publication runs. Runtime remains default-disabled.

Increment-9 final processor check: production TypeScript build and all 32 tests passed, including actual gateway-adapter correlation forwarding/reported-model selection through a mocked HTTP response. The new fixture initially needed explicit strict TypeScript parameter types; corrected before the passing run. Worldbuilder relative links and final whitespace checks passed.

## Increment 10 — private story definitions, bounded memory and scenario authoring

Migration 004 adds immutable owner-private definitions, nullable definition links for sessions and atomic turn/session memory, preserving all existing narration/state/pins. Current external requires schema 4. Definitions validate visible pinned cast and exact retry content; reusable setup remains private creative guidance. Turns persist narration, state, summary/coverage and choices in one version-checked transaction, including original-memory replay after later turns. Story generation carries only the newest three bounded turns plus summary/definition/provisional state and checks expectedVersion before inference. Returned sessionVersion is server-owned and retained through human save. Narrative generation returns prose with optional choices, no operations; UI can use it as a private premise while retaining its original canon pin.

LOCAL_VERIFIED (2026-10-10): external `go test -race ./...` passed. Full disposable database suite passed with all four migration reruns/empty down/up, populated legacy revision/session/turn upgrade, role denials, private definition/pin/immutability tests, bounded context/stale-generation rejection, original-memory retry after later turns, and 14-table dump/restore checksums. Final log `/tmp/worldbuilder-story-memory-upgrade.log`. A first new test compared JSONB response bytes and failed on key ordering; corrected it to compare semantic content, then reran successfully. Existing history and provisional-state invariants remain covered.

Processor build and 34 tests passed, including private-memory injection/citation boundaries, summary/choice bounds, definition scope and narrative mutation rejection. UI build and 37 tests passed, including immutable definition pin reuse and stable definition/session IDs across uncertain retries. Two targeted setup tests passed again after narrowing retry identity to the actual selected definition. No manifests changed; increment-9 render/schema checks remain applicable. Legacy clients must send expectedVersion for story generation; incompatible old binaries stay disabled against the newer schema. No production migration ran.

Budget increment `4a3f778` completed [external 38071739472](https://github.com/labiraus/homelab/actions/runs/38071739472), [processor 38071739497](https://github.com/labiraus/homelab/actions/runs/38071739497), [UI 38071739503](https://github.com/labiraus/homelab/actions/runs/38071739503) and [Helm 38071739774](https://github.com/labiraus/homelab/actions/runs/38071739774) successfully. Durable jobs, review findings, evaluation/browser fixtures and observability remain offline work; actual model quality/security/storage validation remains gated on LAN access.

## Increment 11 — durable private creative generation

Added explicit migration 005 (current runtime schema 5), private job/checkpoint storage, transactional metadata-only dispatch outbox and opaque per-part inference attribution. External owns saved human intent, identity/membership/version checks, bounded evidence, allowances and validated candidate checkpoints. Orchestrator uses its existing relay-only role and a separate fixed worker endpoint/token; it receives no private request or AI/canon credentials. Narrative jobs contain 1–4 parts, story jobs one version-checked candidate. Default-off chart inputs add narrowly scoped orchestrator-to-external network paths and exact mesh-principal denial rules. UI queues/retries, refreshes, cancels remaining work and inspects private partial results without changing original pins.

Expired claims resume only when the ledger proves their next part never reserved inference; otherwise they stop uncertain. Completed parts and allocations survive failures/cancellation. Error handling reads actual checkpoint state before broker acknowledgement, covering a successful database commit with a lost acknowledgement. Seven-day broker retention requires controlled recovery of lost dispatch; no automatic replay of possibly billed provider work. World transaction serialisation during bounded inference remains a load/cancellation-latency limitation.

LOCAL_VERIFIED (2026-10-10): external and orchestrator `go test -race ./...` passed with documented cache variables. `WORLDBUILDER_TEST_NATS_URL=nats://127.0.0.1:39975 WORLDBUILDER_TEST_TRANSPORT=exec GOPATH=/tmp/worldbuilder-gopath bash scripts/worldbuilder/test-postgres.sh` passed migration 001–005 reruns/empty rollback/populated upgrade/refusal, role isolation, atomic queue rollback, private access, stale story/revoked membership, duplicate workers, fresh-store checkpoints, ledger-based recovery, ambiguous acknowledgement state and all 16-table restore checksums. A disposable NATS 2.11.9 stream passed actual publish dedup, delayed redelivery and terminal acknowledgement; `/tmp/worldbuilder-jobs-final-postgres.log`. Single-node transport does not establish deployed three-replica quorum.

UI build/39 tests and processor build/35 tests passed, including retry IDs/original scope/partial candidate inspection and adversarial prior-prose/checkpoint validation. `python3 scripts/worldbuilder/check-manifests.py` passed all 26 base/overlay combinations and job missing-secret/AI/principal/parent-flag refusal plus positive activation renders; `/tmp/worldbuilder-manifests-8853dibk`. All 76 Flux objects passed local installed-CRD validation; other CRD coverage remains separate work. No cluster mutation, production migration, credential installation or feature activation occurred.

Story increment `0963cfe` completed successful [external CI 38072460994](https://github.com/labiraus/homelab/actions/runs/38072460994), [processor CI 38072460998](https://github.com/labiraus/homelab/actions/runs/38072460998) and [UI CI 38072461015](https://github.com/labiraus/homelab/actions/runs/38072461015). This is publication evidence, not observed cluster readiness.

## Increment 12 — structured advisory consistency and review

Added validated advisory findings and server-owned deterministic check metadata. Consistency validates the whole stored snapshot first; optional proposal context reuses current-base/default-timeline/immutable-identity/domain checks before inference allocation. Proposed operations remain untrusted context and cannot become citations. Findings preserve severity, uncertainty and resolvable published evidence in the immutable AI audit. Review UI displays the independent-human policy, proposal check results and partial-evidence limitations. No schema/config/manifest changes; AI remains gated.

LOCAL_VERIFIED (2026-10-10): UI production build and 40 tests passed; processor build and 36 tests passed. Disposable PostgreSQL harness passed all schema-5 migration, role, domain, review/story/jobs and 16-table restore checks (`/tmp/worldbuilder-findings-postgres.log`), including stale/unknown proposal refusal before inference and unchanged draft status after advisory checks. Unit cases reject forged evidence, invented certainty/severity, workflow escape and model approval fields. Existing job transport and manifest validation from increment 11 remain applicable.

Generation increment `02f030b` is published on origin/main; CI outcome is pending observation at this checkpoint. Publication remains distinct from LAN activation/readiness.

## Increment 13 — aligned evaluation adapters and synthetic quality cases

Default RAGAS document evaluation now calls authenticated external search, preserving the deployed OpenSearch/filter/citation path. It bounds responses, validates citation index/version, refuses redirects and never manufactures identity headers. The Make target no longer fetches CNPG credentials or starts a port-forward; historical pgvector comparison is retained under explicit `--backend legacy-postgres`. Only explicitly requested env files load. Active evaluation/RAG docs are reconciled.

Added a synthetic bridge-keeper world with eight grounded/unknown/injection/foreign-citation/semantic-ambiguity/narrative-boundary cases, run through the real processor graph/output validator. LOCAL_VERIFIED: processor build and 44 tests passed; standard-library local HTTP adapter test passed authentication/filter/version/redirect boundaries; Python compilation and `make -n ragas-chunking-eval` passed. Actionlint 1.7.7 passed the new read-only evaluation workflow with shellcheck disabled. No live RAGAS score, model quality or private-source evaluation is claimed; these remain LAN gates documented in `evals/worldbuilder/README.md`.

Generation `02f030b` completed successful [external 38073770008](https://github.com/labiraus/homelab/actions/runs/38073770008), [orchestrator 38073770012](https://github.com/labiraus/homelab/actions/runs/38073770012), [UI 38073770016](https://github.com/labiraus/homelab/actions/runs/38073770016), [processor 38073770019](https://github.com/labiraus/homelab/actions/runs/38073770019) and [Helm 38073770065](https://github.com/labiraus/homelab/actions/runs/38073770065) CI. Advisory increment `3904d6f` is published on main; its CI outcome is not yet observed here. No cluster readiness is inferred.

## Increment 14 — private operational signals and recovery dashboard

Worldbuilder browser/tool/worker HTTP boundaries now use the existing external Prometheus registry with fixed method/outcome labels and no private identifiers. Added an importable Grafana source dashboard and read-only aggregate SQL diagnostics for publication/dispatch backlog, generation/lease state, inference allowance/latency, governance and story counts. Existing bounded histograms retain their finite one-second ceiling; mean duration is useful, high inference percentiles require separate LAN measurement. Worker HTTP success is explicitly distinct from generated-job success.

LOCAL_VERIFIED (2026-10-10): external `go test -race ./...` passed including preserved HTTP response/status and absence of private metric labels. Full disposable PostgreSQL schema-5 harness and 16-table restore passed with the aggregate queries executed in a read-only transaction (`/tmp/worldbuilder-observability-postgres.log`). Grafana JSON parses locally. No dashboard, scrape target or alert was installed; actual signal visibility/thresholds/load and production recovery remain LAN gates. Evaluation increment `23b1285` is published on main with its CI outcome still pending observation here.

## Increment 15 — real browser acceptance, complete custom schemas and pinned traversal

Added pinned Playwright 1.64.0 and a test-only loopback server serving the production UI build plus actual canon HTTP handler against disposable PostgreSQL. Separate editor/reviewer browser contexts stage a typed entity, submit, independently approve/publish, find it in wiki search, explore connections, and create/refresh/resume a private pinned story that the other user cannot read. Fixture requests are restricted to their loopback origin. UI CI now runs this acceptance before publication; production binaries contain no test identity route.

Added authorised forward/back typed-reference traversal with pagination and revision/timeline/fictional-time preservation in API/UI. Added a 5,000-record domain benchmark, reproducible public CRD exporter and fail-closed custom-resource validator. No schema migration or runtime feature flag changed; current schema remains 5.

LOCAL_VERIFIED (2026-10-10):

- UI production build and 41 component tests passed; npm audit reports zero vulnerabilities after the pinned browser dependency.
- Full database/browser harness passed with `WORLDBUILDER_BROWSER_TEST=1`, forced disposable exec transport and documented Go caches. This includes the real Chromium flow, connection authorisation/temporal tests, all prior domain/job/recovery cases, read-only operator diagnostics and 16-table restore (`/tmp/worldbuilder-release-postgres.log`). Local Chromium used temporary extracted Debian libraries/fonts via `LD_LIBRARY_PATH=/tmp/worldbuilder-browser-libs/usr/lib/x86_64-linux-gnu`, `FONTCONFIG_FILE=/tmp/worldbuilder-browser-libs/fonts.conf` and `PLAYWRIGHT_BROWSERS_PATH=/tmp/worldbuilder-browsers`. Initial failures were missing libraries/fonts and an exact-label selector on a native select; using its accessible combobox role fixed the selector. Disposable services cleaned up.
- External `go test -race ./...` passed. `BenchmarkBoundedCanon5000` ran ten cycles at 8,151,627 ns/op, 9,992,274 B/op and 30,058 allocations/op on the local Intel Core Ultra 7 255H environment. This is not database/provider/LAN capacity evidence.
- All 144 rendered custom resources, including feature-enabled variants, passed explicit CRD structural schemas: Envoy Gateway v1.7.0, AI Gateway v1.0.0, CNPG chart 0.23.2, KEDA chart 2.16.1, Istio comparison 1.30.5 and Flux CLI 2.9.6. Reproducible export `/tmp/worldbuilder-crds-repro` includes checksums; renders `/tmp/worldbuilder-manifests-8853dibk`. Missing/invalid schema regression test passed. Built-in schemas, admission/CEL and actual installed versions are explicitly outside this local validator.
- Evaluation adapter test, Python compilation, shell syntax, actionlint 1.7.7 (shellcheck disabled), dashboard JSON, relative documentation links and whitespace checks passed. Scripted processor suite remains 44 passing tests from increment 13.

Prior publication evidence is complete: advisory `3904d6f` passed [external 38073936587](https://github.com/labiraus/homelab/actions/runs/38073936587), [UI 38073936551](https://github.com/labiraus/homelab/actions/runs/38073936551) and [processor 38073936504](https://github.com/labiraus/homelab/actions/runs/38073936504). Evaluation `23b1285` passed [adapter 38074137139](https://github.com/labiraus/homelab/actions/runs/38074137139) and [processor 38074137240](https://github.com/labiraus/homelab/actions/runs/38074137240). Observability `6795b51` passed [external 38074262674](https://github.com/labiraus/homelab/actions/runs/38074262674).

Required remote implementation and feasible offline acceptance are complete. Remaining gates are real production migrations/grants/backups, authenticated users and mesh/network identities, provider semantics/pricing/embeddings, observed registry/Flux/pod readiness, representative load/recovery, and external Cassandra inventory/cutover. Optional graph/calendar visualisations and the four moderate incompatible MinIO transitive findings remain documented follow-ups. No cluster mutation, production schema/secret change, activation or Cassandra removal occurred.

## Remote handover — publication verified

Implementation checkpoint `cf48b66` is published on `origin/main`. All relevant workflows completed successfully: [external tests/image/chart 38074975298](https://github.com/labiraus/homelab/actions/runs/38074975298), [UI component/browser/database tests and image/chart 38074975295](https://github.com/labiraus/homelab/actions/runs/38074975295), [evaluation/schema adapter checks 38074975296](https://github.com/labiraus/homelab/actions/runs/38074975296), and [Helm workflow 38074975312](https://github.com/labiraus/homelab/actions/runs/38074975312). The final documentation-only checkpoint records this evidence and reconciles the original execution prompt with the user's explicit main-publication override.

All migration changes are on main; temporary database/NATS/OpenSearch services were removed and the pre-existing unrelated container was retained. No cluster mutations or live readiness observations were performed. Source/artifact publication is complete; Worldbuilder flags remain disabled. Begin with the ordered read-only commands in [LAN-RUNBOOK.md](LAN-RUNBOOK.md), then execute its separately controlled backup, migrations 001–005/grants, identity and staged activation gates. Preserve populated data on rollback. No Cassandra removal is authorised by its absence from this checkout.

## Increment 16 — processor storage dependency remediation and broader offline audit

The continuing user goal covers every feasible offline follow-up. [OFFLINE-WORK.md](OFFLINE-WORK.md) records remaining visualisation/calendar, governance/timeline, migration and recovery work; prior baseline verification does not close those items.

Replaced the processor's read-only MinIO Node adapter with exact AWS SDK S3 3.1149.0. Public configuration/interface/error contracts and private endpoints remain; endpoint schemes must agree with the TLS flag. Added signed HTTP fixtures for credential/region/path scope, encoded keys, binary/empty reads, S3 errors, redirect refusal and incomplete streams. Processor publication now rejects moderate or worse dependency findings and still requires actual-client storage acceptance.

LOCAL_VERIFIED (2026-10-11): processor build and 50 unit/HTTP-contract tests passed; fresh processor and UI audits report zero vulnerabilities. The full disposable SeaweedFS harness passed migration/recovery, Go reads/reverts and the actual new Node adapter with no integration skips (`/tmp/homelab-aws-sdk-storage.log`). No production storage endpoint, schema, credentials or Worldbuilder activation changed. CI publication outcome is recorded after observation.

Source `cf28887` is published on main. [Processor CI/CD 38131145032](https://github.com/labiraus/homelab/actions/runs/38131145032) passed the processor tests/audit, required disposable storage job and image/chart publication. This closes the tracked dependency finding, not the broader offline audit or live readiness gates. Subsequent audit confirmed signed-date precision loss through ordinary JavaScript JSON parsing; exact date transport is the next correctness item before calendar/chronology presentation.
