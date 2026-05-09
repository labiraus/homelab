# orchestrator

`orchestrator` is the internal Go control-plane service.

It owns reconciliation and workflow decisions, including when documents should be queued for asynchronous processing.

The current implementation accepts MinIO-backed document references, reconciles MinIO bucket inventory into Postgres, writes processable control-plane rows as `pending`, and publishes JetStream jobs for the `processor`.

## Endpoints

- `POST /documents`
- `POST /documents/curation`
- `POST /documents/edit-text`
- `POST /documents/scan-bucket`
- `POST /documents/reprocess`
- `/readiness`
- `/liveness`

The health endpoints are provided by [pkg/api](/workspaces/homelab/apps/pkg/api).

## Document Submission Contract

`POST /documents` currently expects a reference payload, not inline text.

Required fields:

- `documentId`
- `bucket`
- `objectKey`
- `sourceUri`
- `contentType`

Optional fields:

- `versionMarker`
- `etag`
- `sizeBytes`
- `lastModified`
- `metadata`
- `processingVersion`

This slice supports MinIO-backed `text/*` objects only.

## Bucket Scan Contract

`POST /documents/scan-bucket` reconciles objects from the configured documents bucket into `rag.documents`.

Optional fields:

- `bucket`, defaulting to `MINIO_BUCKET`
- `prefix`
- `maxKeys`
- `processingVersion`

The scan:

- inventories each object with bucket, object key, source URI, ETag or version marker, size, last modified timestamp, content type, and desired processing version
- marks non-`text/*` objects as `unsupported`
- preserves status for unchanged known objects while refreshing `last_reconciled_at`
- queues new or changed `text/*` objects through the same Postgres plus JetStream path as `POST /documents`

## Curation Contract

`POST /documents/curation` updates curated metadata for an existing inventory document without touching the raw MinIO object or derived chunks.

Required fields:

- `documentId`
- `metadata`, a JSON object

Optional fields:

- `replace`, defaulting to `false`. When omitted, the metadata object is merged into existing `rag.documents.metadata`; when true, it replaces the full metadata object.

## Text Edit Contract

`POST /documents/edit-text` overwrites the raw MinIO text object for an existing inventory document and queues a newer processing version.

Required fields:

- `documentId`
- `text`, which may be an empty string when intentionally clearing a text object

Optional fields:

- `contentType`, defaulting to the current document content type and still limited to `text/*`
- `metadata`, merged into existing document metadata alongside `editedBy: orchestrator.editText`
- `processingVersion`, defaulting to the next version after the current desired or completed version

The endpoint rejects non-`text/*` inventory rows, writes the replacement object to the existing bucket and object key, captures the new object ETag/version metadata, and reuses the normal pending-row plus JetStream queue path.

## Reprocess Contract

`POST /documents/reprocess` queues an existing inventory row for a newer processing version.

Required fields:

- `documentId`

Optional fields:

- `processingVersion`, defaulting to the next version after the current desired or completed version

The endpoint reads the existing inventory row from Postgres, rejects non-`text/*` documents, preserves the raw MinIO source metadata, and reuses the normal pending-row plus JetStream queue path.

Runtime configuration now expects both Postgres/NATS settings and the standard documents-bucket MinIO settings:

- `MINIO_ENDPOINT`
- `MINIO_ACCESS_KEY`
- `MINIO_SECRET_KEY`
- `MINIO_USE_SSL`
- `MINIO_REGION`
- `MINIO_BUCKET`
