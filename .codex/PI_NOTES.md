# Pi Notes For Codex

Use this file for durable setup notes about Raspberry Pi hosts when the repo does not yet contain dedicated infrastructure code for them.

## svartalfheim

- Hostname: `svartalfheim`
- Address: `192.168.8.177`
- Hardware note: Raspberry Pi with an attached `8 TB` hard drive
- Disk note: existing data disk is mounted from NTFS partition UUID `C2629529629522E9`
- Current mount intent: media and storage are currently being attached under `/srv/minio`
- Current workload note: Plex is being installed to serve video content already present on the attached drive
- Intended role: planned Proxmox host
- Intended storage role: planned MinIO host
- Management note: MinIO on this host should be managed with the repo's Ansible workflow
- Bootstrap note: repo now includes an Ansible external-host playbook that installs MinIO on `svartalfheim` and stores object data under `/srv/minio/minio-data`
- Bootstrap note: this machine was recently set up and initially needed password-based SSH for first access
