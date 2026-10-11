#!/usr/bin/env bash
# Fetch/render public schema artifacts only. No Kubernetes connection or mutation.
set -euo pipefail
schema_dir=${1:-$(mktemp -d /tmp/worldbuilder-crds-XXXXXX)}
mkdir -p "$schema_dir/helm-cache/repository"
export HELM_CACHE_HOME="$schema_dir/helm-cache"
helm template cnpg cloudnative-pg --version 0.23.2 --repo https://cloudnative-pg.github.io/charts --include-crds > "$schema_dir/cnpg.yaml"
helm template keda keda --version 2.16.1 --repo https://kedacore.github.io/charts --include-crds > "$schema_dir/keda.yaml"
helm template istio base --version 1.30.5 --repo https://istio-release.storage.googleapis.com/charts --include-crds > "$schema_dir/istio.yaml"
helm template ai-crds oci://docker.io/envoyproxy/ai-gateway-crds-helm --version v1.0.0 --include-crds > "$schema_dir/ai-gateway.yaml"
python3 - "$schema_dir" <<'PY'
import hashlib,json,pathlib,sys,urllib.request
out=pathlib.Path(sys.argv[1])
(out/'flux.yaml').write_bytes(urllib.request.urlopen('https://github.com/fluxcd/flux2/releases/download/v2.9.6/install.yaml',timeout=30).read())
base='https://raw.githubusercontent.com/envoyproxy/gateway/v1.7.0/charts/gateway-helm/crds/'
for name in ['gatewayapi-crds.yaml','generated/gateway.envoyproxy.io_backends.yaml','generated/gateway.envoyproxy.io_envoyproxies.yaml']:
    (out/('envoy-'+name.split('/')[-1])).write_bytes(urllib.request.urlopen(base+name,timeout=30).read())
metadata={'versions':{'envoyGateway':'v1.7.0','envoyAI':'v1.0.0','cnpgChart':'0.23.2','kedaChart':'2.16.1','istioComparison':'1.30.5','flux':'v2.9.6'},'sha256':{p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in out.glob('*.yaml')}}
(out/'sources.json').write_text(json.dumps(metadata,indent=2)+'\n')
PY
printf 'Schema artifacts: %s\n' "$schema_dir"
