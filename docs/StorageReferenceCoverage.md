# Storage reference coverage

`scripts/storage/reference_coverage.py` prepares the database-reference gate for
the [SeaweedFS migration](SeaweedFSMigration.md). It exports structured version
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

Finish the [archive/copy verification](StorageVersionMigration.md) first. The
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

## Recorded acceptance — 2026-10-11

Source `c0fda1b` passed [storage CI 38137124323](https://github.com/labiraus/homelab/actions/runs/38137124323): 20 archive/reference unit tests, six explicit disposable PostgreSQL cases, storage chart validation and the existing complete SeaweedFS migration/recovery plus real Go/Node application clients. Local database acceptance also passed (`/tmp/homelab-reference-postgres-final.log`, ephemeral). These results prove the offline tooling gate; no production inventory or cutover was performed.
