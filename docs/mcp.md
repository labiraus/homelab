# Labiraus MCP

`apps/mcp` is a first-class integration surface. Its tools delegate to existing
application APIs. Worldbuilder tools cannot approve, publish, administer worlds
or read another person's private sessions. Native client login is separate from
browser Google authentication; follow [authentication](authentication.md).

Use the [service README](../apps/mcp/README.md) for transports, environment settings
and tool/resource discovery, and the [API delegation contract](worldbuilder/api.md#scoped-mcp-delegation)
for signed request scope. Discovery alone does not prove authenticated reads.
The catalogue in [manifest.go](../apps/mcp/manifest.go) is the source interface;
[status](status.md) records dated deployment evidence.

## Prompt catalogue

Call `prompts/list` to discover arguments, then `prompts/get` with the selected
name. Unknown argument names return invalid params. Omitted optional arguments
render as `not supplied`. A prompt explains a tool; fetching it does not execute it.

| Prompt | Required arguments | Optional arguments | Purpose |
| --- | --- | --- | --- |
| `documents.submit.example` | `documentId`, `sourceUri`, `contentType` | — | Reference an existing S3 text object for ingestion; tool payload includes bucket/key and source/content type. |
| `documents.scanBucket.plan` | — | `bucket`, `prefix`, `maxKeys`, `processingVersion` | Bound a bucket scan, reconcile inventory and queue new/changed supported objects. |
| `documents.reprocess.prompt` | `documentId` | `processingVersion` | Queue a new derived generation; inspect the returned version through history. |
| `documents.curation.update.prompt` | `documentId` | `metadata` | Update exact-match metadata filters through orchestrator. |
| `documents.editText.prompt` | `documentId` | `contentType`, `metadata`, `processingVersion` | Replace a known text object's content and queue refreshed derived state. Replacement text belongs in the tool payload. |
| `documents.context.prompt` | `query` | `prefix`, `documentId`, `metadata`, `limit`, `maxChars` | Assemble compact cited context from processed chunks. |
| `documents.search.prompt` | `query` | `prefix`, `documentId`, `metadata`, `limit` | Retrieve scoped processed chunks and citation objects. |
| `documents.inventory.prompt` | — | `status`, `prefix`, `documentId`, `metadata` | Read inventory independent of matching search chunks. |
| `documents.history.prompt` | `documentId` | `processingVersion`, `limit` | Inspect durable events for a bounded processing attempt. |
| `postgres.auth.userCount.prompt` | — | — | Exercise a simple PostgreSQL-backed auth read; this is not complete application-health acceptance. |
| `minio.documents.browse.prompt` | — | `prefix` | Browse folder-aware S3 tools/resources; the compatibility name remains stable. |
| `documents.notifications.subscribe.prompt` | `documentId` | — | Subscribe to document lifecycle resource updates. |

## Tool behavior

Orchestrator owns document mutation and generations. Scans preserve unchanged
known status, mark unsupported objects and queue new/changed `text/*` objects,
including HTML. A stale inventory prefix may need a fresh scan. Reprocess defaults
to the next generation; pass an explicit version only intentionally. Curation
merges metadata unless the tool's `replace` is true. Text edits write the existing
bucket/key, merge optional metadata, record `editedBy: orchestrator.editText` and
queue the next generation by default. See [documents](documents.md) for ownership,
extraction and failure contracts.

Search/context use OpenSearch. The current shared query builder emits lexical
matching; stored vectors, a configured search pipeline or a prompt's older
“neural” wording alone do not prove semantic ranking. Context references identify
source URI, object key, chunk ID/index and processing version. Inventory exposes
current/desired generations and latest lifecycle summaries; history reads
`rag.document_lifecycle_events` with original payloads.

## Notifications

The lifecycle is `documents.events.minio.stored` (browser upload), then
`documents.events.processor.queued`, `started`, and either `completed` or `failed`.
The full processor subjects retain the `documents.events.processor.` prefix.
These compatibility names do not require MinIO as the target provider.

External, orchestrator and processor publish under `documents.events.>` on NATS
JetStream. Subscribe with `resources/subscribe` to
`homelab://mcp/documents/notifications/{documentId}` and keep the session's
`GET /mcp` SSE stream open with `MCP-Session-Id`. Matching events are forwarded as
`notifications/resources/updated`. Delivery is best-effort: reconnect and read
inventory/history for durable state; notifications are not the audit ledger.
