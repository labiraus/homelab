# Disposable controller acceptance

`scripts/worldbuilder/test-controllers.py` tests real Prometheus Operator reconciliation in a new kind cluster. It needs Linux amd64/arm64, a local Docker Unix socket with privileged-container support, Helm 4.3.0, Python 3.11+ with the [platform dependencies](../../scripts/worldbuilder/requirements.txt), and public registry/release access. It does not require LAN access.

First produce the source renders and actual Go metrics using the commands in [observability acceptance](OBSERVABILITY.md#disposable-acceptance), then run:

```bash
python3 scripts/worldbuilder/test-docker-exec.py
python3 scripts/worldbuilder/test-controllers.py \
  "$artifacts/controllers" "$artifacts/renders/observability" "$artifacts/metrics.txt"
```

Choose a new output directory. The reusable platform workflow runs this drill after API-server acceptance, alongside Grafana/Prometheus runtime acceptance; all jobs must pass before callers can publish images/charts. CI uploads only public inputs, synthetic results and logs. Generated kubeconfig stays under a private directory excluded from artifacts.

If your public registry connection uses a trusted corporate proxy, explicitly set `WORLDBUILDER_TEST_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt` for this command. The harness validates a bounded public PEM bundle, adds it only to the owned node and restarts its containerd before workload pulls. Evidence records the bundle hash; TLS verification remains enabled. CI normally uses the node's original trust store. Do not disable certificate verification to clear an image-pull error.

## Fixture and evidence

The installer verifies architecture-specific official SHA-256 checksums for [kind 0.31.0](https://github.com/kubernetes-sigs/kind/releases/tag/v0.31.0), with its Kubernetes 1.35.0 node image pinned by digest. The API binds host loopback. Every command uses the generated fixture name/context, including teardown. Commands inside the node use its generated admin configuration, so unreachable Docker host forwarding cannot redirect tests to a real cluster. Name and kind ownership labels are checked before exec or removal.

The test renders kube-prometheus-stack 92.3.0, installs its unmodified CRDs and operator resources, and waits for real Ready nodes, CoreDNS, operator and Prometheus Pods. Unrelated monitoring components and admission webhooks are explicitly disabled in this fixture. The test applies the repository-rendered ServiceMonitor with only its scrape interval/timeout shortened. A synthetic Python endpoint serves metrics emitted by the real external Go registry/handlers.

Required observations:

- Exactly one healthy target appears through Kubernetes discovery and operator-generated configuration.
- The known Go counter value is present; unrelated metric names and methods are filtered.
- Wrong monitor labels, Service labels, namespaces and named ports cannot add targets. The operator must process the negative fixtures before these results are assessed.
- Changing Service labels and deleting the ServiceMonitor remove targets; restoring them recovers exactly one target.
- Operator and Prometheus Pod deletion creates new Ready Pod UIDs and restores scraping without duplicate targets.
- The owned node is removed after success or failure. A cleanup failure fails acceptance.

`controller-result.json` is written only after the controller assertions pass. The final process success additionally requires teardown. Review it together with the workflow conclusion and `cleanup.log`.

## Docker transport

Some Docker-outside-Docker proxies close an exec attach stream early and return success while the command is still running. This was reproduced with a delayed command that eventually exited 7. Archive copy also acknowledged success without writing files in the same local kind environment. The fixture-scoped Docker wrapper records each exec ID, polls its actual terminal exit status, and transfers private input/output in bounded chunks with output-length checks. It never replaces the global Docker executable or uses a production container. Delayed binary I/O, stderr, nonzero status, timeout and ownership-refusal checks cover this boundary.

Do not treat a client exit, partial output or successful `docker cp` response as acceptance evidence after such a failure. Inspect the actual exec state and required postconditions. Timeout means failure; the owning fixture teardown stops any command still running. Large image transfers are not the purpose of this transport: Kubernetes pulls the pinned/versioned public workload images normally.

## Boundaries

This is controller discovery and process recovery evidence. It does not establish installed versions, Flux rollout, production data coverage, persistent TSDB restoration, admission webhooks, network-policy enforcement or Istio mTLS. Default kind networking does not enforce the repository's mesh/security policies; no such success is inferred from this fixture. The separate [CNPG backup drill](BACKUP-ACCEPTANCE.md) exercises real physical recovery and CNPG admission; Velero restore, Prometheus admission and disposable mesh enforcement remain in [offline work](OFFLINE-WORK.md). Actual estate identity, storage and hardware checks remain LAN gates. No production feature, schema or storage setting is activated.
