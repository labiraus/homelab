#!/usr/bin/env bash
# Fetch/render public schema artifacts only. No Kubernetes connection or mutation.
set -euo pipefail
schema_dir=${1:-$(mktemp -d /tmp/worldbuilder-crds-XXXXXX)}
mkdir -p "$schema_dir/helm-cache/repository"
export HELM_CACHE_HOME="$schema_dir/helm-cache"
helm template cnpg cloudnative-pg --version 0.29.1 --repo https://cloudnative-pg.github.io/charts --include-crds > "$schema_dir/cnpg.yaml"
helm template keda keda --version 2.21.0 --repo https://kedacore.github.io/charts --include-crds > "$schema_dir/keda.yaml"
helm template istio base --version 1.30.7 --repo https://istio-release.storage.googleapis.com/charts --include-crds > "$schema_dir/istio.yaml"
helm template ai-crds oci://docker.io/envoyproxy/ai-gateway-crds-helm --version v1.2.0 --include-crds > "$schema_dir/ai-gateway.yaml"
helm pull kube-prometheus-stack --version 92.14.1 --repo https://prometheus-community.github.io/helm-charts --untar --untardir "$schema_dir/stack-chart"
helm template prometheus "$schema_dir/stack-chart/kube-prometheus-stack" --kube-version 1.37.0 --include-crds > "$schema_dir/prometheus.yaml"
python3 - "$schema_dir" <<'PY'
import hashlib,json,pathlib,sys,urllib.request
out=pathlib.Path(sys.argv[1])
(out/'flux.yaml').write_bytes(urllib.request.urlopen('https://github.com/fluxcd/flux2/releases/download/v2.10.0/install.yaml',timeout=30).read())
base='https://raw.githubusercontent.com/envoyproxy/gateway/v1.9.3/charts/gateway-helm/charts/crds/crds/'
for name in ['gatewayapi-crds.yaml','generated/gateway.envoyproxy.io_backends.yaml','generated/gateway.envoyproxy.io_envoyproxies.yaml','generated/gateway.envoyproxy.io_clienttrafficpolicies.yaml']:
    (out/('envoy-'+name.split('/')[-1])).write_bytes(urllib.request.urlopen(base+name,timeout=30).read())
metadata={'versions':{'envoyGateway':'v1.9.3','envoyAI':'v1.2.0','cnpgChart':'0.29.1','kedaChart':'2.21.0','istioComparison':'1.30.7','flux':'v2.10.0','prometheusStackComparison':'92.14.1'},'sha256':{p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in out.glob('*.yaml')}}
(out/'sources.json').write_text(json.dumps(metadata,indent=2)+'\n')
PY
printf 'Schema artifacts: %s\n' "$schema_dir"
