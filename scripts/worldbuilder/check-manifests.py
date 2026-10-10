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
# Delegation activation requires separate principals and an indirect secret.
for chart in ['external','mcp']:
    directory=root/'helm/apps'/chart
    cmd=['helm','template','homelab-'+chart,str(directory),'-f',str(directory/'values-ghcr.yaml'),'--set','global.allowMissingGeneratedSecretRefs=true','--set','worldbuilder.enabled=true']
    if chart=='external':
        cmd+=['--set','worldbuilder.browserPrincipal=cluster.local/ns/homelab/sa/browser','--set','worldbuilder.mcp.enabled=true','--set','worldbuilder.mcp.delegationSecret=worldbuilder-mcp']
        principal='worldbuilder.mcp.principal'
    else:
        cmd+=['--set','worldbuilder.delegationSecret=worldbuilder-mcp']
        principal='worldbuilder.trustedEdgePrincipals[0]'
    for invalid in ['', '*']:
        rejected=subprocess.run(cmd+(['--set',principal+'='+invalid] if invalid else []),capture_output=True,text=True)
        if rejected.returncode==0:failures.append(chart+' accepted unverified delegation principal')
    valid='cluster.local/ns/homelab/sa/'+('homelab-mcp' if chart=='external' else 'browser')
    result=subprocess.run(cmd+['--set',principal+'='+valid],capture_output=True,text=True)
    (out/(chart+'-delegation-enabled.yaml')).write_text(result.stdout+result.stderr)
    if result.returncode:failures.append(chart+' delegation enabled template')
# AI must fail closed without an operator-defined allowance (values are test-only).
ai=['helm','template','homelab-external',str(root/'helm/apps/external'),'--set','global.allowMissingGeneratedSecretRefs=true','--set','worldbuilder.enabled=true','--set','worldbuilder.browserPrincipal=cluster.local/ns/homelab/sa/browser','--set','worldbuilder.aiSecret=worldbuilder-ai']
if subprocess.run(ai,capture_output=True).returncode==0:failures.append('AI activation accepted missing allowance')
result=subprocess.run(ai+['--set','worldbuilder.aiBudget.requestMicros=100','--set','worldbuilder.aiBudget.dailyMicros=1000','--set','worldbuilder.aiBudget.currency=TST','--set','worldbuilder.aiBudget.gatewayRoute=offline-fixture'],capture_output=True,text=True)
(out/'external-ai-budget-enabled.yaml').write_text(result.stdout+result.stderr)
if result.returncode:failures.append('AI explicit allowance template')
# Background workers require a separately scoped credential and exact mesh identity.
allowance=['--set','worldbuilder.aiBudget.requestMicros=100','--set','worldbuilder.aiBudget.dailyMicros=1000','--set','worldbuilder.aiBudget.currency=TST','--set','worldbuilder.aiBudget.gatewayRoute=offline-fixture']
for chart in ['external','orchestrator']:
    base=['helm','template','homelab-'+chart,str(root/'helm/apps'/chart),'--set','global.allowMissingGeneratedSecretRefs=true','--set','worldbuilder.enabled=true','--set','worldbuilder.jobs.enabled=true']
    if chart=='external':
        base+=['--set','worldbuilder.browserPrincipal=cluster.local/ns/homelab/sa/browser','--set','worldbuilder.jobs.principal=cluster.local/ns/homelab/sa/homelab-orchestrator']
        if subprocess.run(base,capture_output=True).returncode==0:failures.append('jobs accepted missing AI configuration')
        base+=['--set','worldbuilder.aiSecret=worldbuilder-ai']+allowance
    if subprocess.run(base,capture_output=True).returncode==0:failures.append(chart+' jobs accepted missing worker secret')
    base+=['--set','worldbuilder.jobs.workerSecret=worldbuilder-worker']
    if chart=='external':
        for principal in ['', '*','cluster.local/ns/homelab/sa/browser']:
            if subprocess.run(base+['--set','worldbuilder.jobs.principal='+principal],capture_output=True).returncode==0:failures.append('jobs accepted invalid worker principal')
    result=subprocess.run(base,capture_output=True,text=True)
    (out/(chart+'-jobs-enabled.yaml')).write_text(result.stdout+result.stderr)
    if result.returncode:failures.append(chart+' jobs enabled template')
    if subprocess.run(base+['--set','worldbuilder.enabled=false'],capture_output=True).returncode==0:failures.append(chart+' jobs accepted disabled Worldbuilder')
print(f'{len(checked)} base/overlay combinations checked; local artifacts: {out}')
print('Failures: '+(', '.join(failures) if failures else 'none'))
sys.exit(bool(failures))
