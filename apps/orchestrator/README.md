# orchestrator

`orchestrator` is the internal Go control-plane service.

It owns reconciliation and workflow decisions, including when documents should be queued for asynchronous processing.

The current implementation accepts MinIO-backed document references, writes the control-plane row in Postgres as `pending`, and publishes a JetStream job for the `processor`.

## Endpoints

- `POST /documents`
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
