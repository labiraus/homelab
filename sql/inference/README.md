# Independent inference ledger

Apply `001_up.sql` then `grants.sql` explicitly as the independent database migration owner. Provision `inference_runtime` as a non-owner login with NOINHERIT, NOSUPERUSER, NOCREATEROLE and NOBYPASSRLS first. Use a separate database/credentials from Worldbuilder. No startup migration or role creation occurs.

The ledger stores immutable request identities, profile/tariff versions and integer USD-micro reservations. Runtime may insert, read and settle once through narrow column grants and a trigger; it cannot delete or rewrite reservations. Duplicate IDs never dispatch twice. Financial holds survive expired capacity leases and restarts. There is deliberately no populated down migration or automatic reservation release.

`bash scripts/inference/test-postgres.sh` creates an owned disposable PostgreSQL instance, tests concurrent accounting with independent pools and role isolation, and verifies populated dump/restore checksums. Backups must include this database and its roles; an older restore or empty database must not reset a live month's budget. Keep cloud disabled until restored history is reconciled. See [operator procedures](../../docs/Inference.md).
