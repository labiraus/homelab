# SeaweedFS migration

SeaweedFS replaces MinIO as the external S3 storage target. Explicit cutover overlays are prepared for `192.168.8.177:8333` on `svartalfheim`; routine CI/CD publication retains the port-9000 source settings until migration is verified. The wired address is explicit because the host also has Wi-Fi `192.168.8.202`, which LAN DNS can return but workload egress does not permit. No live deployment, object migration or backup restore is implied. The existing service and its files remain the migration source until validation and cutover are complete.

## Target and source boundaries

| Concern | Target |
| --- | --- |
| Host/service | `svartalfheim` (`192.168.8.177`), Ansible-managed `seaweedfs.service` |
| Release | SeaweedFS 4.48; amd64/arm64 downloads pinned by SHA-256 |
| S3 | Private `http://192.168.8.177:8333` on `svartalfheim`, authenticated identities |
| Internal interfaces | All SeaweedFS listeners on `127.0.0.1`; systemd socket proxy exposes only S3 HTTP on the private address; administration over SSH |
| Volume data | `/srv/minio/seaweedfs/volumes` on the existing external disk |
| Metadata | `/var/lib/seaweedfs/master` and `/var/lib/seaweedfs/filer` on the host's native filesystem |
| Buckets | `documents` (versioned), `velero`, `cnpg-backups`, `static-sites-site1` |
| Secrets | `homelab/documents-s3-credentials`, `data/cnpg-seaweedfs-s3`, `velero/velero-s3-credentials` |

The disk mount name is retained to avoid changing mounts and file shares during a storage-engine migration. SeaweedFS must never open `/srv/minio/minio-data` as its own volume directory. Copy through S3, not by copying MinIO's backend files. This single-host design has no storage replication; both volume files and filer/master metadata need independent backups. Hosting a database backup on this host does not back up SeaweedFS metadata itself.

The service requires the disk mount and stops when its mount unit disappears, preventing writes into the root-filesystem fallback directory. Existing NTFS ownership maps to `oliver`; LevelDB metadata stays off NTFS. Check native-filesystem capacity before bootstrap. The current Samba `storage` share exports `/srv/minio`: restrict SMB access to the new `seaweedfs` subtree before use so clients cannot alter volume files. Preserve existing media and Minecraft backup directories.

The official [release](https://github.com/seaweedfs/seaweedfs/releases/tag/4.48), [server flags](https://github.com/seaweedfs/seaweedfs/blob/4.48/weed/command/server.go) and [S3 configuration](https://github.com/seaweedfs/seaweedfs/wiki/Amazon-S3-API) describe the upstream interfaces. Actual compatibility is a validation gate, not an assumption from the S3 label.

## Configuration and compatibility

Run the [bootstrap](StorageBootstrap.md) to install the target alongside the source. Credentials are distinct per workload; application users receive bucket-scoped Read/Write/List actions, not Admin. Bucket bootstrap creates missing buckets and enables document versioning. It never deletes buckets, copies source data, sets lifecycle expiry or applies cluster Secrets.

The Go `minioutil` package and processor's AWS SDK v3 adapter implement S3 client compatibility using the existing `MINIO_*` environment keys. The processor's isolated SDK replacement removes the old MinIO transitive advisories and is checked by wire-contract tests and actual SeaweedFS acceptance; see [dependency validation](worldbuilder/DEPENDENCIES.md). Public MCP `minio.documents.*` names and `documents.events.minio.stored` subjects remain unchanged. Inventory direct MCP storage configuration and static-site consumers during LAN discovery; the current MCP chart does not declare its own storage credentials.

Application Helm defaults (`external`, `orchestrator`, `processor`), CNPG and `values/velero-values.yaml` retain the source endpoint and existing source Secrets. Each application and CNPG chart has a prepared `values-seaweedfs.yaml`; Velero has `values/velero-seaweedfs-values.yaml`. The application overlays switch endpoint, credentials and private S3 egress together. Helm replaces lists, so their generated Secrets and egress lists are complete. Base/GHCR/ECR renders and both storage modes are checked by `scripts/storage/check_charts.py`.

Routine source publication is authorised through CI/CD. After the gates below, merge the reviewed target overlay into the appropriate published chart/Flux values through the normal pipeline; merely adding an overlay file does not activate it. Check live Flux overrides before that cutover. No ad hoc Helm upgrade or direct cluster mutation is part of remote preparation. Source revision `8718422` previously published target defaults; this correction prevents subsequent publication from implicitly cutting over, but neither publication proves which configuration is running.

## Ordered migration gates

1. **Inventory and capacity.** Record the actual source endpoint, bucket policies, users, lifecycle rules, objects, sizes, metadata, version IDs and delete markers. Record all consumers, including CNPG WAL/base backups, Velero, static-site uploads and direct MCP clients. Check free space for both stores and native metadata, disk health/mount behavior, private routing, DNS and Samba restrictions. Take a separately restorable backup of the source and Postgres history.
2. **Provision the destination.** Supply fresh credentials and run the SeaweedFS playbook. Check the generated unit, private listener and loopback internal interfaces. Verify anonymous access fails, each workload can access its bucket, and cross-bucket access is denied. Test service restart and missing-disk refusal on a disposable host before depending on it.
3. **Prove S3 semantics.** Test list/head/get/put/copy/delete, multipart upload, range reads, content type/user metadata, version listing, exact version retrieval, delete markers and reverting a prior version. Exercise the actual Go and Node application clients against a disposable bucket. The local smoke test covers core S3 behavior; it is not a full app or production acceptance test.
4. **Resolve historical version references before copying.** `rag.documents` and audit history contain explicit source object version markers; saved assistant results and legacy/extended event JSON also need inspection. Current lifecycle `processingVersion` values identify derived-index generations, not S3 versions. `apps/orchestrator/revert.go` reads an exact `VersionID`. A normal S3 copy issues new destination version IDs. A current-object-only sync loses history and is insufficient. The [archive/copy tool and pinned compatibility resolver](StorageVersionMigration.md) preserve those markers without rewriting immutable history. Export every required version/delete marker, reconcile the complete mapping against actual database/audit/event references, and prove historical reverts with the real clients before cutover. Tool implementation is not evidence that production references are covered.
5. **Copy and reconcile.** Copy every required object/version through the authenticated S3 APIs. Preserve content type, metadata, ordering and delete semantics. Compare full content hashes and manifests; ETags alone are not reliable checksums for multipart data. Keep independent reports for missing versions, failed copies and metadata differences. Do not apply expiry during migration.
6. **Validate backups by restoring.** Copy complete CNPG base-backup and WAL chains, then restore into a disposable database and verify recovery. Validate a new backup/WAL archive against SeaweedFS too. Copy Velero backup data and restore into an isolated namespace/cluster with the intended plugin. Keep old backup storage available until retention and restore coverage are satisfied. The existing 90-day source policies are not automatically transferred; decide retention only after restore evidence exists.
7. **Cut over once.** Quiesce uploads, document edits, processors and backup writers. The initial archive/copy tool requires writers frozen for the entire final export/copy/verification into an empty destination; it does not implement incremental live sync. Install the verified map as a pinned, versioned S3 object and configure both orchestrator mapping variables. Apply generated destination Secrets, publish matching app charts and data/backup settings through the normal CI/Flux path, then verify installed values and pod revisions. Resume one consumer at a time and check uploads, reads, processing, authenticated browser delivery, historical reverts, CNPG archiving and Velero backups. Do not expose the S3 endpoint or internal APIs publicly.
8. **Observe and retire.** Keep the source read-only for an agreed observation window and verify fresh restore points on SeaweedFS. Only then schedule separate source-service retirement and data disposal. This task performs neither.

## Rollback

Before destination writes, restore the recorded endpoint/Secret/chart revisions and resume the unchanged source. Keep the old Secrets and source configuration available through the observation window.

After destination writes, freeze writers first. Inventory and reconcile new SeaweedFS objects, versions and database references before switching back; merely changing port 8333 back to 9000 would discard accepted changes. The tool supports recovery to a fresh store with `--prior-map`, composing original and intermediate markers while preserving new writes. It has no automatic rollback into a populated source or automatic resume after an uncertain copy. Prove the recovery procedure with an isolated database/application restore before cutover. Keep the source and destination backups independently recoverable.

## Local validation

Build the full devcontainer, then verify `weed version`, `terraform version` and `tflint --version`. Run Ansible syntax checks and Helm rendering offline with missing-secret placeholders. `scripts/seaweedfs-smoke.py` exercises a disposable loopback SeaweedFS server using synthetic identities and the actual role templates; it must never be pointed at production. See its invocation in [storage bootstrap](StorageBootstrap.md).

Earlier local validation on 2026-10-11 covered the full feature-enabled devcontainer build, Terraform/TFLint/SeaweedFS commands, Ansible syntax, application/CNPG renders, systemd units and disposable S3 semantics. The storage acceptance workflow now adds archive/copy checks, historical mapping and actual Go/Node application clients; it gates orchestrator/processor image and chart publication. Pi host deployment, production reference coverage, actual source-client compatibility and CNPG/Velero restores remain LAN gates. See [offline migration acceptance](StorageVersionMigration.md) for reproducible commands and limits.
