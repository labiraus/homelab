# Worldbuilder decisions

## 2026-10-10 — Adapt to verified monorepo

The target's separate engine/infra repos, Cassandra and Fastify are assumptions, not present implementation. Use existing Go external, Node HTTP processor with LangGraph, React UI, pgx and CNPG. Do not introduce a framework, database, BFF or new service. Keep Labiraus infrastructure/resource identifiers and MCP transport names stable; Worldbuilder is the application direction. No Cassandra removal is justified by local absence.

## 2026-10-10 — Additive, default-disabled migration

Use a separate `worldbuilder` schema and explicitly configured external-only connection. Legacy document APIs and MinIO data remain available; they are source material, never canon. No automatic startup migration or Helm migration hook. Human-controlled migration precedes feature activation. Existing shared app credentials are insufficient for canon isolation: schema ownership and runtime role grants must be validated before activation.

## 2026-10-10 — Governance and historical reads

Use world-scoped stable IDs, relational current projections and complete immutable revision snapshots initially (target thousands of entities). Publication takes a world row lock, rechecks membership, approval and expected revision, then commits projections, history, audit and outbox together. Prefer explicit independent reviewer approval over self-approval. Full snapshots simplify provable pinned reads and compensation; incremental storage optimisation requires measurements.

## 2026-10-10 — Offline boundary

No cluster mutations, remote database access, pushes, CI dispatches or automatic deployments. Future LAN instructions are a separately authorised gate. Preserve existing recovery and external MinIO rules.

## 2026-10-10 — Subsequent user instruction authorises main-branch CI delivery

The user explicitly changed the earlier no-deploy instruction: work from main so CI deploys, then validate on LAN. Switched back to main and will publish tested source via existing CI/Flux. This supersedes the earlier feature-branch/no-push decision for source delivery. Canon schema installation, dedicated credentials, mesh identity validation and feature activation remain separately gated; no direct cluster command is authorised by that source-delivery change. All new runtime flags default off.

## 2026-10-10 — Reuse a relational projection and immutable full snapshots

A single `record` projection has structured kind/reference/type/predicate/time columns and composite world-scoped foreign keys. JSONB holds scalar value, schema and presentation extensions. Immutable revision snapshots support exact historical reads and pinned story sessions. Current publication rewrites at most 5,000 records under the world lock; this favours reviewable transaction correctness over premature incremental projection complexity. Types, entities, sources, timelines, facts, relationships, events and rules share IDs; refs are validated by kind in external. Automatic timeline inheritance is absent.

## 2026-10-10 — Browser publication boundary, initial MCP deferral

Deferral superseded by the scoped delegation decision below; the browser publication boundary remains.

Existing auth middleware accepts trusted forwarded identity headers. It cannot distinguish an AI request carrying a user's header from a human review. Require mesh-enforced oauth2-proxy-only access to all new world routes before activation; deny processor/MCP direct world access. External supplies authorised canon context to processor over a private token endpoint. Processor has no tool router or canon DB privileges. A future scoped MCP delegation contract needs separate read/propose capabilities and must never allow review/publication. Existing MCP transports and document tools remain compatible.

## 2026-10-10 — Outbox relay and projection stay in orchestrator

Orchestrator already owns durable async coordination. Add optional outbox relay and a durable NATS projection consumer there, using a read-only canon snapshot grant and delivery-only outbox update grant. Projection is a deterministic copy, not extraction or inference. OpenSearch external versioning handles duplicate/out-of-order events; PostgreSQL remains user-facing retrieval authority initially. No new service is needed. Canon embedding/vector search and index-backed candidate discovery are follow-ups; no model/provider capability is assumed.

## 2026-10-10 — Explicit bounded reasoning and story candidates

LangGraph supports four workflows with fixed evidence, bounded context, strict output and resolvable IDs; no model-selected tool calls exist. Reasoning returns candidates only. UI actions explicitly save draft proposals or provisional story turns. Session version and request IDs guard atomic persistence, and outcome reconciliation refuses silent rebase when current canon differs from the pin. Model grounding validation proves scope/reference integrity, not semantic truth; live quality evaluation remains required. Per-process rate/token ceilings are not distributed financial cost caps.

## 2026-10-10 — Main publication must depend on tests

Audit found that app CI skipped main-branch tests and Go jobs ran from the non-module repository root. Corrected all app pipelines to run tests on main, build TypeScript/frontend, use each Go module's working directory, and make publication depend on tests. External CI additionally runs disposable migration/integrity/recovery tests. Existing artifact tagging/signing and Flux discovery patterns are preserved.

The CI runtime update was driven by actionlint failures on the existing setup-go v3, setup-node v2 and cache v2/v3 actions. Replacement major tags were verified against the upstream repositories; setup actions use their documented Node 24 runtime ([setup-go](https://github.com/actions/setup-go), [setup-node](https://github.com/actions/setup-node), [cache](https://github.com/actions/cache)). No app framework/toolchain version was changed by that action-runtime fix.

## 2026-10-10 — Preserve AI proposal base revision

Review found that a generated candidate could otherwise be saved with a later UI-selected revision. Canon proposal inference now requires the current revision, returns its explicit `baseRevision`, and the UI retains that value when saving the draft. Stale proposals conflict rather than silently acquiring a new base. Regression tests also prove unauthorised calls never invoke the reasoner and inference alone creates no proposal.

## 2026-10-10 — Explicit reconciliation and compensation

Add revision metadata/diffs and three-way record comparisons inside external, using existing immutable snapshots and per-world transaction locks. Conservatively flag concurrent record modification/deletion; the user chooses current or proposed, then ordinary validation catches changed references, schemas and rules. Reconciliation produces a new draft and independent review, never transfers approval, overwrites old operations or repins a story. Store source lineage and exact request in immutable audit for safe retries after canon advances. Restore uses the same publication engine to compensate, with a 100-operation cap; no schema change or history rewrite is needed.

UI previews retain their observed canon revision and story version. Manual staging must retain its original revision and before-state too: review exposed an older implementation that substituted freshly loaded current revision when saving. Corrected it and added an editor regression test and app operating rule. No extra model inference or cluster capability is needed for these workflows.

## 2026-10-10 — Complete records and pinned reference catalogs in the editor

Whole-record replacement requires complete prefill; editing one field must not drop other type fields, relationships or attributes. Split structured editing into its own UI component and add dynamic schema rows. Fetch reference kinds separately from filtered wiki pages, pin every catalog/continuation request to the displayed revision, and validate kind filters server-side. Keep offset pagination compatible with existing APIs and deduplicate mutable list results; refresh observes concurrent insertions. Bound catalogs by the existing 5,000-record world cap. No additional service or query authority is needed.

## 2026-10-10 — Governed settings and administrative lifecycle

World name/default timeline/style settings are canonical records and must pass ordinary review/publication. Archive and membership/ownership are separate owner administration, serialized under the same world lock with their own expected version and immutable request/result audit. Archive is reversible and preserves all canon and story history; membership revocation still applies immediately. Transfer promotes an existing member and demotes the former owner to admin atomically, with a unique-owner index.

Add migration 002 instead of modifying migration 001. Preserve v1 snapshots exactly, backfill permanent record-kind identities from all revisions, and abort on historical cross-kind ID reuse. New settings records in older worlds require explicit review; do not silently patch canon during migration. Current runtime requires schema 2; rollback disables the feature and retains data. No populated schema downgrade is supported.

## 2026-10-10 — Tested dependency upgrades and high-severity CI gates

Use supported security fixes with build/test evidence. Upgrade Vitest to maintained 4.1.11 for the advisory fixed only in maintained majors, retain Vite 7 at patched 7.3.7, and refresh compatible transitive packages. Both application CI pipelines now reject high/critical npm advisories before publication. Preserve MinIO 8 while its remaining transitive fixes require incompatible module/API changes or a forced downgrade; record that limitation in [dependency validation](DEPENDENCIES.md), rather than claim a clean processor audit.

## 2026-10-10 — Explicit temporal rule policies

Represent additional machine checks as reviewed rule records using existing structured type/predicate/timeline/validity columns and bounded string configuration. Count relationship edges and true boolean facts with interval sweeps, not sampling or floating point time arithmetic. Scope minimum cardinality to rule/entity validity; exclusivity counts only true facts on the declared timeline. Chronology exceptions must themselves be canonical true facts valid at the event tick. This adds deterministic enforcement without inventing implicit resurrection semantics or treating model prose as an exception. Numeric life-date facts are restricted to exact safe integers, while event and interval ticks retain signed 64-bit comparisons.

## 2026-10-10 — Resilient disposable database transport

Repeated host-port failures were infrastructure preflight failures, not domain test results. Reuse the available Docker API to relay binary PostgreSQL traffic from a loopback-only socat socket to the exact generated disposable container. Expose explicit exec-transport selection and automatic fallback, with cleanup limited to the local relay and owned container. This preserves real PostgreSQL coverage without reaching a cluster, changing host networking or installing another persistent database.

## 2026-10-10 — Reconcile discovery and declare partial AI evidence

OpenSearch contributes bounded candidate IDs only; external checks membership first and resolves content and temporal visibility from PostgreSQL. Historical reads bypass the current index. Canonical lexical ranking plus a complete bounded scan preserves results/pagination through index lag or outage. This prioritises correctness at the existing 5,000-record world limit; it is not yet an indexed performance optimisation or vector search. Large-world prompts use bounded reference closures, retain required rules/settings/story dependencies, and declare selected evidence in request, response, audit and UI. Model output cannot override that declaration. Whole-world consistency and absence cannot be inferred from a partial selection.

## 2026-10-10 — Request-bound MCP delegation with separate capabilities

Use a dedicated short-lived HMAC assertion plus existing mTLS mesh restrictions rather than raw forwarded email on human routes. The signed actor must remain an active application identity and current world member. A narrow operation switch permits reads and validated draft creation only, even for owners. Reuse `pkg/api` for request-signing primitives and external’s domain validator; add no service, table or alternative canon writer. Exact draft request IDs make bounded replay idempotent. Signing keys remain secret references; activation requires separate observed MCP/edge principals and sanitisation checks. Existing MCP transport and document behaviour stays intact.

## 2026-10-10 — Durable per-actor inference reservations

A per-process limiter cannot coordinate replicas or survive crashes. Reserve each attempt in PostgreSQL under a per-actor advisory lock, using a separate small connection pool and no world FK to avoid self-deadlock with the authorisation transaction’s world lock. Persist allowances even on provider failure, cancellation or audit rollback; expired leases release only concurrency. Configure a conservative amount per bounded request and a daily amount/currency explicitly rather than inventing rates or refunding unknown provider work. The ledger is allocation/provenance evidence, not a billing oracle. Gateway route labels are operator configuration; provider identity/tariffs remain unverified. Schema 3 is explicit and additive, with populated rollback refused.

## 2026-10-10 — Immutable private definitions and explicit provisional memory

Store reusable story setups separately from canon, private to their creator and pinned to an immutable revision/timeline. Immutable definitions avoid silently changing running sessions; revisions use a new definition ID. Add bounded summary/choice checkpoints to the atomic turn transaction rather than replacing canon or losing narration history. Carry recent-turn truncation and summary coverage explicitly. Require a session-version match before generation and retain it through human save, preventing stale generated content from adopting a refreshed version. Scenario generation returns prose only. Shared/public templates and automatic narrative truth verification are not implied by private definitions.

## D019 — private checkpointed creative jobs

Use existing external/orchestrator/PostgreSQL/JetStream surfaces; no new service or provider tool authority. Queue metadata through an atomic outbox and reload private intent in external under its stored human owner. Bound narrative jobs to four independently validated and allocated parts; story jobs to one version-checked candidate. Only an absent reservation proves expired work never reached inference. Otherwise stop uncertain work instead of risking repeated provider cost. Retain partial output, private ownership and original pins. Acknowledgement follows actual saved state even after an ambiguous commit result. Network policy additions and dedicated mesh/token authentication are conditional on default-off jobs flags. Single-node disposable transport proves protocol behaviour, not LAN quorum/security or model quality.
