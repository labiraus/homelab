# Requirements and remaining acceptance work

This matrix tracks the architecture and M1–M8 independently of the implemented slice. `LOCAL_VERIFIED` means the cited offline checks passed; it does not mean deployed. `PARTIAL` and `TODO_OFFLINE` require more implementation. `BLOCKED_CLUSTER_ACCESS` applies only to checks needing the real LAN estate. Optional features are explicitly identified.

| Requirement | Evidence / implementation | Status / next work |
|---|---|---|
| M0 repository and documentation inventory | [Audit](DOCS-AUDIT.md), [status](IMPLEMENTATION-STATUS.md), archived prior plan | LOCAL_VERIFIED; no separate engine/infra or Cassandra code found; live inventory remains gated |
| M1 PostgreSQL migration, dedicated roles, transactions | `sql/worldbuilder/001_*`, `grants.sql`, disposable harness | LOCAL_VERIFIED up/repeat/down/up, role denials, restore checksums; real CNPG/MinIO capacity, owner isolation and backups BLOCKED_CLUSTER_ACCESS |
| M2 world/member/settings/lifecycle | World creation/list, member assignment and per-world authorisation | PARTIAL: settings/default timeline, archive, member listing/revocation and ownership transfer remain offline work |
| M2 typed canon and structured projections | Types, entities, facts, relationships, events, sources, timelines and life-bound rules | LOCAL_VERIFIED initial domain; richer cardinality/exclusivity policies and style-guide representation remain offline work |
| M2 fictional time and history | Signed fictional ticks, half-open intervals, real audit timestamps, immutable snapshots | LOCAL_VERIFIED; calendar presentation and timeline divergence/inheritance are optional extensions |
| M2 stable identity and query navigation | World-local IDs and scoped FKs, current/historical reads | PARTIAL: enforce kind identity across deletion/recreation, improve relationship traversal API |
| M3 governance and atomic publication | Independent review, current-role/revision checks, atomic projection/history/audit/outbox | LOCAL_VERIFIED concurrency, stale approval, revoked role, injected transaction rollback and retry tests |
| M3 diff, reconciliation and compensation | `history.go`, record-level three-way conflicts, exact audited retries, fresh approval | LOCAL_VERIFIED database tests; manual conflict choices and cross-record validation, immutable history retained |
| M3 richer review findings/policies | Deterministic validation and terminal rejection exist | PARTIAL: structured advisory findings and review-policy UX; corrective proposals are fresh drafts |
| M4 dashboard/wiki/editor/review | Existing authenticated Worlds tab | LOCAL_VERIFIED structured editing, history/diff paging, other list continuation and links resolving unloaded canon; full browser multi-user acceptance remains gated |
| M4 no-AI collaborative acceptance | API fixture creation -> draft -> independent review -> publication, UI component tests | LOCAL_VERIFIED core path; full browser multi-user acceptance remains to prepare offline and observe on LAN |
| M5 grounded reasoning and injection boundary | External-scoped evidence, token-authenticated LangGraph, strict citations/output, no publish capability | LOCAL_VERIFIED structural tests; realistic fixture evaluations and nuanced consistency findings remain offline work; provider quality BLOCKED_CLUSTER_ACCESS |
| M5 derived retrieval/rebuild | Transactional outbox, real disposable NATS/OpenSearch tests, external versioned projection | LOCAL_VERIFIED indexing/rebuild; candidate discovery plus PostgreSQL reconciliation remains offline work; embeddings feature support BLOCKED_CLUSTER_ACCESS |
| M5 MCP integration | Existing Labiraus document tools preserved | TODO_OFFLINE: scoped verifiable read/proposal delegation; retain human-only approval boundary; mesh acceptance BLOCKED_CLUSTER_ACCESS |
| M5 quotas and provenance | Per-process request/context/concurrency limits; model/usage audit | PARTIAL: distributed budget reservations, provider/latency/correlation attribution and observability remain offline work; real tariff/provider identity must be configured, not invented |
| M6 persistent isolated stories | Owner-only pinned sessions, atomic turns, typed provisional state, retries/concurrency/restart | LOCAL_VERIFIED disposable DB and authoritative API tests |
| M6 definitions/narrative/memory | Initial session title and short `story_turn` generation | PARTIAL: story definitions, scenario/narrative generation, persistent bounded summaries, choices and richer memory remain offline work |
| M6 durable generation and recovery | Existing outbox infrastructure available; generation currently bounded synchronous | TODO_OFFLINE: durable long-generation requests, idempotent recovery and cost attribution |
| M7 outcome reconciliation | Private session original/current/provisional comparison -> fresh validated draft -> ordinary review/publish | LOCAL_VERIFIED including advanced canon, session-version guard, exact retries after later turns and preserved pin |
| M7 visualisation | Wiki links | Optional graph/chronology views remain offline enhancements; no new authoritative store |
| M8 recovery and cutover | Disposable pg_dump/restore, OpenSearch rebuild and concrete LAN runbook | LOCAL_VERIFIED initial recovery; operator recovery dashboards/fixtures and full custom-resource validation remain offline work; actual storage/security/cutover BLOCKED_CLUSTER_ACCESS |
| M8 dependency and retained tool cleanup | Existing npm advisories and old RAGAS pgvector harness documented | TODO_OFFLINE: assess supported dependency fixes and align or retire obsolete evaluation paths without deleting source data |
| M8 Cassandra inventory/decommission | No local resources found; export/checksum/restore plan in runbook | BLOCKED_CLUSTER_ACCESS; destructive removal requires separate operator approval even after inventory |
| Delivery and deployment | Main-branch source publication authorised; CI gates images/charts | Initial slice CI green; increment publication evidence tracked in status; flags remain off; no live readiness observed |

Next work proceeds through the remaining offline items, with tested main-branch increments. Actual database installation, identity verification, activation and cutover follow [LAN-RUNBOOK.md](LAN-RUNBOOK.md). The overall migration remains incomplete until its acceptance gates have evidence.
