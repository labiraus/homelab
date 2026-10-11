# Historical storage versions and offline acceptance

`scripts/storage/migrate_versions.py` archives every listed version and delete marker through S3, then copies them oldest first into an **empty version-enabled destination**. It preserves bytes, content headers, user metadata and tags, verifies full SHA-256 hashes, checks latest/deleted state and writes an old-to-new version map only after destination verification passes. The source is never modified. S3 supplies new version IDs and modification times; original timestamps remain in the archive.

The orchestrator uses the map only for historical reverts. Existing inventory, lifecycle and immutable audit markers retain their original values. New writes and their audits use destination-native IDs. A mapped revert checks the retrieved bytes against the recorded hash and size before writing anything. Delete markers, malformed maps, endpoint/bucket mismatches, duplicate entries and ambiguous source/destination ID collisions fail closed. Native destination versions continue to work normally.

The implementation follows the S3 [version listing](https://docs.aws.amazon.com/boto3/latest/reference/services/s3/client/list_object_versions.html) and [versioned writes](https://docs.aws.amazon.com/boto3/latest/reference/services/s3/client/put_object.html) contracts. It needs real source inventory/acceptance before production use.

## Disposable acceptance

The [storage workflow](../.github/workflows/storage-acceptance.yml) runs without LAN access or production credentials. It installs the Ansible role's checksum-pinned official SeaweedFS release, renders the actual service/identity configuration, exercises the socket proxy and runs:

- bucket isolation, versioned reads, metadata, ranges, copies, multipart operations and restart persistence;
- export/copy/reconciliation of history, delete markers, tags, empty files and source preservation;
- recovery into a fresh store with original/intermediate version aliases and post-cutover writes preserved;
- archive corruption/path/order checks before any destination write;
- Go application helpers and the orchestrator's pinned-map revert reader;
- the processor's actual Node SDK reading Go-created UTF-8 content and refusing cross-bucket access;
- 20 source/target chart combinations and both Velero configurations.

Reproduce locally with a Go toolchain, Node dependencies, Helm, `weed`, `systemd-socket-activate` and `systemd-socket-proxyd`:

```bash
python3 -m venv /tmp/storage-venv
/tmp/storage-venv/bin/pip install -r scripts/storage/requirements.txt
npm ci --prefix apps/processor
/tmp/storage-venv/bin/python -m unittest discover -s scripts/storage -p 'test_*.py'
/tmp/storage-venv/bin/python scripts/storage/check_charts.py
GOPATH=/tmp/storage-go GOMODCACHE=/tmp/storage-go-mod GOCACHE=/tmp/storage-go-cache \
  /tmp/storage-venv/bin/python scripts/seaweedfs-smoke.py --application-clients
```

The harness has fixed loopback endpoints, synthetic credentials and temporary data. It never accepts a supplied production endpoint. Normal unit runs skip the integration-only tests; the explicit harness invocation is required as acceptance evidence. The test server inherits a minimal environment because SeaweedFS also interprets environment variables as command flags.

## Controlled production procedure — prepared, not executed

Complete the [storage migration gates](SeaweedFSMigration.md) first. Freeze source and destination writers, lifecycle expiry and mutable tags throughout the final export/copy/verification. Use workload-appropriate private AWS profiles or runtime environment credentials; do not put credentials in command arguments. Archive directories contain private document data, so keep them outside Git with restricted access and independent backups. The CLI sets umask 0077 and refuses to reuse an existing export directory.

```bash
# Read-only source export. The archive path must not exist yet.
python3 scripts/storage/migrate_versions.py export /private-backups/documents-final \
  --endpoint http://svartalfheim:9000 --bucket documents --profile migration-source --writers-quiesced

# Entirely offline hash/manifest verification.
python3 scripts/storage/migrate_versions.py check /private-backups/documents-final

# Writes only to the already provisioned, empty destination bucket.
python3 scripts/storage/migrate_versions.py import /private-backups/documents-final \
  --endpoint http://svartalfheim:8333 --bucket documents --profile migration-target --writers-quiesced

# Repeat remote verification before installing the mapping object or resuming writers.
python3 scripts/storage/migrate_versions.py verify /private-backups/documents-final \
  --endpoint http://svartalfheim:8333 --bucket documents --profile migration-target
```

Use the exact same endpoint origin and bucket name as the application configuration. The general copier can target another bucket for isolated tests; the runtime resolver intentionally requires the original and destination bucket names to match. It rejects a hostname/IP alias mismatch instead of silently applying a map to the wrong store.

`version-map.json` is the verified result; `import-journal.jsonl` records acknowledged writes and is not an activation map. Reconcile every actual historical reference against the map and prove an isolated database/application restore. Upload the reviewed map to a reserved key such as `.migration/version-map.json` in the versioned destination documents bucket, with `application/json` content type. Record the immutable version ID returned by that upload, then set both values in the reviewed orchestrator chart/Flux configuration:

```yaml
env:
  S3_VERSION_MAP_KEY: .migration/version-map.json
  S3_VERSION_MAP_VERSION: "<actual immutable mapping-object version>"
```

The map is bounded to 32 MiB, loaded once at startup through existing bucket-scoped credentials, and pinned to that exact object version. A missing/unreadable/invalid configured map prevents startup. Keep its version and backup for the lifetime of historical references. Never replace the map pin with an unverified upload. Do not commit private manifests or object bodies to the repository. Publish reviewed cutover values through CI/CD only after the restore/reference gates pass.

## Refusal boundaries and recovery

- This is a frozen export and fresh-destination copy, not a live incremental migration. Changed source inventory during export fails without a completed manifest. Equal timestamps within one key are rejected when original ordering cannot be proved. Copies of successive versions of the same key are spaced by just over one second so subsequent export/recovery remains unambiguous even with whole-second S3 timestamps; include that cost in the maintenance window estimate.
- The initial copier supports individual objects up to 5 GiB. Larger objects, object lock/legal hold, customer-key encryption and non-standard storage classes require a separately validated procedure. Source bucket policies, lifecycle rules, encryption configuration, ACLs and retention must be inventoried separately; they are not cloned by the data tool.
- Interrupted imports stop. A version may have been written even if its acknowledgement or journal entry was lost. Do not rerun into the same populated target or delete source data. Inspect the journal and destination inventory, retain evidence, then recover using a separately prepared empty destination. No automatic source cleanup is implemented.
- Verification requires the destination to contain exactly the archived versions. Upload the mapping object only after that verification; subsequent application writes intentionally invalidate an exact frozen-copy comparison.
- Before destination writes, rollback retains the original source configuration/data. After writes, source and destination diverge: freeze writers, preserve both histories and prove a reconciled restore plus composed version mapping before switching back. Automatic rollback into the populated source is intentionally unavailable.

For recovery after destination writes, export the frozen destination into a new archive and import into a separately provisioned empty recovery store with `--prior-map /private-backups/documents-final/version-map.json`. This composes the original markers with the destination-native markers, verifies every previously mapped version still exists with the same content, and carries post-cutover writes too. The resulting map resolves both generations of references. Repeat the isolated database/application restore before switching the endpoint and map pin through CI. Retain both populated stores; the tool never merges into or deletes them. Missing historical versions or conflicting aliases stop recovery before any destination write.

These offline checks do not establish Pi disk/mount health, production source coverage, backup recoverability, deployed Flux state or live application readiness.

## Recorded validation — 2026-10-11

Local archive/refusal tests: 9 passed. Orchestrator `go test -race ./...` passed; processor build and 44 unit tests passed (the integration-only test is deliberately skipped in that command). The full disposable storage harness then passed both actual-client tests with no skips, version copy/reconciliation and recovery with composed aliases. Local log: `/tmp/homelab-storage-acceptance.log` (ephemeral). The run used the checksum-verified SeaweedFS 4.48 binary and temporary extracted Debian systemd libraries; no host installation or LAN access was required.

All 20 storage chart combinations, both Velero configurations and the existing 26 Worldbuilder chart combinations passed. Actionlint 1.7.7 passed the changed workflows with shellcheck disabled. No production data, Secrets, schemas or feature flags were changed directly. CI publication results are recorded separately from these local observations.
