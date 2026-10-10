#!/usr/bin/env python3
"""Local render/build checks. Never invokes cluster discovery, apply or reconciliation."""
import pathlib, subprocess, tempfile, sys
root=pathlib.Path(__file__).resolve().parents[2]
out=pathlib.Path(tempfile.mkdtemp(prefix='worldbuilder-manifests-'))
(out/'dockerconfig.json').write_text('{"auths":{}}')
charts=['apps/external','apps/orchestrator','apps/processor','apps/mcp','apps/ui','data/postgres','bootstrap/flux-apps','bootstrap/flux-data','bootstrap/flux-infra','infra/vllm','infra/envoy-ai-gateway']
failures=[]; checked=[]
for chart in charts:
    directory=root/'helm'/chart
    for overlay in ['values.yaml','values-ghcr.yaml','values-ecr.yaml']:
        if not (directory/overlay).exists(): continue
        name=chart.replace('/','-')+'-'+overlay.removesuffix('.yaml')
        args=['-f',str(directory/overlay),'--set','global.allowMissingGeneratedSecretRefs=true']
        if chart=='bootstrap/flux-apps':
            args+=['--set-file','imagePullSecret.dockerconfigjson='+str(out/'dockerconfig.json')]
        if chart.startswith('bootstrap/'):
            args+=['--set','image.repository=ghcr.io/labiraus/homelab']
        for action in ['lint','template']:
            cmd=['helm',action]+(['homelab-'+directory.name] if action=='template' else [])+[str(directory)]+args
            result=subprocess.run(cmd,capture_output=True,text=True)
            (out/(name+'.'+action)).write_text(result.stdout+result.stderr)
            if result.returncode: failures.append(name+' '+action)
            elif action=='template':
                build=out/name;build.mkdir();(build/'resources.yaml').write_text(result.stdout)
                (build/'kustomization.yaml').write_text('resources:\n  - resources.yaml\n')
                result=subprocess.run(['kubectl','kustomize',str(build)],capture_output=True,text=True)
                (out/(name+'.kustomize')).write_text(result.stdout+result.stderr)
                if result.returncode:failures.append(name+' kustomize')
        checked.append(name)
# Explicit activation render and missing-principal refusal. No secret values used.
for chart in ['external','orchestrator']:
    directory=root/'helm/apps'/chart
    cmd=['helm','template','homelab-'+chart,str(directory),'-f',str(directory/'values-ghcr.yaml'),'--set','global.allowMissingGeneratedSecretRefs=true','--set','worldbuilder.enabled=true']
    if chart=='external':
        missing=subprocess.run(cmd,capture_output=True,text=True)
        if missing.returncode==0:failures.append('external activation accepted missing browserPrincipal')
        cmd+=['--set','worldbuilder.browserPrincipal=cluster.local/ns/homelab/sa/homelab-oauth2-proxy']
    result=subprocess.run(cmd,capture_output=True,text=True)
    (out/(chart+'-enabled.yaml')).write_text(result.stdout+result.stderr)
    if result.returncode:failures.append(chart+' enabled template')
print(f'{len(checked)} base/overlay combinations checked; local artifacts: {out}')
print('Failures: '+(', '.join(failures) if failures else 'none'))
sys.exit(bool(failures))
