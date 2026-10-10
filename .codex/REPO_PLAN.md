# Worldbuilder AI — current repository plan

The authoritative product target is [Worldbuilder architecture](../docs/worldbuilder/01-WORLDBUILDER-ARCHITECTURE.md). The [migration](../docs/worldbuilder/02-WORLDBUILDER-MIGRATION.md) defines M1–M8; [implementation status](../docs/worldbuilder/IMPLEMENTATION-STATUS.md) records actual source readiness, tests and gaps. The [prior document-platform plan](../docs/worldbuilder/archive/PRE-WORLDBUILDER-PLAN.md) is historical.

## Current direction

- Existing monorepo apps and infra remain; no separate engine/infra or Cassandra implementation was found locally.
- PostgreSQL is authoritative for world canon, immutable history, governance and persistent stories. Dedicated schema/runtime roles isolate canon from existing processor/MCP database credentials.
- External owns validated, world-authorised changes and atomic publication. A separate human reviewer is required. AI receives no publication capability.
- Processor owns bounded LangGraph reasoning through Envoy AI Gateway. Story state is provisional and pinned to a canon revision; outcomes require ordinary human review.
- Orchestrator relays transactional outbox events to JetStream and projects immutable snapshots into a rebuildable OpenSearch index. PostgreSQL queries remain the initial wiki/search authority.
- Existing document/assistant APIs and MinIO source objects remain compatible source tooling, never an alternate canon store.
- Flux OCI delivery, authentication, observability, CNPG, Longhorn and external MinIO on svartalfheim remain.

## Immediate priorities

1. Continue the remaining offline requirements in the [acceptance matrix](../docs/worldbuilder/ACCEPTANCE-MATRIX.md), validating and publishing each default-disabled increment on main. Initial source and CI correction are published; revision diff/reconciliation/compensation is published as 4eea01b; structured editing and list continuation are published as ea0ab1c, with both increments green in CI. Schema-v2 lifecycle/settings/identity is published as c20ac2d with external/UI CI green. Dependency remediation is published as ff2744d with UI/processor CI green: UI audit clear, processor four moderate MinIO findings; new CI gates block high/critical advisories.
2. On LAN verify backup/restore, migration-owner isolation, dedicated role credentials, proxy service-account identity/header sanitisation and network policies before activation.
3. Validate actual model JSON quality, embeddings support, JetStream delivery and OpenSearch idempotency/rebuild under failure.
4. Complete durable story definitions/memory/jobs, evaluation fixtures and observability. Schema-v3 per-actor inference reservations and correlation/model/usage/latency/allowance provenance are locally verified; AI activation requires explicit conservative allowances and LAN tariff verification. Scoped MCP reads/drafts with signed request identity and separate mesh gates are locally verified but disabled until LAN acceptance. Scoped search reconciliation/fallback and bounded partial-evidence selection are locally verified against disposable PostgreSQL/OpenSearch; rules increment 9e17a7b is green in external/UI CI. Temporal cardinality/exclusivity and explicit chronology exceptions are now locally verified. Reviewed settings, owner lifecycle, stable deleted-ID kinds, explicit story reconciliation, structured multi-field editing, history/diff paging and list continuation are implemented; live multi-user acceptance remains gated.
5. Inventory any external Cassandra consumers and data. Never remove legacy storage without explicit inventory, export/checksum/restore evidence and separate approval.

No live readiness or deployment success has been observed in this remote session.
