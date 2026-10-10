# Worldbuilder schema version 1

An additive schema alongside retained `auth`, `rag` and `assistant`. Never install as the shared legacy app user: use an independent migration owner. No application or Helm startup hook runs migrations.

- `001_up.sql`: transaction-scoped advisory lock, rerunnable initial tables, relational references and immutable history triggers.
- `001_down.sql`: drops only an empty foundation; refuses once any world exists. Production rollback disables the feature and retains data.
- `grants.sql`: grants only the dedicated nonprivileged external and relay logins. Provision passwords through existing operator secret workflows; no passwords are stored here.

Future schema evolution adds numbered migrations and compatibility tests. Do not revise an applied production migration to change its meaning. Version 1 is pre-deployment preparation in this session.

Run `scripts/worldbuilder/test-postgres.sh` from repo root. It starts a disposable PostgreSQL 16.13 container, tests reruns and rollback, role denial, Go integration/race tests and dump/restore checksums, then removes only that container. It never sources `.devcontainer/.env`, touches cluster credentials or connects to a production database. Docker-outside-Docker uses `host.docker.internal` when the host-published loopback port is not visible inside the devcontainer. If Docker-assigned high ports are unreachable, set `WORLDBUILDER_TEST_PORT=25433`; the harness preflights connectivity and fails promptly.

See [API and invariants](../../docs/worldbuilder/API.md) and [controlled LAN steps](../../docs/worldbuilder/LAN-RUNBOOK.md).
