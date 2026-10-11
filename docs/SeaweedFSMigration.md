# SeaweedFS migration

SeaweedFS replaces MinIO as the external S3 storage target. Source configuration is prepared for `svartalfheim:8333`; no live deployment, object migration or backup restore is implied. The existing service on port 9000 and its files remain the migration source until validation and cutover are complete.

## Target and source boundaries

| Concern | Target |
| --- | --- |
| Host/service | `svartalfheim` (`192.168.8.177`), Ansible-managed `seaweedfs.service` |
| Release | SeaweedFS 4.48; amd64/arm64 downloads pinned by SHA-256 |
| S3 | Private `http://svartalfheim:8333`, authenticated identities |
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

The Go `minioutil` package, Node MinIO SDK and `MINIO_*` environment keys currently implement S3 client compatibility. Their names do not require a MinIO server. Keep these contracts for this migration; replacing SDKs, public MCP `minio.documents.*` names and `documents.events.minio.stored` subjects is a separate API migration. Inventory direct MCP storage configuration and static-site consumers during LAN discovery; the current MCP chart does not declare its own storage credentials.

Application Helm defaults (`external`, `orchestrator`, `processor`) use port 8333 and the new documents Secret; their S3 egress is limited to `192.168.8.177/32`. CNPG uses the new endpoint/Secret, and `values/velero-values.yaml` uses path-style S3 with its new Secret. Check any live Flux values overrides as well as repository defaults. **Do not publish these changed charts until the gates below pass**: normal publication can trigger Flux deployment.

## Ordered migration gates

1. **Inventory and capacity.** Record the actual source endpoint, bucket policies, users, lifecycle rules, objects, sizes, metadata, version IDs and delete markers. Record all consumers, including CNPG WAL/base backups, Velero, static-site uploads and direct MCP clients. Check free space for both stores and native metadata, disk health/mount behavior, private routing, DNS and Samba restrictions. Take a separately restorable backup of the source and Postgres history.
2. **Provision the destination.** Supply fresh credentials and run the SeaweedFS playbook. Check the generated unit, private listener and loopback internal interfaces. Verify anonymous access fails, each workload can access its bucket, and cross-bucket access is denied. Test service restart and missing-disk refusal on a disposable host before depending on it.
3. **Prove S3 semantics.** Test list/head/get/put/copy/delete, multipart upload, range reads, content type/user metadata, version listing, exact version retrieval, delete markers and reverting a prior version. Exercise the actual Go and Node application clients against a disposable bucket. The local smoke test covers core S3 behavior; it is not a full app or production acceptance test.
4. **Resolve historical version references before copying.** `rag.documents`, audit history and lifecycle payloads contain source object version markers; `apps/orchestrator/revert.go` reads an exact `VersionID`. A normal S3 copy issues new destination version IDs. A current-object-only sync loses history and is insufficient. Export every required version and delete marker with an old-to-new mapping keyed by bucket, key and source version ID. Design and test a transactional database/reference migration or a compatibility resolver for those mappings, including immutable audit/event history; neither is implemented by this configuration task. This is a hard cutover gate. Never rewrite immutable history ad hoc.
5. **Copy and reconcile.** Copy every required object/version through the authenticated S3 APIs. Preserve content type, metadata, ordering and delete semantics. Compare full content hashes and manifests; ETags alone are not reliable checksums for multipart data. Keep independent reports for missing versions, failed copies and metadata differences. Do not apply expiry during migration.
6. **Validate backups by restoring.** Copy complete CNPG base-backup and WAL chains, then restore into a disposable database and verify recovery. Validate a new backup/WAL archive against SeaweedFS too. Copy Velero backup data and restore into an isolated namespace/cluster with the intended plugin. Keep old backup storage available until retention and restore coverage are satisfied. The existing 90-day source policies are not automatically transferred; decide retention only after restore evidence exists.
7. **Cut over once.** Quiesce uploads, document edits, processors and backup writers; capture final source state and perform a final delta/version reconciliation. Apply generated destination Secrets, publish matching app charts and data/backup settings through the normal CI/Flux path, then verify installed values and pod revisions. Resume one consumer at a time and check uploads, reads, processing, authenticated browser delivery, historical reverts, CNPG archiving and Velero backups. Do not expose the S3 endpoint or internal APIs publicly.
8. **Observe and retire.** Keep the source read-only for an agreed observation window and verify fresh restore points on SeaweedFS. Only then schedule separate source-service retirement and data disposal. This task performs neither.

## Rollback

Before destination writes, restore the recorded endpoint/Secret/chart revisions and resume the unchanged source. Keep the old Secrets and source configuration available through the observation window.

After destination writes, freeze writers first. Inventory and reconcile new SeaweedFS objects, versions and database references before switching back; merely changing port 8333 back to 9000 would discard accepted changes. Test the reverse mapping/restore procedure as part of the version-reference migration. Keep the source and destination backups independently recoverable.

## Local validation

Build the full devcontainer, then verify `weed version`, `terraform version` and `tflint --version`. Run Ansible syntax checks and Helm rendering offline with missing-secret placeholders. `scripts/seaweedfs-smoke.py` exercises a disposable loopback SeaweedFS server using synthetic identities and the actual role templates; it must never be pointed at production. See its invocation in [storage bootstrap](StorageBootstrap.md).

Validated locally on 2026-10-11: full feature-enabled devcontainer build; Terraform/TFLint/SeaweedFS commands; Ansible syntax; all three application charts with base, GHCR and ECR values; CNPG rendering; systemd unit verification; S3 socket proxy, authenticated bucket isolation, idempotent bootstrap, metadata, object versions, range reads, copy, multipart upload, delete markers and restart persistence. Pi host deployment, actual Go/Node client acceptance, source-version migration and CNPG/Velero restores remain unverified LAN gates.
