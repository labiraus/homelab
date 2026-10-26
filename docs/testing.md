# Testing and acceptance

Run checks appropriate to the changed boundary. Offline fixtures use owned synthetic
resources and never load operator credentials or substitute a production endpoint.
Passing source tests, publishing artifacts and observing a deployment are different
evidence levels. Record dated outcomes in [status](status.md).

## Entry points

| Boundary | Entry point |
| --- | --- |
| UI components / production build | `npm test` / `npm run build` in `apps/ui` |
| Workspace viewport/focus journeys | `npm run test:workspace` in `apps/ui` |
| Real service worker and offline behavior | `npm run test:pwa` in `apps/ui` |
| Real PostgreSQL/API/browser | `WORLDBUILDER_BROWSER_TEST=1 bash scripts/worldbuilder/test-postgres.sh` |
| Canon relay/index/semantic reconstruction | `python3 scripts/worldbuilder/test-canon-index.py` (see fixture below) |
| Kubernetes renders / CRDs | `bash scripts/worldbuilder/test-platform.sh OUTPUT` |
| Storage migration and clients | `.github/workflows/storage-acceptance.yml`, commands in [storage](storage.md) |
| Inference accounting/controller | Commands in [inference](inference.md) and `scripts/inference/AGENTS.md` |
| Evaluation | [Worldbuilder](../evals/worldbuilder/README.md), [RAGAS](../evals/ragas/README.md) |

Run Go checks from each actual module directory. Locked npm dependencies and a
successful component suite do not establish real API or deployed browser acceptance.
Use a prepared Python virtualenv for scripts requiring PyYAML/boto3. The examples
below use `artifacts` as a fresh temporary output directory; obtain schemas with
`test-platform.sh` before dependent controller/backup runs.

## Browser gates

Finish the production build before serving it. `test:browser` selects
`worldbuilder.spec.js` and requires the disposable API/PostgreSQL launcher in
`apps/external/worldbuilder/browser_test.go`. Workspace and PWA fixtures are
separate gates. `WORLDBUILDER_BROWSER_URL` must be an owned HTTP loopback forward;
the workspace configuration rejects external hosts. Deployed-asset fixture checks
prove that bundle's behavior against fixtures, not production Google/model journeys.

On Windows/Node 26, `NODE_OPTIONS=--no-experimental-webstorage` lets jsdom provide
browser storage. Reinstall Linux-installed `node_modules` with the lockfile for
Windows tools. Before repeating a full harness after an environment failure,
prove Chromium render/click with its actual binary, libraries and Fontconfig.

Fixture authentication exists only in Go `_test.go` serving disposable loopback
data. Keep requests confined to that origin and retain API-browser traces separately
from workspace/PWA artifacts. Real Android installation, software keyboard,
dictation and screen-reader acceptance require actual device evidence.

## Offline application and API-server checks

Commands use disposable local services and public source artifacts. The API-server fixture installs CRDs and synthetic namespaces in temporary loopback etcd; workload manifests use strict server dry runs. Operator kubeconfig and SQL `.env` files are not loaded. Public tool/schema/browser downloads need internet access, not LAN credentials.

### Real browser and canon database

Build the UI, install its pinned Playwright Chromium, then invoke the database harness:

```bash
(cd apps/ui && npm ci && npm run build && npx playwright install --with-deps chromium)
WORLDBUILDER_BROWSER_TEST=1 bash scripts/worldbuilder/test-postgres.sh
```

The test-only Go server binds loopback, serves the production UI build and actual canon handler against disposable PostgreSQL, with explicit fixture identities in separate browser contexts. The fixture auth route exists only in `_test.go`, never a deployed binary. Chromium drives typed drafting, independent review/publication, wiki discovery and a private pinned story with refresh/resume and cross-owner denial. This proves browser/API/database integration; it does not establish actual user registration or replace separately recorded LAN OAuth/mesh checks. CI UI publication is gated on this test. Traces default to `/tmp/worldbuilder-browser-artifacts` and contain only fixture data.

A minimal devcontainer may lack desktop libraries/fonts. Prefer the approved Playwright dependency installation in a disposable development environment; temporary extracted libraries may alternatively be supplied with `LD_LIBRARY_PATH` and a private Fontconfig configuration. Do not weaken production authentication or add test identity switches to runtime code to make the test pass.

The suite has three browser flows: independent Authors and private story/history, Administrator user registration with an explicit reasoned publication override, and [snapshot branch creation](worldbuilder/api.md#snapshot-branches). The branch flow starts from a filtered wiki, creates only a draft, reviews/publishes it, changes the parent and checks unchanged branch evidence and historical origin. Shared catalog changes, retired identities, exact retries, complete-copy limits, stale rebase refusal and audit rollback also have real PostgreSQL or domain cases. The harness applies all migrations through schema 10; branches use the ordinary snapshot/proposal/audit tables.

The first flow also saves a provisional event as a private story turn, reloads/resumes it, reconciles it into a new draft and obtains the configured two independent approvals through the UI. Assertions require no search result before publication or at the old revision, a correctly scoped result after publication, fictional-time exclusion before the event, continued cross-owner story refusal and an unchanged original story pin/state. This uses human-entered narration to isolate the product workflow from model quality; live AI outcome acceptance remains separate.

Date acceptance in the browser/database gate includes both signed int64 extremes, editing a one-tick interval, numeric/string retry equivalence, half-open boundaries and historical reads after deletion. Processor tests additionally inspect exact dates across the real HTTP handler/provider prompt/model-candidate boundary; MCP tests check delegated signatures and structured evidence. These fixtures exercise no LAN services.

The graph/chronology browser scenario includes mouse/keyboard SVG navigation, extreme-date chronology, exclusion of staged candidates, historical revision reads, half-open end filtering and alternate-timeline isolation. It saves a fixture-only `canon-views.png` in Playwright's test output directory for visual review. Component tests additionally exercise reverse/self edges, bounded diagrams with complete tables, focus on omitted nodes, exact adjacent large dates, ASCII ID ties and partial-page coverage.

### Custom-resource schemas

```bash
python3 -m venv /tmp/worldbuilder-checks
/tmp/worldbuilder-checks/bin/pip install -r scripts/worldbuilder/requirements.txt
python3 scripts/worldbuilder/check-manifests.py
bash scripts/worldbuilder/export-crd-schemas.sh /tmp/worldbuilder-schema-check
/tmp/worldbuilder-checks/bin/python scripts/worldbuilder/validate-custom-resources.py /tmp/worldbuilder-schema-check /tmp/worldbuilder-manifests-REPLACE_WITH_PRINTED_DIRECTORY
```

The exporter uses Helm rendering/pulls and public HTTP downloads. It writes version/checksum metadata beside the schemas. Envoy Gateway v1.7.0 and AI Gateway v1.0.0 match repository pins; Gateway API v1.4.1 is the separate bootstrap CRD pin; CNPG chart 0.23.2 and KEDA 2.16.1 satisfy repository ranges. Istio 1.30.5 and kube-prometheus-stack 92.3.0 are explicit comparison snapshots because the repo's default upstream chart selectors are unconstrained. Flux comes from the pinned official 2.9.6 install manifest, including its optional controller CRDs. Verify installed versions/CRDs read-only on LAN and rerun against those exact schemas before activation.

Every custom resource in base/overlay and feature-activation renders must have a schema; missing schemas fail. This lightweight structural check handles types/required fields/enums/bounds, including Kubernetes IntOrString/nullable conversion. The complete API-server gate below additionally executes built-in validation, CRD validation/CEL and strict unknown-field checks. Neither establishes installed admission behavior.

### Disposable API-server acceptance

With Helm 4.3.0, Go 1.25.6 and Python available:

```bash
python3 -m venv /tmp/platform-checks
/tmp/platform-checks/bin/pip install -r scripts/worldbuilder/requirements.txt
PATH=/tmp/platform-checks/bin:$PATH \
GOPATH=/tmp/platform-go GOMODCACHE=/tmp/platform-go-mod GOCACHE=/tmp/platform-go-build \
  bash scripts/worldbuilder/test-platform.sh
```

The [reusable workflow](../.github/workflows/platform-acceptance.yml) runs this gate before app image/chart publication and before Helm publication/dispatch. It downloads official Kubernetes 1.35.0 envtest binaries with pinned architecture-specific SHA-512 verification, builds a separate locked Go test module, renders the configured base/overlay combinations and activation variants against Kubernetes 1.35.0, and exports the public schema snapshots above. Observability checks add opt-in renders, refusal/default-off cases and the selected upstream Grafana Deployment/ConfigMap and Prometheus resource. The harness uses explicit binary paths and a fresh TLS loopback connection. It deliberately supplies unusable `KUBECONFIG`, existing-cluster and binary-override environment settings to prove those inputs cannot select a live cluster.

The harness records the CRD/resource inventory for each run; fixture counts change as source evolves. CRDs are installed without rewriting their schemas/conversion settings; a conversion webhook requires an explicit local fixture and otherwise fails. Strict YAML parsing rejects duplicate keys and preserves large integers. The structural safe loader also preserves legal bare `=` enum strings in upstream monitoring CRDs. Every render is submitted with server-side dry run and strict field validation. Explicit invalid-port, contradictory CEL and unknown-field cases must fail, while a valid dry-run fixture must remain absent from storage. Missing schemas/APIs, empty inputs or invalid resources fail the gate. Temporary API-server/etcd processes stop on completion or handled interruption.

Kubernetes 1.35.0 is a source comparison baseline, not an assertion about the installed cluster. No kubelet, workload controller, operator, service mesh or external admission webhook is started. This gate does not prove pod scheduling, Secret existence, image availability, networking, operator reconciliation, conversion-webhook behavior or production restores. The separate fixtures below exercise controllers and backups; actual installed versions/identities and live admission remain estate gates. CI retains only public schemas, synthetic renders and binary-checksum metadata, excluding generated control-plane credentials. Set `KUBEBUILDER_ATTACH_CONTROL_PLANE_OUTPUT=true` locally for startup diagnostics.

The workflow also requires [disposable observability acceptance](#observability-runtime-fixture): real Go metric output, numerical PromQL/scope checks, internal-network Prometheus scraping/filtering and Grafana file provisioning/datasource queries. That fixture substitutes a fixed internal target for Kubernetes discovery. The additional [controller drill](#controller-fixture) executes real Prometheus Operator TLS admission/refusal/recovery, discovery, selection, removal/recreation and Pod replacement in an owned kind cluster. [Physical backup acceptance](#physical-backup-fixture) executes CNPG admission, base backup/WAL, source removal and latest/named-PITR restore against external SeaweedFS. [Velero recovery](#velero-fixture) adds file/resource backup, source-volume loss, credential refusal and controller/node-agent replacement before mapped restore. [TSDB recovery](recovery.md) adds explicit opt-in storage renders, WAL replay, snapshot restore after volume loss and corrupt-index refusal. Mesh enforcement uses its separate fixture below.

### Bounded domain load

```bash
(cd apps/external && go test ./worldbuilder -run '^$' -bench BenchmarkBoundedCanon5000 -benchtime=10x -count=1)
```

The benchmark validates/applies an update and queries a 5,000-record world, reporting time and allocations. It is a local domain microbenchmark, not concurrent database/provider/network throughput. Use [operations](operations.md) for realistic latency, world-lock contention, cancellation and provider limits before raising record or concurrency caps.

Calendar acceptance stages and reviews settings, formats events when search omits
settings, preserves the published calendar during edits, and returns raw ticks
before calendar configuration. Migration checks retain numeric/history checksums,
repeat/empty rollback and unexpected constraint-drift refusal.

Governance acceptance exercises current Author/Reader roles, administrative policy
pins and explicit override reasons, membership-authority renewal, immutable reviews,
concurrent rejection/publication and legacy migration checksums. The populated
restore fixture covers all 22 current Worldbuilder tables. See the
[governance contract](worldbuilder/api.md#governance-and-established-canon).

## Canon index fixture

Run `python3 scripts/worldbuilder/test-canon-index.py /tmp/canon-index-result` from the repository root with Docker, Go 1.25.6 and Python 3 available. Use a fresh output directory. The platform workflow requires this gate before dependent publication.

The fixture creates PostgreSQL 16.13, NATS 2.11.9 and OpenSearch 3.9.0 containers with `network=none`. Generated loopback bridges reach only their exact labelled container identities. Finite database commands use the inspected exec-ID transport; a streaming Docker CLI result never establishes migration completion. No host ports, operator endpoints, environment files or production credentials are used. Fixture credentials and records are synthetic.

The gate applies the actual canon migrations to separate disposable databases, then requires the selected Go tests to pass without skips:

- Real nested OpenSearch discovery returns only bounded IDs with world, timeline, visibility and revision scope. Canonical search still checks the authoritative snapshot and creates its own citations.
- A synthetic loopback embedding model drives actual exact vector scoring, semantic-only recall, cache reuse, mapping/profile refusal and vector reconstruction. External exercises current canon scope, half-open dates, canonical citations, durable search/generation allowances (including failures), ranking tokens and large-world AI reference closure.
- The production outbox relay publishes to real JetStream and records durable acknowledgement. A repeated event ID is deduplicated; the consumed event drives actual projection.
- Initial revision 0 is queued atomically on world creation, without duplicate events on exact retries. Injected enqueue failure rolls back creation. The actual broker/parser/projector also processes the initial snapshot without needing a later change publication.
- Delivering an older revision cannot replace the current document.
- After deletion of the entire disposable index, the actual rebuild recreates the latest canonical scope and record content from PostgreSQL.
- All owned containers and anonymous volumes are removed, and removal is verified before the report can pass.

Inspect `index-result.json`, all three Go logs and the container logs together. The report records immutable image IDs. Ownership refusal tests cover incorrect fixture names, roles, labels and attached networks. Test failure, skipped coverage, persistent startup failure and cleanup failure cannot pass.

### Scope and remaining work

Recorded source/publication evidence, including the retained index artifact hash, is in [status](status.md). Routine Go suites can skip endpoint-dependent cases; require the complete owned index gate with cleanup.

The index harness pulls pinned images before creating containers, with a bounded 1,800-second cold pull per image and separate 180-second OpenSearch readiness limit. A cold-image timeout is a failed run, not index acceptance; require verified cleanup before a fresh retry.

Default GET search ranks a complete authorized snapshot lexically. Its OpenSearch candidates only contribute reconciliation diagnostics. The optional [POST hybrid adapter](worldbuilder/api.md) changes ranking and includes semantic-only records under the approved interactive allowance. It is disabled by default and requires current canon. Historical queries bypass the current projection. The document embedding pipeline is separate and does not establish canon semantic retrieval.

The retained `EmbedText` in [embeddingutil](../apps/pkg/embeddingutil) implements 384-dimensional token hashing, not model inference. The document query builder in `apps/pkg/opensearchrag` also emits lexical matching; stored document vectors alone do not prove that a query uses them. Any configured search-pipeline transformation requires its own observed acceptance.

The configurable vector adapter now has an explicit model/dimension/profile contract, bounded inference, semantic-only recall and scoped pagination/recovery acceptance with a synthetic model. A separate real pinned CPU model smoke passes basic adapter/batch/Unicode compatibility and four top-match cases; wider quality, deployed identity enforcement and activation remain separate gates. An index mapping or a successful embedding response alone cannot close them. Optional timeline inheritance, installed topology, multi-node loss and physical storage recovery also remain open.

## Controller fixture

`scripts/worldbuilder/test-controllers.py` tests real Prometheus Operator reconciliation in a new kind cluster. It needs Linux amd64/arm64, a local Docker Unix socket with privileged-container support, Helm 4.3.0, Python 3.11+ with the [platform dependencies](../scripts/worldbuilder/requirements.txt), and public registry/release access. It does not require LAN access.

First produce the source renders and actual Go metrics using the commands in [observability runtime fixture](#observability-runtime-fixture), then run:

```bash
python3 scripts/worldbuilder/test-docker-exec.py
python3 scripts/worldbuilder/test-controllers.py \
  "$artifacts/controllers" "$artifacts/renders/observability" "$artifacts/metrics.txt"
```

Choose a new output directory. The reusable platform workflow runs this drill after API-server acceptance, alongside Grafana/Prometheus runtime acceptance; all jobs must pass before callers can publish images/charts. CI uploads only public inputs, synthetic results and logs. Generated kubeconfig stays under a private directory excluded from artifacts.

If your public registry connection uses a trusted corporate proxy, explicitly set `WORLDBUILDER_TEST_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt` for this command. The harness validates a bounded public PEM bundle, adds it only to the owned node and restarts its containerd before workload pulls. Evidence records the bundle hash; TLS verification remains enabled. CI normally uses the node's original trust store. Do not disable certificate verification to clear an image-pull error.

### Fixture and evidence

The installer verifies architecture-specific official SHA-256 checksums for [kind 0.31.0](https://github.com/kubernetes-sigs/kind/releases/tag/v0.31.0), with its Kubernetes 1.35.0 node image pinned by digest. The API binds host loopback. Every command uses the generated fixture name/context, including teardown. Commands inside the node use its generated admin configuration, so unreachable Docker host forwarding cannot redirect tests to a real cluster. Name and kind ownership labels are checked before exec or removal.

The test renders kube-prometheus-stack 92.3.0, installs its unmodified CRDs and operator resources, and waits for real Ready nodes, CoreDNS, operator and Prometheus Pods. Unrelated monitoring components are disabled. Admission remains enabled with TLS and `failurePolicy: Fail`; the fixture executes the original certificate-generation and CA-patching hook Jobs in order. Their extended disposable TTL retains evidence until teardown. The test applies the repository-rendered ServiceMonitor with only its scrape interval/timeout shortened. A synthetic Python endpoint serves metrics emitted by the real external Go registry/handlers.

Initial Prometheus readiness allows ten minutes for cold image downloads. Inspect events and pull progress before attributing a timeout to reconciliation. Exhausting the bound fails and retains events; admission and replacement assertions remain required.

Required observations:

- Real admission accepts a valid PrometheusRule and coerces numeric annotations to strings; malformed PromQL and an AlertmanagerConfig with a missing receiver are rejected for their semantic errors.
- Both validating and mutating webhook configurations reject a valid request under incorrect CA trust, then recover when the original trust is restored.
- Stopping the owned operator makes a valid admission request fail; a fresh operator Pod UID restores successful admission and mutation.

- Exactly one healthy target appears through Kubernetes discovery and operator-generated configuration.
- The known Go counter value is present; unrelated metric names and methods are filtered.
- Wrong monitor labels, Service labels, namespaces and named ports cannot add targets. The operator must process the negative fixtures before these results are assessed.
- Changing Service labels and deleting the ServiceMonitor remove targets; restoring them recovers exactly one target.
- Operator and Prometheus Pod deletion creates new Ready Pod UIDs and restores scraping without duplicate targets.
- The owned node is removed after success or failure. A cleanup failure fails acceptance.

`controller-result.json` is written only after the controller assertions pass. The final process success additionally requires teardown. Review it together with the workflow conclusion and `cleanup.log`.

### Admission details

The checks exercise the [operator admission contract](https://prometheus-operator.dev/docs/platform/webhook/) through server-side dry runs against the real API server. Semantic refusal must contain the tested PromQL/receiver error; an unrelated failure cannot satisfy it. Incorrect trust must report an unknown certificate authority. Both webhook kinds require nonempty CA bundles and fail-closed policies. All changes are confined to the disposable cluster. The test restores trust and the operator even after an assertion fails.

A Ready Pod may precede working Service routing. Initial startup and restart require a bounded successful admission request. Operator replacement uses the Deployment selector because certificate hook Pods share its broader name label. The fixture saves `operator.log` and `webhooks.log` alongside public manifests and synthetic results. Private certificate keys and kubeconfig are excluded. These checks do not cover certificate rotation, conversion-webhook compatibility or the installed cluster.

### Docker transport

Some Docker-outside-Docker proxies close an exec attach stream early and return success while the command is still running. This was reproduced with a delayed command that eventually exited 7. Archive copy also acknowledged success without writing files in the same local kind environment. The fixture-scoped Docker wrapper records each exec ID, polls its actual terminal exit status, and transfers private input/output in bounded chunks with output-length checks. It never replaces the global Docker executable or uses a production container. Delayed binary I/O, stderr, nonzero status, timeout and ownership-refusal checks cover this boundary.

Do not treat a client exit, partial output or successful `docker cp` response as acceptance evidence after such a failure. Inspect the actual exec state and required postconditions. Timeout means failure; the owning fixture teardown stops any command still running. Large image transfers are not the purpose of this transport: Kubernetes pulls the pinned/versioned public workload images normally.

### Boundaries

This is controller discovery and process recovery evidence. It does not establish installed versions, Flux rollout, production data coverage, persistent TSDB restoration, installed admission, network-policy enforcement or Istio mTLS. Default kind networking does not enforce the repository's mesh/security policies; no such success is inferred from this fixture. The separate [CNPG backup drill](#physical-backup-fixture) exercises real physical recovery and CNPG admission; [Velero recovery](#velero-fixture) separately covers files, Kubernetes resources and backup-controller replacement. The separate [TSDB recovery gate](recovery.md) verifies persistent history and snapshots; disposable mesh enforcement uses the [mesh fixture](#mesh-fixture). Actual estate identity, storage and hardware checks remain LAN gates. No production feature, schema or storage setting is activated.

## Mesh fixture

`scripts/worldbuilder/test-mesh.py` exercises the source-rendered external/MCP NetworkPolicies, STRICT PeerAuthentication, browser/delegation/worker/metrics AuthorizationPolicies, native-auth boundaries, Gateway and HTTPRoutes. It uses a fresh, owned Kubernetes 1.35.0 kind cluster with Cilium 1.20.2 and Istio 1.30.5. Production kubeconfig, credentials, source data and network endpoints are never used.

The shared fixture disables kind's default CNI only for this test, installs Cilium before waiting for node/DNS readiness, and retains the existing ownership-checked exec transport and loopback-only API. Other controller/backup tests keep their normal networking. The bootstrap follows the [Cilium kind installation](https://docs.cilium.io/en/stable/installation/kind/) and [Istio Helm installation](https://istio.io/latest/docs/setup/install/helm/) contracts. Public registry trust can be supplied through the existing bounded `WORLDBUILDER_TEST_CA_BUNDLE` input; TLS verification remains enabled.

Run after preparing the source schema artifacts with `scripts/worldbuilder/test-platform.sh`:

```bash
set -euo pipefail
python3 scripts/worldbuilder/test-docker-exec.py
python3 scripts/worldbuilder/test-mesh-artifacts.py
python3 scripts/worldbuilder/test-mesh.py /tmp/worldbuilder-mesh /tmp/platform-evidence/schemas
python3 scripts/worldbuilder/test-mesh.py --check-artifacts /tmp/worldbuilder-mesh
```

Use a fresh output directory. Docker, Python/PyYAML, Helm and OpenSSL are required. The fixture creates its own kubeconfig and deletes only its owned kind cluster. Its private directory contains ephemeral cluster and test TLS keys and is removed on successful cleanup. Retain public render hashes, certificate SAN identities, observed image IDs, synthetic request results, diagnostic logs and `mesh-result.json`; a failed assertion or failed cleanup cannot report success. Cilium startup allows fifteen minutes and synthetic workload readiness ten minutes for cold downloads; expiry still fails acceptance.

Retained render copies redact every Secret `data`/`stringData` value. The unmodified Secret objects are applied directly from memory inside the owned fixture. This is necessary because Cilium's upstream chart itself renders CA and Hubble private keys; excluding the private kubeconfig directory alone does not exclude those keys. Redacted render copies are review evidence, not manifests to reapply. CI independently runs `test-mesh.py --check-artifacts OUTPUT` before upload, even after a failed test; an unredacted Secret or plaintext/base64 private key blocks upload.

The acceptance matrix requires:

- Positive GET and POST requests from the exact browser, MCP and generation-worker identities, with cross-role requests denied. Matching Pod labels and forged forwarded identity/certificate headers do not replace the service-account identity.
- Actual certificate SAN inspection, STRICT plaintext refusal, namespace/Pod selector refusal and source egress scope/port checks. An unrestricted client first proves every tested synthetic destination/port is reachable.
- A distinct metrics reader allowed only on GET `/metrics`, with forged scraper identity, application routes and other methods denied.
- The real generated Istio Gateway serving a source-configured TLS hostname with certificate verification and explicit unrelated-CA refusal. All four source-configured MCP transport paths, including actual MCP/message POST methods, require the source CUSTOM authorization call; missing/invalid credentials and forged user headers are refused, and accepted identity headers come from the authority.
- No public route to private worker/delegation/authority operations. Direct browser-to-MCP transport is blocked by source policy.
- Unavailable authorization backend refusing previously accepted callers, followed by recovery through a replacement backend and real Istio/Cilium/Gateway Pod replacement with positive and negative requests repeated.

`mesh-fixture.py` supplies synthetic HTTP services and a fixed synthetic credential authority only inside the disposable cluster. It does not replace the separate actual-application tests of OAuth, database membership, signed assertions or world authorization. The Gateway uses a ClusterIP because the fixture has no external load balancer; the listeners, route matching and TLS/authentication policies remain source-derived. This gate cannot establish installed policy/CRD parity, real OAuth registration, physical cross-node routing, certificate rotation, production capacity or LAN outage recovery.

Retained mesh artifact custody is recorded in [status](status.md). Sanitized archive bytes differ from original execution hashes; do not imply they are identical. The independent artifact checker must still refuse every unredacted private key.

For the custom-CNI fixture, extend the disposable CoreDNS progress deadline before Cilium installation, then require actual node and DNS readiness. Default-networking fixtures retain their original deadline.

The Gateway trust-negative check keeps valid SNI and supplies an unrelated CA, requiring `SSLCertVerificationError`; unknown SNI may reset before presenting a certificate. Istio startup/replacement requires a real admission request after the validator has a CA and `failurePolicy: Fail`. A Ready Pod or initial `Ignore` policy is insufficient.

## Physical backup fixture

The [CNPG harness](../scripts/worldbuilder/test-backups.py) executes real base backup, WAL archiving, latest recovery and named point-in-time recovery against external SeaweedFS. It runs on an owned kind cluster with synthetic credentials/data and no LAN access. The repository's CNPG chart range, PostgreSQL image/bootstrap and SeaweedFS target version supply the comparison inputs.

With the [platform prerequisites](#custom-resource-schemas), Docker and local `psql` available:

```bash
python3 -m pip install -r scripts/worldbuilder/requirements.txt -r scripts/storage/requirements.txt
python3 scripts/worldbuilder/test-backups-unit.py
python3 scripts/worldbuilder/test-backups.py \
  "$artifacts/backup-acceptance" "$artifacts/schemas"
```

Use the public schemas produced by `test-platform.sh` and a new output directory. For a corporate registry proxy, the explicit `WORLDBUILDER_TEST_CA_BUNDLE` option from [controller acceptance](#controller-fixture) also applies. The shared [kind lifecycle](../scripts/worldbuilder/kind-fixture.py) owns all contexts and loopback port mappings; no operator kubeconfig is used. CI runs this job after API-server acceptance and requires it before callers publish app/Helm artifacts.

### What the drill proves

1. Install pinned CNPG chart 0.23.2/operator 1.25.1 against Kubernetes 1.35.0. Keep its actual admission webhooks and generated certificate path. Conflicting bootstrap methods must be rejected; a valid dry run must fail while the operator/webhook is stopped and succeed after restart.
2. Start SeaweedFS 4.48 from a digest-pinned image outside Kubernetes, on the private Docker fixture network. It publishes no host ports, keeps master/filer/volume listeners on container loopback, and requires scoped S3 credentials. The image version must match the Ansible target.
3. Render the repository PostgreSQL chart with synthetic names, `standard` local-path PVCs and the owned S3 endpoint. Confirm PostgreSQL bootstrap, including `vector`. Automatic scheduling and NetworkPolicy enforcement are outside this fixture; production chart defaults remain unchanged.
4. Apply every ordered Worldbuilder up migration and the dedicated grants. The synthetic SQL fixture populates all 22 current Worldbuilder tables and the shared auth registry, including immutable history/reviews, signed-date extremes, Unicode text, private story state, pending outboxes, uncertain inference/generation work, established-account supersession and completed shared messages. Members use the current Author role. The ordinary PostgreSQL harness also clones the freshly migrated schema and checks that this seed populates every canon table before running API tests. Missing or empty tables fail the checksum preflight.
5. Complete an actual CNPG `Backup` on SeaweedFS. Make additional canon/audit writes, create a named restore point and force/wait for WAL archiving. Make later writes and archive those WAL records too.
6. Remove the source Cluster, Pods and PVCs. An attempted restore with invalid S3 credentials must fail without a Ready database. Recover two new clusters from the same base backup/archive: latest must contain the later writes, while named PITR must exclude them.
7. Compare row counts and full-row checksums for every populated table. Compare dedicated-role privilege attributes, schema/table/sequence/column/function ACLs and immutable-trigger definitions against the source, without reading password hashes. Also exercise restored grant denials, immutable-history rejection and non-reusing audit/conversation sequences. Established-account changes/removal, revision-based erasure and completed-message rewriting must still fail after restore; the runtime retains read-only ledger access. Require both new databases to have completed recovery and accept writes.
8. Remove owned Kubernetes and S3 containers/anonymous fixture volumes. Cleanup failure fails the gate.

The restore configuration follows the [CNPG 1.25 recovery contract](https://cloudnative-pg.io/docs/1.25/recovery/): a new cluster reads the original server's archive, and named PITR specifies both its backup ID and restore point. Restored fixtures do not configure a writer to the source archive. No empty-archive safety check is bypassed.

### Transport and evidence

Local `psql` first probes only the owned Docker loopback mapping, using synthetic credentials and TLS. It excludes inherited PostgreSQL configuration, client certificates and GSS authentication. A newly created NodePort may close the first connection while its route propagates; bounded retries allow readiness to settle, but persistent closure fails. Repeated connection refusal/timeout can select the owned-node Unix-socket exec fallback; authentication, SQL and certificate errors cannot be hidden by that fallback. The local Docker-outside-Docker environment required exec; CI's actual transport is recorded per recovered database.

`backup-result.json` records source removal, backup ID, seed/security hashes, observed image IDs, PostgreSQL version, transport and per-table checksums. `operator.yaml`, `cluster.yaml` and `postgres-inputs.yaml` contain public/synthetic configuration references. Pod/event/operator/storage logs support diagnosis. Generated kubeconfig and Secrets remain under excluded private state or inside the disposable containers. A result file records assertions; final process/workflow success additionally requires teardown.

The fixture establishes CNPG/Barman/SeaweedFS controller compatibility and recovery mechanics. Production inventory and backup-chain coverage, migration of actual source backups, installed versions, Longhorn/CSI snapshots, Pi disks/mounts, power-loss durability and real credentials/identities still require the estate. [Velero recovery](#velero-fixture) has its own disposable gate and shares the scoped external S3 fixture; mesh enforcement uses its separate fixture. No production schema, feature, endpoint or backup-storage cutover is performed.

## Velero fixture

The [Velero harness](../scripts/worldbuilder/test-velero.py) exercises the real controller, node-agent, AWS plugin and Kopia file backup against authenticated external SeaweedFS. It uses synthetic files, credentials and Kubernetes resources in an owned kind cluster. It requires no LAN access or operator kubeconfig.

With the Docker/Helm/Python prerequisites from [offline acceptance](#offline-application-and-api-server-checks):

```bash
python3 -m pip install -r scripts/worldbuilder/requirements.txt -r scripts/storage/requirements.txt
python3 scripts/worldbuilder/test-velero-unit.py
python3 scripts/worldbuilder/test-velero.py "$artifacts/velero-acceptance"
```

Use a fresh output directory. The explicit registry CA option from [controller acceptance](#controller-fixture) also applies. The reusable platform workflow requires this job before its app/Helm callers can publish artifacts.

### Source and fixture boundaries

The input is `values/velero-seaweedfs-values.yaml`, retaining `deployNodeAgent: true`, disabled snapshots, path-style S3, the `velero` bucket and TLS-verification policy. Fixture overrides supply only its private endpoint, namespace and synthetic credential reference. Comparison pins are Helm chart 12.2.1 / Velero 1.18.2 and AWS plugin 1.14.4; they are not an assertion about the installed estate. Original chart CRDs are applied before controller resources, so the Helm CRD-upgrade hook is disabled inside the fixture.

The shared [S3 fixture](../scripts/worldbuilder/s3-fixture.py) uses SeaweedFS 4.48 pinned by digest and the Ansible target's bucket-scoped identity actions. Master/filer/volume listeners stay on container loopback; S3 is on the private Docker network with no host ports. A separate synthetic repository password is created before the first Kopia backup.

Velero [file-system backup](https://velero.io/docs/v1.18/file-system-backup/) supports local PVs, but not hostPath volumes. The fixture's PVC explicitly requests `volumeType: local` and verifies the actual PV and claim binding. Application startup only serves existing files; it cannot recreate missing payloads. Writes stop before backup. This is file/resource recovery, not a claim of live database consistency or CSI/Longhorn snapshot support.

### Required acceptance

1. The real Deployment, node-agent and authenticated BackupStorageLocation become Ready/Available. Anonymous S3 requests are denied.
2. A Deployment serves a PVC containing a 16 MiB binary payload, Unicode text/path, hidden and empty files, an empty directory and a relative symlink. The fixture records SHA-256, byte count, mode, owner/group and modification time. ConfigMap/Secret mounts, a Service and a restricted ServiceAccount/Role/RoleBinding are populated too.
3. A real Backup and its PodVolumeBackup complete. Partial, failed, warning-bearing or error-bearing results fail the gate; a Kopia snapshot ID is required. The generated source `kube-root-ca.crt` gets Velero's [exclusion label](https://velero.io/docs/v1.18/resource-filtering/#veleroioexclude-from-backuptrue) so the mapped namespace keeps fresh cluster trust. An annotation is not an exclusion.
4. Delete the source namespace and wait for its PVC/PV and original data directory to disappear. No source data is available to satisfy subsequent reads.
5. Wrong scoped S3 credentials make backup storage Unavailable with an actual S3 denial. The attempted restore must fail without destination Pods/PVCs. Restore valid credentials, replace both controller and node-agent Pods, and require new Ready UIDs and available storage.
6. Restore through a [namespace mapping](https://velero.io/docs/v1.18/restore-reference/) and require zero errors/warnings plus a completed PodVolumeRestore. The destination must have distinct PVC/PV UIDs and a different data path. Compare the full application file manifest and portable configuration/resource state; only allocated Service IPs, bound volume names and the intended RoleBinding namespace mapping differ.
7. Read restored mounted ConfigMap/Secret values, exercise allowed configuration reads and denied Secret/cross-namespace reads as the restored ServiceAccount, and fetch restored text through its Service.
8. Remove the owned kind and S3 containers/anonymous fixture volumes. Cleanup failure fails the process and CI job.

The reserved `.velero` restore-helper directory is outside the synthetic application manifest. No other extra/missing application file is accepted. File data, immutable ConfigMap/Secret content, Deployment spec, service selection/ports, PVC request and RBAC definitions are compared before and after recovery.

### Evidence and remaining gates

`velero-result.json` records file metadata/hashes, a hash of portable resource content, observed image IDs, source removal and refusal/restart assertions. Public chart inputs/rendering and controller/node-agent/restore/status/event logs are retained. Private generated kubeconfig is excluded. Require the final process/workflow result as well as the JSON assertions, because teardown follows the assertions.

Production source backups, historical Restic/Kopia compatibility, backup-chain migration, repository-password custody, actual generated cluster Secrets, installed versions, application quiescing and hardware/Longhorn/power-loss behavior remain estate-specific checks. This drill does not install or upgrade production Velero, switch backup storage or activate application features. Prometheus admission, persistent TSDB recovery and mesh enforcement have separate fixtures.

## Observability runtime fixture

With the [platform dependencies](#custom-resource-schemas) and Docker available, choose a new artifact directory:

```bash
artifacts=$(mktemp -d /tmp/worldbuilder-observability-XXXXXX)
bash scripts/worldbuilder/test-platform.sh "$artifacts"
(
  cd apps/external
  WORLDBUILDER_METRICS_FIXTURE="$artifacts/metrics.txt" \
    go test -count=1 -run '^TestWorldbuilderObservabilityFixture$' .
)
python3 scripts/worldbuilder/test-observability.py "$artifacts/renders/observability" "$artifacts/metrics.txt"
```

The reusable platform workflow runs API-server, observability runtime and [controller acceptance](#controller-fixture) before publication. Render checks cover base/GHCR/ECR defaults and opt-in settings, release/namespace overrides, 16 refusal cases, endpoint/Service/Pod alignment, exact network/identity boundaries, datasource binding and the pinned upstream chart's real Grafana mounts and Prometheus selectors. Public monitoring CRDs and the selected upstream Grafana Deployment/ConfigMap and Prometheus resource also pass the disposable API server. The controller fixture separately executes the pinned operator with unrelated components/webhooks disabled; it verifies discovery and process recovery, with mesh enforcement remaining a separate gate.

Runtime acceptance runs pinned chart-selected Prometheus/Grafana images and a Python fixture on an internal Docker network with no published ports. Promtool evaluates the actual dashboard expressions against numerical rates/means, unrelated namespace/job/method series and empty input. Prometheus scrapes the real Go handler/registry fixture through the rendered metric filters; Grafana loads the rendered provider/dashboard and queries the datasource proxy. Anonymous dashboard API access must fail. The fixture's scrape interval and timeout are reduced to one second, and Kubernetes discovery is replaced with a fixed internal target. Image digests and results are recorded; containers/network are removed afterward. This proves source provisioning and metric/query behavior, while installed controllers, mesh, discovery, real workload load and alert thresholds remain LAN gates.

## Disposable database test transport

Docker-outside-Docker published ports can become unreachable despite a healthy container. Use the Worldbuilder harness with `WORLDBUILDER_TEST_TRANSPORT=exec` for its loopback-only Docker-exec bridge when needed; do not switch these offline tests to a LAN/production DSN. A failed transport preflight is not a passing integration test.

Finite commands in the disposable PostgreSQL fixture also require the labelled, fixture-scoped exec-ID transport in `scripts/worldbuilder/docker-exec.py`; plain Docker attach may return success before `createdb`, migrations or restores finish. Preserve the original streaming CLI only for the loopback database bridge.

Real canon index acceptance needs PostgreSQL, NATS and OpenSearch together. Use `scripts/worldbuilder/test-canon-index.py`; routine Go success with skipped endpoint-dependent tests is not relay/discovery evidence. Require index deletion before rebuild comparison, and exact generated container names, ownership labels and `network=none` for its loopback bridges.

When evolving the canon schema, update `scripts/worldbuilder/backup-seed.sql` and the physical restore table inventory in `test-backups.py` alongside the API/database harness. The physical fixture applies every migration before seeding, so retired roles and unpopulated new tables must fail. Keep the fast seed preflight in `test-postgres.sh` and preserve full-table checksums and post-restore ledger/sequence checks.

## Disposable controller transport

For disposable kind work, set the generated `KUBECONFIG` for creation and deletion; cleanup must not lock or edit the operator's home configuration. The controller harness supplies both explicit arguments and environment settings.

Docker exec attach can terminate early with exit 0 while its command is still running, and archive-copy acknowledgements can lack the expected files in a proxied kind environment. Do not infer command completion from that client status. Use the fixture-scoped exec-ID transport in `scripts/worldbuilder/docker-exec.py`, verify terminal status and output/postconditions, and keep ownership checks and timeout failure intact. Never redirect an offline fixture to production to bypass transport failures.

Resolve the real Docker CLI before prepending a fixture's shim directory to `PATH`. Resolving `command -v docker` in a later assignment on the same command can select the shim itself and recurse; `WORLDBUILDER_DOCKER_CLI` must point to the original executable.

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

## Disposable TSDB history

A healthy scrape or replacement Pod does not prove historical recovery. Stop producers, wait for quiescent raw samples, pin the comparison time, verify PVC identity and the actual `prometheus-db` mount, and compare exact sample labels/timestamps/values after WAL replay and snapshot restore. Delete only owned fixture volumes, prove the old paths are gone and use a fresh PVC after a corrupt-data refusal. Keep the admin API confined to the disposable source and disabled in published storage settings.

## Dependency and artifact checks

Use lockfile installs and high/critical dependency checks in existing CI. The
processor uses its AWS SDK v3 storage adapter; obsolete MinIO transitive-advisory
exceptions are not a current waiver. Do not force incompatible dependency overrides
to make an audit green. Node images copy committed lockfiles, run `npm ci`, and
exclude local `node_modules`/build output so a later COPY cannot replace tested code.
Validate actual images when changing installation. Audit results are dated evidence,
not permanent vulnerability guarantees.

Use `bash scripts/build/test-helm-values.sh` to test packaged overlay parity.
Missing/null/comment-only overlays must retain defaults; reject non-map and
multi-document overlays. Render the composed parent/child values, including JSON
conversion of integers. Local `helm template -f` alone does not prove published
archive contents or signature creation.
