# Operations and deployment

Use the existing CI/OCI/Flux workflow. App HelmReleases live in `flux-system` and
target workloads in `homelab`; discover resource namespaces independently. Both
`main` and `dev` publish the watched `uat` series, so `dev` is not an isolated estate.
This guide is a procedure, not authorization to execute it or a current health report.
See [status](status.md) for dated evidence and remaining gates.

## Read-only discovery

Verify the intended context, actual installed identities and source revisions first:

```bash
kubectl config current-context
kubectl cluster-info
kubectl get nodes -o wide
flux get all -A
kubectl get ocirepositories.source.toolkit.fluxcd.io -A
kubectl get helmreleases.helm.toolkit.fluxcd.io -A
kubectl -n flux-system get helmrelease external orchestrator processor ui postgres
kubectl -n data get cluster app-db
kubectl -n data get scheduledbackup,backup
kubectl get pvc -A
kubectl get storageclass
kubectl -n homelab get deployment,serviceaccount,networkpolicy
kubectl -n homelab get peerauthentication,authorizationpolicy
```

Inspect app logs and broker/storage readiness within the relevant namespaces. Use
Labiraus MCP alongside local `psql`, app logs and Flux evidence when evaluating the
deployed application. List Secret metadata only. Match source labels, image/chart
digests and installed revisions to the exact CI run. Current GHCR actions stamp
source and publish digests; that does not create or verify artifact signatures.

Use the devcontainer local `psql` path (`make postgres`, or refreshed ignored local
settings for scripted checks), not database-pod exec unless local transport is
unavailable. Read schema versions, ownership and grants before proposing migration.
A previously installed identity must not be replaced just because a local cache is
missing. Never run the destructive legacy auth seed against a populated registry.

## Publication and activation

1. Require healthy backup/WAL coverage and an isolated restore of the relevant
   recovery point before schema or storage changes. A manifest or Ready Pod is
   insufficient. Follow [recovery](recovery.md).
2. Rehearse populated upgrades with independent migration ownership and dedicated
   non-owner runtime roles. Current enabled external and generation-worker binaries
   require schema **10**. Apply ordered migrations through `010_up.sql`, then
   `grants.sql`; apply `auth-grants.sql` separately as its existing owner/operator.
   No application or Helm hook installs schemas. See [SQL](../sql/worldbuilder/README.md).
3. Preserve existing managed Secrets. External alone receives canon-write credentials;
   relay can read published snapshots/outbox only. Processor/MCP never receive
   canon owner/runtime credentials. Close migration-owner login after use.
4. Render the effective parent/child values, including GHCR overlays. Verify exact
   browser/MCP/worker principals, proxy header sanitization, STRICT mTLS and refusal
   of wildcard or empty identities. Observe native sidecars in `initContainers`
   with `restartPolicy: Always`, not just ordinary containers.
5. Publish the reviewed stage through normal CI/Flux. Pass applicable tests before
   publishing app images and matching charts. Observe actual OCI and Helm readiness,
   installed image digests, Pod endpoints and expected flags; publication alone is
   not deployment acceptance. Do not force an ad hoc Helm upgrade as a substitute.
6. Validate core canon with real intended identities: world access, Reader suggestions,
   Author reviews, policy readiness, atomic publication, immutable accounts, history,
   revocation and cross-world denial. Administrator overrides require reasons and
   cannot establish independent multi-human acceptance.
7. Validate relay/index delivery and fresh derived-index rebuilding; compare canonical
   content and citations to PostgreSQL. Historical and index-outage reads must retain
   authoritative results. Explicitly enable optional semantic search only after its
   model/profile/quality/readiness gates; disable rebuild-on-start after recovery.
8. Activate AI, native MCP, delegation and durable jobs through their separate
   configuration/identity gates. Preserve existing verified stages while upgrading;
   do not disable them merely to replay an old initial-install checklist.

For inference use [the proxy/legacy rollout procedure](inference.md), including
schema-10 profile pins, draining existing work, budget/credential isolation and
no paid fallback. Legacy local allowances are historical compatibility settings,
not the proxy's current local cost policy. Model smoke commands perform inference
and are not read-only discovery.

For native login follow [authentication](authentication.md); verify actual client
consent, initialize, tools and document reads. Worldbuilder delegation additionally
requires the shared signing key only in external/MCP, exact edge identities on both
transports, unsigned/expired/wrong-audience refusal, scoped read/draft retries and
no approval/publication/private-session capability. Use [API](worldbuilder/api.md)
for its limits. A working MCP route does not establish database-backed application
health or every browser journey.

## Public connectivity

Run `python3 scripts/validation/public-endpoints.py --output /tmp/public-endpoints.json`
for credential-free public checks. The corresponding hosted workflow runs after
publication and records bounded attempts. Success is under `attempts[].ok`, not a
top-level `ok`; cluster reconciliation and authenticated functionality remain
unverified fields requiring separate evidence.

For recurring failure, compare ordinary public DNS with the currently observed WAN
address and the private Gateway before changing application or TLS configuration.
A diagnostic address override must preserve hostname/SNI and certificate verification.
It is not ordinary public-DNS or hosted-runner acceptance. Never send credentials
through a hostname/certificate mismatch. Public recovery does not establish database,
Flux or authenticated application recovery.

## Stop and rollback boundaries

Stop activation when identity, backup, schema, capacity or endpoint evidence differs
from the reviewed stage. Disable affected features through published values while
preserving data/Secrets; older enabled binaries reject newer schemas. Canon rollback
uses a reviewed compensating revision. Database restore uses a new isolated target
and independently reviewed connection cutover. Never downgrade populated schemas or
rewrite history. After destination storage writes, endpoint-only rollback is unsafe;
follow [storage reconciliation](storage.md).

External Cassandra consumers, if found, need inventory, consistent export, checksum
and restore acceptance before separately authorized retirement. Absence from this
checkout is not permission to delete data or volumes.

## Monitoring

External uses the existing Prometheus endpoint/registry. `external_ops_total` has fixed `method` labels `worldbuilder`, `worldbuilder_tools`, `worldbuilder_worker`; `state` is `call`, `success`, `error`, `denied`, `conflict`, `budget` or `invalid`. Each completed HTTP call increments `call` and one outcome. The browser route groups canon/stories/reasoning together; worker success means a valid protocol response, not necessarily a successful generated job. Job status comes from PostgreSQL diagnostics. Labels never include world/job IDs, email, prompts, citations or narrative text.

`external_processing_duration_seconds_{sum,count,bucket}` retains existing service histogram buckets. Use sum/count for mean latency; its one-second finite bucket ceiling cannot establish a useful high percentile for 20-second inference. LAN load testing should measure raw request times and world-lock contention separately. The single [Grafana dashboard source](../helm/observability/prometheus/files/worldbuilder-dashboard.json) is packaged with the Prometheus wrapper chart. It also supports manual import with an explicit datasource input. Queries select the `worldbuilder-external` scrape job and the dashboard's application namespace; empty data is not a zero-success signal.

## Opt-in Helm provisioning

Both chart options default off in normal publication. They can be rendered and verified without activating canon or contacting a cluster. Populate their selectors and identities from LAN discovery before enabling them through the existing Flux chart values flow.

The external chart's `worldbuilderMonitoring` creates a ServiceMonitor in the application's namespace, selecting this release's Service and its named `http` port. It uses `/metrics`, 30-second scrapes, a 10-second timeout, `honorLabels: false`, and bounded sample/target/label counts. Relabeling sets `job=worldbuilder-external`; metric filtering retains only Worldbuilder operation counters and duration histograms. Kubernetes service discovery supplies the namespace label. Private SQL diagnostics remain separate.

Required inputs:

| External chart value | Observation required |
| --- | --- |
| `worldbuilderMonitoring.labels` | Nonempty labels selected by the installed Prometheus `serviceMonitorSelector`; its namespace selector must also discover the external namespace |
| `worldbuilderMonitoring.scraperNamespace` | Namespace containing the actual scraper Pods |
| `worldbuilderMonitoring.scraperPodLabels` | Nonempty exact Pod labels, combined with that namespace in an ingress NetworkPolicy |
| `worldbuilderMonitoring.scraperPrincipal` | Exact authenticated mesh principal in that namespace, distinct from browser, MCP and worker identities |

The chart retains STRICT mTLS and requires network policy. It adds a narrow network ingress rule for that scraper and a DENY policy: other identities cannot access `/metrics`, and the scraper can only use `GET /metrics` on the application port. It does not introduce a workload-wide ALLOW policy. Existing mesh ALLOW/CUSTOM rules, scraper egress, service discovery RBAC and actual mTLS client capability must also permit the scrape. This change does not inject a Prometheus sidecar, distribute certificates or relax mTLS; observe and configure the installed client's mesh path through its owning chart before activation.

The Prometheus wrapper's `worldbuilderObservability.enabled` provisions a dashboard ConfigMap alongside the upstream Grafana deployment and supplies `grafana.dashboardProviders` / `grafana.dashboardsConfigMaps`. These mount the file under `/var/lib/grafana/dashboards/worldbuilder` and provision the Worldbuilder folder. The existing upstream sidecar dashboard provider remains present. UI edits to this provisioned dashboard are disabled; change the tracked JSON source.

| Prometheus wrapper value | Required configuration |
| --- | --- |
| `stack.chartVersion` | Exact reviewed installed/upgraded chart version; wildcard is refused when provisioning is enabled |
| `stack.targetNamespace` | Upstream stack/Grafana namespace; existing default remains `istio-system` |
| `worldbuilderObservability.datasourceUID` | Existing Prometheus datasource UID, 1–40 letters/digits/`_`/`-`; no endpoint or credential is stored in the dashboard |
| `worldbuilderObservability.externalNamespace` | Exact application namespace used in dashboard queries |

The chart removes import-only `__inputs` and binds panel UIDs and the namespace constant explicitly. It does not create or replace the production datasource. The default upstream chart selector remains unchanged while provisioning is off. The offline comparison uses [kube-prometheus-stack 92.3.0](https://github.com/prometheus-community/helm-charts/tree/kube-prometheus-stack-92.3.0/charts/kube-prometheus-stack), including Grafana chart 13.5.0, Grafana 13.2.3 and Prometheus 3.15.0. These are test snapshots, not installed-version observations. Compare the exact installed chart before activating; [Prometheus monitor selection](https://prometheus-operator.dev/docs/developer/getting-started/) is a separate layer from target Service selection.

## Durable diagnostics

Run `psql -X -v ON_ERROR_STOP=1 -f sql/worldbuilder/ops_dashboard_queries.pgsql`
through the verified operator connection. It uses a read-only transaction and a
five-second statement timeout, exposing aggregate backlog, leases, reservations,
governance and saved-turn counts without prompts, emails or world names. Do not
broaden MCP/processor grants for dashboard access.

Growing outbox age calls for relay/broker acknowledgement checks. Expired jobs need
claim/ledger inspection before replay; uncertain attempts are not refunded. Separate
expected authentication probes and stale-revision conflicts from service errors.
Index failure must preserve canonical reads. Measure actual request latency and
world-lock contention; the existing one-second finite histogram ceiling cannot
prove useful inference tail latency.

Prometheus persistence and historical recovery are separate from successful scrape
or dashboard provisioning. See [recovery](recovery.md) and [testing](testing.md).
