#!/usr/bin/env bash
# Exercise the publication merger and render its real packaged Prometheus chart.
set -euo pipefail
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
merge_values() { bash "$repo_root/scripts/build/merge-helm-values.sh" "$@"; }
cat > "$fixture/base.yaml" <<'YAML'
feature:
  enabled: false
  retained: default
items: [one, two]
YAML
for overlay in '' '# comment only' 'null' '{}'; do
  printf '%s\n' "$overlay" > "$fixture/overlay.yaml"
  merge_values "$fixture/base.yaml" "$fixture/overlay.yaml" "$fixture/merged.yaml"
  diff <(yq -o=json '.' "$fixture/base.yaml") <(yq -o=json '.' "$fixture/merged.yaml")
done
printf 'feature:\n  enabled: true\nitems: [three]\n' > "$fixture/overlay.yaml"
merge_values "$fixture/base.yaml" "$fixture/overlay.yaml" "$fixture/merged.yaml"
yq -e '(.feature.enabled == true) and (.feature.retained == "default") and ((.items | length) == 1) and (.items[0] == "three")' "$fixture/merged.yaml" >/dev/null
for overlay in 'false' '[one]' $'{}\n---\n{}' '['; do
  printf '%s\n' "$overlay" > "$fixture/overlay.yaml"
  cp "$fixture/base.yaml" "$fixture/merged.yaml"
  if merge_values "$fixture/base.yaml" "$fixture/overlay.yaml" "$fixture/merged.yaml" 2>/dev/null; then
    echo 'Invalid overlay unexpectedly accepted' >&2; exit 1
  fi
  cmp "$fixture/base.yaml" "$fixture/merged.yaml"
done
cp -R "$repo_root/helm/observability/prometheus" "$fixture/prometheus"
merge_values "$fixture/prometheus/values.yaml" "$fixture/prometheus/values-ghcr.yaml" "$fixture/prometheus/values.yaml"
helm package "$fixture/prometheus" --destination "$fixture" >/dev/null
package=$(find "$fixture" -maxdepth 1 -name 'prometheus-*.tgz' -print)
test -n "$package"
helm show values "$package" | yq -e '(.worldbuilderObservability.enabled == false) and (.prometheusStorage.enabled == false) and (.stack.targetNamespace == "istio-system")' >/dev/null
helm template packaged "$package" > "$fixture/packaged.yaml"
helm template packaged "$repo_root/helm/observability/prometheus" -f "$repo_root/helm/observability/prometheus/values-ghcr.yaml" > "$fixture/source.yaml"
diff "$fixture/source.yaml" "$fixture/packaged.yaml"
echo 'PASS: empty/null overlays preserve defaults; map/array merge, refusal and packaged Prometheus parity'
helm template bootstrap "$repo_root/helm/bootstrap/flux-bootstrap" --set image.repository=ghcr.io/labiraus/homelab --show-only templates/istio.yaml > "$fixture/parent.yaml"
helm template istio "$repo_root/helm/bootstrap/istio" --show-only templates/istiod-release.yaml > "$fixture/child.yaml"
yq 'select(.kind == "HelmRelease") | .spec.values.meshConfig.extensionProviders' "$fixture/parent.yaml" > "$fixture/parent-auth.yaml"
yq '.spec.values.meshConfig.extensionProviders' "$fixture/child.yaml" > "$fixture/child-auth.yaml"
diff "$fixture/parent-auth.yaml" "$fixture/child-auth.yaml"
yq -e '.[0].envoyExtAuthzHttp.pathPrefix == "/oauth2/auth?"' "$fixture/parent-auth.yaml" >/dev/null
echo 'PASS: bootstrap overrides preserve the child auth-only provider configuration'
