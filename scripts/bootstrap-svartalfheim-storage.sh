#!/usr/bin/env bash
set -euo pipefail

scripts_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${scripts_dir}/.." && pwd)"

# Credentials come from the local ansible/.env. SeaweedFS never fetches MinIO keys.
export ANSIBLE_FETCH_MINIO_SECRETS=0
cd "${repo_root}"
exec bash "${scripts_dir}/ansible-run-playbook.sh" \
  -i ansible/inventory/hosts.ini ansible/playbooks/seaweedfs-external-pi-host.yml "$@"
