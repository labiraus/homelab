# Worldbuilder operations and recovery signals

External uses the existing Prometheus endpoint/registry. `external_ops_total` has fixed `method` labels `worldbuilder`, `worldbuilder_tools`, `worldbuilder_worker`; `state` is `call`, `success`, `error`, `denied`, `conflict`, `budget` or `invalid`. Each completed HTTP call increments `call` and one outcome. The browser route groups canon/stories/reasoning together; worker success means a valid protocol response, not necessarily a successful generated job. Job status comes from PostgreSQL diagnostics. Labels never include world/job IDs, email, prompts, citations or narrative text.

`external_processing_duration_seconds_{sum,count,bucket}` retains existing service histogram buckets. Use sum/count for mean latency; its one-second finite bucket ceiling cannot establish a useful high percentile for 20-second inference. LAN load testing should measure raw request times and world-lock contention separately. The [Grafana dashboard](worldbuilder-dashboard.json) is an importable source artifact with an explicit Prometheus datasource input, not an automatically installed dashboard. Verify actual scrape labels/endpoint access before interpreting empty panels.

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

Actual scrape discovery, Grafana import, alert thresholds, installed broker topology/storage, production restore and representative concurrent load remain `BLOCKED_CLUSTER_ACCESS`. The [three-node disposable broker drill](BROKER-RECOVERY.md) now verifies leader/quorum/process recovery locally and in CI. Offline database fixtures also exercise injected publication/outbox failures, redelivery, process restarts between generation checkpoints, lost acknowledgements, expired reservations and all-table restore. The [LAN runbook](LAN-RUNBOOK.md) separates read-only investigation from controlled replay/activation. The [offline audit](OFFLINE-WORK.md) keeps remaining dashboard/scrape provisioning and disposable platform checks open.
