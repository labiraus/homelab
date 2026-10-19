## Scope

This file applies to everything under `sql/`.

## Purpose

`sql/` holds bootstrap and utility SQL used by applications and local operator workflows.

Keep SQL small, readable, and aligned with how it is invoked elsewhere in the repo.

## Working Rules

- Prefer idempotent or safely re-runnable SQL where practical.
- Keep schema assumptions explicit in the SQL itself or in nearby docs.
- If SQL is coupled to app behavior, update the relevant app README or operator documentation in the same task.
- When troubleshooting Postgres, prefer the devcontainer `psql` workflow described in the root `AGENTS.md`.

## Safety Notes

- Do not embed real credentials in SQL files.
- Avoid destructive SQL changes without clear operator intent and documentation.

## Self-Learning Loop

If a query, migration, or bootstrap step repeatedly causes confusion, document the rule here and update the relevant runbook or README.

## Worldbuilder schema evolution

Use numbered additive migrations; never rewrite immutable revision history or silently change record kinds during upgrades. Current external requires schema version 9. Apply migrations and dedicated grants explicitly before activation; no startup hook may install them. Production rollback disables the feature and preserves populated schemas. Validate populated upgrade/backfill as well as empty rollback in the disposable harness.

Administrator grants retain the legacy superuser table name; they are not PostgreSQL superuser roles. Schema 9 adds a narrow audited, fixed-search-path function for Administrator management with final-Administrator protection; runtime still has no direct grant-table write permission. Apply `worldbuilder/auth-grants.sql` separately as the existing auth table owner/operator; never give the canon migration owner the legacy app role. User registration preserves existing rows and appends an audit in the same transaction. Never run the destructive legacy `auth.pgsql` seed against a populated registry.

`CREATE SCHEMA IF NOT EXISTS` still requires database CREATE privilege even when the migration schema already exists. Grant the independent migration owner CREATE on the intended database for the explicit migration session, then revoke it; precreating the owned schema alone is insufficient. Rehearse against a production restore with actual separate legacy ownership.

## Native MCP authorization

`sql/mcp/` is separate from canon migrations. Its runtime owns no tables and cannot write `auth.users` or read canon. Apply grants explicitly; never seed or truncate the existing registry. Before exposing any restored OAuth database, run its credential-revocation script to prevent resurrection of previously revoked access. Ordinary app replacement preserves valid credentials.

Privilege audits across inaccessible schemas should use catalog OIDs and check schema USAGE separately. Name-based `has_table_privilege` lookup can raise a schema-permission error before a SQL filter excludes that schema; do not broaden the operator or runtime grants merely to collect fingerprints.

Schema 9 established accounts are append-only independently of the replaceable record projection. Ledger insertion belongs to the fixed-search-path publication trigger; runtime receives SELECT only. Never rewrite account history in restores, reconciliation or membership/policy upgrades.
