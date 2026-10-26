# orchestrator

Worldbuilder index projections retain original JSON number tokens in derived record text, including full-range signed fictional dates. The index remains rebuildable and external rechecks candidates against authorised canon; see the [date wire contract](../../docs/worldbuilder/api.md).

Optional Worldbuilder outbox relay and canon projection live in `worldbuilder.go`. `WORLDBUILDER_RELAY_ENABLED=true` requires `WORLDBUILDER_RELAY_DATABASE_URL` (nonprivileged `worldbuilder_relay`), `WORLDBUILDER_OPENSEARCH_URL` and existing `NATS_URLS`. Both canon and generation relay publishes use a five-second context deadline, preserving shutdown cancellation without conflicting JetStream timeout options. The relay acknowledges durable publish before marking delivered; projection uses external revision versions and can rebuild with `WORLDBUILDER_REBUILD_ON_START=true`. See [API/events](../../docs/worldbuilder/api.md). Disabled by default. Existing document queue semantics below are retained.

Canon index startup retries twelve checks with five-second gaps while remaining unready; each HTTP request retains its ten-second timeout and shutdown cancels the wait. Persistent refusal still fails startup. Keep the OpenSearch ingress allowlist aligned in both its child chart and the Flux data parent.

Run `python3 scripts/worldbuilder/test-canon-index.py /tmp/canon-index-result` from the repository root for actual disposable PostgreSQL → JetStream → OpenSearch acceptance, including duplicate delivery, stale-event refusal and reconstruction after lexical/vector index loss. Optional [semantic projection](../../docs/worldbuilder/api.md) uses immutable published snapshots, a shared pinned-model adapter, changed-record vector reuse, separate serialized background throughput and broker heartbeats. It remains disabled until explicitly configured and rebuilt. See [canon index acceptance](../../docs/testing.md) for isolation and evidence limits.

Generation startup refuses an existing stream unless it uses three replicas, file storage and seven-day limits retention with the expected subject. Its durable consumer explicitly requires three replicas. Run `bash scripts/worldbuilder/test-jetstream.sh` from the repository root for the CI-gated, disposable three-node leader/quorum/restart drill. See [broker recovery](../../docs/recovery.md) for coverage and remaining LAN gates.

`orchestrator` is the internal Go control-plane service.

It owns reconciliation and workflow decisions, including when documents should be queued for asynchronous processing.

The current implementation accepts S3-backed document references, reconciles S3 bucket inventory into Postgres, writes processable control-plane rows as `pending`, and publishes JetStream jobs for the `processor`.

`orchestrator` should remain the control-plane owner as retrieval quality, assistant proposals, file-type support, and recovery workflows grow. It should not become an extraction worker, LLM service, or public query surface.

When work is queued, the orchestrator emits the `documents.events.processor.queued` lifecycle notification and appends the same event to `rag.document_lifecycle_events` on a best-effort basis. The durable queueing decision remains the `rag.documents` row and the JetStream job publish; lifecycle history must not block successful queueing after that transaction commits.

## Endpoints

- `POST /documents`
- `POST /documents/create-text`
- `POST /documents/curation`
- `POST /documents/edit-text`
- `POST /documents/revert`
- `POST /documents/scan-bucket`
- `POST /documents/reprocess`
- `/readiness`
- `/liveness`

The health endpoints are provided by [pkg/api](../pkg/api).

## Document API

All mutation routes are private and reached through external. Submission references
an existing S3 object; create/edit receive text. The supported type policy is
`text/*`, including HTML. MinIO-compatible environment names serve either the
retained source or SeaweedFS target.

| POST path | Required body | Optional body |
| --- | --- | --- |
| `/documents` | `documentId`, `bucket`, `objectKey`, `sourceUri`, `contentType` | `versionMarker`, `etag`, `sizeBytes`, `lastModified`, `metadata`, `processingVersion` |
| `/documents/scan-bucket` | — | `bucket` (default `MINIO_BUCKET`), `prefix`, `maxKeys`, `processingVersion` |
| `/documents/curation` | `documentId`, object `metadata` | `replace` (default false: merge; true: full replacement) |
| `/documents/create-text` | `objectKey`, `text` (empty allowed) | `documentId` (default `s3://<bucket>/<objectKey>`), `contentType` (default `text/plain; charset=utf-8`), `metadata`, `processingVersion`, `actorEmail`, `conversationId`, `proposalId` |
| `/documents/edit-text` | `documentId`, `text` (empty allowed) | `contentType` (current type), `metadata`, `processingVersion`, `actorEmail`, `conversationId`, `proposalId` |
| `/documents/revert` | `documentId`, S3 `versionMarker` | `contentType` (current type), `metadata`, `processingVersion`, `actorEmail`, `conversationId`, `proposalId` |
| `/documents/reprocess` | `documentId` | `processingVersion` |

Scans use version listings and `IsLatest`, retaining literal `null`. Historical
versions and delete markers are excluded before applying `maxKeys`. Missing version
identity/listing refusal fails. Unchanged objects keep status and the highest
desired/completed generation; changed objects advance beyond both. Unsupported
files remain inventoried. Exact fields and worker fencing are in [documents](../../docs/documents.md).

Curation changes metadata without rewriting objects/chunks. Create/edit/revert
record `rag.document_change_audits`; edit preserves bucket/key and adds
`editedBy: orchestrator.editText`. Revert reads a historical version and writes it
as a new current version. Edit, revert and reprocess default to the next derived
generation and reject unsupported inventory types. Empty text is an intentional
valid create/edit, distinct from MCP's nonblank text-upload contract.

## Configuration and checks

Configure PostgreSQL/NATS and `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`,
`MINIO_SECRET_KEY`, `MINIO_USE_SSL`, `MINIO_REGION`, `MINIO_BUCKET`.
Optional `S3_VERSION_MAP_KEY` and `S3_VERSION_MAP_VERSION` must both pin a verified
immutable mapping-object version in that bucket. Startup validates schema,
endpoint/bucket and identities; mapped reverts validate hash/size and reject delete
markers. New writes use destination-native IDs. With both unset, native reads
are unchanged. Follow [storage](../../docs/storage.md) before activation.

Run `go test ./...` and `go build ./...` here. Use the owned PostgreSQL, canon-index,
broker and storage gates linked above for actual service boundaries. Ordinary
unit success may skip endpoint-dependent cases and does not prove live acceptance.

Optional Worldbuilder generation dispatch requires schema 10 (aligned with current external), the existing relay flag and `WORLDBUILDER_JOBS_ENABLED=true`, `WORLDBUILDER_JOB_TOKEN` and `WORLDBUILDER_JOB_API_URL`. It relays metadata-only generation outbox events and invokes external's fixed worker endpoint one checkpoint at a time. Dedicated relay grants cannot read private jobs or mutate canon. Both jobs flags default off; configure the dedicated secret, verified mesh identity and conditional network path using the [LAN runbook](../../docs/operations.md). Orchestrator publication runs the disposable migration harness, including a startup/schema probe that stops before broker activation; schema and harness changes trigger this gate too.

Canon event decoding accepts explicit initial revision 0 and rejects omitted/null or negative revisions. New worlds now reach both lexical and optional vector projections through the ordinary durable relay; prior revision-0 worlds without events remain covered by the explicit rebuild path.
