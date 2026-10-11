# Disposable Velero recovery

The [Velero harness](../../scripts/worldbuilder/test-velero.py) exercises the real controller, node-agent, AWS plugin and Kopia file backup against authenticated external SeaweedFS. It uses synthetic files, credentials and Kubernetes resources in an owned kind cluster. It requires no LAN access or operator kubeconfig.

With the Docker/Helm/Python prerequisites from [offline acceptance](OFFLINE-ACCEPTANCE.md):

```bash
python3 -m pip install -r scripts/worldbuilder/requirements.txt -r scripts/storage/requirements.txt
python3 scripts/worldbuilder/test-velero-unit.py
python3 scripts/worldbuilder/test-velero.py "$artifacts/velero-acceptance"
```

Use a fresh output directory. The explicit registry CA option from [controller acceptance](CONTROLLER-ACCEPTANCE.md) also applies. The reusable platform workflow requires this job before its app/Helm callers can publish artifacts.

## Source and fixture boundaries

The input is `values/velero-seaweedfs-values.yaml`, retaining `deployNodeAgent: true`, disabled snapshots, path-style S3, the `velero` bucket and TLS-verification policy. Fixture overrides supply only its private endpoint, namespace and synthetic credential reference. Comparison pins are Helm chart 12.2.1 / Velero 1.18.2 and AWS plugin 1.14.4; they are not an assertion about the installed estate. Original chart CRDs are applied before controller resources, so the Helm CRD-upgrade hook is disabled inside the fixture.

The shared [S3 fixture](../../scripts/worldbuilder/s3-fixture.py) uses SeaweedFS 4.48 pinned by digest and the Ansible target's bucket-scoped identity actions. Master/filer/volume listeners stay on container loopback; S3 is on the private Docker network with no host ports. A separate synthetic repository password is created before the first Kopia backup.

Velero [file-system backup](https://velero.io/docs/v1.18/file-system-backup/) supports local PVs, but not hostPath volumes. The fixture's PVC explicitly requests `volumeType: local` and verifies the actual PV and claim binding. Application startup only serves existing files; it cannot recreate missing payloads. Writes stop before backup. This is file/resource recovery, not a claim of live database consistency or CSI/Longhorn snapshot support.

## Required acceptance

1. The real Deployment, node-agent and authenticated BackupStorageLocation become Ready/Available. Anonymous S3 requests are denied.
2. A Deployment serves a PVC containing a 16 MiB binary payload, Unicode text/path, hidden and empty files, an empty directory and a relative symlink. The fixture records SHA-256, byte count, mode, owner/group and modification time. ConfigMap/Secret mounts, a Service and a restricted ServiceAccount/Role/RoleBinding are populated too.
3. A real Backup and its PodVolumeBackup complete. Partial, failed, warning-bearing or error-bearing results fail the gate; a Kopia snapshot ID is required. The generated source `kube-root-ca.crt` gets Velero's [exclusion label](https://velero.io/docs/v1.18/resource-filtering/#veleroioexclude-from-backuptrue) so the mapped namespace keeps fresh cluster trust. An annotation is not an exclusion.
4. Delete the source namespace and wait for its PVC/PV and original data directory to disappear. No source data is available to satisfy subsequent reads.
5. Wrong scoped S3 credentials make backup storage Unavailable with an actual S3 denial. The attempted restore must fail without destination Pods/PVCs. Restore valid credentials, replace both controller and node-agent Pods, and require new Ready UIDs and available storage.
6. Restore through a [namespace mapping](https://velero.io/docs/v1.18/restore-reference/) and require zero errors/warnings plus a completed PodVolumeRestore. The destination must have distinct PVC/PV UIDs and a different data path. Compare the full application file manifest and portable configuration/resource state; only allocated Service IPs, bound volume names and the intended RoleBinding namespace mapping differ.
7. Read restored mounted ConfigMap/Secret values, exercise allowed configuration reads and denied Secret/cross-namespace reads as the restored ServiceAccount, and fetch restored text through its Service.
8. Remove the owned kind and S3 containers/anonymous fixture volumes. Cleanup failure fails the process and CI job.

The reserved `.velero` restore-helper directory is outside the synthetic application manifest. No other extra/missing application file is accepted. File data, immutable ConfigMap/Secret content, Deployment spec, service selection/ports, PVC request and RBAC definitions are compared before and after recovery.

## Evidence and remaining gates

`velero-result.json` records file metadata/hashes, a hash of portable resource content, observed image IDs, source removal and refusal/restart assertions. Public chart inputs/rendering and controller/node-agent/restore/status/event logs are retained. Private generated kubeconfig is excluded. Require the final process/workflow result as well as the JSON assertions, because teardown follows the assertions.

Production source backups, historical Restic/Kopia compatibility, backup-chain migration, repository-password custody, actual generated cluster Secrets, installed versions, application quiescing and hardware/Longhorn/power-loss behavior remain estate-specific checks. This drill does not install or upgrade production Velero, switch backup storage or activate application features. Prometheus admission, persistent TSDB recovery and mesh enforcement remain separate offline work.
