#!/usr/bin/env bash
set -euo pipefail

scripts_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${scripts_dir}/.." && pwd)"
ansible_dir="${repo_root}/ansible"
ansible_env_file="${ansible_dir}/.env"
source_host="${ANSIBLE_SECRET_SOURCE_HOST:-svartalfheim}"
source_env_path="${ANSIBLE_MINIO_ENV_PATH:-/etc/default/minio}"

require_env_key() {
  local key="$1"

  if ! grep -q "^${key}=" "${ansible_env_file}" 2>/dev/null; then
    echo "Error: ${ansible_env_file} is missing ${key}." >&2
    exit 1
  fi
}

if [[ ! -f "${ansible_env_file}" ]]; then
  echo "Error: ${ansible_env_file} does not exist." >&2
  echo "Create it with MINIO_EXTERNAL_ADMIN_ACCESS_KEY, MINIO_EXTERNAL_ADMIN_SECRET_KEY, and SVARTALFHEIM_SAMBA_PASSWORD first." >&2
  exit 1
fi

require_env_key "MINIO_EXTERNAL_ADMIN_ACCESS_KEY"
require_env_key "MINIO_EXTERNAL_ADMIN_SECRET_KEY"
require_env_key "SVARTALFHEIM_SAMBA_PASSWORD"

if ssh "${source_host}" "test -f ${source_env_path}"; then
  export ANSIBLE_FETCH_MINIO_SECRETS=1
  echo "Detected existing MinIO env on ${source_host}; bootstrap will refresh admin credentials before running."
else
  export ANSIBLE_FETCH_MINIO_SECRETS=0
  echo "No MinIO env found on ${source_host}; bootstrap will use local ansible/.env values for first install."
fi

cd "${ansible_dir}"
exec "${scripts_dir}/ansible-run-playbook.sh" -i inventory/hosts.ini playbooks/minio-external-pi-host.yml
