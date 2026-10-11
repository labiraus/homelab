#!/usr/bin/env bash
# Merge an optional YAML overlay without discarding defaults for an empty file.
set -euo pipefail
if [[ $# -ne 3 ]]; then
  echo "Usage: $0 BASE OVERLAY OUTPUT" >&2
  exit 2
fi
for source_file in "$1" "$2"; do
  test -f "$source_file"
  [[ $(yq eval-all '[.] | length' "$source_file") == 1 ]]
  yq --exit-status '(tag == "!!map") or (tag == "!!null")' "$source_file" >/dev/null
done
merged_values=$(mktemp "${3}.XXXXXX")
trap 'rm -f "$merged_values"' EXIT
yq eval-all '(select(fileIndex == 0) // {}) * (select(fileIndex == 1) // {})' "$1" "$2" > "$merged_values"
yq --exit-status 'tag == "!!map"' "$merged_values" >/dev/null
mv "$merged_values" "$3"
