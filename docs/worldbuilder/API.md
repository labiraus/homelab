# Implemented Worldbuilder contracts

This describes the **source implementation**, default-disabled until the [LAN gate](LAN-RUNBOOK.md). Existing document routes are unchanged. No production database migration runs automatically.

## Ownership and identity

`apps/external/worldbuilder/` is the only canon writer. Every route requires validated auth context plus current world membership. Unknown/unauthorised worlds return 404. Owners manage member roles; editor/owner/admin create proposals; reviewer/owner/admin approve and publish. Authors cannot approve their own changes. Membership and reviewer authority are rechecked at publication while holding the world lock.

The current shared auth middleware trusts proxy identity headers. Therefore activation **requires** the chart's Istio DENY policy limiting `/api/worlds` and `/api/worlds/*` to the verified oauth2-proxy mTLS service principal, plus proxy header sanitisation. Processor/MCP cannot call these routes directly. Neither header conventions nor private networking alone prove a human approval boundary. MCP world delegation is intentionally gated until it has a scoped, verifiable identity contract. Existing MCP document tools are unchanged.

External uses `WORLDBUILDER_DATABASE_URL` with nonprivileged login `worldbuilder_external`. Install schema as an independent migration owner, never the legacy shared `app` login. The processor/MCP shared login must have no schema USAGE and no membership in owner/runtime roles. Orchestrator's dedicated `worldbuilder_relay` can read immutable snapshots and deliver outbox records but cannot publish or alter canon. See [SQL notes](../../sql/worldbuilder/README.md).

## Routes

All paths below are relative to `/api/worlds`. JSON bodies reject unknown fields, trailing JSON and bodies above 256 KiB. Failures contain `error` and `code`; 401 means invalid identity, 403 denied role, 404 absent/inaccessible scope, 409 concurrency/state/idempotency conflict, 422 validation, 503 disabled dependency. Internal errors do not expose SQL or provider bodies.

| Method/path | Contract |
|---|---|
| GET `/` | Current member worlds; paginated |
| POST `/` | `{id: UUID, name}`; idempotent owner retry; initial revision 0 and `main` timeline |
| GET `/{world}` | World name, current revision, caller role |
| POST `/{world}/members` | Owner only: `{principal: email, role: admin/editor/reviewer/reader}`; owner cannot demote self |
| GET `/{world}/canon` | Required `timeline`; optional `revision` (default current), `at` fictional tick, lexical `q`, `offset`, `limit` (1–500, default 100); returns records, resolvable citations, total and revision |
| GET `/{world}/records/{id}` | Same required timeline and revision scope; resolves a single canonical citation |
| GET `/{world}/changes` | Latest proposals (paginated), including operations, review and publication state |
| POST `/{world}/changes` | `{id: UUID, baseRevision, reason, operations}`; creates validated immutable draft |
| POST `/{world}/changes/{id}/submit` | `{}`; author submits for independent review after deterministic validation |
| POST `/{world}/changes/{id}/review` | `{decision: approved/rejected, reason}`; requires independent reviewer |
| POST `/{world}/changes/{id}/publish` | `{}`; atomically publishes an approved current proposal; retries return original published revision |
| POST `/{world}/reason` | `{workflow, question, timeline, revision, sessionId?}`; workflows `canon_query`, `canon_proposal`, `consistency_check`, `story_turn`; returns candidate answer/operations/citations/model/usage; never publishes or saves an outcome |
| GET/POST `/{world}/sessions` | List caller's sessions (paginated); create with `{id: UUID, title, pinnedRevision, timeline}` |
| GET `/{world}/sessions/{id}` | Only session owner and current world member can resume |
| GET/POST `/{world}/sessions/{id}/turns` | Paginated turns, newest first; append `{requestId: UUID, expectedVersion, narration, operations}` atomically with state |
| POST `/{world}/sessions/{id}/outcomes` | `{id: UUID, expectedVersion, baseRevision, reason}`; transforms provisional state into a draft; independent review still required |

List routes accept `offset` and `limit` (1–500, default 100). The initial UI loads at most 500 canon records and 100 proposals/sessions/turns; richer paging controls remain a UX follow-up.

Important writes use client UUIDs as idempotency keys. Reusing keys with different payloads yields 409. Submitted proposal operations cannot change; corrections/rebase create a new proposal and need fresh review. The synchronous validation phase is represented by the transition `draft -> ready_for_review`; no pretend async `validating` state is persisted. Rejecting is terminal for that proposal.

World deletion, ownership transfer, arbitrary membership removal, and editing submitted drafts are not exposed by this initial slice. Preserve history; use canon delete operations followed by compensating puts where needed. Full world settings and archive lifecycle remain follow-ups.

## Canon model

A world contains at most 5,000 records initially. IDs are stable world-scoped `[a-zA-Z0-9_-]{1,64}` strings. Operations are `{action: put, id, record}` or `{action: delete, id}`; 1–100 per proposal, with no duplicate IDs. A put replaces a whole record. References are world-local and must survive final batch validation; composite SQL foreign keys recheck projection integrity.

- `type`: `fields` maps predicate names to string/number/boolean/entity value types. `relations` maps relationship names to allowed target type IDs.
- `entity`: requires type and label; presentation attributes are string pairs.
- `timeline`: named explicit continuity. No automatic inheritance or divergence semantics yet.
- `source`: stable named source evidence record; optional attributes can identify a source object. No URL is fetched by inference.
- `fact`: subject entity, declared predicate/valueType/value, source and timeline. A predicate is single-valued within overlapping validity intervals. Entity-valued facts resolve inside the same snapshot.
- `relationship`: subject, target, type-declared predicate, source, timeline.
- `event`: label, source, timeline and fictional `from` tick; optional subject participant.
- `rule`: versioned natural-language label, plus optional `attributes.check=life_bounds`, `birth` and `death` predicate names. This deterministic rule rejects participant events before birth or at/after death. Other machine rule types are rejected. Natural-language consistency findings are advisory.

Fictional `from`/`until` use signed integer ticks and half-open intervals `[from, until)`, unrelated to real `timestamptz` audit timestamps. Missing bounds are unbounded. The API does not assume a Gregorian calendar. Timeline inheritance, custom calendar presentation, relationship cardinalities beyond type validation and domain-specific life exceptions require additional design/tests.

Current projections have typed/reference/time columns in `worldbuilder.record`; JSONB holds type schemas, typed scalar values and presentation attributes. Complete immutable snapshots in `worldbuilder.revision` provide historical reads, story pins and compensating revision recovery. The initial projection rewrite is O(world size) and serialises writes per world; measure before optimising.

## Atomic publication and events

Within one transaction external locks the world, rechecks current membership/approval/revision, revalidates the complete result, replaces projections, appends immutable revision/audit, advances the world and marks the proposal published, then inserts an outbox event. Any failure rolls the entire transaction back. History triggers reject UPDATE/DELETE. Rollback is a new reviewed compensating proposal, never history rewrite.

Outbox payload version 1: `{eventId, worldId, revision, changeId, schemaVersion: 1}`. Orchestrator uses `FOR UPDATE SKIP LOCKED`, publishes subject `worldbuilder.canon.published` to JetStream stream `worldbuilder-canon`, waits for broker acknowledgement and only then sets `delivered_at`. A crash can redeliver; `Nats-Msg-Id` and idempotent projection handle it. The dedicated consumer is `worldbuilder-index-v1`. The first runtime creates a file-backed stream with three replicas and seven-day transport retention; immutable PostgreSQL snapshots remain recovery authority.

The derived `worldbuilder-canon-v1` OpenSearch index stores one current snapshot document per world with nested record IDs/kinds/timelines/text and explicit `visibility=canon`. External version numbers are `canon revision + 1` with `external_gte`, so older arrivals cannot overwrite newer canon. Rebuild reads the latest immutable revision per world; enable `WORLDBUILDER_REBUILD_ON_START` explicitly for recovery then disable it. Initial user-facing wiki/search reads PostgreSQL directly; no raw OpenSearch results enter answers. Neural canon search/embeddings and current-auth filtering before any future index-backed retrieval remain follow-ups.

## Grounded AI and stories

Processor receives authorised evidence selected by external; no canonical database credential, arbitrary HTTP tool, approval or publish capability is available to the graph. It calls only the configured Envoy AI Gateway. Context is capped at 100 records/100 KiB, question at 4,000 characters, answer at 16,000 characters, model output at 768 tokens and HTTP time at 20 seconds. Per-process caps are two concurrent requests and 20 requests/minute; these are not distributed quotas or financial cost guarantees. Model and returned token usage are recorded in the external audit. Underlying provider attribution, latency/cost accounting and distributed quotas remain follow-ups; no provider identity or monetary rate is fabricated.

Strict output validates allowed keys, operation shape, world/timeline/revision and citation IDs. External independently validates canonical references and typed operations. Missing evidence returns explicit uncertainty. These checks prevent capability expansion and invented IDs; they do not prove that every model sentence is semantically supported. Quality, realistic adversarial evaluation and provider compatibility remain LAN gates.

Session definitions (title, owner, timeline) and pinned revision live in PostgreSQL. Turns store narration, request, resulting provisional state and version together. Retries preserve the original response; concurrent expected-version writes conflict. Only facts/relationships/events can enter provisional state, and only on the pinned timeline. They never alter canon. Story outcomes are idempotent drafts. When canon has advanced, outcome creation returns a reviewable conflict; the author compares pinned/current snapshots and creates an explicitly reconciled fresh proposal. Automatic rebasing, long-running generation jobs, persistent summaries and full story-definition authoring are follow-ups.
