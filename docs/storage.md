# Storage

SeaweedFS is the private external S3 target; MinIO is the retained migration and
rollback source. This guide describes the configured architecture and controlled
migration, not a completed production cutover. [Status](status.md) records dated
evidence; [secrets](secrets.md) owns credential custody.

## Target and source boundaries

| Concern | Target |
| --- | --- |
| Host/service | `svartalfheim` (`192.168.8.177`), Ansible-managed `seaweedfs.service` |
| Release | SeaweedFS 4.48; amd64/arm64 downloads pinned by SHA-256 |
| S3 | Private `http://192.168.8.177:8333` on `svartalfheim`, authenticated identities |
| Internal interfaces | All SeaweedFS listeners on `127.0.0.1`; systemd socket proxy exposes only S3 HTTP on the private address; administration over SSH |
| Volume data | `/srv/minio/seaweedfs/volumes` on the existing external disk |
| Metadata | `/var/lib/seaweedfs/master` and `/var/lib/seaweedfs/filer` on the host's native filesystem |
| Buckets | `documents` (versioned), `velero`, `cnpg-backups`, `static-sites-site1` |
| Secrets | `homelab/documents-s3-credentials`, `data/cnpg-seaweedfs-s3`, `velero/velero-s3-credentials` |

The disk mount name is retained to avoid changing mounts and file shares during a storage-engine migration. SeaweedFS must never open `/srv/minio/minio-data` as its own volume directory. Copy through S3, not by copying MinIO's backend files. This single-host design has no storage replication; both volume files and filer/master metadata need independent backups. Hosting a database backup on this host does not back up SeaweedFS metadata itself.

The service requires the disk mount and stops when its mount unit disappears, preventing writes into the root-filesystem fallback directory. Existing NTFS ownership maps to `oliver`; LevelDB metadata stays off NTFS. Check native-filesystem capacity before bootstrap. The current Samba `storage` share exports `/srv/minio`: restrict SMB access to the new `seaweedfs` subtree before use so clients cannot alter volume files. Preserve existing media and Minecraft backup directories.

The official [release](https://github.com/seaweedfs/seaweedfs/releases/tag/4.48), [server flags](https://github.com/seaweedfs/seaweedfs/blob/4.48/weed/command/server.go) and [S3 configuration](https://github.com/seaweedfs/seaweedfs/wiki/Amazon-S3-API) describe the upstream interfaces. Actual compatibility is a validation gate, not an assumption from the S3 label.

## Configuration and compatibility

Run the [bootstrap](#seaweedfs-bootstrap) to install the target alongside the source. Credentials are distinct per workload; application users receive bucket-scoped Read/Write/List actions, not Admin. Bucket bootstrap creates missing buckets and enables document versioning. It never deletes buckets, copies source data, sets lifecycle expiry or applies cluster Secrets.

The Go `minioutil` package and processor's AWS SDK v3 adapter implement S3 client compatibility using the existing `MINIO_*` environment keys. The processor's isolated SDK replacement removes the old MinIO transitive advisories and is checked by wire-contract tests and actual SeaweedFS acceptance; see [dependency validation](testing.md). Public MCP `minio.documents.*` names and `documents.events.minio.stored` subjects remain unchanged. Inventory direct MCP storage configuration and static-site consumers during LAN discovery; the current MCP chart does not declare its own storage credentials.

Application Helm defaults (`external`, `orchestrator`, `processor`), CNPG and `values/velero-values.yaml` retain the source endpoint and existing source Secrets. Each application and CNPG chart has a prepared `values-seaweedfs.yaml`; Velero has `values/velero-seaweedfs-values.yaml`. The application overlays switch endpoint, credentials and private S3 egress together. Helm replaces lists, so their generated Secrets and egress lists are complete. Base/GHCR/ECR renders and both storage modes are checked by `scripts/storage/check_charts.py`.

Routine source publication follows CI/CD; storage cutover requires the separate gates below. After the gates below, merge the reviewed target overlay into the appropriate published chart/Flux values through the normal pipeline; merely adding an overlay file does not activate it. Check live Flux overrides before that cutover. No ad hoc Helm upgrade or direct cluster mutation is part of remote preparation. The presence or publication of an overlay does not prove which configuration is installed.

## Ordered migration gates

1. **Inventory and capacity.** Record the actual source endpoint, bucket policies, users, lifecycle rules, objects, sizes, metadata, version IDs and delete markers. Record all consumers, including CNPG WAL/base backups, Velero, static-site uploads and direct MCP clients. Check free space for both stores and native metadata, disk health/mount behavior, private routing, DNS and Samba restrictions. Take a separately restorable backup of the source and Postgres history.
2. **Provision the destination.** Supply fresh credentials and run the SeaweedFS playbook. Check the generated unit, private listener and loopback internal interfaces. Verify anonymous access fails, each workload can access its bucket, and cross-bucket access is denied. Test service restart and missing-disk refusal on a disposable host before depending on it.
3. **Prove S3 semantics.** Test list/head/get/put/copy/delete, multipart upload, range reads, content type/user metadata, version listing, exact version retrieval, delete markers and reverting a prior version. Exercise the actual Go and Node application clients against a disposable bucket. The local smoke test covers core S3 behavior; it is not a full app or production acceptance test.
4. **Resolve historical version references before copying.** `rag.documents` and audit history contain explicit source object version markers; saved assistant results and legacy/extended event JSON also need inspection. Current lifecycle `processingVersion` values identify derived-index generations, not S3 versions. `apps/orchestrator/revert.go` reads an exact `VersionID`. A normal S3 copy issues new destination version IDs. A current-object-only sync loses history and is insufficient. The [archive/copy tool and pinned compatibility resolver](#historical-version-migration) preserve those markers without rewriting immutable history. Export every required version/delete marker, reconcile the complete mapping against actual database/audit/event references, and prove historical reverts with the real clients before cutover. Tool implementation is not evidence that production references are covered.
5. **Copy and reconcile.** Copy every required object/version through the authenticated S3 APIs. Preserve content type, metadata, ordering and delete semantics. Compare full content hashes and manifests; ETags alone are not reliable checksums for multipart data. Keep independent reports for missing versions, failed copies and metadata differences. Do not apply expiry during migration.
6. **Validate backups by restoring.** Copy complete CNPG base-backup and WAL chains, then restore into a disposable database and verify recovery. Validate a new backup/WAL archive against SeaweedFS too. Copy Velero backup data and restore into an isolated namespace/cluster with the intended plugin. Keep old backup storage available until retention and restore coverage are satisfied. The existing 90-day source policies are not automatically transferred; decide retention only after restore evidence exists.
7. **Cut over once.** Quiesce uploads, document edits, processors and backup writers. The initial archive/copy tool requires writers frozen for the entire final export/copy/verification into an empty destination; it does not implement incremental live sync. Install the verified map as a pinned, versioned S3 object and configure both orchestrator mapping variables. Apply generated destination Secrets, publish matching app charts and data/backup settings through the normal CI/Flux path, then verify installed values and pod revisions. Resume one consumer at a time and check uploads, reads, processing, authenticated browser delivery, historical reverts, CNPG archiving and Velero backups. Do not expose the S3 endpoint or internal APIs publicly.
8. **Observe and retire.** Keep the source read-only for an agreed observation window and verify fresh restore points on SeaweedFS. Only then schedule separate source-service retirement and data disposal. Neither is part of bootstrap or cutover.

## Rollback

Before destination writes, restore the recorded endpoint/Secret/chart revisions and resume the unchanged source. Keep the old Secrets and source configuration available through the observation window.

After destination writes, freeze writers first. Inventory and reconcile new SeaweedFS objects, versions and database references before switching back; merely changing port 8333 back to 9000 would discard accepted changes. The tool supports recovery to a fresh store with `--prior-map`, composing original and intermediate markers while preserving new writes. It has no automatic rollback into a populated source or automatic resume after an uncertain copy. Prove the recovery procedure with an isolated database/application restore before cutover. Keep the source and destination backups independently recoverable.

## SeaweedFS bootstrap

Put all ten nonempty credentials in local `ansible/.env` (do not commit them):

```text
SEAWEED_ADMIN_ACCESS_KEY=...
SEAWEED_ADMIN_SECRET_KEY=...
SEAWEED_DOCUMENTS_ACCESS_KEY=...
SEAWEED_DOCUMENTS_SECRET_KEY=...
SEAWEED_CNPG_ACCESS_KEY=...
SEAWEED_CNPG_SECRET_KEY=...
SEAWEED_VELERO_ACCESS_KEY=...
SEAWEED_VELERO_SECRET_KEY=...
SEAWEED_SITE1_ACCESS_KEY=...
SEAWEED_SITE1_SECRET_KEY=...
```

Use separate access keys and secrets for every identity. Check the mounted disk, native metadata capacity and Samba subtree access described in the migration plan, then run:

```bash
make ansible-seaweedfs-host
```

`make bootstrap-svartalfheim-storage` is equivalent. The playbook installs the
checksum-pinned binary, private `/etc/seaweedfs/s3.json` (0600), persistent filer
configuration and mount-dependent units. It creates missing buckets and enables
document versioning without lifecycle expiry. It does not stop MinIO or change
Samba; verify the required backend-subtree exclusions separately.

Generated credentials are `ansible/out/seaweedfs/k8s-secrets.yaml` (0600), installed
only during cutover. Reapply using the same local values; changing them rotates
identities and requires coordinating consumers. Never use the legacy MinIO secret
fetch to populate SeaweedFS credentials. An offline syntax check is:

```bash
ANSIBLE_ROLES_PATH="$PWD/ansible/roles" ansible-playbook \
  -i ansible/inventory/hosts.ini ansible/playbooks/seaweedfs-external-pi-host.yml --syntax-check
```

The disposable checks below require the `systemd` package's socket utilities and
prove client/proxy behavior, not the Pi's systemd lifecycle, NTFS or power-loss safety.

## Legacy MinIO and Samba bootstrap reference

The following describes the old bootstrap flow, retained for migration/rollback context. `make bootstrap-svartalfheim-storage` now invokes SeaweedFS; use the explicit `make ansible-minio-host` or `make ansible-minio-state` only when operating the legacy source, with a separately installed `mc` client.

`make ansible-minio-host` reapplies the legacy host playbook and explicitly refreshes existing MinIO root credentials. It also requires the existing Samba password in local `ansible/.env`. The old first-install auto-detection wrapper has been replaced; use the SeaweedFS bootstrap for new storage setup.

## Mount Drift Recovery

If CNPG WAL archiving starts failing with `SlowDownWrite`, check the `svartalfheim` host before changing MinIO users or bucket policy:

```bash
make minio-host-checks
```

Known bad pattern:

- `/dev/sda2` with UUID `C2629529629522E9` is still present
- `/srv/minio` reports `not-mounted`
- MinIO is still configured for `/srv/minio/minio-data`
- host logs show `Storage resources are insufficient for the write operation` or `no online disks found`

That means MinIO has fallen back to the root-filesystem directory instead of the external NTFS disk.

Recovery sequence:

```bash
set -euo pipefail
ssh svartalfheim 'sudo mount /srv/minio'
ssh svartalfheim 'mount | grep "/srv/minio" && df -h /srv/minio'
ssh svartalfheim 'sudo systemctl restart minio'
make minio-host-checks
make document-platform-checks
```

If the mount does not come back cleanly, stop and inspect the host disk state before reapplying Ansible or rotating any MinIO credentials.

## Historical version migration

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

Complete the [storage migration gates](#ordered-migration-gates) first. Freeze source and destination writers, lifecycle expiry and mutable tags throughout the final export/copy/verification. Use workload-appropriate private AWS profiles or runtime environment credentials; do not put credentials in command arguments. Archive directories contain private document data, so keep them outside Git with restricted access and independent backups. The CLI sets umask 0077 and refuses to reuse an existing export directory.

```bash
set -euo pipefail
# Read-only source export. The archive path must not exist yet.
python3 scripts/storage/migrate_versions.py export /private-backups/documents-final \
  --endpoint http://192.168.8.177:9000 --bucket documents --profile migration-source \
  --source-listing-dialect minio-v2 --writers-quiesced

# Entirely offline hash/manifest verification.
python3 scripts/storage/migrate_versions.py check /private-backups/documents-final

# Writes only to the already provisioned, empty destination bucket.
python3 scripts/storage/migrate_versions.py import /private-backups/documents-final \
  --endpoint http://192.168.8.177:8333 --bucket documents --profile migration-target --writers-quiesced

# Repeat remote verification before installing the mapping object or resuming writers.
python3 scripts/storage/migrate_versions.py verify /private-backups/documents-final \
  --endpoint http://192.168.8.177:8333 --bucket documents --profile migration-target
```

Use the exact same endpoint origin and bucket name as the application configuration. The general copier can target another bucket for isolated tests; the runtime resolver intentionally requires the original and destination bucket names to match. It rejects a hostname/IP alias mismatch instead of silently applying a map to the wrong store.

`version-map.json` is the verified result; `import-journal.jsonl` records acknowledged writes and `multipart-journal.jsonl` records upload attempts, acknowledgements and uncertainty. Neither journal is an activation map. Use the [read-only reference exporter and coverage checker](#database-reference-coverage) to reconcile structured historical references against the map. Resolve every reported gap and audit the recorded scope, then prove an isolated database/application restore. Upload the reviewed map to a reserved key such as `.migration/version-map.json` in the versioned destination documents bucket, with `application/json` content type. Record the immutable version ID returned by that upload, then set both values in the reviewed orchestrator chart/Flux configuration:

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

## Database reference coverage

`scripts/storage/reference_coverage.py` prepares the database-reference gate for
the [SeaweedFS migration](#ordered-migration-gates). It exports structured version
references through local `psql`, then checks them offline against the verified
version maps. It never changes database rows, rewrites history, copies objects
or activates a map.

## Export from a consistent read-only snapshot

Use Python 3 and local `psql` with the normal libpq environment, password file or
service configuration. The connection must have SELECT access to every scanned
table and visibility of all rows; application credentials with restricted
visibility are insufficient. The tool disables `.psqlrc`, password prompts and
row filtering, and runs one repeatable-read, read-only transaction. Missing
permissions or enforced row-level security fail the export instead of silently
omitting references. This follows PostgreSQL's documented
[snapshot isolation](https://www.postgresql.org/docs/16/transaction-iso.html) and
[row-security export behavior](https://www.postgresql.org/docs/16/app-pgdump.html).

For final cutover evidence, freeze database/object writers and lifecycle expiry
through database export, S3 archive/copy and verification. The database snapshot
alone does not coordinate an S3 snapshot. The source endpoint below is an
operator-declared binding to the frozen store, not a discovered endpoint.

```bash
# Connection details come from an already configured read-only libpq service.
PGSERVICE=migration_inventory python3 scripts/storage/reference_coverage.py export \
  /private-backups/document-references \
  --source-endpoint http://svartalfheim:9000
```

The directory must not already exist. It is created with mode 0700; files use
0600. Keep it outside Git and protect it as private data. `references.jsonl`
contains table/row locators, JSON paths and object/version identities. It omits
document bodies, prompts, conversation text and unrelated metadata values.
`psql.log` retains private database diagnostics. Only a successful transaction
and matching per-table row counts produce `manifest.json`, which records the
catalog scope, snapshot, capture time, extractor hash and reference checksum.
An interrupted directory without a manifest is not a completed export.

The catalog scan includes non-system ordinary and partition-root tables with
JSON/JSONB (including direct domains) or recognized version-marker columns.
It selects structured values, recognized identity fields and primary keys.
Tables without primary keys use snapshot-local table OID/CTID locators.
Required document tables must be present: `rag.documents`,
`rag.document_change_audits`, `rag.document_lifecycle_events`, `rag.chunks`,
`assistant.messages`, `assistant.tool_calls` and `assistant.file_proposals`.
New matching tables/columns are included automatically, including Worldbuilder
JSON tables when installed. No join against current inventory can discard
historical audit rows whose document was deleted.

Recognized marker names, ignoring case and underscores, are `versionMarker`,
`oldVersionMarker`, `newVersionMarker`, `revertedToVersionMarker`, `versionId`,
`sourceVersion` and `destinationVersion`. Bucket/key context follows nested
objects, but cannot cross sibling results or conflicting document/bucket
identities. JSON encoded inside saved tool-result strings is inspected too.
Duplicate JSON keys, more than 64 nesting levels, over 100,000 visited nodes per
row, rows/references above 16 MiB or database query failures stop sealing the
export. PostgreSQL fetches in batches of 100 rows.

`processingVersion` identifies derived indexes, not S3 versions. Literal S3
version `"null"` is preserved and can be mapped; SQL/JSON null optional audit
edges are counted separately. A current inventory row with no marker is an
unversioned-reference failure. Versioned URIs and malformed JSON-looking saved
results are reported for review rather than guessed from URL encoding or
current inventory.

LAN discovery found legacy inventory rows without markers because ordinary S3
listings omit version IDs. MinIO HEAD also omitted the version header for an
observed literal-null object. The orchestrator scan now uses version listings
and `IsLatest`, excluding deleted/history entries and refusing missing IDs.
After deployment, reconcile current inventory through that normal workflow,
verify derived processing, and take a fresh frozen reference export. This is
not permission to rewrite immutable audit markers or claim cutover complete.

Processor jobs now pass their queued version marker and ETag into S3 reads.
Opaque and literal-null versions remain explicit, and a changed object fails
its conditional read without falling back to the newest content. Legacy events
without source pins retain their existing behavior. Drain source work before
cutover, reconcile mutable inventory against destination-native current IDs,
and verify derived processing before exporting final coverage evidence.

## Check against the verified migration map

Finish the [archive/copy verification](storage.md) first. The
coverage command makes no S3 calls and does not independently prove that a map's
destination bytes exist. Preserve that separate verification evidence.

```bash
python3 scripts/storage/reference_coverage.py check \
  /private-backups/document-references \
  --map /private-backups/documents-final/version-map.json \
  --report /private-backups/document-coverage.jsonl
```

Repeat `--map` for distinct source buckets. Maps must match the declared source
origin, retain the same source/destination bucket name required by the runtime
resolver, fit its 32 MiB limit, and have unambiguous source/alias/destination
identities. Endpoint aliases are not assumed equivalent. Composed recovery
aliases are accepted without changing old database markers.

The report is private and exclusive: it never overwrites prior evidence. Each
reference has a stable `referenceID`, its original locator and a coverage
outcome. The final summary includes map hashes and the export checksum. Only
that summary establishes a completed check. Exit codes are:

- **0:** every recognized reference is covered, optionally with reviewed
  context; absent optional markers are explicitly counted.
- **2:** the check completed but found missing versions, delete-marker targets,
  unmapped buckets, unversioned inventory or unresolved context/formats.
- **1:** invalid, inconsistent, stale, oversized or incomplete evidence, database
  failure, or an output file already exists. Partial reports have no summary.

Checksums detect corruption and bind artifacts; they do not authenticate an
operator or prove facts entered into a hand-written map.

## Resolve missing context without rewriting history

A saved tool argument can retain a version and document ID without a bucket or
key. Do not infer its historical identity from today's inventory. Verify that
identity against frozen source records/backups, then prepare a reviewed file
bound to this exact export. Copy `referenceID` from the failed report and
`sha256` from the export manifest:

```json
{
  "schemaVersion": 1,
  "referenceSHA256": "<export manifest sha256>",
  "reviewedBy": "<reviewing operator>",
  "entries": [
    {
      "referenceID": "<referenceID from report>",
      "bucket": "documents",
      "key": "<verified original opaque key>",
      "reason": "<the evidence used to verify this identity>"
    }
  ]
}
```

Supply it with `--contexts /private-backups/reviewed-contexts.json` and a new
report path. This can fill missing bucket/key fields only. It cannot override a
known identity, change a version, excuse a missing object/delete marker or
bypass an unparseable reference. Duplicate, stale and unused entries fail.
The report distinguishes `covered_with_reviewed_context` from direct coverage,
records the evidence hash, and retains the original reference unchanged.
The `reviewedBy` field is an operator attestation, not an authentication check.

## Disposable acceptance and remaining gates

Run `bash scripts/storage/test-reference-postgres.sh` with Docker, local `psql`,
`socat` and Python 3. It creates a loopback-only PostgreSQL 16.13 database and a
SELECT-only reader, using the repository document/assistant DDL. It checks
preserved database checksums, deleted-inventory audits, saved tool references,
automatic JSON/domain discovery, concurrent-write isolation, duplicate JSON
refusal and RLS/permission failures. The pgvector-only derived embeddings table
is outside this fixture; it contains no structured version-reference columns.
The harness does not load operator environment files and removes its services.

The storage CI gate runs these database checks separately from ordinary Python
tests, where they are deliberately skipped. It also runs the existing real
SeaweedFS migration and Go/Node client acceptance.

Coverage applies to recognized structured contracts in the recorded table
scope. Free prose, opaque binary/custom encodings, views, foreign tables,
external databases and broker-only history are not asserted to be inventoried.
Audit those sources and unusual legacy formats during LAN discovery. Actual
production coverage, verified context attestations, map authenticity, frozen
writer state and isolated database/application restore remain cutover gates.

## Storage chronology evidence

Treat S3 version IDs as opaque. Retain mixed listing/pagination evidence for timestamp ties, and use `IsLatest` when it proves a unique position: a latest null version is not ambiguous merely because its timestamp ties another version. Refuse null ties only when multiple chronological positions remain possible; never infer order from the provider's placement of null last.

The observed MinIO source uses form-encoded listing keys and v2 cache suffixes in pagination markers. Use the explicit `minio-v2` source-listing dialect and retain it with both scans; do not globally reinterpret plus signs or strip provider markers before sending them back. Check apparent `NoSuchVersion` failures against the decoded key and actual SDK read before declaring source loss. SeaweedFS/default evidence keeps its existing interpretation.
