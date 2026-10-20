# Worldbuilder schema version 9

An additive schema alongside retained `auth`, `rag` and `assistant`. Never install as the shared legacy app user: use an independent migration owner. No application or Helm startup hook runs migrations.

- `001_up.sql`: transaction-scoped advisory lock, rerunnable initial tables, relational references and immutable history triggers.
- `001_down.sql`: drops only an empty foundation; refuses once any world exists. Production rollback disables the feature and retains data.
- `002_up.sql`: additive archive/administration version/default timeline columns, reviewed settings kind, single-owner/settings indexes and immutable record identity registry backfilled from all revisions. Historical reuse of an ID with different kinds aborts before schema changes; inventory such data rather than rewriting history.
- `005_up.sql`: private generation jobs and transactional dispatch outbox; inference attempts gain opaque job/part attribution. Relay cannot read private jobs. Runtime changes only checkpoint/status fields.
- `006_up.sql`: optional reviewed calendar JSON projection and `tick` fact type. Existing values, immutable snapshots and retry content remain unchanged. The original anonymous fact constraint is identified by its column dependency; unexpected drift fails explicitly.
- `006_down.sql`: empty database only; removes calendar projection and restores the previous fact constraint. Disable Worldbuilder instead of downgrading populated production data.
- `007_up.sql`: immutable per-authority human reviews, bounded quorum, settings projection and preserved publication principals. Legacy decisions backfill without changing history; unknown review times remain null. Role changes rotate authority so re-granting a role cannot revive old votes.
- `007_down.sql`: empty database only. Refresh dedicated grants after applying 007; runtime can insert/read reviews but cannot rewrite them, and relay cannot read them.
- `005_down.sql`: empty database only; rollback order is 009, 008, 007, 006, 005, 004, 003, 002, 001.
- `004_up.sql`: immutable private story definitions and atomic session/turn memory. Legacy sessions retain their original pin/state/narration with empty memory defaults.
- `004_down.sql`: empty database only; apply after 005 rollback.
- `003_up.sql`: durable inference attempt/allowance ledger; deliberately separate from world-row locking so failed canon transactions cannot erase usage reservations. No world/history rewrite.
- `003_down.sql`: empty database only; refuses any world or inference attempt. Apply after 004 has been rolled back in an empty database.
- `002_down.sql`: removes version-2 additions only from an empty database; refuses populated data.
- `grants.sql`: grants only the dedicated nonprivileged external and relay logins. Provision passwords through existing operator secret workflows; no passwords are stored here.

Future schema evolution adds numbered migrations and compatibility tests. Do not revise an applied production migration to change its meaning. Retained deployment checkpoints are recorded separately in the Worldbuilder status docs. Current external requires version 9; run 001 then 002 then 003 then 004 then 005 then 006 then 007 then 008 then 009 then 010 before grants/activation. Existing v1 snapshots are unchanged and do not gain synthetic settings records; propose the first world settings record through ordinary review. A v1 application cannot start enabled against v2; rollback by disabling Worldbuilder while preserving data, not by downgrading a populated schema.

Run `scripts/worldbuilder/test-postgres.sh` from repo root. It starts a disposable PostgreSQL 16.13 container, tests all ten migration reruns/empty rollback, populated v1 upgrade with deleted-ID backfill, conflicting legacy-ID refusal, role denial, Go integration/race tests and dump/restore checksums across all 22 canon/lifecycle/story/inference/generation/governance/conversation tables plus the identity registry, then removes only that container. It never sources `.devcontainer/.env`, touches cluster credentials or connects to a production database. Docker-outside-Docker uses `host.docker.internal` when the host-published loopback port is not visible inside the devcontainer. If Docker-assigned high ports are unreachable, set `WORLDBUILDER_TEST_PORT=25433`; the harness preflights connectivity and fails promptly.

See [API and invariants](../../docs/worldbuilder/API.md) and [controlled LAN steps](../../docs/worldbuilder/LAN-RUNBOOK.md).

If published ports remain unreachable, `WORLDBUILDER_TEST_TRANSPORT=exec bash scripts/worldbuilder/test-postgres.sh` uses a local loopback socat bridge through Docker exec into the disposable container. The harness also attempts this fallback automatically after TCP preflight fails when socat is installed. The relay and container are removed on exit; no database or cluster credential files are loaded.

Current enabled binaries require v10; older binaries fail their startup version check against v10. Production rollback disables the feature and retains every populated table, including unresolved inference reservations. Inference-ledger runtime grants permit insert/read and completion fields only, not allocation/identity/time rewrites or deletion. The established-account ledger permits runtime reads only; publication triggers append records.

`ops_dashboard_queries.pgsql` is an operator-only read-only aggregate diagnostic script, tested against disposable data by the migration harness. It reports backlogs, job/lease states, allowance and governance counts without prompts, narrative output, emails or world names. See [observability](../../docs/worldbuilder/OBSERVABILITY.md).

## Historical superuser class (008)

Migration 008 introduced operator-owned superuser grants and immutable account audit history, with explicitly audited self-review. Schema 009 replaces that policy with global Administrator access and reasoned approval-count overrides; independent self-review is no longer accepted. Run ordinary `grants.sql` as the canon migration owner. Separately run `auth-grants.sql` as the existing auth table owner/database operator; it preserves the registry and creates its empty shape only in a fresh database. External gets column-scoped identity reads/inserts, never identity updates/deletes. The migration owner does not inherit the legacy app login.

Grant an already registered human using a parameterised operator SQL insert into `worldbuilder.superuser(principal,granted_by)`; do not embed an account in the migration. The grant and any deletion are automatically audited with database operator identity. Revoke by deleting the grant; grant rows cannot be updated, and regrant creates a fresh authority UUID. External cannot insert/update/delete grants. Its fixed-search-path `lock_superuser(text)` function only reads/locks an existing row and prevents revocation racing registration or publication. The global class does not bypass world membership, world archival, deterministic validation or the configured count of distinct human approvals. A superuser contributes one vote.

Self-review rows retain the grant authority alongside membership authority. Either revocation invalidates an unpublished vote, and restoration cannot revive it. Published identities remain immutable. 008 rollback refuses populated worlds/grants/audits and preserves the shared auth registry. Schema-8 backup acceptance covered 19 Worldbuilder tables plus `auth.users`.

## Collective authorship and PWA (009)

Migration 009 maps legacy owner/admin/editor/reviewer roles to Author and retains
Readers, removes the single-owner index, backfills active administrative policy
from existing settings, and invalidates pending schema-8 authority without
rewriting published decisions. It adds the immutable established-account ledger,
shared conversations/messages, and explicit story mode/scene state. External and
orchestrator now require schema 10. Refresh runtime grants: ledger writes occur only
through publication triggers; the runtime has no ledger update/delete authority.
Administrator changes use a narrow audited function with final-Administrator
protection. Auth registry access still uses separately applied auth grants.

Empty rollback is tested; populated rollback must disable the service and retain
all 22 tables plus auth.users. See [PWA operator procedure](../../docs/worldbuilder/PWA-IMPLEMENTATION.md#operator-procedure).


## Independent inference profiles (010)

Migration 010 preserves positive historical allocations and permits explicit proxy-managed zero-cost USD audit attempts. It adds server-assigned profile pins to attempts, durable generation jobs and conversation messages. Old rows retain `{"mode":"legacy"}`; they cannot silently execute under a proxy profile. Runtime grants exclude updates to these identity/configuration fields. Cloud costs live in the independent `sql/inference` ledger, not the old application allowance.

Activate `mode: proxy` only after migration/grants and local proxy acceptance. New queued work pins alias/revision at creation. A later configuration change cannot switch it to a different provider; unavailable revisions refuse execution. Populated downgrade is refused. Preserve both ledgers on rollback. The logical/physical backup seeds include legacy and proxy attempts and profile pins, covered by full-table checksums. See [inference operations](../../docs/Inference.md).
