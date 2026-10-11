# Retention And Recovery

The current platform policy is preserve-by-default while the Worldbuilder migration proceeds.

## Retention Defaults

- MinIO `documents` bucket: versioning enabled, current-object expiration disabled.
- Worldbuilder canon revisions, change sets, review/audit records, story turns/outcomes and outbox: preserve indefinitely; never expire unpublished events or pinned revisions.
- Postgres `rag.documents`: retained as the workflow source of truth.
- Legacy Postgres `rag.chunks` and `rag.embeddings`: retained until an explicit migration cleanup policy exists.
- OpenSearch RAG indexes: retained until an explicit reindexing or cleanup policy exists.
- `rag.document_lifecycle_events`: retained indefinitely as processing audit history.
- `rag.document_change_audits`: retained indefinitely as create/edit/revert audit history.
- assistant conversations, messages, memories, tool calls, file proposals, and proposal decisions: retained indefinitely and scoped by authenticated email at query time.
- NATS JetStream processing jobs and lifecycle notifications: operational transport only, not the durable retention layer.

## Cleanup Gate

Before enabling destructive lifecycle cleanup, update this file and the relevant chart/app docs with:

- the records or objects affected
- the retention duration
- the backup source that can restore them
- the restore procedure tested
- the user or operator behavior after cleanup
- the metrics or logs that prove cleanup is working

## Recovery Priorities

Recover in this order:

1. Postgres CNPG app database, including `worldbuilder`, `rag` and `assistant` schemas.
2. External MinIO `documents` bucket on `svartalfheim`, including object versions.
3. Generated Kubernetes secrets used by app, MinIO, CNPG backup, and Flux registry access.
4. Flux app releases from the published OCI charts.
5. Derived chunks and embeddings through controlled reprocessing when needed.

Worldbuilder derived state can be rebuilt from PostgreSQL revision snapshots; legacy document derived state can be rebuilt from raw MinIO objects, but audit history and user-approved assistant state live in Postgres and should be treated as primary data.

## Backup Baseline

Postgres recovery rehearsal assumes the repo-managed CNPG chart keeps both of these paths alive:

- WAL archiving to the external MinIO bucket through the `cnpg-backup-s3` secret
- a `ScheduledBackup` resource for the `app-db` cluster so base backups exist alongside WAL files

Check that baseline before treating Postgres recovery as covered:

```bash
kubectl -n data get cluster app-db -o yaml
kubectl -n data get scheduledbackup
kubectl -n data get backup
```

The repo-native shortcut for this baseline is:

```bash
make document-platform-checks
```

Do not count this baseline as healthy unless:

- `status.conditions` on `app-db` shows `type=ContinuousArchiving`, `status=True`
- at least one `ScheduledBackup` exists for `app-db`
- the current WAL archive logs do not show repeated `SlowDownWrite` / `PutObject` failures against `svartalfheim`

## Disposable physical restore acceptance

The [CNPG backup drill](worldbuilder/BACKUP-ACCEPTANCE.md) verifies physical base backup, archived WAL and latest/named point-in-time recovery against authenticated external SeaweedFS with synthetic Worldbuilder data. It removes the source Pods/PVCs before recovery and checks every populated table, permissions, immutable triggers and sequence behavior. CI requires the drill before app/chart publication. This prepares the target recovery mechanism; actual production backup inventory, all application schemas, migration-chain coverage and the LAN restore rehearsal above remain required.

The separate [Velero drill](worldbuilder/VELERO-ACCEPTANCE.md) restores synthetic files, mounted Secrets/configuration, workload resources and mapped RBAC after deleting its source namespace and volume. It exercises actual controller/node-agent replacement and S3 credential refusal. Production backup inventory, repository-password custody, all real Secrets and historical backup compatibility remain required.

## Verification

After recovery:

- run `documents.inventory.list` for a known prefix
- run `documents.history.list` for a known document
- run `documents.search` for a known gold query
- run `make ragas-chunking-eval` against the private gold cases
- run `make document-platform-checks`
- run `make vllm-gateway-smoke`
- open the Assistant tab and verify conversations, memories, proposals, and audit rows load for the authenticated user
