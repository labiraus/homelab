# Worldbuilder AI — current repository plan

The authoritative product target is [Worldbuilder architecture](../docs/worldbuilder/01-WORLDBUILDER-ARCHITECTURE.md). The [migration](../docs/worldbuilder/02-WORLDBUILDER-MIGRATION.md) defines M1–M8; [implementation status](../docs/worldbuilder/IMPLEMENTATION-STATUS.md) records actual source readiness, tests and gaps. The [prior document-platform plan](../docs/worldbuilder/archive/PRE-WORLDBUILDER-PLAN.md) is historical.

## Current direction

- Existing monorepo apps and infra remain; no separate engine/infra or Cassandra implementation was found locally.
- PostgreSQL is authoritative for world canon, immutable history, governance and persistent stories. Dedicated schema/runtime roles isolate canon from existing processor/MCP database credentials.
- External owns validated, world-authorised changes and atomic publication. A separate human reviewer is required. AI receives no publication capability.
- Processor owns bounded LangGraph reasoning through Envoy AI Gateway. Story state is provisional and pinned to a canon revision; outcomes require ordinary human review.
- Orchestrator relays transactional outbox events to JetStream and projects immutable snapshots into a rebuildable OpenSearch index. PostgreSQL queries remain the initial wiki/search authority.
- Existing document/assistant APIs and MinIO source objects remain compatible source tooling, never an alternate canon store.
- Flux OCI delivery, authentication, observability, CNPG and Longhorn remain. SeaweedFS on external `svartalfheim` is the new storage target; see [migration plan](../docs/SeaweedFSMigration.md). Devcontainer/Ansible and explicit Helm cutover overlays are prepared. Routine CI/CD publication retains source-storage defaults until cutover. Version archive/copy tooling, a pinned historical-revert resolver and disposable actual-client CI acceptance are implemented; production reference coverage, data migration and backup restores remain required.

## LAN validation and remaining priorities

Required remote source implementation and feasible offline acceptance are complete through increment 15; tested changes are published on main with Worldbuilder flags off. Current external requires explicit schema migrations 001–005 and dedicated grants. The [status log](../docs/worldbuilder/IMPLEMENTATION-STATUS.md) preserves all implementation/test/publication checkpoints; the [acceptance matrix](../docs/worldbuilder/ACCEPTANCE-MATRIX.md) distinguishes local evidence from LAN gates.

1. Run ordered read-only discovery in the LAN runbook; verify actual contexts, installed CRDs, identities, storage and GitOps artifact/pod readiness.
2. Verify production backup/restore, independent migration ownership, dedicated role secrets, then apply explicit migrations/grants only through controlled deployment.
3. Validate actual OAuth users, forwarded-header sanitisation and exact mesh/network identities before staged canon, relay/index, AI, MCP and durable-job activation.
4. Observe provider JSON/semantic quality, embeddings support, verified pricing/allowances and realistic concurrent latency/cancellation. Local structural fixtures and domain benchmarks are not live quality/capacity evidence.
5. Exercise real broker quorum/redelivery, uncertain-work recovery, all-table restore and derived-index rebuild; verify operational scrapes/dashboard baselines.
6. Inventory external Cassandra consumers/data and obtain separate export/checksum/restore/removal approval before cutover.
7. Follow compatible upstream remediation for four moderate MinIO transitive advisories. Optional graph/calendar visualisations and configurable review quorum are future extensions; fixed independent review and typed-reference traversal are implemented.

No live readiness or deployment success has been observed in this remote session. Source publication is authorised; production activation remains LAN-gated.
