# Worldbuilder AI — Migration documentation index

Status: **approved target architecture; initial baseline and multiple live application checks verified; remaining [offline follow-ups](OFFLINE-WORK.md), physical control-plane recovery and storage cutover remain open**. Use [current implementation status](IMPLEMENTATION-STATUS.md) for exact source, publication and deployed evidence.

This documentation supersedes Evolve's previous document-first product direction for **application behaviour**. It does not automatically supersede infrastructure, security, development, or operations documents.

## Read in order

1. [01-WORLDBUILDER-ARCHITECTURE.md](01-WORLDBUILDER-ARCHITECTURE.md) — target architecture, domain boundaries, data model, workflow contracts and acceptance criteria.
2. [02-WORLDBUILDER-MIGRATION.md](02-WORLDBUILDER-MIGRATION.md) — executable milestones, documentation reconciliation, component migration, test and release gates.
3. [03-WORLDBUILDER-REMOTE-DEPLOYMENT.md](03-WORLDBUILDER-REMOTE-DEPLOYMENT.md) — explicit remote-now/LAN-later runbook.
4. [04-CODEX-EXECUTION-PROMPT.md](04-CODEX-EXECUTION-PROMPT.md) — instructions to give Codex inside the checkout.

## Key decisions

- Existing repositories, hardware, GitOps, cluster architecture, authentication, networking and inference entry point remain unless inspection reveals a specific blocker.
- PostgreSQL is the **single authoritative store** for canon, governance and persistent story state; Cassandra is retired only after verified cutover.
- Canon uses flexible typed entities, facts, relationships, events and timelines, not only arbitrary JSON documents.
- Approved change sets publish atomically in PostgreSQL and produce outbox events. OpenSearch is a rebuildable derived search index.
- External (Go) owns world/canon APIs and publication. Processor (TypeScript, LangGraph/LangChain) owns AI reasoning; orchestrator (Go/NATS) owns durable async work.
- Stories use pinned canon revisions and isolated provisional state; outcomes generate proposals, never immediate canon mutations.
- During remote-only work, **do not assume cluster connectivity or deploy blindly**. Prepare and verify changes locally, then stop at the explicit LAN validation gate.

## Authority and change discipline

The existing codebase and deployed manifests are evidence of the **current state**, not of the desired state. This set defines the **target** but is not evidence that target code or components already exist. All illustrative paths, tables, endpoints and events must be aligned with the actual repository after inventory. Mark unverified assumptions instead of guessing.

## Execution records

- [Documentation audit](DOCS-AUDIT.md)
- [Implementation status and tests](IMPLEMENTATION-STATUS.md)
- [Requirement acceptance matrix and remaining work](ACCEPTANCE-MATRIX.md)
- [Dependency validation](DEPENDENCIES.md)
- [Decisions](DECISIONS.md)
- [Troubleshooting](TROUBLESHOOTING.md)
- [API/event contracts](API.md)
- [Reviewed snapshot timeline branches](TIMELINE-BRANCHES.md)
- [Optional metered canon semantic search](SEMANTIC-SEARCH.md)
- [Concrete LAN runbook](LAN-RUNBOOK.md)

- [Operational metrics, dashboard and read-only recovery diagnostics](OBSERVABILITY.md).
- [Reproducible browser, schema and bounded-load acceptance](OFFLINE-ACCEPTANCE.md).
