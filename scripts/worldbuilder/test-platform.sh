#!/usr/bin/env bash
# Disposable source acceptance only. Never reads an operator kubeconfig.
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
artifacts=${1:-$(mktemp -d /tmp/homelab-platform-XXXXXX)}
mkdir -p "$artifacts"
artifacts=$(cd "$artifacts" && pwd)
export KUBECONFIG=/dev/null
export HELM_CACHE_HOME="$artifacts/helm-cache"
mkdir -p "$HELM_CACHE_HOME/repository"
export GOPATH=${GOPATH:-$artifacts/go}
export GOMODCACHE=${GOMODCACHE:-$GOPATH/pkg/mod}
export GOCACHE=${GOCACHE:-$artifacts/go-build}
export GOWORK=off
python3 "$root/scripts/worldbuilder/install-control-plane.py" "$artifacts/assets"
export PATH="$artifacts/assets:$PATH"
for chart in apps/external apps/orchestrator apps/processor apps/mcp infra/oauth2-proxy; do
  helm dependency build "$root/helm/$chart"
done
bash "$root/scripts/worldbuilder/export-crd-schemas.sh" "$artifacts/schemas"
python3 "$root/scripts/worldbuilder/check-manifests.py" --output "$artifacts/renders"
python3 "$root/scripts/worldbuilder/check-observability.py" "$artifacts/renders" "$artifacts/schemas/stack-chart/kube-prometheus-stack"
python3 "$root/scripts/worldbuilder/test-custom-resources.py"
python3 "$root/scripts/worldbuilder/validate-custom-resources.py" "$artifacts/schemas" "$artifacts/renders"
(
  cd "$root/scripts/worldbuilder/platform"
  go mod verify
  go test -mod=readonly ./...
  go build -mod=readonly -o "$artifacts/platform-acceptance" .
)
# These deliberately wrong settings prove explicit test paths/config win.
USE_EXISTING_CLUSTER=true KUBECONFIG="$artifacts/does-not-exist" \
  KUBEBUILDER_ASSETS="$artifacts/incorrect-assets" \
  TEST_ASSET_KUBE_APISERVER="$artifacts/incorrect-apiserver" \
  TEST_ASSET_ETCD="$artifacts/incorrect-etcd" TEST_ASSET_KUBECTL="$artifacts/incorrect-kubectl" \
  "$artifacts/platform-acceptance" "$artifacts/assets" "$artifacts/schemas" "$artifacts/renders"
printf 'PASS: disposable platform acceptance; artifacts: %s\n' "$artifacts"
