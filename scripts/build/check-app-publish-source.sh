#!/usr/bin/env bash
# A later workflow run ID must not promote older application content.
set -euo pipefail
app=${1:?application name required}
[[ "$app" =~ ^[a-z][a-z0-9-]*$ ]] || { echo 'Invalid application name' >&2; exit 1; }
case "${GITHUB_REF:-}" in refs/heads/main|refs/heads/dev) ;; *) echo 'Publication requires main or dev' >&2; exit 1;; esac
git fetch --no-tags --depth=1 origin "$GITHUB_REF"
paths=("apps/$app" apps/pkg "helm/apps/$app" helm/libraries .github/actions/docker .github/actions/helm scripts/build ".github/workflows/app-$app.yml")
if ! git diff --quiet HEAD FETCH_HEAD -- "${paths[@]}"; then
  echo 'Refusing stale application publication; rebuild the current branch revision.' >&2
  git diff --name-only HEAD FETCH_HEAD -- "${paths[@]}" >&2
  exit 1
fi
echo "PASS: $app publication content matches the current branch"
