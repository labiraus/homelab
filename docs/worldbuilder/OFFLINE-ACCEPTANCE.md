# Reproducible offline acceptance

Commands use disposable local services and public source artifacts. The API-server fixture installs CRDs and synthetic namespaces in temporary loopback etcd; workload manifests use strict server dry runs. Operator kubeconfig and SQL `.env` files are not loaded. Public tool/schema/browser downloads need internet access, not LAN credentials.

## Real browser and canon database

Build the UI, install its pinned Playwright Chromium, then invoke the database harness:

```bash
(cd apps/ui && npm ci && npm run build && npx playwright install --with-deps chromium)
WORLDBUILDER_BROWSER_TEST=1 bash scripts/worldbuilder/test-postgres.sh
```

The test-only Go server binds loopback, serves the production UI build and actual canon handler against disposable PostgreSQL, with two explicit fixture identities in separate browser contexts. The fixture auth route exists only in `_test.go`, never a deployed binary. Chromium drives typed drafting, independent review/publication, wiki discovery and a private pinned story with refresh/resume and cross-owner denial. This proves browser/API/database integration; OAuth/header sanitisation, mesh/network policy and real user registration still need LAN acceptance. CI UI publication is gated on this test. Traces default to `/tmp/worldbuilder-browser-artifacts` and contain only fixture data.

A minimal devcontainer may lack desktop libraries/fonts. Prefer the approved Playwright dependency installation in a disposable development environment; temporary extracted libraries may alternatively be supplied with `LD_LIBRARY_PATH` and a private Fontconfig configuration. Do not weaken production authentication or add test identity switches to runtime code to make the test pass.

The suite now has three browser flows: independent reviewers and private story/history, explicit superuser registration/self-review, and [snapshot branch creation](TIMELINE-BRANCHES.md). The branch flow starts from a filtered wiki, creates only a draft, reviews/publishes it, changes the parent and checks unchanged branch evidence and historical origin. Shared catalog changes, retired identities, exact retries, complete-copy limits, stale rebase refusal and audit rollback also have real PostgreSQL or domain cases. The server uses schema 8 throughout; no new branch storage or migration is introduced.

Date acceptance in the browser/database gate includes both signed int64 extremes, editing a one-tick interval, numeric/string retry equivalence, half-open boundaries and historical reads after deletion. Processor tests additionally inspect exact dates across the real HTTP handler/provider prompt/model-candidate boundary; MCP tests check delegated signatures and structured evidence. These fixtures exercise no LAN services.

The graph/chronology browser scenario includes mouse/keyboard SVG navigation, extreme-date chronology, exclusion of staged candidates, historical revision reads, half-open end filtering and alternate-timeline isolation. It saves a fixture-only `canon-views.png` in Playwright's test output directory for visual review. Component tests additionally exercise reverse/self edges, bounded diagrams with complete tables, focus on omitted nodes, exact adjacent large dates, ASCII ID ties and partial-page coverage.

## Custom-resource schemas

```bash
python3 -m venv /tmp/worldbuilder-checks
/tmp/worldbuilder-checks/bin/pip install -r scripts/worldbuilder/requirements.txt
python3 scripts/worldbuilder/check-manifests.py
bash scripts/worldbuilder/export-crd-schemas.sh /tmp/worldbuilder-schema-check
/tmp/worldbuilder-checks/bin/python scripts/worldbuilder/validate-custom-resources.py /tmp/worldbuilder-schema-check /tmp/worldbuilder-manifests-REPLACE_WITH_PRINTED_DIRECTORY
```

The exporter uses Helm rendering/pulls and public HTTP downloads. It writes version/checksum metadata beside the schemas. Gateway v1.7.0 and AI Gateway v1.0.0 match repository pins; CNPG chart 0.23.2 and KEDA 2.16.1 satisfy repository ranges. Istio 1.30.5 and kube-prometheus-stack 92.3.0 are explicit comparison snapshots because the repo's default upstream chart selectors are unconstrained. Flux comes from the pinned official 2.9.6 install manifest, including its optional controller CRDs. Verify installed versions/CRDs read-only on LAN and rerun against those exact schemas before activation.

Every custom resource in base/overlay and feature-activation renders must have a schema; missing schemas fail. This lightweight structural check handles types/required fields/enums/bounds, including Kubernetes IntOrString/nullable conversion. The complete API-server gate below additionally executes built-in validation, CRD validation/CEL and strict unknown-field checks. Neither establishes installed admission behavior.

## Disposable API-server acceptance

With Helm 4.3.0, Go 1.25.6 and Python available:

```bash
python3 -m venv /tmp/platform-checks
/tmp/platform-checks/bin/pip install -r scripts/worldbuilder/requirements.txt
PATH=/tmp/platform-checks/bin:$PATH \
GOPATH=/tmp/platform-go GOMODCACHE=/tmp/platform-go-mod GOCACHE=/tmp/platform-go-build \
  bash scripts/worldbuilder/test-platform.sh
```

The [reusable workflow](../../.github/workflows/platform-acceptance.yml) runs this gate before app image/chart publication and before Helm publication/dispatch. It downloads official Kubernetes 1.35.0 envtest binaries with pinned architecture-specific SHA-512 verification, builds a separate locked Go test module, renders 29 base/overlay combinations and activation variants against Kubernetes 1.35.0, and exports the public schema snapshots above. Observability checks add opt-in renders, refusal/default-off cases and the selected upstream Grafana Deployment/ConfigMap and Prometheus resource. The harness uses explicit binary paths and a fresh TLS loopback connection. It deliberately supplies unusable `KUBECONFIG`, existing-cluster and binary-override environment settings to prove those inputs cannot select a live cluster.

The current source set contains 75 CRDs and 337 rendered resources (160 built-in, 177 custom). CRDs are installed without rewriting their schemas/conversion settings; a conversion webhook requires an explicit local fixture and otherwise fails. Strict YAML parsing rejects duplicate keys and preserves large integers. The structural safe loader also preserves legal bare `=` enum strings in upstream monitoring CRDs. Every render is submitted with server-side dry run and strict field validation. Explicit invalid-port, contradictory CEL and unknown-field cases must fail, while a valid dry-run fixture must remain absent from storage. Missing schemas/APIs, empty inputs or invalid resources fail the gate. Temporary API-server/etcd processes stop on completion or handled interruption.

Kubernetes 1.35.0 is a source comparison baseline, not an assertion about the installed cluster. No kubelet, workload controller, operator, service mesh or external admission webhook is started. This gate does not prove pod scheduling, Secret existence, image availability, networking, operator reconciliation, conversion-webhook behavior or production restores. The separate platform audit still needs feasible disposable controller/backup drills; actual installed versions/identities and live admission remain LAN gates. CI retains only public schemas, synthetic renders and binary-checksum metadata, excluding generated control-plane credentials. Set `KUBEBUILDER_ATTACH_CONTROL_PLANE_OUTPUT=true` locally for startup diagnostics.

The workflow also requires [disposable observability acceptance](OBSERVABILITY.md#disposable-acceptance): real Go metric output, numerical PromQL/scope checks, internal-network Prometheus scraping/filtering and Grafana file provisioning/datasource queries. That fixture substitutes a fixed internal target for Kubernetes discovery. The additional [controller drill](CONTROLLER-ACCEPTANCE.md) executes real Prometheus Operator TLS admission/refusal/recovery, discovery, selection, removal/recreation and Pod replacement in an owned kind cluster. [Physical backup acceptance](BACKUP-ACCEPTANCE.md) executes CNPG admission, base backup/WAL, source removal and latest/named-PITR restore against external SeaweedFS. [Velero recovery](VELERO-ACCEPTANCE.md) adds file/resource backup, source-volume loss, credential refusal and controller/node-agent replacement before mapped restore. [TSDB recovery](TSDB-RECOVERY.md) adds explicit opt-in storage renders, WAL replay, snapshot restore after volume loss and corrupt-index refusal. Mesh and remaining platform drills stay in the offline audit.

## Bounded domain load

```bash
(cd apps/external && go test ./worldbuilder -run '^$' -bench BenchmarkBoundedCanon5000 -benchtime=10x -count=1)
```

The benchmark validates/applies an update and queries a 5,000-record world, reporting time and allocations. It is a local domain microbenchmark, not concurrent database/provider/network throughput. Use the LAN runbook for realistic latency, world-lock contention, cancellation and provider limits before raising record or concurrency caps.

Calendar acceptance uses the same real-browser fixture to stage settings, independently review/publish them, format an event when search omits settings, and retain the published calendar during another staged edit. Historical reads before configuration show raw ticks. Migration 006 tests numeric record/history checksums, repeat/empty rollback and explicit refusal of unexpected fact-constraint drift, with the existing 16-table restore drill. See [calendar contract](CALENDARS.md).

Schema-7 review acceptance adds a third independent browser identity: a proposal raises the threshold to two, remains unpublished after one approval, and publishes only after the second. PostgreSQL acceptance covers membership-authority renewal, immutable decisions, concurrent review/rejection/publication, legacy review backfill/checksums and all 17-table restore. See [review-policy contracts](REVIEW-POLICY.md).
