# orchestrator

`orchestrator` is the internal Go control-plane service.

It owns reconciliation and workflow decisions, including when documents should be queued for asynchronous processing.

The current implementation accepts MinIO-backed document references, reconciles MinIO bucket inventory into Postgres, writes processable control-plane rows as `pending`, and publishes JetStream jobs for the `processor`.

## Endpoints

- `POST /documents`
- `POST /documents/scan-bucket`
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

Runtime configuration now expects both Postgres/NATS settings and the standard documents-bucket MinIO settings:

- `MINIO_ENDPOINT`
- `MINIO_ACCESS_KEY`
- `MINIO_SECRET_KEY`
- `MINIO_USE_SSL`
- `MINIO_REGION`
- `MINIO_BUCKET`
