# Design decisions

This is the retained rationale for the current design. Superseded execution plans
and intermediate checkpoints are in Git history. [API](api.md) owns contracts;
[status](../status.md) owns evidence. These decisions do not grant permission to
publish, migrate or retire infrastructure.

| Decision | Rationale and consequence |
| --- | --- |
| Reuse the monorepo | Keep Go external/MCP/orchestrator, Node HTTP/LangGraph processor, React/Vite and existing PostgreSQL. Illustrative separate-repo/Fastify/Cassandra assumptions do not override actual source. |
| Keep one canon writer | External owns authorization, deterministic validation and atomic publication. AI and MCP can generate/draft but never approve or publish. |
| Relational projection plus immutable snapshots | Typed/reference/time columns support integrity and reads; complete historical JSON snapshots preserve pins and restore. The initial O(world-size) rewrite is bounded to 5,000 records; measure before changing storage. |
| Collective authorship | Schema 9 replaces ownership transfer and legacy roles with Authors/Readers. Administrators configure publication policy; reasoned approval-count overrides preserve revision, validation, archive and immutable-account checks. |
| Administrative policy separate from canon | A versioned policy pin prevents old proposals from acquiring new approval semantics. Historical canonical `reviewPolicy` cannot alter active governance. |
| Immutable established accounts | A separate append-only ledger and publication triggers preserve witnessed accounts even through restore/rebase. Clarification/supersession creates new identities; interpretations and linked records remain editable. |
| Explicit pins and reconciliation | Generated and manual drafts keep inspected revisions; story candidates retain session version. Conflicts create fresh drafts/reviews, not silently rebased content. Compensation appends history. |
| Exact fictional time | Full signed-int64 ticks and half-open intervals are independent of real timestamps. JavaScript preserves source tokens/decimal strings; reviewed calendars control presentation at the queried snapshot. |
| Snapshot branches | Copy complete timeline-scoped records and references through review, retaining shared catalog/settings and immutable origin. No automatic parent synchronization, merge automation or partial-copy publication. |
| Derived search | PostgreSQL authorizes and supplies result content. Lexical fallback preserves bounded reads; optional model search has explicit coverage, profile, metering and continuation contracts. Hash embeddings are not semantic fallback. |
| Transactional outbox in orchestrator | Reuse durable relay/indexing rather than adding a service. PostgreSQL checkpoints remain authority; duplicate delivery is expected and projections are rebuildable. |
| Bounded reasoning and advisory findings | Strict structured output/citations constrain capabilities but do not prove prose truth. Deterministic rules govern publication; semantic findings remain uncertain advice. |
| Separate durable accounting | Reservations survive canon rollback and failed calls. Schema 10 adds immutable profile pins; local proxy has zero monetary/daily quota, paid selection is explicit and centrally budgeted. Historical legacy charges are not repriced. |
| Private definitions, scenes and jobs | Immutable premise/pins and versioned turns preserve private play. Durable generation checkpoints parts; uncertain dispatched inference is never automatically repeated or refunded. |
| Request-bound MCP delegation | A short-lived signed assertion restricts audience, actor, operation and exact payload. No database/approval/private-session capability; existing document transports remain supported. |
| Conversation-first PWA | Keep the existing shell and five sections. Shared threads use real persistence; private play stays private. Device writing uses explicit account-bound immutable requests and human conflict resolution. |
| Directly manipulable Ask/Create results | Inline edits, refinement, stable IDs and review/return handoffs use existing APIs. Unsaved edits remain memory-only; no invented durable candidate/task schema. |
| Preserve storage interfaces | The processor's AWS SDK S3 adapter closes the old dependency issue while preserving `MINIO_*` and public tool/event names. Actual Go/Node SeaweedFS checks are required; SDK replacement is not cutover. |
| Gate publication on meaningful tests | Test source, actual images/packaged charts and disposable service boundaries. Public probes observe availability; source labels/digests do not imply signing or Flux reconciliation. |
| Owned offline fixtures | Real PostgreSQL/NATS/OpenSearch/controllers reduce mock gaps. Scoped exec-ID transport resolves local Docker forwarding limits without using production or weakening TLS. |
| Evaluate the exposed retrieval contract | RAGAS defaults to authenticated external retrieval, with explicit legacy comparison. Private gold data stays outside Git; structural fixtures do not prove model quality. |

Changes to these decisions must update the corresponding canonical contract and
relevant acceptance tests. Preserve already published history and source data;
new requirements do not authorize destructive migration or resource retirement.
