# external

Fictional record dates accept exact decimal strings as well as legacy integer JSON tokens across the signed 64-bit range. Responses/history remain numeric and equivalent representations retain canonical idempotency. See the [date wire contract](../../docs/worldbuilder/API.md); this requires no schema migration.

External now contains the default-disabled Worldbuilder canon engine, authorised wiki/history, proposal/review/publication, bounded reasoning proxy and persistent story APIs under `/api/worlds`. See [API contract](../../docs/worldbuilder/API.md). `WORLDBUILDER_ENABLED=true` requires schema version 9 and a dedicated `worldbuilder_external` login supplied through `WORLDBUILDER_DATABASE_URL`. Startup never migrates. Existing document and assistant routes below remain compatible.

GET canon search retains canonical lexical ranking. Optional POST hybrid search and large-world AI evidence use the shared pinned-model vector adapter, canonical reconciliation and existing durable allowance. See [semantic configuration and limits](../../docs/worldbuilder/SEMANTIC-SEARCH.md). It remains disabled by default. The [disposable canon index gate](../../docs/worldbuilder/INDEX-ACCEPTANCE.md) tests actual OpenSearch retrieval and relay/rebuild without production endpoints.

`POST /api/worlds/{world}/timeline-branches` creates a reviewed snapshot branch proposal from an explicit immutable parent revision. It copies the complete timeline-specific record set (at most 99 records), remaps scoped references and retains the shared current world catalog. It never publishes automatically. Published creation origin is available at `/timelines/{timeline}/origin?revision=N`; audit, exact retries, stale/partial rebase refusal and retired identity checks preserve lineage. No new schema is required. See [timeline branches](../../docs/worldbuilder/TIMELINE-BRANCHES.md).

Revision lists/diffs, proposal and story reconciliation, and compensating restore are available through the same authenticated API. Reconciliation compares original/current/proposed records, requires explicit conflict decisions and creates a freshly validated draft; it never transfers approval or changes a session pin. Audit records preserve lineage and exact retry requests. These original history features were introduced in schema 2; current startup requires all migrations through schema 9. Canon lists also accept a validated comma-separated `kind` filter before pagination, allowing complete editor reference catalogs without widening world/timeline/revision scope.

`external` is the public Go HTTP API for the homelab app stack.

It is the browser-facing API for `ui` and should remain a stable surface even as the internal async pipeline evolves behind `orchestrator` and `processor`.

Keep document and assistant workflows on the existing `/api/documents/*` and `/api/assistant/*` shapes. Retrieval, cited context, and assistant calls should not introduce a parallel public RAG API unless the repo plan explicitly changes.

## Endpoints

- `/api/auth/status`
- `/api/auth/providers`
- `/api/auth/users/{email}`
- `/api/assistant/...`
- `/api/users/count`
- `/api/documents/tree`
- `/api/documents/object`
- `/api/documents/move`
- `/api/documents/upload`
- `/api/documents/events`
- `/api/documents/inventory`
- `/api/documents/history`
- `/api/documents/search`
- `/api/documents/context`
- `/api/documents/create-text`
- `/api/documents/curation`
- `/api/documents/edit-text`
- `/api/documents/reprocess`
- `/api/documents/revert`
- `/api/documents/scan-bucket`
- `/readiness`
- `/liveness`
- `/metrics`

Browser application routes are published through `oauth2-proxy` at `/api/...`. The separately gated native MCP authorization surface also exposes its bounded discovery/registration/token/revocation endpoints; its consent route remains behind the browser proxy.

## Authentication

`external` now attaches auth middleware from `apps/pkg/api`.

- certificate auth is derived from trusted `X-Forwarded-Client-Cert` details from Istio
- OIDC auth is expected to arrive from `oauth2-proxy` through trusted upstream identity headers, preferring `X-Forwarded-Email` and falling back to related proxy user/email headers or Basic Auth username when needed
- the authenticated email is validated against `auth.users` in Postgres
- `/api/auth/status` returns the resolved mode, email, validity, and invalid reason
- `/api/auth/providers` returns the configured federated login providers for browser clients
- in the current repo choice, `OIDC_LOGIN_URL` should point at the local `oauth2-proxy` browser start URL, not directly at Google

## Document Browser Surface

`external` now also fronts the browser-facing MinIO document browser.

- `/api/documents/tree` returns the immediate folders and files for a prefix in the documents bucket
- `/api/documents/object` streams a document back for inline preview or download
- `/api/documents/upload` accepts multipart uploads for the current folder view
- `/api/documents/inventory` returns Postgres-backed document inventory rows, processing versions, latest lifecycle summary fields, and curated metadata
- `/api/documents/history` returns the durable Postgres-backed lifecycle history for a document, optionally narrowed to one processing version
- `/api/documents/search` runs OpenSearch neural search against the current processed chunk version and returns ranked matches with document metadata, chunk metadata, and citation objects for the source URI plus chunk identity
- `/api/documents/context` uses the same retrieval path and assembles a compact context block with `[1]`, `[2]` style references plus citation objects and any available chunk-level citation hints
- `/api/documents/create-text` proxies text-object creation to `orchestrator` `POST /documents/create-text`
- `/api/documents/curation` proxies metadata-only updates to `orchestrator` `POST /documents/curation`
- `/api/documents/edit-text` proxies guarded text-object replacement requests to `orchestrator` `POST /documents/edit-text`
- `/api/documents/reprocess` proxies explicit processing refresh requests to `orchestrator` `POST /documents/reprocess`
- `/api/documents/revert` proxies MinIO-version-backed raw text reverts to `orchestrator` `POST /documents/revert`
- `/api/documents/scan-bucket` proxies bucket or prefix reconciliation requests to `orchestrator` `POST /documents/scan-bucket`

Inventory, search, and context requests accept optional `documentId`, folder-like `prefix`, and `metadata` filters. Inventory also accepts `status`. The `metadata` filter is an exact-match JSON object applied to `rag.documents.metadata`, for example `{"metadata":{"tag":"runbook"}}`.

History requests require `documentId` and accept optional `processingVersion` and `limit` fields. Results are read from `rag.document_lifecycle_events`, while `rag.documents.last_event_*` remains the quick inventory summary.

When `processor` extracts HTML, retrieval hits can also include chunk metadata such as the source page title and heading path. Citation labels use that metadata when available, but older text chunks still fall back to the existing `path chunk N` label shape.

Control action requests require `ORCHESTRATOR_BASE_URL` and preserve the orchestrator response status and JSON body. `external` validates method, configuration, and JSON shape, but the orchestrator remains responsible for document existence checks, raw text writes, version decisions, metadata persistence, bucket reconciliation, and queueing.

The MinIO-backed browser routes expect the standard MinIO runtime configuration:

- `MINIO_ENDPOINT`
- `MINIO_ACCESS_KEY`
- `MINIO_SECRET_KEY`
- `MINIO_USE_SSL`
- `MINIO_REGION`
- `MINIO_BUCKET`

The UI treats these document routes as authenticated functionality for recognized users, and the API is expected to stay behind the same trusted auth middleware as the rest of the browser-facing surface.

Semantic search and cited context assembly use OpenSearch neural search:

- `OPENSEARCH_BASE_URL`
- `OPENSEARCH_RAG_INDEX`, defaulting to `rag-documents`
- `OPENSEARCH_RAG_SEARCH_PIPELINE`, optional; leave empty for lexical retrieval
- optional `OPENSEARCH_USERNAME` and `OPENSEARCH_PASSWORD`

Document control proxying also uses:

- `ORCHESTRATOR_BASE_URL`

## Assistant Edge Surface

`/api/assistant/...` proxies the authenticated browser assistant API to `processor`, which owns assistant state, prompt/tool orchestration, model calls, and tool-call logging.
The proxy forwards the resolved authenticated email through `X-Forwarded-Email` and `UserID` so processor state remains scoped by user.

The assistant edge proxy expects:

- `PROCESSOR_BASE_URL`

## Document Event Stream

`external` also exposes an authenticated server-sent events stream at `/api/documents/events`.

- authenticated browser clients connect with `Accept: text/event-stream`
- each event payload is the raw lifecycle JSON emitted on NATS under `documents.events.>`
- successful browser uploads publish `documents.events.minio.stored` after the raw object is written to MinIO
- the current lifecycle subjects are `documents.events.minio.stored`, `documents.events.processor.queued`, `documents.events.processor.started`, `documents.events.processor.completed`, and `documents.events.processor.failed`
- the handler sends keepalives so long-lived browser connections stay open through the edge

The document event bridge expects:

- `NATS_URLS`
- `NATS_EVENTS_STREAM` with a default of `document-events`
- `NATS_EVENTS_SUBJECT` with a default of `documents.events.>`

Lifecycle delivery to the browser is best-effort. The document processing path remains successful even if the notification fan-out path is temporarily unavailable.

World names/default timelines/settings are reviewed `world_settings` records. Owner-only administration separately controls archive/reopen, versioned member changes, revocation and ownership transfer; immutable audit binds exact retries. Archived worlds allow reads and membership administration, while canon/story/inference writes are blocked. The database retains immutable record kinds after deletion.

The canon validator also enforces temporal relationship cardinality, mutually exclusive boolean facts and explicit canonical chronology exceptions. Rule changes are ordinary reviewed operations and can be grouped atomically with affected records; see the rule contracts in the Worldbuilder API documentation.

Scoped `/api/worlds/{world}/search` reconciles OpenSearch candidate IDs against authorised PostgreSQL snapshots, with canonical fallback and stable pinned pagination. Large-world AI evidence selection retains rule/settings/reference groups and reports limited coverage. See [API contracts](../../docs/worldbuilder/API.md).

Optional MCP delegation uses only `/api/worldbuilder-tools` with signed, expiring, exact-request identity assertions and active-identity/membership checks. Reads and validated draft proposals are the entire allowlist; human routes remain separately mesh-restricted. Configure only after the [LAN gates](../../docs/worldbuilder/LAN-RUNBOOK.md).

AI activation additionally requires migration-003 inference reservations and explicit `worldbuilder.aiBudget` settings. Daily UTC request/concurrency/allocated-allowance limits are per actor across replicas/worlds, persist across failures/restarts, and use a separate small database pool. See [budget/provenance contracts](../../docs/worldbuilder/API.md); no provider tariff is assumed.

Schema 4 adds owner-private immutable story definitions and atomic turn memory/choices. Story generation checks expected session version before spending allowance and returns that version for later human save. Narrative generation returns prose only; see the API contract.

Durable creative jobs were introduced in schema 5; current external requires schema 9. `WORLDBUILDER_JOBS_ENABLED` remains false by default. Enabling requires configured reasoning/budget plus dedicated `WORLDBUILDER_JOB_TOKEN`; the fixed worker endpoint reloads private human intent and checks current access. Job lists/details/cancellation live under each world's `/generations`; queued output remains a candidate. See the [durable API contract](../../docs/worldbuilder/API.md#durable-creative-generations-schema-5).

Consistency checks now run deterministic validation first and optionally bind a proposal via `changeId`. Structured evidence-backed findings are advisory and audited with model provenance; they never change review status. Stale proposal checks fail before allocating inference.

Worldbuilder endpoints emit fixed request/outcome labels in existing external Prometheus metrics. The [operator guide](../../docs/worldbuilder/OBSERVABILITY.md) covers opt-in ServiceMonitor/network/mesh configuration, provisioned Grafana dashboard, disposable runtime acceptance and privacy-preserving read-only PostgreSQL diagnostics. Private IDs and narrative text never become metric labels. `worldbuilderMonitoring` remains disabled until actual scraper selectors and its dedicated mTLS identity are verified; it does not activate canon.

`/records/{id}/connections` adds authorised, paginated forward/back typed-reference traversal at a pinned timeline/revision/time. Optional `WORLDBUILDER_BROWSER_TEST=1` in the disposable harness runs Chromium against a loopback server defined only in `worldbuilder/browser_test.go`; no runtime auth switch is introduced. [Offline acceptance](../../docs/worldbuilder/OFFLINE-ACCEPTANCE.md) also records the 5,000-record domain benchmark and exact CRD schema checks.

Schema 6 adds reviewed calendar projection and exact `tick` facts. Canon/search metadata carries the requested snapshot calendar even when records are filtered. Life-bound rules accept full-range date strings while preserving historical safe-integer numeric facts. See [calendar contracts](../../docs/worldbuilder/CALENDARS.md).

Historical schema 7 introduced reviewed 1–5 human approval quorums and immutable per-membership decisions. Policy changes require the stricter count; role changes/removal invalidate old votes permanently for future publication. Published approval identities remain historical evidence. See [review policy](../../docs/worldbuilder/REVIEW-POLICY.md).

Historical schema 8 introduced operator-granted superuser accounts, authenticated user registration and audited self-review. At that checkpoint the class did not bypass world membership, archival, validation or the configured distinct-human count. Schema 9 replaces those rules with global Administrator access and explicit reasoned approval-count overrides, while enforcing validation, privacy and archival. User registration remains separate from granting Administrator access. Refresh canon grants and the separately owned `auth-grants.sql` before activation. See [account API](../../docs/worldbuilder/API.md#superuser-accounts-schema-8).

## Native MCP authorization

The opt-in `mcpauth/` package supplies native client registration, S256 PKCE, explicit browser consent and resource-bound tokens while preserving Google website login. It requires an independently migrated schema and isolated `mcp_oauth_runtime` connection; all native flags default off. See [activation, tests and restore handling](../../docs/MCPAuthentication.md). The gateway validates tokens through a private route and forwards only the resulting identity. This does not grant canon access or permit native human review/publication.

External CI also runs real Chromium consent and callback acceptance against an owned TLS loopback fixture. HTTP-only protocol tests cannot detect browser referrer-policy or form-action redirect failures; see the [native authorization test instructions](mcpauth/README.md).

World creation also queues its initial revision-0 canon snapshot in the same transaction. Exact creation retries keep one event; a failed outbox insert rolls back the whole new world. New-world indexing no longer depends on a later reviewed publication. See [initial publication/events](../../docs/worldbuilder/API.md#atomic-publication-and-events).

Current schema-9 governance, immutable established accounts, conversations and offline contracts supersede the earlier role/policy descriptions above. See [PWA implementation](../../docs/worldbuilder/PWA-IMPLEMENTATION.md). Source verification does not imply production activation.
