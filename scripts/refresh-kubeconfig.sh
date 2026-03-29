#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target_path="${KUBECONFIG_TARGET_PATH:-${repo_root}/.devcontainer/kubeconfig}"
source_host="${KUBECONFIG_SOURCE_HOST:-yggdrasil}"
source_path="${KUBECONFIG_SOURCE_PATH:-~/.kube/config}"

mkdir -p "$(dirname "${target_path}")"
ssh "${source_host}" "cat ${source_path}" > "${target_path}"
chmod 600 "${target_path}"

echo "Refreshed kubeconfig at ${target_path} from ${source_host}:${source_path}"
