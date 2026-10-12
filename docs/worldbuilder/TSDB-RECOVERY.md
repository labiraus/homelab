# Persistent Prometheus recovery

The repository's Prometheus wrapper previously supplied only Grafana settings; it did not configure a Prometheus PVC. Existing Pod replacement checks demonstrated discovery recovery, not preserved history. Persistent storage is now an explicit opt-in, and a separate disposable CI gate tests WAL replay and snapshot restoration after source-volume loss.

## Prepared storage settings

Select these inputs only after observing the installed stack, storage class, available capacity, ingestion rate and intended retention:

```yaml
stack:
  chartVersion: '92.3.0' # Tested source baseline; verify the installed target before selecting.
prometheusStorage:
  enabled: true
  storageClassName: REPLACE_WITH_OBSERVED_CLASS
  size: REPLACE_WITH_CAPACITY
  retention: REPLACE_WITH_RETENTION
```

`size` accepts positive whole `Mi`, `Gi` or `Ti` quantities; `retention` accepts positive whole `h`, `d` or `w` durations. The example deliberately fails validation until actual inputs replace its placeholders. The wrapper requires an exact chart version and an explicit storage class. It renders a ReadWriteOnce claim and keeps the admin API disabled. Base, GHCR and ECR values retain their existing defaults while disabled; enabling dashboard provisioning and storage together preserves both configurations.

The [Prometheus storage guidance](https://prometheus.io/docs/prometheus/latest/storage/) describes local durability and consistent snapshots. Size for ingestion, WAL/head data and compaction headroom; a retention duration is not a disk-space guarantee. The [operator storage contract](https://prometheus-operator.dev/docs/platform/storage/) explains PVC provisioning. Changing storage for an existing installation needs a reviewed migration and backup; publishing this opt-in does not move existing data or select a real storage class.

## Disposable acceptance

Use the Linux/Docker/Helm/Python prerequisites and source-render commands in [controller acceptance](CONTROLLER-ACCEPTANCE.md) and [observability acceptance](OBSERVABILITY.md#disposable-acceptance). The API-server render pass now emits `storage-upstream-values.yaml` under its `renders/observability` directory. Run with a new output directory:

```bash
python3 scripts/worldbuilder/test-tsdb-unit.py
python3 scripts/worldbuilder/test-tsdb.py \
  "$artifacts/tsdb" "$artifacts/renders/observability" "$artifacts/metrics.txt"
```

The mandatory `tsdb-restore` platform job runs alongside the existing controller, CNPG, Velero and runtime-observability jobs before callers publish images/charts. It uses the same pinned kind/Kubernetes and kube-prometheus-stack inputs and the same fixture ownership, exec-status transport and teardown checks. Only synthetic metrics, public renders, hashes and logs are uploaded; kubeconfig and private certificates stay outside artifacts.

The fixture uses the source-rendered claim and retention settings with kind's `standard` storage class. It adds the local-volume annotation and enables the admin API only inside its owned cluster. It then requires:

- Actual Go counter and histogram samples plus a phase marker, followed by stopped producers and a quiescent, fixed-time raw sample matrix.
- A bound PVC/PV with verified claim identity, live WAL files and no pre-existing compacted blocks. A new Prometheus Pod UID on the same PVC must replay WAL and return the exact original labels, timestamps and values.
- A real [snapshot including head samples](https://prometheus.io/docs/prometheus/latest/querying/api/#snapshot), with complete block metadata, indexes, chunks and SHA-256 file hashes. Unsafe paths, links, missing data, unexpected live-WAL layout, changed content and oversized archives are refused before extraction.
- Positive evidence of samples written after the snapshot, then removal of the source workload, PVC, PV and its actual data path.
- Restore to a different volume, byte/hash comparison before startup, the exact same Prometheus image digest, no scrape jobs and no admin API. Restored history must match exactly and omit later samples.
- An intentionally corrupted index rejected by Prometheus itself, followed by recovery from the verified snapshot on a fresh PVC.
- Successful owned-cluster teardown after all assertions. A failed check or failed cleanup cannot pass acceptance.

Read `tsdb-result.json`, `snapshot-manifest.json`, `source-wal.log`, `corrupt-index.log`, runtime/cleanup logs and the final process/workflow conclusion together. The archive parser's 32 MiB compressed / 64 MiB expanded limits are deliberate bounds for synthetic fixtures, not production backup limits or a general restore utility.

Hosted orchestrator run `38189769201` recovered exact history but encountered connection refusal on its first direct request to the new restored Service. API-server Service-proxy reads had already succeeded; they do not prove kube-proxy's ClusterIP route is ready. The admin-isolation check now retries only curl connection failure (exit 7) for at most 45 seconds, then requires Prometheus's explicit disabled-API response. DNS, timeout and TLS errors, malformed responses and an enabled admin API fail immediately. Nine local snapshot/refusal tests pass; the corrected complete hosted gate remains required.

## Live history checkpoint — 2026-10-11

The installed Prometheus v3.15.0 uses `emptyDir` on `alfheim`; its admin API is disabled. A bounded capture verified the exact Pod UID, container ID and host-mounted TSDB path, paused that task for 0.305 seconds with an independent 20-second resume fallback, copied the files and resumed the same process. Its restart count remained zero. The 260,761,600-byte archive is retained both on the source worker and in local private recovery storage, with SHA-256 `b3ce46d979881a3945030e14847ba4d402f3a579d9c1e8faace51bdae5c373f6`. This is a one-time recovery checkpoint, not a scheduled backup system.

Restore acceptance passes on a separate 2 GiB Longhorn PVC in owned namespace `recovery-prometheus-20261011`: all 16 files match byte counts and hashes before startup, the exact source image replays WAL, and 45 series / 27,360 fixed-time historical samples match their labels, timestamps and values exactly. The restored instance has no scrape targets, no admin API, no public Service and deny-all network policy. Its process was stopped gracefully after comparison; the namespace/PVC remain pending API recovery and ownership-checked cleanup. Private reports are `prometheus-before-pvc-capture.json`, `prometheus-before-pvc-samples.json` and `prometheus-history-restore.json` under `.local/lan-recovery-20261011/`.

The original API-streamed archive transfer failed while both API and control-plane SSH became unavailable. No cause is established. Restore continued through worker SSH only after verifying the exact owned Pod/container, Longhorn claim and mount; hashes were checked before the startup gate was released. Production Prometheus remained running throughout. Production PVC migration still requires a fresh cutover checkpoint that includes later samples, normal Flux publication and verification of the actual new mount/history. This earlier restore alone does not authorize discarding the live `emptyDir`.

## Remaining live gates

This verifies the prepared storage contract and recovery mechanism with synthetic data. It does not prove installed versions, Longhorn replication, physical node/disk failure, realistic ingestion/capacity, backup scheduling/transport/custody, production inventory or cross-version compatibility. Admin snapshot access remains disabled in published settings; any production backup procedure needs its own authenticated, restricted access and external backup retention plan. The [LAN runbook](LAN-RUNBOOK.md) remains authoritative for discovery and controlled activation.
