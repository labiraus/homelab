#!/usr/bin/env bash
set -euo pipefail

scripts_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${scripts_dir}/.." && pwd)"
ansible_dir="${repo_root}/ansible"
ansible_local_tmp="${ANSIBLE_LOCAL_TEMP:-/tmp/homelab-ansible-local-${UID}}"
ansible_control_path_dir="${ANSIBLE_SSH_CONTROL_PATH_DIR:-/tmp/homelab-ansible-cp-${UID}}"

mkdir -p "${ansible_local_tmp}" "${ansible_control_path_dir}"
chmod 700 "${ansible_local_tmp}" "${ansible_control_path_dir}"

load_env_file() {
  local env_file="$1"

  if [[ -f "${env_file}" ]]; then
    set -a
    # shellcheck disable=SC1090
    . "${env_file}"
    set +a
  fi
}

load_env_file "${repo_root}/.devcontainer/.env"

if [[ "${ANSIBLE_FETCH_MINIO_SECRETS:-1}" == "1" ]]; then
  "${scripts_dir}/ansible-fetch-secrets.sh"
fi

load_env_file "${ansible_dir}/.env"

export ANSIBLE_ROLES_PATH="${ansible_dir}/roles${ANSIBLE_ROLES_PATH:+:${ANSIBLE_ROLES_PATH}}"
export ANSIBLE_LOCAL_TEMP="${ansible_local_tmp}"
export ANSIBLE_SSH_CONTROL_PATH_DIR="${ansible_control_path_dir}"

exec ansible-playbook "$@"
