# Worldbuilder AI — Target architecture and contracts

## Mission

Build a collaborative worldbuilding application where **the world, not the LLM, is the source of truth**. Contributors create, query, revise and explore fictional universes containing entities, facts, relationships, chronology, rules and narrative content. Every canonical mutation is inspectable, versioned, permissioned and explicitly approved. Interactive stories may generate candidate canonical outcomes but cannot directly change canon.

## Existing-to-target component mapping

| Component | Decision | Target role |
|---|---|---|
| Existing UI SPA | Keep, redesign features | AI conversation workspace with supporting wiki, structured editor, review queue, story experiences, timelines and graphs |
| `external` (Go) | Expand | AuthZ, worlds, canonical queries, typed data contracts, change sets, atomic publication, story persistence |
| `processor` (TypeScript; audited implementation uses Node HTTP) | Refactor | LangGraph workflows, grounded retrieval, AI proposals, consistency reasoning, narrative generation |
| `orchestrator` (Go) + NATS JetStream | Keep | Durable background indexing, import, bulk consistency, longer generation |
| `mcp` (Go) | Keep thin | Authorised proxy into existing API capabilities; no bypass to canonical DB |
| `bff` | Inspect | Retain only if it provides a real client-aggregation boundary |
| Cassandra | Inventory gate | None found in available checkout; any external consumers/data require export and verified cutover before retirement |
| PostgreSQL | Reuse existing CNPG; add canon schema | Only authoritative store for canon, audit/change governance and persistent story state |
| OpenSearch | Keep | Rebuildable text/vector/hybrid search, **not authority** |
| Envoy AI Gateway | Keep | Central provider-neutral inference and usage governance |
| Object storage | Add/reuse | Manuscripts, large sources and long narrative artefacts; link from PostgreSQL |
| Keycloak/oauth2-proxy, Istio, Cilium, Flux/Helm, metrics/logging | Keep | Existing security and platform capabilities |

Avoid creating a dedicated canon or story microservice until code complexity or scaling warrants extraction. Maintain module boundaries inside `external` first.

## Domain model (logical, not exact SQL)

- `world`: identity, name, settings, default timeline, canonical revision number.
- `world_member`: principal and per-world role (owner, admin, editor, reviewer, reader).
- `entity_type`: world-scoped schema and validation, permitted relationship types.
- `entity`: stable identity, world, type, label, extensible presentation attributes.
- `fact`: typed subject/predicate/value assertion, provenance, timeline and in-world validity interval. Query-critical facts must be structured, not buried inside prose/JSONB.
- `relationship`: typed from/to entity edge, properties, in-world validity, source and timeline. Enforce both ends in same world and valid types.
- `event`: history item and ordered fictional date; participants and links to entities.
- `timeline`: continuity context, optional divergence/inheritance; never leak facts between timelines unintentionally.
- `world_rule` and `style_guide`: versioned machine-checkable rules plus natural-language conventions.
- `source_document` / `source_reference`: durable source IDs and evidence for facts and answers.
- `change_set`, `change_operation`, `change_review`: proposal operations and decisions, based on a known canon revision.
- `canon_revision` and immutable revision history: reconstruct and diff historical snapshots; rollback through a compensating revision.
- `audit_event`: actor, timestamp, reason, correlation IDs, resource and action.
- `outbox_event`: same-transaction event staging for durable asynchronous projections.
- `story`, `story_session`, `story_turn`, `story_state_transition`, `story_outcome`: definition, pinned canon revision, provisional state and outcome proposals.

Use PostgreSQL relational constraints and transactions for integrity, with JSONB only for type-specific extensibility. Provide versioned schemas, indexes, migrations, backup/restore tests and a schema evolution policy. Store real-world timestamps separately from **fictional-world time**; support non-Gregorian calendars without relying on ordinary SQL timestamps for fictional dates.

## Core invariants

1. No AI service or tool receives direct authority to publish canon.
2. All persisted domain objects are world-scoped, with per-world authZ enforced server-side; timeline and canon revision scope are mandatory for retrieval where applicable.
3. Published canon is immutable history plus a transactionally updated current projection. Drafts and story state cannot be mistaken for published canon.
4. Change sets are grouped, validated and approved; publication is atomic and idempotent. Publishing locks the world revision, checks stale proposals, revalidates against the **current** canon, persists a new revision and an outbox event in one transaction.
5. OpenSearch projections can always be rebuilt from authoritative data; stale search results must be reconciled with current canon before answering factual questions.
6. Sessions use a pinned published revision and isolated provisional facts. Story outcomes create reviewed change sets; conflicts from intervening canon revisions require revalidation/rebase.
7. A citation references resolvable canonical/source IDs and relevant revision, not invented text-only citations.
8. Prompt injection in retrieved content must never expand tool permissions or mutate data.

Human-review policy now includes the explicit user-requested superuser exception: an operator-granted superuser may register users and review their own proposals while retaining world membership, ordinary validation, immutable audit and the configured distinct-human count. Other users require independent reviewers; AI gains no publication authority. See [review policy](REVIEW-POLICY.md).

## Change-set lifecycle

`draft -> submitted -> validating -> ready_for_review -> approved -> published` with branches to `needs_changes` or `rejected`. Validation checks types, references, cardinality, exclusivity, chronology and explicit world rules. AI contradictions are advisory findings with evidence, severity and uncertainty; do not silently treat model output as deterministic verification. Preserve author, reviewers, timestamps, before/after operations, rules and model metadata.

## AI workflows

- `canon_query`: retrieve structured entities/facts/relationships/timelines via authorised external API, combine with OpenSearch discovery, ground response and citations, express uncertainty.
- `canon_proposal`: turn natural language into a **draft** structured change set; validate, review and publish through ordinary API.
- `consistency_check`: run deterministic rules first, AI semantic checks second.
- `narrative_generate`: create grounded story or scenario content with world/style rules.
- `story_turn`: load pinned canon + provisional session state, retrieve only relevant evidence, generate proposed narration and typed session transitions, validate and persist turn+state atomically.
- `story_outcome`: summarise candidate facts/events/relationships from session; reconcile against current revision; submit change set for human approval.

Use Envoy AI Gateway for all supported model calls, including embeddings where practical and validated against the actual OpenSearch integration. Record model/provider, usage, latency, request/session/correlation IDs and costs. Distinguish inference-provider portability from guarantee of identical model behaviour.

## Suggested application API resources

Illustrative only: `/worlds`, `/worlds/{id}/entities`, `/facts`, `/relationships`, `/events`, `/timelines`, `/rules`, `/change-sets`, `/reviews`, `/revisions`, `/stories`, `/story-sessions`, `/turns`, `/outcomes`, `/search`. Match existing route conventions; avoid incompatible wholesale route renames. Contracts need pagination, filtering, stable IDs, world authZ, concurrency token/expected version, typed validation errors and idempotency keys on important writes.

## Frontend target

The [AI-first specification](05-AI-FIRST-UI.md) governs the interaction design. The [local implementation](UI-IMPLEMENTATION.md) retains Vite/React, the existing hash/auth shell and external API contracts. Conversation is the default world workspace; Explore, Drafts & Proposals, Stories and World Settings support it. Chat is currently ephemeral; persisted stories and reviewed canon retain their independent authority.

- World browser + member/settings UI; structured entity/wiki pages and full-text search.
- Entity, fact, relationship, event and timeline editor; no AI needed for basic CRUD.
- Canon-aware chat with linked evidence and uncertainty display.
- Change-set diff/review/publish interface with deterministic and AI findings.
- Story creation/play UI with narration, choices, visible session state and resume; story-outcome proposal review.
- Optional graph and timeline visualisations from API; no new authoritative datastore.

## Security and NFRs

Preserve Keycloak/oauth2-proxy, Gateway API, Cilium and existing observability. Validate authN/authZ at application API for each operation, not only at proxy; apply rate limits and cost caps to inference; avoid leaking other worlds/drafts through embeddings, logs or model context. Ensure backup restore, idempotent jobs, OpenSearch rebuild, migrations, audit retention and disaster recovery have tests. Target thousands of entities first; optimise only with measured workload.

## Product acceptance scenarios

1. Two editors collaborate on a world; an editor proposes multiple related additions; a reviewer approves; one revision applies all operations atomically.
2. Conflicting edits based on old revisions cannot overwrite newer canon silently.
3. AI answers a chronology question with resolvable fact/event sources; says unknown if not established.
4. Rules catch dead characters acting after death where no canonical exception applies; AI flags nuanced inconsistencies for review.
5. Story session resumes after restart, sees pinned canon and its own provisional state, and cannot publish a plot event directly.
6. Completed story creates a proposal; after reconciliation and approval it becomes canon and appears in wiki/search.
7. Rebuild OpenSearch and recover PostgreSQL from backup without treating generated prose as authority.
