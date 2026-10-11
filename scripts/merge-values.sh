#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "Usage: $0 <chart-folder> <values-suffix>"
  echo "Example: $0 ./charts/myapp single"
  exit 1
fi

CHART_DIR="$1"
SUFFIX="$2"

BASE_VALUES="${CHART_DIR}/values.yaml"
MERGED_OUTPUT="${CHART_DIR}/values.effective.yaml"
OVERLAY_VALUES="${CHART_DIR}/values-${SUFFIX}.yaml"

if [[ ! -f "$BASE_VALUES" ]]; then
  echo "Error: ${BASE_VALUES} not found"
  exit 1
fi

if [[ ! -f "$OVERLAY_VALUES" ]]; then
  echo "Error: ${OVERLAY_VALUES} not found"
  exit 1
fi

echo "Merging:"
echo "  Base:    ${BASE_VALUES}"
echo "  Overlay: ${OVERLAY_VALUES}"
echo

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
bash "$repo_root/scripts/build/merge-helm-values.sh" "$BASE_VALUES" "$OVERLAY_VALUES" "$MERGED_OUTPUT"

echo "Successfully merged values-${SUFFIX}.yaml into values.yaml"
