# Worldbuilder operations and recovery signals

External uses the existing Prometheus endpoint/registry. `external_ops_total` has fixed `method` labels `worldbuilder`, `worldbuilder_tools`, `worldbuilder_worker`; `state` is `call`, `success`, `error`, `denied`, `conflict`, `budget` or `invalid`. Each completed HTTP call increments `call` and one outcome. The browser route groups canon/stories/reasoning together; worker success means a valid protocol response, not necessarily a successful generated job. Job status comes from PostgreSQL diagnostics. Labels never include world/job IDs, email, prompts, citations or narrative text.

`external_processing_duration_seconds_{sum,count,bucket}` retains existing service histogram buckets. Use sum/count for mean latency; its one-second finite bucket ceiling cannot establish a useful high percentile for 20-second inference. LAN load testing should measure raw request times and world-lock contention separately. The single [Grafana dashboard source](../../helm/observability/prometheus/files/worldbuilder-dashboard.json) is packaged with the Prometheus wrapper chart. It also supports manual import with an explicit datasource input. Queries select the `worldbuilder-external` scrape job and the dashboard's application namespace; empty data is not a zero-success signal.

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

## Disposable acceptance

With the [platform dependencies](OFFLINE-ACCEPTANCE.md) and Docker available, choose a new artifact directory:

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

The reusable platform workflow runs both acceptance jobs before publication. Render checks cover base/GHCR/ECR defaults and opt-in settings, release/namespace overrides, 16 refusal cases, endpoint/Service/Pod alignment, exact network/identity boundaries, datasource binding and the pinned upstream chart's real Grafana mounts and Prometheus selectors. Public monitoring CRDs and the selected upstream Grafana Deployment/ConfigMap and Prometheus resource also pass the disposable API server. The full upstream render is retained for inspection; it is not claimed as an executed operator deployment.

Runtime acceptance runs pinned chart-selected Prometheus/Grafana images and a Python fixture on an internal Docker network with no published ports. Promtool evaluates the actual dashboard expressions against numerical rates/means, unrelated namespace/job/method series and empty input. Prometheus scrapes the real Go handler/registry fixture through the rendered metric filters; Grafana loads the rendered provider/dashboard and queries the datasource proxy. Anonymous dashboard API access must fail. Only the fixture's scrape interval is accelerated to one second and Kubernetes discovery is replaced with a fixed internal target. Image digests and results are recorded; containers/network are removed afterward. This proves source provisioning and metric/query behavior, while installed controllers, mesh, discovery, real workload load and alert thresholds remain LAN gates.

For durable state, run the aggregate [operator queries](../../sql/worldbuilder/ops_dashboard_queries.pgsql) through the repository's local `psql` route after authentication and schema verification:

```bash
# Uses an explicitly selected operator connection; no credentials in this command.
psql -X -v ON_ERROR_STOP=1 -f sql/worldbuilder/ops_dashboard_queries.pgsql
```

The script runs a read-only transaction with a five-second statement timeout and prints aggregate publication/dispatch backlog, job states/expired claims, inference allowances/latencies, review states and saved-turn counts. It prints no prompt, output, email or world names. Run only with the authorised operator role; do not grant shared MCP/processor logins canon access for dashboards.

Initial investigation signals, to baseline under actual load:

- A pending outbox age that grows across several relay intervals: inspect relay/broker acknowledgements and connectivity before replay.
- Expired running jobs or increasing `uncertain`: compare claim and inference ledger before controlled recovery. No automatic retry/refund of possibly billed work.
- Sustained HTTP error rate or longer means: inspect correlation IDs, provider timeouts, database locks and allowance completion failures. Logs record request correlation, not prompt bodies.
- Conflicts indicate concurrent canon/session changes; use reconciliation and fresh review. Denials can be expected authentication probes. A budget response is a policy limit, not a provider invoice.
- An index lag or outage must retain PostgreSQL-backed search results. Use the existing rebuild drill; OpenSearch is disposable derived state.

Actual scrape discovery, Grafana availability, alert thresholds, installed broker topology/storage, production restore and representative concurrent load remain `BLOCKED_CLUSTER_ACCESS`. The [three-node disposable broker drill](BROKER-RECOVERY.md) verifies leader/quorum/process recovery locally and in CI. Offline database fixtures also exercise injected publication/outbox failures, redelivery, process restarts between generation checkpoints, lost acknowledgements, expired reservations and all-table restore. The [LAN runbook](LAN-RUNBOOK.md) separates read-only investigation from controlled replay/activation. The [offline audit](OFFLINE-WORK.md) tracks provisioning publication evidence and remaining disposable controller/backup checks.
