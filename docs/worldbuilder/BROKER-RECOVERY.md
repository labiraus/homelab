# Disposable generation broker recovery

Run `bash scripts/worldbuilder/test-jetstream.sh` on Linux with Go 1.25.6. The
harness builds NATS Server **2.11.9** using the public Go checksum database and
starts three loopback-only processes with separate temporary file stores. It
does not load operator environment files, accept a broker URL, or contact the
cluster. Initial dependency downloads require internet access. Processes and
stores are removed after the test; Linux parent-death signals also kill brokers
if the test process is terminated.

The orchestrator CI test job runs this drill before image/chart publication.
Ordinary `go test` skips it unless the harness supplies its server binary. A
skipped test alone is not recovery acceptance.

## Exercised behavior

- The real startup helper creates a three-replica, file-backed, seven-day
  limits-retention generation stream and a three-replica durable consumer.
- A metadata-only job dispatch reaches the actual worker HTTP adapter and a
  loopback fixture representing an incomplete checkpoint. The stream leader is
  killed before acknowledgement. The surviving majority elects a leader,
  retains message-ID deduplication and redelivers the original job reference.
- The adapter resumes that reference; the completed fixture response is
  acknowledged synchronously. Stopping a second server prevents a new publish
  from receiving a successful acknowledgement.
- The uncertain publication is retried with its original message ID after
  quorum recovery. The test verifies one stored copy rather than assuming that
  a timeout proves non-delivery.
- All three processes are killed and restarted using their existing stores.
  Original message bytes, sequence identities, deduplication and durable
  acknowledgement state survive. Completed work is not redelivered.
- Retention drift is refused without rewriting the existing stream; an
  existing single-replica consumer is refused. Unit cases also cover wrong
  subjects, memory storage, one replica and incompatible retention policies.

The consumer redelivery timer is shortened from the production two minutes to
one second after checking the production configuration. The drill follows the
[NATS quorum model](https://docs.nats.io/running-a-nats-service/configuration/clustering/jetstream_clustering)
and [acknowledgement semantics](https://docs.nats.io/using-nats/developer/develop_jetstream/model_deep_dive).

## Production gates retained

This is protocol and process-recovery evidence for the pinned test server.
It does not establish the installed NATS version, mesh connectivity, node or
Longhorn durability under power loss, seven days of elapsed retention, or live
provider behavior. The job runner here is a deterministic HTTP fixture;
PostgreSQL checkpoint, allowance and uncertain-provider recovery are exercised
separately by `test-postgres.sh`.

On activation, existing stream configuration must match its subjects, three
replicas, file storage and seven-day limits retention. Existing consumers must
match the requested three replicas and acknowledgement settings. Startup
refuses incompatible resources instead of silently changing their history.
Inspect and back up the actual stream/consumer under the
[LAN runbook](LAN-RUNBOOK.md) before any deliberate repair. Do not delete broker
state to clear a startup error or automatically replay uncertain provider work.
