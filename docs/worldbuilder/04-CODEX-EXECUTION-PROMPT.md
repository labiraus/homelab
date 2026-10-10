# Codex execution prompt — Worldbuilder AI pivot

Copy the text below into Codex while at the root of the existing Evolve checkout (with the infrastructure repository available as normal). This file is an execution instruction; no blanket permission for cluster deployment is implied.

---

You are the lead engineer carrying out an incremental architectural pivot of the existing Evolve platform into Worldbuilder AI. Read `docs/worldbuilder/00-WORLDBUILDER-INDEX.md`, `01-WORLDBUILDER-ARCHITECTURE.md`, `02-WORLDBUILDER-MIGRATION.md`, and `03-WORLDBUILDER-REMOTE-DEPLOYMENT.md` in full first. Read all applicable root and nested `AGENTS.md` files and existing repository conventions. The target documents are architectural requirements, not evidence that the current implementation matches them.

## Operating conditions and hard safety rules

I am currently working remotely and have no cluster network access. You may build, refactor, test and render manifests locally, but **must not attempt cluster mutations or live deployments**. Do not run `kubectl apply`, `helm upgrade`, `flux reconcile`, destructive Terraform, or database/volume destruction. Do not push/merge into any GitOps branch that automatically applies to the cluster. Work on a feature branch or unmerged PR if the repo workflow supports this; first verify the GitOps branch/source behaviour. Cluster validation and rollout will occur separately when LAN access returns tomorrow. Never mark a deployment successful without live observations. Do not assume secrets, context, storage configuration or registry availability. Ask for intervention only for genuine hard blockers; otherwise continue with safe local implementation and record limitations.

## First pass — inspect and reconcile (mandatory before implementation)

1. Inventory the actual source tree for external, processor, orchestrator, mcp, bff, ui, Cassandra clients/schemas, OpenSearch, NATS subjects, LangChain/LangGraph, Envoy AI Gateway and the infra Helm/Flux/Terraform structure. Report paths and code entry points actually found, and mark unknowns.
2. Inspect existing docs, READMEs, AGENTS guidance, architecture diagrams, ADRs, API specifications and release/runbooks. Create `docs/worldbuilder/DOCS-AUDIT.md` showing each impacted file's `retain/update/supersede/archive/delete-candidate` disposition and replacement cross-reference. Align active docs to the new Worldbuilder product, update their conflicting content rather than merely adding footnotes, preserve accurate platform guidance, and archive historical decisions rather than erase them. Fix broken links and regenerate generated docs from source. Do not mass-replace identifiers indiscriminately.
3. Create/update `docs/worldbuilder/IMPLEMENTATION-STATUS.md`, `DECISIONS.md`, and `TROUBLESHOOTING.md`. Describe verified current state, target state, exact test commands/results, unresolved assumptions, deployment blockers and decisions. Avoid duplicate logs if conventions already provide equivalent files.
4. Produce a dependency-ordered implementation map based on verified repository findings. Maintain backwards compatibility while migrating where practical.

## Implementation — work through milestones in dependency order

Follow M1-M8 in `02-WORLDBUILDER-MIGRATION.md`, prioritising a coherent end-to-end vertical slice (world CRUD -> typed facts/relationships -> change proposal -> review/atomic publish -> wiki -> canon grounded AI -> persistent interactive story -> proposed canon outcome). Do not stop merely after writing a plan. Implement as much as possible now, checkpoint and document incomplete work honestly. Prefer small cohesive changes with compiling/tested states, not a giant rewrite.

Design guardrails:
- PostgreSQL is canonical. Migrate Cassandra callers intentionally; do not silently delete Cassandra or its PVs/data.
- Use stable world-scoped IDs, typed facts and relationships, revision history, fictional time, timeline scope, and DB constraints/transactions where appropriate.
- All canonical writes flow through external's validated, authorised change-set/publication engine. AI tools can propose but cannot publish. Publication atomically checks current revision and approval, applies changes, records history and outbox event.
- Processor handles LangGraph reasoning, source-backed RAG, semantic checks and creative narratives through authorised tools and Envoy AI Gateway. OpenSearch remains a rebuildable derived index and is not the source of truth.
- Interactive stories pin canon revisions, maintain isolated persistent provisional state and propose outcomes for human review. Sessions may not directly mutate canon.
- Orchestrator/NATS handles durable async operations, using idempotent handlers and a reliable transactional outbox pattern for canonical publication events.
- Retain existing cluster, network, authentication, observability, container signing, release and deployment patterns wherever appropriate. Do not add microservices/databases or switch framework without justifying measured need.
- Keep security boundaries, cross-world authorisation, privacy and audit, prompt-injection resistance, rate/cost limits, provenance and rollback in scope.

## Tests and offline preparation

Run all feasible Go, TypeScript and frontend checks and integration tests against disposable local services if available. Add automated tests for atomic approval, conflicting revisions, temporal queries, world isolation, grounded citations, prompt injection, story-turn persistence, concurrency and story-outcome proposals. Validate migrations on a disposable PostgreSQL instance. Run Helm lint/template and Flux/Kustomize build/schema checks for each relevant environment/config overlay. Inspect existing GitOps auto-deploy policy and prevent live drift from your work. Capture precise check results, not generic 'tested' statements; label unavailable cluster work as `BLOCKED_CLUSTER_ACCESS`. Prepare a read-only discovery and staged LAN deployment runbook, with backup/restore, observed readiness gates, reversibility and separate Cassandra removal approval.

## Delivery requirements for this Codex session

- Make real source, tests, docs and manifest changes rather than outputting only advice.
- Continuously update active docs and the project status/decision/troubleshooting logs alongside code; record superseded docs and their replacements.
- Do not invent repository paths, test passes, provider behaviours or deployed resources.
- When a milestone cannot safely finish offline, leave a clearly described gated checkpoint and proceed with other independent safe work.
- Finish with a concise report: files/components changed, implemented milestones, tests run with outcomes, documentation reconciled, unresolved issues, what remains blocked pending LAN access, and explicit ordered commands for read-only cluster assessment and then **separately authorised** staged rollout.

Start by inspecting the repository and its documentation; then implement the pivot, maintaining a working build at milestone boundaries.
