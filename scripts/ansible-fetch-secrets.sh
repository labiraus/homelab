#!/usr/bin/env bash
set -euo pipefail

scripts_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${scripts_dir}/.." && pwd)"
ansible_dir="${repo_root}/ansible"
shared_env_file="${repo_root}/.devcontainer/.env"
target_env_file="${ansible_dir}/.env"
source_host="${ANSIBLE_SECRET_SOURCE_HOST:-svartalfheim}"
source_env_path="${ANSIBLE_MINIO_ENV_PATH:-/etc/default/minio}"

if [[ -f "${shared_env_file}" ]]; then
  set -a
  # shellcheck disable=SC1090
  . "${shared_env_file}"
  set +a
fi

tmp_remote_env="$(mktemp)"
trap 'rm -f "${tmp_remote_env}"' EXIT

ssh "${source_host}" "sudo cat ${source_env_path}" > "${tmp_remote_env}"

set -a
# shellcheck disable=SC1090
. "${tmp_remote_env}"
set +a

if [[ -z "${MINIO_ROOT_USER:-}" || -z "${MINIO_ROOT_PASSWORD:-}" ]]; then
  echo "Error: ${source_env_path} on ${source_host} did not provide MINIO_ROOT_USER and MINIO_ROOT_PASSWORD." >&2
  exit 1
fi

mkdir -p "$(dirname "${target_env_file}")"
touch "${target_env_file}"
chmod 600 "${target_env_file}"

upsert_env() {
  local key="$1"
  local value="$2"

  if grep -q "^${key}=" "${target_env_file}"; then
    sed -i "s|^${key}=.*|${key}='${value//\'/\'\\\'\'}'|" "${target_env_file}"
  else
    printf "%s='%s'\n" "${key}" "${value}" >> "${target_env_file}"
  fi
}

upsert_env "MINIO_EXTERNAL_ADMIN_ACCESS_KEY" "${MINIO_ROOT_USER}"
upsert_env "MINIO_EXTERNAL_ADMIN_SECRET_KEY" "${MINIO_ROOT_PASSWORD}"

echo "Synced MinIO admin credentials from ${source_host}:${source_env_path} into ${target_env_file}"
