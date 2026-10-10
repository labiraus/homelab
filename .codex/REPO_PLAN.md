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

1. Finish offline validation, record limitations and publish the default-disabled implementation through CI on main as subsequently authorised by the user.
2. On LAN verify backup/restore, migration-owner isolation, dedicated role credentials, proxy service-account identity/header sanitisation and network policies before activation.
3. Validate actual model JSON quality, embeddings support, JetStream delivery and OpenSearch idempotency/rebuild under failure.
4. Extend initial context/pagination/editor limits using measured needs; validate multi-user acceptance and story outcome reconciliation.
5. Inventory any external Cassandra consumers and data. Never remove legacy storage without explicit inventory, export/checksum/restore evidence and separate approval.

No live readiness or deployment success has been observed in this remote session.
