## Scope

This file applies to everything under `ansible/`.

## Purpose

`ansible/` manages external services that are intentionally kept outside the Helm-managed Kubernetes manifests, including MinIO and host-level storage sharing on `svartalfheim`, plus dedicated VM-hosted workloads such as Minecraft and post-provision bootstrap for repo-managed Kubernetes worker machines.

Keep this area focused on service-state automation, not general cluster deployment.

## Working Rules

- Preserve the current separation of responsibilities: in-cluster Kubernetes workloads via Helm/Flux, worker-machine guest bootstrap via Ansible, and Proxmox VM lifecycle via Terraform.
- Treat the external MinIO service on `svartalfheim` as the authoritative MinIO target for this repo unless a task explicitly says otherwise.
- Keep inventory, group vars, playbooks, and roles aligned; update all affected layers when behavior changes.
- Prefer environment-provided secrets or vault-backed inputs over tracked secret files.
- When adding or changing Ansible secret inputs, keep shared repo env in `.devcontainer/.env` and Ansible-only secret inputs in ignored `ansible/.env`.
- Prefer pull scripts from the real source of truth, such as `scripts/ansible-fetch-secrets.sh` for `svartalfheim` MinIO admin credentials, over checked-in example secret files.
- If a playbook produces operator-facing outputs or generated manifests, document where they land and how they should be applied.
- Update `ansible/README.md` when changing role expectations, required env vars, or playbook flow.

## Safety Notes

- Do not commit rendered secrets, vault material, or local environment files.
- Treat generated output directories as artifacts unless the task explicitly says otherwise.

## Self-Learning Loop

If MinIO automation exposes a repeated operator mistake or assumption, document it here and update `ansible/README.md` when needed.
