# Disposable physical backup and restore

The [CNPG harness](../../scripts/worldbuilder/test-backups.py) executes real base backup, WAL archiving, latest recovery and named point-in-time recovery against external SeaweedFS. It runs on an owned kind cluster with synthetic credentials/data and no LAN access. The repository's CNPG chart range, PostgreSQL image/bootstrap and SeaweedFS target version supply the comparison inputs.

With the [platform prerequisites](OFFLINE-ACCEPTANCE.md), Docker and local `psql` available:

```bash
python3 -m pip install -r scripts/worldbuilder/requirements.txt -r scripts/storage/requirements.txt
python3 scripts/worldbuilder/test-backups-unit.py
python3 scripts/worldbuilder/test-backups.py \
  "$artifacts/backup-acceptance" "$artifacts/schemas"
```

Use the public schemas produced by `test-platform.sh` and a new output directory. For a corporate registry proxy, the explicit `WORLDBUILDER_TEST_CA_BUNDLE` option from [controller acceptance](CONTROLLER-ACCEPTANCE.md) also applies. The shared [kind lifecycle](../../scripts/worldbuilder/kind-fixture.py) owns all contexts and loopback port mappings; no operator kubeconfig is used. CI runs this job after API-server acceptance and requires it before callers publish app/Helm artifacts.

## What the drill proves

1. Install pinned CNPG chart 0.23.2/operator 1.25.1 against Kubernetes 1.35.0. Keep its actual admission webhooks and generated certificate path. Conflicting bootstrap methods must be rejected; a valid dry run must fail while the operator/webhook is stopped and succeed after restart.
2. Start SeaweedFS 4.48 from a digest-pinned image outside Kubernetes, on the private Docker fixture network. It publishes no host ports, keeps master/filer/volume listeners on container loopback, and requires scoped S3 credentials. The image version must match the Ansible target.
3. Render the repository PostgreSQL chart with synthetic names, `standard` local-path PVCs and the owned S3 endpoint. Confirm PostgreSQL bootstrap, including `vector`. Automatic scheduling and NetworkPolicy enforcement are outside this fixture; production chart defaults remain unchanged.
4. Apply every ordered Worldbuilder up migration and the dedicated grants. The synthetic SQL fixture populates all 22 current Worldbuilder tables and the shared auth registry, including immutable history/reviews, signed-date extremes, Unicode text, private story state, pending outboxes, uncertain inference/generation work, established-account supersession and completed shared messages. Members use the current Author role. The ordinary PostgreSQL harness also clones the freshly migrated schema and checks that this seed populates every canon table before running API tests. Missing or empty tables fail the checksum preflight.
5. Complete an actual CNPG `Backup` on SeaweedFS. Make additional canon/audit writes, create a named restore point and force/wait for WAL archiving. Make later writes and archive those WAL records too.
6. Remove the source Cluster, Pods and PVCs. An attempted restore with invalid S3 credentials must fail without a Ready database. Recover two new clusters from the same base backup/archive: latest must contain the later writes, while named PITR must exclude them.
7. Compare row counts and full-row checksums for every populated table. Compare dedicated-role privilege attributes, schema/table/sequence/column/function ACLs and immutable-trigger definitions against the source, without reading password hashes. Also exercise restored grant denials, immutable-history rejection and non-reusing audit/conversation sequences. Established-account changes/removal, revision-based erasure and completed-message rewriting must still fail after restore; the runtime retains read-only ledger access. Require both new databases to have completed recovery and accept writes.
8. Remove owned Kubernetes and S3 containers/anonymous fixture volumes. Cleanup failure fails the gate.

The restore configuration follows the [CNPG 1.25 recovery contract](https://cloudnative-pg.io/docs/1.25/recovery/): a new cluster reads the original server's archive, and named PITR specifies both its backup ID and restore point. Restored fixtures do not configure a writer to the source archive. No empty-archive safety check is bypassed.

## Transport and evidence

Local `psql` first probes only the owned Docker loopback mapping, using synthetic credentials and TLS. It excludes inherited PostgreSQL configuration, client certificates and GSS authentication. A newly created NodePort may close the first connection while its route propagates; bounded retries allow readiness to settle, but persistent closure fails. Repeated connection refusal/timeout can select the owned-node Unix-socket exec fallback; authentication, SQL and certificate errors cannot be hidden by that fallback. The local Docker-outside-Docker environment required exec; CI's actual transport is recorded per recovered database.

`backup-result.json` records source removal, backup ID, seed/security hashes, observed image IDs, PostgreSQL version, transport and per-table checksums. `operator.yaml`, `cluster.yaml` and `postgres-inputs.yaml` contain public/synthetic configuration references. Pod/event/operator/storage logs support diagnosis. Generated kubeconfig and Secrets remain under excluded private state or inside the disposable containers. A result file records assertions; final process/workflow success additionally requires teardown.

The fixture establishes CNPG/Barman/SeaweedFS controller compatibility and recovery mechanics. Production inventory and backup-chain coverage, migration of actual source backups, installed versions, Longhorn/CSI snapshots, Pi disks/mounts, power-loss durability and real credentials/identities still require the estate. [Velero recovery](VELERO-ACCEPTANCE.md) has its own disposable gate and shares the scoped external S3 fixture; mesh enforcement remains separate offline work. No production schema, feature, endpoint or backup-storage cutover is performed.
