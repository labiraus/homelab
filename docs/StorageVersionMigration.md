# Historical storage versions and offline acceptance

`scripts/storage/migrate_versions.py` archives every listed version and delete marker through S3, then copies them oldest first into an **empty version-enabled destination**. It preserves bytes, content headers, user metadata and tags, verifies full SHA-256 hashes, checks latest/deleted state and writes an old-to-new version map only after destination verification passes. The source is never modified. S3 supplies new version IDs and modification times; original timestamps remain in the archive.

The orchestrator uses the map only for historical reverts. Existing inventory, lifecycle and immutable audit markers retain their original values. New writes and their audits use destination-native IDs. A mapped revert checks the retrieved bytes against the recorded hash and size before writing anything. Delete markers, malformed maps, endpoint/bucket mismatches, duplicate entries and ambiguous source/destination ID collisions fail closed. Native destination versions continue to work normally.

The implementation follows the S3 [version listing](https://docs.aws.amazon.com/boto3/latest/reference/services/s3/client/list_object_versions.html) and [versioned writes](https://docs.aws.amazon.com/boto3/latest/reference/services/s3/client/put_object.html) contracts. It needs real source inventory/acceptance before production use.

## Disposable acceptance

The [storage workflow](../.github/workflows/storage-acceptance.yml) runs without LAN access or production credentials. It installs the Ansible role's checksum-pinned official SeaweedFS release, renders the actual service/identity configuration, exercises the socket proxy and runs:

- bucket isolation, versioned reads, metadata, ranges, copies, multipart operations and restart persistence;
- export/copy/reconciliation of history, delete markers, tags, empty files and source preservation;
- a 5 GiB + 17 byte archive import and S3 re-export with full hashes, 81 streamed parts and Python peak memory below 256 MiB;
- lost initiation/part/completion replies against the real server, known-upload cleanup and refusal to activate or retry uncertain imports;
- recovery into a fresh store with original/intermediate version aliases and post-cutover writes preserved;
- archive corruption/path/order checks before any destination write;
- retained mixed version/delete-marker listings, timestamp ties across different page sizes, every copied historical position and latest/null-version handling;
- read-only PostgreSQL reference inventory and offline map coverage, including deleted inventory, saved tool results, aliases and unresolved identities;
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
  /tmp/storage-venv/bin/python scripts/seaweedfs-smoke.py --application-clients --large-migration
```

The harness has fixed loopback endpoints, synthetic credentials and temporary data. It never accepts a supplied production endpoint. Normal unit runs skip the integration-only tests; the explicit harness invocation is required as acceptance evidence. The test server inherits a minimal environment because SeaweedFS also interprets environment variables as command flags.

The large-object round trip requires at least 12 GiB free temporary disk and fails rather than skipping when space is insufficient. Omitting `--large-migration` still exercises real multipart failures but does not establish large-object acceptance. The memory assertion covers the Linux Python client, not the server or deployment capacity. The harness overrides only the server's volume-slot limit to 256: its many synthetic buckets each allocate seven mostly empty volumes, so production's free-disk-based auto count can run out of slots on hosted runners. No volumes are preallocated. Production retains its automatic capacity setting.

## Retained source ordering

New exports use archive schema 2. `order/before/` and `order/after/` retain the original mixed `Version`/`DeleteMarker` XML responses, with per-page sizes and SHA-256 hashes in `orderEvidence`. The two frozen scans use different page sizes (1000 and 127) and must agree on every version, kind, latest flag, timestamp and chronological position. Import replays the recorded pagination chain entirely offline and compares it with the manifest before any destination request. Keep the XML pages with the archive; they contain private object identities.

The checker validates full-bucket scope, marker echoes and continuation boundaries, complete termination, unique identities, ascending keys, newest-first non-null timestamps and exactly one consistent latest version per key. Keys are decoded only when the response declares URL encoding; version IDs remain opaque. The default `s3` dialect preserves literal plus signs and requires the next marker to equal the last listed identity. XML pages are bounded to 8 MiB, with no DTD/entities, and unexpected listing elements are refused. Hashes detect corruption, not malicious replacement of an entire archive: retain independently backed-up evidence and review the installed source/provider release before migration.

For the observed MinIO `RELEASE.2025-09-07T16-13-09Z`, select `--source-listing-dialect minio-v2` during export. Its encoded key fields use `+` for spaces and `%2B` for literal plus signs; raw/unencoded fields and version IDs are unchanged. Its next-key marker can append a [v2 cache suffix](https://github.com/minio/minio/blob/07c3a429bfed433e49018cb0f78a52145d4bedeb/cmd/metacache-marker.go). The explicit dialect accepts only the observed `return:` or UUID/pool/set forms after the exact last key, still requiring its exact version ID. It sends the complete marker back and verifies the response echo. Unknown suffixes/dialects are refused. Repeated version identities stop capture immediately even if the provider changes its cache token. Both scans retain the dialect so offline checks reproduce the same interpretation; existing evidence without that field retains the previous default. This option applies only to source export, not SeaweedFS destination verification.

Equal timestamps between non-null versions retain the provider's original mixed wire order. SeaweedFS 4.48 lists `null` last even when it is latest; the checker places that version using a distinct timestamp or an independently decisive latest flag. A latest null is newest even when tied; a non-latest null whose only tied peer is latest follows that peer. Other null ties have multiple possible positions and fail without a completed archive, as do inconsistent latest/order, changing inventory and defective legacy pagination. Do not guess order from IDs, rewrite timestamps or remove evidence to bypass refusal. Inspect the original provider/history and obtain independent chronology evidence for unsupported cases. Two agreeing scans validate observed listings; they cannot prove a defective provider exposed every stored version, so production source inventory/restore remains a separate gate.

Archive schema 1 remains readable with its original unique-timestamp requirement. The application version map remains schema 1. New copies no longer sleep between versions: destination verification now checks every historical position using the mixed listing, in addition to hashes, headers, tags and latest/deleted state. A later re-export retains evidence even when copied versions share timestamps.

## Controlled production procedure — prepared, not executed

Complete the [storage migration gates](SeaweedFSMigration.md) first. Freeze source and destination writers, lifecycle expiry and mutable tags throughout the final export/copy/verification. Use workload-appropriate private AWS profiles or runtime environment credentials; do not put credentials in command arguments. Archive directories contain private document data, so keep them outside Git with restricted access and independent backups. The CLI sets umask 0077 and refuses to reuse an existing export directory.

```bash
# Read-only source export. The archive path must not exist yet.
python3 scripts/storage/migrate_versions.py export /private-backups/documents-final \
  --endpoint http://svartalfheim:9000 --bucket documents --profile migration-source \
  --source-listing-dialect minio-v2 --writers-quiesced

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

`version-map.json` is the verified result; `import-journal.jsonl` records acknowledged writes and `multipart-journal.jsonl` records upload attempts, acknowledgements and uncertainty. Neither journal is an activation map. Use the [read-only reference exporter and coverage checker](StorageReferenceCoverage.md) to reconcile structured historical references against the map. Resolve every reported gap and audit the recorded scope, then prove an isolated database/application restore. Upload the reviewed map to a reserved key such as `.migration/version-map.json` in the versioned destination documents bucket, with `application/json` content type. Record the immutable version ID returned by that upload, then set both values in the reviewed orchestrator chart/Flux configuration:

```yaml
env:
  S3_VERSION_MAP_KEY: .migration/version-map.json
  S3_VERSION_MAP_VERSION: "<actual immutable mapping-object version>"
```

The map is bounded to 32 MiB, loaded once at startup through existing bucket-scoped credentials, and pinned to that exact object version. A missing/unreadable/invalid configured map prevents startup. Keep its version and backup for the lifetime of historical references. Never replace the map pin with an unverified upload. Do not commit private manifests or object bodies to the repository. Publish reviewed cutover values through CI/CD only after the restore/reference gates pass.

## Refusal boundaries and recovery

- This is a frozen export and fresh-destination copy, not a live incremental migration. Changed source inventory during export fails without a completed manifest. Equal timestamps require the retained ordering evidence described above; ambiguous null versions and inconsistent provider listings remain refused.
- Objects at least 64 MiB use sequential multipart upload with seekable file windows and reads bounded to 1 MiB. Parts default to 64 MiB and grow to stay within 10,000 parts, with a 5 GiB maximum part size. These are protocol bounds, not a claim of provider capacity; the real acceptance fixture is 5 GiB + 17 bytes. Object lock/legal hold, customer-key encryption and non-standard storage classes require a separately validated procedure. Source bucket policies, lifecycle rules, encryption configuration, ACLs and retention must be inventoried separately; they are not cloned by the data tool.
- Interrupted imports stop. A version may have been written even if its acknowledgement or journal entry was lost. Do not rerun into the same populated target or delete source data. Inspect the journal and destination inventory, retain evidence, then recover using a separately prepared empty destination. No automatic source cleanup is implemented.
- Verification requires the destination to contain exactly the archived versions. Upload the mapping object only after that verification; subsequent application writes intentionally invalidate an exact frozen-copy comparison.
- Before destination writes, rollback retains the original source configuration/data. After writes, source and destination diverge: freeze writers, preserve both histories and prove a reconciled restore plus composed version mapping before switching back. Automatic rollback into the populated source is intentionally unavailable.

Imports refuse pre-existing unfinished multipart uploads and any prior import journal/map. SDK mutation retries must be disabled. Multipart intent is flushed to disk before initiation, each part and completion. Failures attempt to abort only a known upload ID, even if the journal has stopped accepting writes. An `abort_acknowledged` event does not prove that completion never committed an object. A lost initiation reply can leave an upload whose ID the tool never learned; a lost completion reply can leave a complete immutable version. A hard stop may leave only the preceding intent. Preserve both journals, inspect destination versions and unfinished uploads, and reconcile the affected key before preparing a fresh destination. No automated retry, guessed upload abort or source cleanup is performed.

For recovery after destination writes, export the frozen destination into a new archive and import into a separately provisioned empty recovery store with `--prior-map /private-backups/documents-final/version-map.json`. This composes the original markers with the destination-native markers, verifies every previously mapped version still exists with the same content, and carries post-cutover writes too. The resulting map resolves both generations of references. Repeat the isolated database/application restore before switching the endpoint and map pin through CI. Retain both populated stores; the tool never merges into or deletes them. Missing historical versions or conflicting aliases stop recovery before any destination write.

These offline checks do not establish Pi disk/mount health, production source coverage, backup recoverability, deployed Flux state or live application readiness.

## Recorded validation — 2026-10-11

MinIO dialect sources `28cd70d` / `70dca55` pass full storage CI [38185792059](https://github.com/labiraus/homelab/actions/runs/38185792059) / [38186045068](https://github.com/labiraus/homelab/actions/runs/38186045068): unit/refusal checks, explicit real database reference cases, target/default chart validation, larger-than-5-GiB migration and actual Go/Node storage clients. The follow-up proves that rotating a cache token cannot prolong a repeated-page loop. Retained production listing evidence also replays offline with the final source.

Read-only production preflight found that the default parser misread MinIO's space encoding and rejected its cache markers. Explicit MinIO interpretation now passes actual full-bucket listing with page sizes 1,000 and 17 (one and seven pages): 119 readable versions, 117 current keys, zero delete markers and 618,958 bytes across history. All 116 version references from the independently restored pre-freeze database are present. Private evidence is `.local/lan-recovery-20261011/source-history-minio-preflight/`; the failed default-parser evidence is retained separately. This does not freeze writers, create a migration archive/map, or establish final reference coverage. Synthetic SDK export/offline replay covers Unicode, spaces, literal plus/percent characters, mixed version/delete-marker pagination and opaque version IDs, with malformed boundary/dialect refusal tests.

Ordering follow-up: source `626c905` passed [storage CI 38139117870](https://github.com/labiraus/homelab/actions/runs/38139117870), including 47 unit cases, six explicit database cases, mixed-kind ties across distinct page sizes, every copied/re-exported historical position, provable null positions and ambiguous non-latest null refusal. The complete >5-GiB/Go/Node regression also passed, with hosted Python peak RSS 63,464 KiB. The first hosted attempt exhausted automatic fixture volume slots; the bounded test-only override resolved it. See [increment 25](worldbuilder/IMPLEMENTATION-STATUS.md).

Multipart follow-up: 31 archive/multipart/reference unit cases pass (six database cases run through the separate explicit harness). The full disposable SeaweedFS 4.48 run passes lost initiation/part/completion replies, the 5 GiB + 17 byte import through 81 parts, full hash/metadata/tag verification, S3 re-export and real Go/Node clients. Python peak RSS was 61,348 KiB; local log `/tmp/homelab-multipart-acceptance-final.log` is ephemeral. CI evidence is recorded in [increment 24](worldbuilder/IMPLEMENTATION-STATUS.md). This closes the tested large-object scope independently of source-order ambiguity and deployment capacity.

Local archive/refusal tests: 9 passed. Orchestrator `go test -race ./...` passed; processor build and 44 unit tests passed (the integration-only test is deliberately skipped in that command). The full disposable storage harness then passed both actual-client tests with no skips, version copy/reconciliation and recovery with composed aliases. Local log: `/tmp/homelab-storage-acceptance.log` (ephemeral). The run used the checksum-verified SeaweedFS 4.48 binary and temporary extracted Debian systemd libraries; no host installation or LAN access was required.

All 20 storage chart combinations, both Velero configurations and the existing 26 Worldbuilder chart combinations passed. Actionlint 1.7.7 passed the changed workflows with shellcheck disabled. No production data, Secrets, schemas or feature flags were changed directly. CI publication results are recorded separately from these local observations.

Source checkpoint `8ca05dd` is published on `main`. Hosted checks and publication passed:

| Workflow | Evidence |
| --- | --- |
| Storage Migration Acceptance | [38130220596](https://github.com/labiraus/homelab/actions/runs/38130220596): full disposable migration/recovery and actual Go/Node clients, archive refusal tests and storage chart combinations |
| Orchestrator CI/CD | [38130220730](https://github.com/labiraus/homelab/actions/runs/38130220730): unit tests, required storage gate, image and matching chart publication |
| Processor CI/CD | [38130220746](https://github.com/labiraus/homelab/actions/runs/38130220746): build/tests/audit, required storage gate, image and matching chart publication |
| External CI/CD | [38130232254](https://github.com/labiraus/homelab/actions/runs/38130232254): tests, image and matching chart publication |
| Helm CI/CD | [38130220634](https://github.com/labiraus/homelab/actions/runs/38130220634): PostgreSQL chart publication and external pipeline dispatch |

These results establish publication through the requested CI/CD path. They do not establish installed values, Flux reconciliation, production object migration or Worldbuilder activation.

The first post-publication [public endpoint check 38130239942](https://github.com/labiraus/homelab/actions/runs/38130239942) failed. Additional automatic app-publication probes were still running at this documentation checkpoint. No authenticated application acceptance or installed-state observation was obtained. Earlier certificate/routing evidence remains in [public validation](worldbuilder/PUBLIC-VALIDATION.md); the fresh failure alone does not identify a new cause or prove Flux is stalled. Production cutover and activation remain gated.
