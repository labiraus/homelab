# Worldbuilder schema version 5

An additive schema alongside retained `auth`, `rag` and `assistant`. Never install as the shared legacy app user: use an independent migration owner. No application or Helm startup hook runs migrations.

- `001_up.sql`: transaction-scoped advisory lock, rerunnable initial tables, relational references and immutable history triggers.
- `001_down.sql`: drops only an empty foundation; refuses once any world exists. Production rollback disables the feature and retains data.
- `002_up.sql`: additive archive/administration version/default timeline columns, reviewed settings kind, single-owner/settings indexes and immutable record identity registry backfilled from all revisions. Historical reuse of an ID with different kinds aborts before schema changes; inventory such data rather than rewriting history.
- `005_up.sql`: private generation jobs and transactional dispatch outbox; inference attempts gain opaque job/part attribution. Relay cannot read private jobs. Runtime changes only checkpoint/status fields.
- `005_down.sql`: empty database only; rollback order is 005, 004, 003, 002, 001.
- `004_up.sql`: immutable private story definitions and atomic session/turn memory. Legacy sessions retain their original pin/state/narration with empty memory defaults.
- `004_down.sql`: empty database only; apply after 005 rollback.
- `003_up.sql`: durable inference attempt/allowance ledger; deliberately separate from world-row locking so failed canon transactions cannot erase usage reservations. No world/history rewrite.
- `003_down.sql`: empty database only; refuses any world or inference attempt. Apply after 004 has been rolled back in an empty database.
- `002_down.sql`: removes version-2 additions only from an empty database; refuses populated data.
- `grants.sql`: grants only the dedicated nonprivileged external and relay logins. Provision passwords through existing operator secret workflows; no passwords are stored here.

Future schema evolution adds numbered migrations and compatibility tests. Do not revise an applied production migration to change its meaning. Versions 1 through 5 remain pre-deployment preparation. Current external requires version 5; run 001 then 002 then 003 then 004 then 005 before grants/activation. Existing v1 snapshots are unchanged and do not gain synthetic settings records; propose the first world settings record through ordinary review. A v1 application cannot start enabled against v2; rollback by disabling Worldbuilder while preserving data, not by downgrading a populated schema.

Run `scripts/worldbuilder/test-postgres.sh` from repo root. It starts a disposable PostgreSQL 16.13 container, tests all five migration reruns/empty rollback, populated v1 upgrade with deleted-ID backfill, conflicting legacy-ID refusal, role denial, Go integration/race tests and dump/restore checksums across all 16 canon/lifecycle/story/inference/generation tables, then removes only that container. It never sources `.devcontainer/.env`, touches cluster credentials or connects to a production database. Docker-outside-Docker uses `host.docker.internal` when the host-published loopback port is not visible inside the devcontainer. If Docker-assigned high ports are unreachable, set `WORLDBUILDER_TEST_PORT=25433`; the harness preflights connectivity and fails promptly.

See [API and invariants](../../docs/worldbuilder/API.md) and [controlled LAN steps](../../docs/worldbuilder/LAN-RUNBOOK.md).

If published ports remain unreachable, `WORLDBUILDER_TEST_TRANSPORT=exec bash scripts/worldbuilder/test-postgres.sh` uses a local loopback socat bridge through Docker exec into the disposable container. The harness also attempts this fallback automatically after TCP preflight fails when socat is installed. The relay and container are removed on exit; no database or cluster credential files are loaded.

Current enabled binaries require v5; older v1/v2/v3/v4 binaries fail their startup version check against v5. Production rollback disables the feature and retains every populated table, including unresolved inference reservations. Ledger runtime grants permit insert/read and completion fields only, not allocation/identity/time rewrites or deletion.
