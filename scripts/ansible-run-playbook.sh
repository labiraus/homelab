#!/usr/bin/env bash
set -euo pipefail

scripts_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${scripts_dir}/.." && pwd)"
ansible_dir="${repo_root}/ansible"

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

exec ansible-playbook "$@"
