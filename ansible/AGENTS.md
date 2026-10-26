# Ansible rules

Apply the root policy and these directory-specific constraints.

## Working rules

Ansible manages external services and guest bootstrap; SeaweedFS is the target, MinIO is legacy migration/rollback tooling. Never commit credentials or rendered Secret outputs.

- Preserve the current separation of responsibilities: in-cluster Kubernetes workloads via Helm/Flux, worker-machine guest bootstrap via Ansible, and Proxmox VM lifecycle via Terraform.
- SeaweedFS on `svartalfheim` is the target storage service; `playbooks/seaweedfs-external-pi-host.yml` owns its host and bucket bootstrap. The existing `minio-external-pi*.yml` playbooks are legacy migration/rollback tooling.
- Discover actual inventory and playbook paths before editing: this repo uses `inventory/hosts.ini` and `inventory/group_vars/minio_external_pi_host.yml` for existing Pi settings, not an assumed `hosts.yml` or `host_vars/svartalfheim.yml`. The storage role is `roles/seaweedfs/`, not `roles/seaweedfs_host/`.
- SeaweedFS must require the external mount before creating volume directories and at service startup. Keep its filer/master metadata on a native filesystem, separate from the NTFS volume data, and never reuse MinIO's data directory.
- SeaweedFS 4.48 shares the S3 bind address with internal gRPC. Keep the server on loopback and expose only S3 HTTP through the systemd socket proxy; do not bind `weed server -s3.ip.bind` directly to the LAN.
- Keep inventory, group vars, playbooks, and roles aligned; update all affected layers when behavior changes.
- Prefer environment-provided secrets or vault-backed inputs over tracked secret files.
- When adding or changing Ansible secret inputs, keep shared repo env in `.devcontainer/.env` and Ansible-only secret inputs in ignored `ansible/.env`.
- Load SeaweedFS credentials from local `ansible/.env`; generated Kubernetes Secrets stay under ignored `ansible/out/`. MinIO credential fetching is explicit legacy behavior and must not run during SeaweedFS bootstrap.
- If a playbook produces operator-facing outputs or generated manifests, document where they land and how they should be applied.
- Update `ansible/README.md` when changing role expectations, required env vars, or playbook flow.
- Before changing an FTB version pin, verify the requested release name against both the live pack API and the version API (`https://api.feed-the-beast.com/v1/modpacks/public/modpack/<pack-id>/<version-id>`). Indexed server-download pages can pair a stale release label with a newer version ID.
