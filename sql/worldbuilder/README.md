# Worldbuilder schema version 10

Apply numbered migrations explicitly as an independent canon migration owner,
then `grants.sql`. Apply `auth-grants.sql` separately as the existing auth owner/
operator. No application or Helm startup hook installs SQL. External and generation
workers require exactly version 10 when enabled. Older enabled binaries fail the
version check; rollback disables the feature and retains populated data.

| Migration | Purpose |
| --- | --- |
| `001_up.sql` | Foundation, advisory migration lock, relational integrity and immutable history |
| `002_up.sql` | Administration/archive/default timeline, settings and permanent record-kind identities |
| `003_up.sql` | Durable inference reservations independent of canon rollback |
| `004_up.sql` | Private immutable story definitions and atomic turn memory |
| `005_up.sql` | Private generation jobs and transactional dispatch outbox |
| `006_up.sql` | Reviewed calendar projection and full-range tick facts |
| `007_up.sql` | Immutable authority-bound reviews and historical approval identities |
| `008_up.sql` | Operator-managed global grants and immutable account audit |
| `009_up.sql` | Authors/Readers, administrative publication policies, established-account ledger and shared conversations |
| `010_up.sql` | Immutable inference profile/accounting-mode pins; preserves historical allocations |

Run all migrations in numeric order for a new schema; rehearse populated upgrades
and migration reruns. Do not rewrite applied migration semantics or immutable history.
Migration 002 refuses historical cross-kind identity reuse rather than changing old
snapshots. Legacy worlds add missing settings through explicit reviewed publication.
Empty-database down migrations run in reverse numeric order (010 through 001) and
are not production rollback. Inspect each migration's refusal conditions.

## Roles and data protection

`worldbuilder_external` and `worldbuilder_relay` are separate non-owner logins;
legacy app/processor/MCP credentials must have no canon schema access or inherited
owner/runtime role. The migration owner must not inherit the legacy app role.
Temporarily grant required database CREATE for explicit migration and revoke it
again; precreating a schema does not remove PostgreSQL's CREATE check.

The legacy `worldbuilder.superuser` name now represents global Administrators, not
PostgreSQL superusers. Runtime cannot directly mutate grant rows. Schema 009 supplies
an audited fixed-search-path management function with final-Administrator protection.
Registration preserves `auth.users`; never apply the truncating `sql/auth.pgsql` seed.

Established-account history is append-only through the publication trigger;
runtime has SELECT only. Restores and revisions cannot omit or rewrite old accounts.
Completed shared messages and published approvals retain immutable identity/content.
Inference runtime grants allow insert/read and narrow completion fields, never
identity/pin/allocation/time rewriting or deletion. Unresolved reservations survive
rollback. Historical pre-009 role/self-review behavior is not current policy.

## Validation and diagnostics

From repository root:

```bash
WORLDBUILDER_TEST_TRANSPORT=exec bash scripts/worldbuilder/test-postgres.sh
```

The owned PostgreSQL 16.13 harness checks all ten migration reruns/empty rollbacks,
populated upgrades, legacy-ID refusal, role denial, Go integration/race tests and
full logical dump/restore. It verifies all 22 Worldbuilder tables plus shared auth,
immutable ledgers and sequences. Fast seed preflight applies
`scripts/worldbuilder/backup-seed.sql` to the migrated schema and fails on missing/
empty tables before API tests. Keep the physical fixture's table inventory aligned.
Optional actual-API Chromium uses `WORLDBUILDER_BROWSER_TEST=1`.

TCP preflight can use a configured disposable `WORLDBUILDER_TEST_PORT`; the owned
exec bridge handles inaccessible Docker-outside-Docker loopback mappings. Never load
operator DSNs. See [testing](../../docs/testing.md) for transport and cleanup rules.

`ops_dashboard_queries.pgsql` is a read-only aggregate operator diagnostic with a
five-second timeout and no private prose/identity output. Use local `psql` through
the verified operator connection. See [operations](../../docs/operations.md),
[API](../../docs/worldbuilder/api.md) and [recovery](../../docs/recovery.md).
