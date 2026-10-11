# Pi Notes For Codex

Use this file for durable setup notes about Raspberry Pi hosts when the repo does not yet contain dedicated infrastructure code for them.

## svartalfheim

- Hostname: `svartalfheim`
- Address: `192.168.8.177`
- Hardware note: Raspberry Pi with an attached `8 TB` hard drive
- Disk note: existing data disk is mounted from NTFS partition UUID `C2629529629522E9`
- Current mount reality: the attached NTFS disk is mounted at `/srv/minio`
- Current mount contents note: the disk currently contains existing media files directly under `/srv/minio`
- Current workload note: Plex is being installed to serve video content already present on the attached drive
- Intended role: planned Proxmox host
- Intended storage role: planned MinIO host
- Management note: MinIO on this host should be managed with the repo's Ansible workflow
- Management note: the attached drive should also be managed as a Samba network share from the repo Ansible workflow
- Bootstrap note: repo now includes an Ansible external-host playbook that installs MinIO on `svartalfheim`, stores object data under `/srv/minio/minio-data`, and exports `/srv/minio` as a Samba share named `storage`
- Bootstrap note: this machine was recently set up and initially needed password-based SSH for first access
- Reality check note: as of 2026-03-28, `svartalfheim` did not yet have `minio.service` or `smbd.service` installed before repo-driven bootstrap
- Failure note: as of 2026-06-20, the NTFS-backed `/srv/minio` mount can silently become inactive while the disk device `/dev/sda2` is still present; MinIO then serves from the root-filesystem directory instead of the external disk and starts logging `Storage resources are insufficient for the write operation` / `no online disks found`, which lines up with CNPG WAL archive `SlowDownWrite` failures

## LAN recovery observations (2026-10-11)

- `yggdrasil` booted after operator filesystem repair. Its ext4 root is `/dev/mmcblk1p2`, UUID `4503a76b-f0a0-4337-8c84-32ca0091e510`; boot is vfat `/dev/mmcblk1p1`. SSH, sudo, kubelet/containerd and the API are working; boot recovery alone is not application-data integrity evidence.
- Both Pi hosts accept the replacement user CA via `/etc/ssh/trusted-user-ca-keys.pub`, preserving old trust and local rollback files. See the SSH recovery runbook for verified certificate fingerprints and custody.
- `svartalfheim` currently reports a 9.1 TiB NTFS data filesystem on `/dev/sda2` mounted at `/srv/minio`, about 4 TiB used and 5.2 TiB available. MinIO and Samba are active; SeaweedFS is not yet installed/running. Earlier capacity/MinIO-target notes are historical. The target is now external private SeaweedFS managed by the dedicated Ansible code; follow `docs/SeaweedFSMigration.md` and retain source data until all copy/reference/restore gates pass.
