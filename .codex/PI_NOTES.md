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
