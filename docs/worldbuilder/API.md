# Worldbuilder API and domain contracts

This guide describes the checked-in Go/SQL/processor contracts. Current enabled
external and generation-worker binaries require schema **10**; apply numbered
migrations and dedicated grants explicitly. No startup hook migrates production.
[SQL inventory](../../sql/worldbuilder/README.md) owns migration details,
[operations](../operations.md) owns activation, and [status](../status.md) owns
publication/deployment evidence.

## Ownership and identity

External is the sole canon writer. Every route requires validated identity and
current world authority; unknown or inaccessible worlds return 404. Current world
roles are `author` and `reader`. Global Administrators use the legacy `superuser`
wire/table name for compatibility, gain Author access to worlds, and remain unable
to read someone else's private sessions or generation jobs.

Only external receives the dedicated `worldbuilder_external` canon runtime login.
The independent migration owner is separate from the legacy shared app role.
`worldbuilder_relay` can read published snapshots and outbox state but cannot publish
canon. Processor and MCP have neither canon credentials nor human publication
capability. Exact browser-proxy mesh identity and sanitized headers are required;
private networking alone cannot establish that boundary. MCP uses the separately
signed delegation endpoint, never browser world routes.

## Governance and established canon

Author and Reader replace the five legacy world roles. Administrators use the
existing `superuser` account field for wire compatibility and have Author access
to every world; they cannot inspect another person's private story or job.
Authors collectively manage membership and archive/reopen. No transfer-owner
workflow remains. The final Author cannot be removed or demoted.

Administrators set a versioned per-world `publicationPolicy` through the audited
administration endpoint: independent (default one, configurable 1–5), direct, or
unanimous. Independent approvals exclude the proposer, including an Administrator.
Unanimous endorsement includes every current assigned Author and an Author
proposer. Global Administrators can contribute reviews but cannot replace missing
assigned Authors under unanimous policy. A reasoned Administrator publication
override skips approval counts only. Stale revisions, stale policy pins,
validation, rejection, archive and immutable accounts still block publication.
Policy changes require fresh proposals; membership changes reevaluate current
authority and unanimous membership. Published approval lists remain historical.

`established_account` records have title (`label`), `synopsis`, `timeline`, optional
exact `from`/`until`, `references` to events/sources, and optional `supersedes` or
`clarifies`. Propose and publish through ordinary governance. The ledger is
append-only; publication inserts ledger entries through a fixed-search-path
trigger, alongside the revision, projection, audit and outbox transaction. Runtime
has SELECT only on the ledger. Revision insertion independently refuses changing
or omitting a prior account, including compensating restores. Clarifications and
supersessions get new identities. The original remains readable and citable;
unique successor links, cycle and timeline validation apply.

Only the account is immutable. Sources, events and interpretations remain editable.
The review response flags changes to referenced records. This is a structural
warning, not a claim to understand prose. AI consistency findings remain advisory.
Reasoning context includes scoped accounts and reviewed guides, instructs the
model to prefer the effective successor and distinguish conflicting reports, and
returns server-derived effective-account citations separately from model prose.
Sources can explicitly say `evidenceStatus: reported` or `unspecified`; existing
sources retain unspecified status. Branch-copy remaps account identities and links
while the existing immutable branch audit retains origin.

## Routes

Orchestrator's derived canon index retains original number tokens in record text, so date searches do not index a rounded value. OpenSearch remains a candidate-discovery aid; external still resolves and authorises all returned records against the pinned canonical snapshot.

Fictional `from`/`until` dates span the full signed 64-bit range (`-9223372036854775808` through `9223372036854775807`). Inputs accept decimal JSON integer tokens or decimal strings; fractions, exponents, whitespace, leading plus/zeroes and overflow are rejected. Missing/null endpoints remain unbounded. Output and persisted canon remain numeric, preserving existing clients, immutable history and canonical retry hashes. Quoted and numeric inputs for the same tick are the same proposal on retry. HTTP `at` query values retain exact decimal text; MCP `at` also accepts numeric or string input.

JavaScript consumers must preserve original JSON tokens before ordinary number conversion. The UI and processor use the [JSON.parse reviver source context](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/JSON/parse): safe integers remain numbers, larger ticks become decimal strings, and edits send those strings back. A browser without source-context support reports an update-required error on numeric dates instead of showing rounded canon. MCP preserves exact numeric tokens through delegated signing and both textual/structured replies. Date formatting and ordering must use integer/BigInt arithmetic, never `Number()` on an unsafe tick. Ordinary `number` fact values retain their existing numeric semantics (including the existing safe-integer restriction for life-bound date facts); the separate schema-6 [calendar and date-fact contract](#calendar-configuration) adds reviewed formatting and an exact `tick` fact type.

All paths below are relative to `/api/worlds`. JSON bodies reject unknown fields, trailing JSON and bodies above 256 KiB. Failures contain `error` and `code`; 401 means invalid identity, 403 denied role, 404 absent/inaccessible scope, 409 concurrency/state/idempotency conflict, 422 validation, 429 exhausted inference allowance/concurrency, 503 disabled dependency. Internal errors do not expose SQL or provider bodies.

| Method/path | Contract |
|---|---|
| GET `/` | Worlds visible through membership or global Administrator authority; paginated |
| POST `/` | `{id: UUID, name}`; idempotent creator retry; initial revision 0, `main` timeline and `world_settings` record |
| GET `/{world}` | World name, current revision, effective caller role, defaultTimeline, archived, adminVersion, publicationPolicy, policyVersion and capabilities |
| GET `/{world}/members` | Author-only paginated principal/role list |
| POST `/{world}/administration` | Author administration `{requestId: UUID, expectedVersion: adminVersion, action, reason, principal?, role?, policy?}` (Administrator required for `set_policy`); actions `set_member`, `remove_member`, `set_policy`, `archive`, `unarchive`; exact audited retries |
| POST `/{world}/members` | Authors only: `{principal: email, role: author/reader}`; final Author cannot be demoted; retained compatibility route also advances adminVersion; new UI uses versioned administration |
| GET `/{world}/canon` | Required `timeline`; optional `revision` (default current), `at` fictional tick, lexical `q`, comma-separated `kind` filter, `offset`, `limit` (1–500, default 100); returns records, resolvable citations, total and revision |
| GET `/{world}/search` | Authorized lexical search, current or historical; required `timeline`, `q`, optional revision/time/page filters |
| POST `/{world}/search` | Optional metered hybrid search: `{revision, timeline, query, at?, limit?, offset?, searchToken?}`; current canon only; [model, allowance, scope and pagination contract](#semantic-discovery) |
| GET `/{world}/records/{id}` | Same required timeline and revision scope; resolves a single canonical citation |
| GET `/{world}/revisions` | Published revision metadata (actor, reason, real timestamp), newest first; paginated |
| GET `/{world}/diff` | Required nonnegative `from`, optional `to` (current); paginated complete before/after records and total across all timelines |
| POST `/{world}/restore` | `{id: UUID, baseRevision, targetRevision, reason}`; draft compensation for an older revision, up to 100 operations; fresh review required |
| POST `/{world}/timeline-branches` | `{id: UUID, baseRevision, parentRevision, parentTimeline, timeline, label, reason}`; complete snapshot branch draft, up to 99 copied timeline records plus its descriptor; [shared catalog and governance contract](#snapshot-branches) |
| GET `/{world}/timelines/{timeline}/origin` | Optional `revision` (default current); published branch creation request/revision and copied-ID map, or `null` for an ordinary isolated timeline; absent historical timeline returns 404 |
| GET `/{world}/changes/{id}/diff` | Complete proposal before/after at its immutable base revision |
| GET `/{world}/changes/{id}/reconciliation` | Original/current/proposed records, record-level conflicts and cross-record validation finding |
| POST `/{world}/changes/{id}/rebase` | `{id: UUID, baseRevision, reason, resolutions: {recordId: current/proposed}}`; `baseRevision` is the preview current revision; creates a fresh draft |
| GET `/{world}/changes` | Latest proposals (paginated), including operations, review and publication state |
| POST `/{world}/changes` | `{id: UUID, baseRevision, reason, operations}`; creates validated immutable draft |
| POST `/{world}/changes/{id}/submit` | `{}`; requesting human submits under the current publication policy after deterministic validation |
| POST `/{world}/changes/{id}/review` | `{decision: approved/rejected, reason}`; requires an eligible Author under the current policy |
| POST `/{world}/changes/{id}/publish` | `{}` or Administrator-only `{overrideReason}`; checks current policy readiness and publishes atomically; exact retries return the original revision |
| POST `/{world}/reason` | `{workflow, question, timeline, revision, at?, references?, changeId?, sessionId?, expectedVersion?}`; workflows `canon_query`, `canon_proposal`, `consistency_check`, `story_turn`, `narrative_generate`; story turns require sessionId/expectedVersion; returns candidate answer/operations/citations/model/usage; never publishes or saves an outcome |
| GET/POST `/{world}/sessions` | List caller's sessions (paginated); create with `{id: UUID, title, pinnedRevision, timeline, definitionId?}` |
| GET `/{world}/sessions/{id}` | Only session owner and current world member can resume |
| GET/POST `/{world}/sessions/{id}/turns` | Paginated turns, newest first; append `{requestId: UUID, expectedVersion, narration, operations, direction?, memorySummary?, choices?}` atomically with state and memory |
| GET `/{world}/sessions/{id}/reconciliation` | Session-owner-only original/current/provisional comparison, including current story `expectedVersion` |
| POST `/{world}/sessions/{id}/reconcile` | Rebase body plus `expectedVersion`; turns explicitly reconciled provisional state into a fresh draft without changing the session pin |
| POST `/{world}/sessions/{id}/outcomes` | `{id: UUID, expectedVersion, baseRevision, reason}`; transforms provisional state into a draft; current publication policy still applies |

List routes accept `offset` and `limit` (1–500, default 100). Published history and revision diffs have page controls. Worlds, canon, proposals, sessions and turns have load-more controls (500 canon records or 100 other rows per request). Canon continuation requests retain the inspected revision and applied filters; editing an unapplied search field does not alter them. Mutable proposal/session/world lists use offset pagination with ID deduplication; refresh to observe concurrent insertions. Editor reference choices fetch the bounded type/entity/source/timeline catalog at one revision independently of wiki filters.

Important writes use client UUIDs as idempotency keys. Reusing keys with different payloads yields 409. Submitted proposal operations cannot change; corrections/rebase create a new proposal and need fresh review. The synchronous validation phase is represented by the transition `draft -> ready_for_review`; no pretend async `validating` state is persisted. Rejecting is terminal for that proposal.

Physical world deletion is not exposed: archive preserves history. Authors collectively archive/reopen and manage members, with final-Author protection. There is no ownership-transfer workflow. Administrators alone set the publication policy. Administration checks `adminVersion` while holding the world lock and audits the full request/result. Archived worlds permit reads but reject writes, including membership administration, until an Author or Administrator reopens them. Existing sessions retain private state/pins; revoked members lose all world access immediately. Name/default timeline/settings changes require ordinary canonical review/publication. Submitted drafts stay immutable.

## Canon model

A world contains at most 5,000 records initially. An immutable identity registry retains record kinds even after deletion; reusing an ID for another kind is rejected, while reviewed restoration of the same kind is permitted. IDs are stable world-scoped `[a-zA-Z0-9_-]{1,64}` strings. Operations are `{action: put, id, record}` or `{action: delete, id}`; 1–100 per proposal, with no duplicate IDs. A put replaces a whole record. References are world-local and must survive final batch validation; composite SQL foreign keys recheck projection integrity.

- `world_settings`: one global record with world name in `label` (max 200), default timeline in `timeline`, and versioned string settings/style guidance in `attributes`. New worlds bootstrap it; existing v1 worlds introduce it through an explicit reviewed proposal. It cannot be deleted, and the default timeline must remain valid. Publication updates world metadata atomically with the same revision.
- `type`: `fields` maps predicate names to string/number/boolean/entity/tick value types. `relations` maps relationship names to allowed target type IDs.
- `entity`: requires type and label; presentation attributes are string pairs.
- `timeline`: named explicit continuity. Reviewed snapshot branches support explicit divergence; no automatic parent inheritance.
- `source`: stable named source evidence record; optional attributes can identify a source object. No URL is fetched by inference.
- `fact`: subject entity, declared predicate/valueType/value, source and timeline. A predicate is single-valued within overlapping validity intervals. Entity-valued facts resolve inside the same snapshot.
- `relationship`: subject, target, type-declared predicate, source, timeline.
- `event`: label, source, timeline and fictional `from` tick; optional subject participant.
- `rule`: versioned natural-language label and optional machine check. `life_bounds` uses `attributes.birth`/`death` date predicates and optional comma-separated `exceptions` boolean predicates. A true canonical exception must be valid for the same participant/timeline at the event tick. Optional record `type`, `timeline`, `from`, `until` narrow applicability. Date facts use `valueType: "tick"` and canonical signed int64 decimal strings; historical `number` facts remain accepted only as exact safe integers. Event comparisons use signed integers.
- `relationship_cardinality` rule: `attributes.check=relationship_cardinality`, required record `type`/`timeline`/`predicate`, and string `attributes.min` and/or `max` in 0–5000. Bounds count simultaneous declared relationship edges for each entity of the subject type in that timeline, intersected with rule/entity validity. Missing min is 0 and missing max is 5000. Adjacent intervals do not overlap; a minimum requires coverage throughout the applicable period.
- `exclusive_facts` rule: required record `type`/`timeline`, `attributes.check=exclusive_facts`, and comma-separated `attributes.predicates` containing 2–20 distinct declared boolean predicates. At most one may be true at any applicable tick for an entity. False values and other timelines do not count. Rule/entity validity narrows the scope. Unknown checks remain rejected; natural-language consistency findings remain advisory.

Fictional `from`/`until` use signed integer ticks and half-open intervals `[from, until)`, unrelated to real `timestamptz` audit timestamps. Missing bounds are unbounded. The API does not assume a Gregorian calendar. Reviewed [fictional calendars](#calendar-configuration) format dates from the requested revision. [Snapshot branches](#snapshot-branches) copy timeline-specific records through review while retaining the shared world catalog; automatic parent inheritance is absent. Cardinality, boolean exclusivity and canonical chronology exceptions are implemented.

Current projections have typed/reference/time columns in `worldbuilder.record`; JSONB holds type schemas, typed scalar values and presentation attributes. Complete immutable snapshots in `worldbuilder.revision` provide historical reads, story pins and compensating revision recovery. The initial projection rewrite is O(world size) and serialises writes per world; measure before optimising.

## Atomic publication and events

World creation atomically queues its initial published snapshot at revision 0 in the same outbox used for later reviewed changes. A repeated successful creation request does not add another event; an outbox failure rolls back the new world, membership, snapshot, projection and audit together. Initial events omit `changeId`. The relay accepts explicit revision 0 and rejects missing/null or negative revisions. Existing worlds without an initial event are covered by the explicit latest-snapshot rebuild.

Within one transaction external locks the world, rechecks current membership/approval/revision, revalidates the complete result, replaces projections, appends immutable revision/audit, advances the world and marks the proposal published, then inserts an outbox event. Any failure rolls the entire transaction back. History triggers reject UPDATE/DELETE. Rollback is a new reviewed compensating proposal, never history rewrite.

Outbox payload version 1: `{eventId, worldId, revision, changeId, schemaVersion: 1}`. Orchestrator uses `FOR UPDATE SKIP LOCKED`, publishes subject `worldbuilder.canon.published` to JetStream stream `worldbuilder-canon`, waits for broker acknowledgement and only then sets `delivered_at`. A crash can redeliver; `Nats-Msg-Id` and idempotent projection handle it. The dedicated consumer is `worldbuilder-index-v1`. The first runtime creates a file-backed stream with three replicas and seven-day transport retention; immutable PostgreSQL snapshots remain recovery authority.

The derived `worldbuilder-canon-v1` OpenSearch index stores one current snapshot document per world with nested record IDs/kinds/timelines/text and explicit `visibility=canon`. External version numbers are `canon revision + 1` with `external_gte`, so older arrivals cannot overwrite newer canon. Rebuild reads the latest immutable revision per world; enable `WORLDBUILDER_REBUILD_ON_START` explicitly for recovery then disable it. `GET /api/worlds/{world}/search?timeline=main&q=...&revision=...&at=...&limit=100&offset=0` checks current membership before discovery. OpenSearch supplies at most 100 candidate IDs with world/revision/visibility metadata; external resolves all content against the requested PostgreSQL snapshot and timeline/fictional tick. Historical queries bypass the current index. Missing, stale, unavailable or invalid projections cannot hide canon or supply answer text: a complete bounded snapshot scan ranks canonical matches by distinct query-term count, then record ID. Queries require 1–64 alphanumeric terms and at most 4,000 bytes. Pagination is stable at a pinned revision across index rebuilds. Results include canonical citations, `discoveryStatus`, optional `indexRevision` and `reconciledCandidates`. This GET contract uses the index as a reconciled hint. The separately enabled POST hybrid contract uses model vectors and the durable interactive allowance; see [semantic search](#semantic-discovery). It reuses external’s existing OpenSearch endpoint/credentials and network path.

## Grounded AI and stories

Processor receives authorised evidence selected by external; no canonical database credential, arbitrary HTTP tool, approval or publish capability is available to the graph. It calls the configured private inference route: the independent proxy when enabled, or the retained legacy gateway. Context is bounded to 100 records/100,000 bytes, with bounded question, answer and model output. Legacy gateway and proxy routes have their own configured deadlines and output limits; see [inference](../inference.md). Processor retains a per-process cap of two concurrent requests; proxy mode has no processor minute quota. External additionally reserves a durable per-actor allowance across worlds and replicas before calling it; the distributed contract is below. Model, returned token usage, request correlation, latency, configured gateway route and allocated allowance are recorded. Underlying provider identity and verified tariffs remain LAN gates; no provider or rate is fabricated.

For timelines above 100 visible records, external selects up to 20 lexical matches, their visible type/source/reference closures and related records, retaining all applicable rules/settings and provisional-state dependencies within the 100-record limit. If required policy/reference groups exceed the budget the request fails rather than silently omitting them. `evidenceScope` reports `selected_records` or `complete_timeline`, selected/total counts and discovery status; external overwrites any model-provided scope. Selected evidence cannot establish global absence or whole-world consistency, and the UI displays that limitation. The graph receives the coverage marker and may cite only supplied evidence.

Strict output validates allowed keys, operation shape, world/timeline/revision and citation IDs. External independently validates canonical references and typed operations. Missing evidence returns explicit uncertainty. These checks prevent capability expansion and invented IDs; they do not prove that every model sentence is semantically supported. Quality, realistic adversarial evaluation and provider compatibility remain LAN gates.

Session definitions (title, owner, timeline) and pinned revision live in PostgreSQL. Turns store narration, request, resulting provisional state and version together. Retries preserve the original response; concurrent expected-version writes conflict. Only facts/relationships/events can enter provisional state, and only on the pinned timeline. They never alter canon. Story outcomes are idempotent drafts. The legacy `/outcomes` route refuses advanced canon. The UI uses `/reconciliation` and `/reconcile`: each changed record shows original/current/proposed content, and concurrent modifications or deletions require an explicit `current` or `proposed` resolution. Other records may also be omitted by choosing `current`. Every surviving operation is validated against current schemas, references, chronology and rules. A changed canon revision or story version invalidates the preview. A new draft must satisfy the current publication policy; no approval is inherited. Exact retries use immutable audit lineage and return the same draft even after publication or later turns. These drafts appear in the ordinary change list, with source session/proposal linkage in audit. Automatic field merging remains outside this contract; durable generation jobs are described below. Private reusable definitions and bounded persisted memory are described below.

## History and compensation

Revision lists and diffs are membership-scoped and read immutable PostgreSQL snapshots, including all timelines. Diff pages are ordered by record ID. Proposal diffs compare against the original base, while reconciliation compares that base with current canon. Deletion and addition use an absent (`null`) side. Changes that already match current canon are omitted from a reconciled draft; an empty result cannot be submitted. Record-level conflict detection intentionally requires review even when edits touch different attributes of one record. Cross-record validation still applies after conflict choices.

Restore creates an ordinary compensating draft against the inspected current revision, with the target recorded in immutable audit. Original snapshots remain unchanged. A stale base returns 409; more than 100 operations returns 422 and requires smaller independently valid proposals. Restore/reconciliation request IDs bind to the complete request and actor, and never imply publication permission.

Branch creation proposals are a deliberate exception to record-by-record reconciliation: prepare a complete new branch draft at the current revision instead. Partial rebase must not omit copies or detach the published branch from its audited origin.

## Structured editing

The UI prefills complete records, locks existing ID/kind, and preserves schemas, relationship declarations and attributes when editing one field. Type definitions support multiple predicates and relationship types, including self-reference. Fact inputs derive value type from the subject schema; boolean false and explicit validity/timeline survive edits. Rules expose natural-language guidance, life bounds with optional exceptions, scoped relationship cardinality and scoped exclusive boolean facts. Duplicate schema names are rejected locally; external remains the authoritative validator. The browser editor preserves full signed int64 tick strings; unsafe numeric tokens are recovered from original JSON source before conversion. A loaded record cannot be staged against a different revision after refresh.

Version-2 migration preserves old snapshots exactly. A restore targeting a pre-settings snapshot can conflict with required settings/default-timeline invariants; prepare a compensating proposal preserving valid settings in that case. Do not delete or rewrite history to make a migration or restore pass.

## Scoped MCP delegation

Default-disabled `worldbuilder.worlds`, `.canon`, `.search`, `.record` and `.propose` tools proxy only `POST /api/worldbuilder-tools`. The first lists current member worlds; canon/search/record use `worldId`, `timeline` and optional `revision`, `at`, `limit`, `offset` (canon additionally `kind`, search requires `query`, record requires `recordId`). Propose accepts `worldId`, stable UUID `id`, `baseRevision`, `reason` and complete typed `operations`. It creates a draft as the authenticated actor, with ordinary role/revision/reference/rule validation and exact retry binding. Submission, independent review and publication remain browser actions. No tool exposes private sessions, inference, restore, membership or world administration.

MCP signs a 60-second HMAC-SHA256 assertion over normalised authenticated actor, audience `worldbuilder-mcp-v1`, method, exact path and request-body digest. The dedicated `WORLDBUILDER_MCP_KEY` must be at least 32 bytes, generated with cryptographic randomness and shared only by external/MCP through a secret. External verifies signature, request binding, expiry and at most five seconds of future clock skew; it rechecks the actor through the existing active-identity validator and membership on every call. The assertion grants no role and cannot be used on human routes. Replays within its lifetime perform the same bounded reads or idempotent draft request; they cannot approve/publish. Successful world-scoped calls append `mcp.*` audit entries without tokens or prompt text. World-list calls return only membership-filtered metadata.

`WORLDBUILDER_MCP_ENABLED=true` is required in both apps. External’s mesh DENY policy admits only the separately verified MCP principal to the delegation endpoint; existing browser-only world-route denial remains. MCP’s additional policy admits only explicit verified edge principals to both Streamable HTTP and legacy transports. No wildcard principal or empty list is accepted. Neither the assertion nor these templates prove the edge authenticates/sanitises headers: observe that on LAN before enabling. A compromised MCP signing key permits only this restricted surface under existing members; rotate the key and disable delegation to revoke it. No canon database credentials are provided to MCP.

The delegation body is capped at 1 MiB, proxy response at 2 MiB, proxy timeout at 10 seconds and redirects are refused. These are transport bounds, not distributed quotas. Legacy MCP transports, advertised protocol versions, Origin/CORS handling and document tools remain compatible.

## Durable inference allowances and provenance

Current binaries require schema 10. `WORLDBUILDER_AI_MODE=proxy` requires a configured `WORLDBUILDER_AI_PROFILE` and `WORLDBUILDER_AI_PROFILE_REVISION`, zero daily request/monetary allocations, USD and one or two concurrent requests. The default activation profile is `local-default`. Local attempts have explicit zero cost and no daily quota. The independent inference proxy centrally enforces the shared $100 UTC-month cloud ceiling across applications, with no automatic paid fallback. Configuration, credentials and rollout are documented in [inference operations](../inference.md).

A separate database pool retains `worldbuilder.inference_attempt` outside canon transaction rollback. Authorization, request hashes, per-actor concurrency and no-replay behavior remain. Attempts use a 90-second lease and immutable profile pins; completion uses an independent bounded context. A failed completion remains uncertain and must not automatically dispatch again. Durable generation and conversation messages pin their profile when created; changing deployment configuration cannot switch their queued provider. Old rows are explicitly marked `legacy`, and mode mismatch refuses execution.

`WORLDBUILDER_AI_MODE=legacy` retains the older positive per-attempt allocation and daily request/currency caps for controlled compatibility. Existing amounts/currencies remain historical; migration 010 does not reprice them. Runtime cannot rewrite identities, pins, allocations or creation timestamps, or delete attempts.

Responses retain trusted application `provenance` and add `inference` with proxy request ID, profile/revision, provider, actual configured model and accounting status (`local`, `estimated`, `uncertain`; `not_requested` when no model was needed). Estimated cloud amounts and held reservations are integer USD micros. The UI distinguishes these from historical allowances. No claim of actual AWS invoice settlement is made. Missing accounting/usage retains conservative reservations at the proxy; local operation remains independent of cloud budget availability. Processor and external independently preserve semantic, citation and canon-authority validation.

## Private story definitions, memory and narrative generation

`GET/POST /api/worlds/{world}/story-definitions` lists paginated owner-private definitions or creates an immutable definition with `{id: UUID, title, pinnedRevision, timeline, premise, objective, style, participants}`. Title is 1–200 bytes; premise is at most 16,000; objective/style at most 2,000 each; participants are at most 20 distinct entity IDs visible at the pin. Owner is server-assigned; existing IDs bind exact creation content. `GET .../story-definitions/{id}` is owner-only even for world administrators. To revise a definition, create a new ID; no update/delete route rewrites a definition used by sessions. Definitions are private creative guidance, not published canon or world rules.

Session creation accepts optional `definitionId` and must match that private definition’s revision/timeline. Existing sessions without definitions remain valid. Session responses include the definition and `memory: {summary, throughVersion, choices}`. Turn requests additionally accept `direction` (4,000 bytes), optional `memorySummary` (4,000 bytes) and up to six `choices` with distinct nonempty IDs (80 bytes), labels (200 bytes) and directions (1,000 bytes). Saving narration, typed state, summary, choices and next version is one transaction. A supplied nonempty summary replaces the prior summary and receives the server-assigned next version; manual turns without one retain its old coverage marker. Every saved turn replaces the offered choices (omitting choices clears them). Exact retries return the original resulting memory even after later turns. Historical turn reads include direction and memory.

`story_turn` inference now requires `expectedVersion` matching the private session before any allowance is reserved. It receives the immutable definition, current summary/coverage, provisional state, and the newest three turns, each capped at 2,000 Unicode characters with an explicit truncation marker. Definition cast is retained in bounded canonical evidence. All narrative/memory input remains untrusted provisional data and cannot be cited as canon. Output may propose summary/choices but cannot persist itself. External overwrites `sessionVersion` with the inspected version; the UI uses it when saving rather than silently applying a stale answer to a newer session. Typed transitions are limited to facts/relationships/events in the pinned timeline at generation and persistence.

`narrative_generate` produces grounded scenario prose and optional choices with no operations or session summary. A user can copy it into a private definition; the UI retains the generation’s revision/timeline pin. Reusable definition creation/session start use stable IDs across uncertain retries. The existing human story-outcome reconciliation/publication path is unchanged. Full story history remains stored, but only bounded recent context and an optional summary reach the model. Summaries are not independently verified facts and may omit details; model quality remains a LAN acceptance gate.

## Durable creative generations

`GET /api/worlds/{world}/generations` returns `{enabled,items}` with standard limit/offset pagination and only the caller's metadata. `POST` accepts `{id,workflow,question,revision,timeline,parts,sessionId?,expectedVersion?}`. `narrative_generate` supports 1–4 bounded parts; `story_turn` requires exactly one part and the session's current pin/version. The UUID retries exactly the same normalised request; changed content conflicts. At most 20 queued/running jobs per actor span worlds. Queueing atomically persists the private request and dispatch outbox. It does not reserve future parts' allowances or write canon. `GET /generations/{id}` returns the owner's request, status, completed results and progress; even world administrators cannot inspect another owner's job. `POST /generations/{id}/cancel` with `{}` cancels remaining parts. An active part may finish first because current world-scoped transactions serialise work.

Each part rechecks active identity, current membership, archive state and story version, selects authorised pinned evidence, independently reserves inference allowance and validates output through the existing reasoning path. Completed results retain trusted `provenance.generationId/generationPart`, original revision and story version. Inspecting or saving a candidate never silently updates those versions. Previous narrative parts are bounded untrusted context, not canonical evidence. A queued story result does not save a turn automatically.

`WORLDBUILDER_JOBS_ENABLED` is false by default in external and orchestrator. External also requires AI configuration, explicit allowance and current schema 10. The worker authenticates using a dedicated `WORLDBUILDER_JOB_TOKEN` (minimum 32 bytes) and verified distinct mesh principal. Only the fixed `POST /api/worldbuilder-jobs/run` accepts `{worldId,jobId}`; human headers cannot invoke it. Orchestrator receives no prompts, AI token or canon-write credential. `worldbuilder-generation` JetStream / `worldbuilder.generation.requested` carries IDs and schema version, uses a transactional outbox/stable message ID, and durable consumer `worldbuilder-generation-v1` acknowledges only terminal jobs. Each request performs at most one part; pending work is negatively acknowledged for continuation.

A claim expires after 90 seconds. With no ledger reservation for its next part, recovery can safely queue that unstarted part again. With a reservation, expired work becomes `uncertain` and is never inferred again automatically. Failures/cancellations preserve completed parts and charged allowances. The user must inspect them before intentionally creating a new request ID. A lost database commit acknowledgement is resolved against actual checkpoint state before broker acknowledgement. Seven-day broker retention is not permanent dispatch storage; PostgreSQL jobs/outbox remain recovery authority. See [broker and generation recovery](../recovery.md#broker-and-generation-recovery).

## Advisory consistency and proposal review

`consistency_check` runs deterministic validation on the complete stored snapshot before reserving inference. Optional `changeId` binds a proposal to its actual base revision and runs the same current-revision/default-timeline/identity/domain validation used for submission. Stale, invalid or missing proposals fail before provider work. Proposed operations are sent in a separate untrusted `review` context, never added to the published evidence list. Semantic evidence remains scoped to the requested timeline and may be selected/bounded; deterministic validation covers the complete proposal. This check neither submits nor approves a change.

The response's server-owned `deterministic` metadata identifies the checked revision/proposal. Optional `findings` contain at most 20 entries with `severity` (`info`, `warning`, `critical`), `uncertainty` (`low`, `medium`, `high`), a message of at most 2,000 UTF-8 bytes, and 1–10 distinct resolvable published citation IDs. Both processor and external validate them. Findings and model/provenance are retained in the immutable AI audit event. They remain advisory; an empty list does not prove consistency, and severity cannot grant publication authority. The UI exposes this check beside each proposal and explains its effective publication policy. Publication always revalidates current roles/revision/rules, regardless of an earlier check.

## Scoped connection traversal

`GET /api/worlds/{world}/records/{id}/connections?revision=...&timeline=...&at=...&limit=...&offset=...` returns direct forward/back typed references as `{recordId,revision,timeline,records,total}`. It checks world membership and root visibility, uses one immutable snapshot, applies half-open fictional-time and timeline visibility, and paginates deterministic record order. A missing/invisible root returns 404; no text/attribute URL becomes a relationship. UI exploration preserves the inspected revision/timeline/time when following another record, including records outside the current wiki page. It does not create a graph store or mutate canon.

The wiki's relationship graph and chronology derive only from its loaded canonical response at the submitted scope. They report loaded/total coverage, retain search filters and grow through the existing pinned pagination. Graph focus and the full loaded relationship table expose explicit edges; the legible SVG diagram is bounded to 24 nodes/36 edges, with record/source links using the same scoped resolver. Chronology uses exact integer ordering (including signed extremes), stable ID ties and half-open intervals, with chronological spacing explicitly unrelated to elapsed duration. Neither view includes staged edits, provisional stories or an independently fetched catalog. No additional API, datastore or deployment flag is required.

## Reviewed calendars and full-range date facts

Optional `world_settings.calendar` defines custom months, repeating leap cycles, weekdays and tick/day mapping. `canon` and `search` include the requested snapshot’s `calendar` independently of filters/paging. `tick` facts use canonical signed int64 decimal strings; numeric fact history remains unchanged. See [configuration, scope and examples](#calendar-configuration).

## Accounts and administrators

`GET /api/worlds/account` returns the authenticated email and current `superuser`
compatibility flag. Administrators use `GET /api/worlds/users?limit=100&offset=0`
and `POST /api/worlds/users` with `{email,displayName}` to register users. Email is
normalized, existing rows are preserved, and a new identity gets immutable account
audit in the same transaction. Registration alone does not assign ordinary world
membership. `POST /api/worlds/administrators` changes global access with registered
`principal`, `enabled` and required `reason`; it protects the final Administrator.
The runtime uses a narrow audited function, not direct grant-table writes.

## Conversations, scenes and discovery

All routes below are under `/api/worlds`, authenticated at external Go. World
members may read shared content; only the attributed requesting human can start a
saved message's inference. Existing CSRF/origin and request-size checks apply.

| Route | Contract |
| --- | --- |
| `GET /:world/established-accounts?revision=&timeline=` | Exact-snapshot records, successor map and ledger publication origins. Generic canon/record/MCP reads also support the new kind. |
| `GET/POST /:world/conversations` | Paginated shared threads; create with stable UUID, title, revision, timeline and optional exact `at`. Pins are immutable. |
| `GET/POST /:world/conversations/:id/messages` | Ordered, attributed messages; POST `{id,request}` is idempotent and cannot contain private session context. |
| `POST /:world/conversations/:id/messages/:message/generate` | SSE progress, then a validated final result. Completed retries return the result without inference. Running/failed/uncertain requests never auto-repeat. GET messages recovers saved results. |
| `POST /:world/sessions` | `mode: canon_eligible` (default) or `sandbox`, part of immutable creation identity. |
| `POST /:world/sessions/:id/turns` | Optional `scene: {location, inventory:[{entityId,quantity}]}` saved atomically with the version-checked turn and exact retry receipt. |
| `GET /search?q=` | Authorized worlds' default timelines, bounded world/result coverage, scoped citations. |
| `GET /activity` | Authorized world creation/submission/publication metadata only; no private story/job payloads. |

Reviewed `worldGuide` and `styleGuide` belong to world settings and stay pinned to
requested canon revisions. Sandbox bypasses fictional consistency checks only;
shape, references, privacy, concurrency and established accounts remain enforced.
Publishing Sandbox outcomes uses ordinary validation and explicit reconciliation.
Reasoning accepts exact signed-int64 `at`; numeric and quoted wire forms preserve
identity. Generation and story pins are never substituted with refreshed versions.

## Calendar configuration

Calendars are optional canon settings, published through the current human governance policy. Edit the world's `world_settings` record in the wiki, enable **Use a fictional calendar**, fill its structured fields, stage and submit the complete record. Reviewers inspect the calendar before publication. Disabling it proposes removal of the calendar field; dates then display raw ticks. World settings themselves cannot be deleted.

Migration 006 introduced calendars. Current binaries require all migrations through 010; CI publishes artifacts but does not install the schema. Follow [operations](../operations.md).

### Calendar contract

`calendar` is permitted only on `world_settings`. For example:

```json
{
  "name": "Harbour calendar",
  "epochTick": "10",
  "epochYear": "-2",
  "ticksPerDay": 4,
  "months": [
    {"name": "Tide", "days": 2},
    {"name": "Rest", "days": 0, "leapDays": 1},
    {"name": "Dusk", "days": 1}
  ],
  "cycleYears": 2,
  "leapYears": [2],
  "weekdays": ["Sun", "Moon", "Star"],
  "weekdayAtEpoch": 1
}
```

- `epochTick` and `epochYear` are canonical signed int64 decimal **strings**, including for small numbers. No exponent, leading plus/zeroes, negative zero or spaces. The epoch begins day 1 of the first nonempty month in the epoch year.
- `ticksPerDay` is an integer from 1 to 1,000,000,000. Subday ticks start at zero; a tick is not implicitly a second.
- `months` contains 1–32 ordered unique names. Ordinary `days` and optional additive `leapDays` are each 0–10,000. A month must have some days, and the ordinary year must be nonempty. A zero-day ordinary month with leap days is intercalary and skipped in ordinary years.
- `cycleYears` is 1–1000. `leapYears` is an increasing unique list of positions in 1–cycleYears; position 1 means the epoch year. The pattern repeats in both directions. Configure leap positions and extra month days together, or neither. A 400-year Gregorian-like pattern or 19-year lunar pattern uses the same representation.
- Optional `weekdays` contains up to 32 unique names. `weekdayAtEpoch` is its zero-based index, or zero with no weekdays. Weekdays advance on every day, including intercalary days, without restarting at year boundaries.
- Calendar, month and weekday names are 1–80 UTF-8 bytes, without surrounding spaces. Months and weekdays are unique within their respective lists. Unknown nested fields fail decoding.

For the example, tick 10 is Moon, 1 Tide, year -2; tick 9 is Sun, 1 Dusk, year -3, subday tick 3. Tick 30 is 1 Rest, year -1; tick 38 starts year 0. Negative dates use floor division. Derived year labels may extend beyond int64 although input ticks/epochs remain within it. Computation uses BigInt, never JavaScript Date or SQL timestamps.

### Scope and history

Canon and search responses include `calendar` from their requested immutable snapshot, or null. This metadata remains available when filtering/pagination omits settings. Wiki dates, relationship intervals and chronology use that returned calendar. Staged settings, the editor catalog, unsaved filters and current-world metadata cannot format a historical view. All timelines share the revision's calendar.

Formatting retains the original tick alongside the civil date. Editing and `at` filters still take raw exact ticks; display does not reinterpret stored history, change interval semantics or infer timeline inheritance. Validity includes the start and excludes the end. This contract supports bounded repeating cycles; irregular historical reforms, per-timeline calendars and civil-date input conversion are outside it.

### Typed dates and life bounds

Declare date predicates as `"fields": {"birth": "tick", "death": "tick"}`. Facts use `"valueType": "tick"` with a canonical decimal **string** value, for example `"value": "-9223372036854775808"`. Numeric JSON values are refused for this new type. Record `from`/`until` retain their numeric history representation and exact numeric/string input compatibility.

`life_bounds` compares `tick` facts across signed int64, accepting birth and excluding death. Existing `number` date facts retain safe-integer validation and are never silently converted. Change an existing schema by proposing its type definition and affected facts together for review. Pinned revisions retain their old schema/values. Other numeric facts keep their existing semantics.

Offline acceptance covers full-range dates/life bounds, leap/intercalary/subday/negative arithmetic, a 400-year cycle, strict configuration, reviewed publication/removal, filtered/historical calendars, structured editing and actual Chromium staging isolation against disposable PostgreSQL. Production installation and deployed acceptance remain LAN gates.

## Snapshot branches

Timeline branches create a fixed copy of timeline-specific canon through the existing proposal, review and publication flow. Adopting later parent changes requires explicit reviewed proposals against branch records; there is no automatic inheritance. Existing timeline labels and attributes never imply a parent.

### What is copied

The parent is an explicit timeline at an immutable canon revision in the same authorized world. The server copies every record scoped to that timeline, including facts, relationships, events, scoped rules and any scoped catalog records. It preserves all fictional intervals, including exact signed-int64 endpoints; current search, pagination and fictional-time filters do not narrow the copy. It creates one new timeline descriptor, assigns new stable record IDs and remaps typed references between copied records. Free-form prose is retained without guessing which text represents an ID.

World-wide records remain shared: unscoped entity/type/source catalog, world settings, calendars, review policy and unscoped rules continue to use the destination's current canon. This is a frozen timeline assertion set, not a frozen copy of the entire world. A later reviewed shared-schema or global-rule change still affects validation in every timeline. Historical-parent copies are revalidated against the current shared catalog. Missing, hidden or incompatible references are refused, never replaced by an unrelated source. Scoped timeline descriptors are refused rather than silently omitted or interpreted as nested inheritance.

Shared rule reference closures must remain visible too. For example, a world-wide rule attached to a parent-scoped type cannot silently stop applying when that type is copied under a new identity: the branch is refused until the rule/type scope is explicitly resolved through canon review.

The complete branch must fit the ordinary 100-operation proposal limit: one new timeline and at most 99 copied records. The existing 5,000-record world limit also applies. Larger branches fail in full; there is no truncation or multi-publication approximation. An empty parent can create an empty alternate timeline. New branch and copied identities must never have been published in that world, including subsequently deleted identities.

### API and governance

`POST /api/worlds/{world}/timeline-branches` accepts:

```json
{
  "id": "11111111-1111-4111-8111-111111111111",
  "baseRevision": 5,
  "parentRevision": 3,
  "parentTimeline": "main",
  "timeline": "alternate",
  "label": "Alternate history",
  "reason": "Explore a different outcome"
}
```

Both revisions are required nonnegative integers; explicit revision 0 is valid. `baseRevision` must be current. Only an Author (including a global Administrator) can create the draft, and the usual archive, membership and same-origin rules apply. The endpoint returns an ordinary `Change` with status `draft`. No canon, revision or outbox event changes until normal submission, approval and atomic publication succeed. The current administrative publication policy applies; independent approvals exclude the proposer and Administrator overrides require a reason.

The proposal and immutable `timeline.branch` audit are written in the same transaction. Proposal UUIDs are normalized before auditing, so equivalent UUID spelling preserves origin and retry identity. Exact actor/request retries return the original proposal even after later publication; changing other request content under the same UUID returns 409. A failed audit write rolls back the draft as well. Stale publication is refused. A stale branch must be prepared again against inspected current canon with a fresh proposal UUID and fresh reviews: ordinary record-by-record rebase is refused so it cannot omit some copies or lose the branch's creation origin, including when the source UUID in the URL uses upper-case hex.

`GET /api/worlds/{world}/timelines/{timeline}/origin?revision=N` verifies membership and the timeline's presence in the requested snapshot. It returns the original request, branch publication revision and source-ID to copied-ID mapping from the published proposal/audit. An ordinary isolated timeline returns `null`; an absent historical timeline returns 404. These are creation details, not a claim that current records still equal their original copies. Editable labels/attributes cannot fabricate this response. Snapshots, proposals, identity history and immutable audit store the complete state; branches require no separate datastore.

Subsequent branch edits are ordinary proposals against the copied IDs. Parent edits and deletions do not update the copies. Citations resolve the copied canonical ID at the branch's queried revision/timeline; the origin endpoint separately resolves creation ancestry to the immutable parent snapshot. Re-import/merge automation and automatic parent synchronization remain outside this bounded contract.

### Browser and acceptance

**Prepare timeline branch** pins the displayed timeline/revision and the observed current revision before form editing. Changing wiki filters does not repin the draft. The form explains the shared catalog and complete-copy limit. An uncertain submission retains its UUID and full request; an unchanged retry remains possible after canon advances, without rebasing. Creating a proposal directs the user to the ordinary review queue. **View timeline origin** reads only the displayed scope and is available to readers.

Unit tests cover complete copy, cross-record remapping, full-range time, unchanged source maps, other-timeline exclusion, exact record-count boundaries, hidden/missing references and collision refusal. Disposable PostgreSQL cases exercise permissions, required pins, exact retries, publication/history/citations, stale rebase refusal, retired identities and injected audit rollback. A third Chromium flow creates a branch while the wiki is filtered, reviews/publishes it, updates its parent and verifies unchanged branch evidence plus historical origin. Run it through the [existing offline browser/database harness](../testing.md). Current execution and publication evidence are recorded in [implementation status](../status.md).

## Semantic discovery

Canon now has an opt-in model/vector adapter used by external and the outbox projector. It is disabled in published defaults. The existing GET search remains a complete bounded lexical scan of authorized PostgreSQL canon, including historical snapshots. The retained document search pipeline and `embeddingutil` token hashes are separate implementations.

### Query and authority

`POST /api/worlds/{world}/search` accepts `{revision, timeline, query, at?, limit?, offset?, searchToken?}`. Revision must equal current canon; historical searches use GET. `at` accepts exact signed-int64 numeric tokens or decimal strings. Limit is 1–500 (default 100). Membership, timeline, query, page and current index readiness are checked before inference. Normal same-origin POST rules apply. A missing/stale projection returns 409; an unavailable projection returns 503 without reserving an attempt. An attempted embedding or vector-query failure returns 502 and retains its allowance charge.

The authorized snapshot supplies the entire visible ID set, filtered by timeline and half-open fictional time. OpenSearch applies those IDs plus exact world/revision/canon/profile filters before exact cosine scoring. It returns at most 100 IDs and scores; all text, metadata, calendars, review policy and citations come from PostgreSQL. Partial shard results, timeout, malformed/missing scores, wrong scope/profile, duplicate or hidden IDs, wrong ordering and incomplete candidate coverage are rejected. OpenSearch 3.9 sorted nested hits can have null `_score`; the adapter validates the explicit score/ID sort values rather than turning null into zero.

Hybrid ranking combines all canonical lexical matches and the top 100 vector candidates using reciprocal rank (constant 60), then record ID for ties. It can return records with no query-word match. `total` describes this combined candidate set, not every semantically relevant record. Responses identify `mode=hybrid`, `semanticProfile`, `semanticCandidates`, `discoveryStatus`, canonical citations and allocation provenance. A SHA-256 `searchToken` binds query, time, world/revision/timeline/profile and the ordered IDs. Continuations require that token; changed rankings return 409 and require a fresh first page. Each page is an explicit new metered POST, without automatic inference retries. The UI keeps submitted scope/token through load-more, advertises the allowance cost and uses lexical reads for save-triggered refreshes.

### Model and background projection

Both applications require the same `WORLDBUILDER_SEMANTIC_ENABLED=true`, `WORLDBUILDER_EMBEDDING_URL`, `WORLDBUILDER_EMBEDDING_MODEL`, `WORLDBUILDER_EMBEDDING_REVISION`, `WORLDBUILDER_EMBEDDING_DIMENSIONS` and optional `WORLDBUILDER_EMBEDDING_QUERY_PREFIX`. An optional `WORLDBUILDER_EMBEDDING_TOKEN` comes from a Secret. The URL is the fixed private gateway's complete `/v1/embeddings` route; requests never follow redirects. Model identity, every response index/vector/dimension and token usage are validated. There is no token-hash fallback or implicit model choice. The revision names immutable weights/tokenizer/pooling; the HTTP model name alone does not prove the deployed weights.

`apps/pkg/canonsearch` splits every UTF-8 byte of a record into at most 256-byte chunks without splitting runes, streams up to 32 records through model requests of at most eight chunks (matching the existing private TEI service), and forms a byte-weighted normalized mean of normalized vectors. Query chunks include the configured query instruction prefix. Records over 128 KiB, invalid UTF-8, zero/nonfinite vectors and invalid batches fail explicitly. This bounded mean representation is lossy; actual model quality requires its own gate.

The representation version, model, revision, dimensions and query prefix determine the profile and `worldbuilder-canon-vector-{profile}` index. One atomically replaced world document contains only canonical record IDs, content digests and vectors. Exact scoring uses a `knn_vector` mapping without an ANN index. Same-profile unchanged digests reuse vectors. External revision versions prevent stale events replacing new documents. A failed initial build may repeat completed inference batches because only a complete world document is published; it never publishes a partial snapshot.

The background limits are separate from interactive use: one batch at a time, at most 32 records. The projector serializes background projections across replicas using a PostgreSQL advisory transaction lock, while each embedder serializes its own batches. Published snapshots alone are read; source canon is unchanged. Projection has a 15-minute deadline, broker heartbeats every ten seconds, owning-database-session checks every second and retryable failure. Losing either owner cancels model work; actual database-session termination/recovery is covered by the disposable gate. Background model usage is logged with correlation/model/token/latency metadata, without narrative text. Background indexing does not consume the interactive per-user ledger. The intended service is the existing private CPU model, with no paid-provider charge.

On initial enablement or profile changes, run the existing explicit `WORLDBUILDER_REBUILD_ON_START=true` recovery path, then disable it after current revision coverage is verified. Earlier profile indexes are retained; configuration changes do not authorize their deletion. Live controller/route readiness and exact gateway access must be verified before activation.

New worlds enqueue revision 0 atomically with creation, so their initial settings/timeline enter the normal index workflow without waiting for a subsequent reviewed change. Exact creation retries preserve one event. Existing revision-0 worlds created before that behavior need the same explicit rebuild as other missing projections.

### Interactive allowance and AI context

Standalone search reserves workflow `canon_search` in the existing durable inference ledger. Legacy mode retains the configured positive per-attempt and daily allocations. Proxy mode instead uses pinned profiles, zero local monetary/daily quotas and the independent shared cloud budget. These are separate from actual provider invoices. The immutable request hash binds normalized scope, page inputs and model profile. Failure/cancellation does not refund a reservation; completion uses its independent bounded database context.

When enabled and current, large-world AI context uses the same hybrid ranks and preserves mandatory rules/settings, requested references and visible reference closures within 100 evidence records. Small complete contexts need no embedding. Historical contexts remain lexical. Missing/stale/unavailable index readiness falls back to lexical context before inference and reports `semantic_unavailable_lexical_context`; an embedding attempt that fails aborts the charged workflow. A successful embedding plus generation is one existing workflow attempt, not an additional search charge. Its request hash binds the authorized request, deterministic initial evidence and semantic profile; final semantic evidence is recorded through citations/coverage, not by rewriting that hash. Trusted embedding usage overrides any model-supplied `retrieval` usage field. Combined retrieval/generation has a 60-second deadline, followed by at most five seconds for completion within the 90-second allowance lease; the dispatcher allows 60 seconds within its 90-second job lease.

### Configuration and acceptance

External/orchestrator chart values expose `worldbuilder.semantic`. Enabling requires explicit model/revision/dimensions; external also requires valid mode-specific AI configuration. The chart only accepts private cluster service embedding URLs and adds egress to the existing exact AI gateway Pod selectors/ports. It does not publish an embedding endpoint or enable a provider. Runtime activation, gateway identity enforcement, deployed immutable weights, realistic corpus quality/latency and production recovery remain unverified for this new canon feature.

Run the [owned canon-index gate](../testing.md). It exercises actual PostgreSQL, JetStream and OpenSearch with a synthetic loopback HTTP embedding model. It proves semantic-only recall, scope/time isolation, canonical citations, metering including failures, large-context reference closure, changed-record reuse, stale-event refusal, profile drift refusal and reconstruction after full derived-index loss. The synthetic model proves integration and refusals; it does not establish real CPU model quality. Shared-client race tests separately cover malformed model responses, redirects, cancellation and full UTF-8 chunk coverage. UI tests cover explicit POST and submitted paging scope.
