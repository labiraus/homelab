#!/usr/bin/env python3
"""Local render/build checks. Never invokes cluster discovery, apply or reconciliation."""
import argparse, pathlib, subprocess, tempfile, sys
import yaml
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--output',type=pathlib.Path,help='New directory for render artifacts')
options=parser.parse_args()
root=pathlib.Path(__file__).resolve().parents[2]
out=options.output or pathlib.Path(tempfile.mkdtemp(prefix='worldbuilder-manifests-'))
if options.output:out.mkdir(parents=True)
(out/'dockerconfig.json').write_text('{"auths":{}}')
charts=['apps/external','apps/orchestrator','apps/processor','apps/mcp','apps/ui','data/postgres','bootstrap/flux-apps','bootstrap/flux-data','bootstrap/flux-infra','bootstrap/flux-observability','infra/vllm','infra/envoy-ai-gateway','observability/prometheus']
failures=[]; checked=[]
def save_render(path,result):
    path.write_text(result.stdout)
    if result.stderr:path.with_suffix(path.suffix+'.stderr').write_text(result.stderr)

for chart in charts:
    directory=root/'helm'/chart
    for overlay in ['values.yaml','values-ghcr.yaml','values-ecr.yaml']:
        if not (directory/overlay).exists(): continue
        name=chart.replace('/','-')+'-'+overlay.removesuffix('.yaml')
        args=['-f',str(directory/overlay),'--kube-version','1.35.0','--set','global.allowMissingGeneratedSecretRefs=true']
        if chart=='bootstrap/flux-apps':
            args+=['--set-file','imagePullSecret.dockerconfigjson='+str(out/'dockerconfig.json')]
        if chart.startswith('bootstrap/'):
            args+=['--set','image.repository=ghcr.io/labiraus/homelab']
        for action in ['lint','template']:
            cmd=['helm',action]+(['homelab-'+directory.name] if action=='template' else [])+[str(directory)]+args
            result=subprocess.run(cmd,capture_output=True,text=True)
            if action=='template':save_render(out/(name+'.'+action),result)
            else:(out/(name+'.'+action)).write_text(result.stdout+result.stderr)
            if result.returncode: failures.append(name+' '+action)
            elif action=='template':
                build=out/name;build.mkdir();(build/'resources.yaml').write_text(result.stdout)
                (build/'kustomization.yaml').write_text('resources:\n  - resources.yaml\n')
                result=subprocess.run(['kubectl','kustomize',str(build)],capture_output=True,text=True)
                (out/(name+'.kustomize')).write_text(result.stdout+result.stderr)
                if result.returncode:failures.append(name+' kustomize')
        checked.append(name)
# Explicit activation render and missing/wildcard principal refusal. No secret values used.
for chart in ['external','orchestrator']:
    directory=root/'helm/apps'/chart
    cmd=['helm','template','homelab-'+chart,str(directory),'--kube-version','1.35.0','-f',str(directory/'values-ghcr.yaml'),'--set','global.allowMissingGeneratedSecretRefs=true','--set','worldbuilder.enabled=true']
    if chart=='external':
        missing=subprocess.run(cmd,capture_output=True,text=True)
        if missing.returncode==0:failures.append('external activation accepted missing browserPrincipal')
        for invalid in ['*', 'cluster.local/ns/homelab/sa/*', '*/ns/homelab/sa/browser']:
            rejected=subprocess.run(cmd+['--set-string','worldbuilder.browserPrincipal='+invalid],capture_output=True,text=True)
            if rejected.returncode==0 or 'Worldbuilder browser principal must be exact' not in rejected.stderr:
                failures.append('external activation failed wildcard browserPrincipal refusal: '+invalid)
        cmd+=['--set','worldbuilder.browserPrincipal=cluster.local/ns/homelab/sa/homelab-oauth2-proxy']
    result=subprocess.run(cmd,capture_output=True,text=True)
    save_render(out/(chart+'-enabled.yaml'),result)
    if result.returncode:failures.append(chart+' enabled template')
# Delegation activation requires separate principals and an indirect secret.
for chart in ['external','mcp']:
    directory=root/'helm/apps'/chart
    cmd=['helm','template','homelab-'+chart,str(directory),'--kube-version','1.35.0','-f',str(directory/'values-ghcr.yaml'),'--set','global.allowMissingGeneratedSecretRefs=true','--set','worldbuilder.enabled=true']
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
    save_render(out/(chart+'-delegation-enabled.yaml'),result)
    if result.returncode:failures.append(chart+' delegation enabled template')
# AI must fail closed without an operator-defined allowance (values are test-only).
ai=['helm','template','homelab-external',str(root/'helm/apps/external'),'--kube-version','1.35.0','--set','global.allowMissingGeneratedSecretRefs=true','--set','worldbuilder.enabled=true','--set','worldbuilder.browserPrincipal=cluster.local/ns/homelab/sa/browser','--set','worldbuilder.aiSecret=worldbuilder-ai']
if subprocess.run(ai,capture_output=True).returncode==0:failures.append('AI activation accepted missing allowance')
result=subprocess.run(ai+['--set','worldbuilder.aiBudget.requestMicros=100','--set','worldbuilder.aiBudget.dailyMicros=1000','--set','worldbuilder.aiBudget.currency=TST','--set','worldbuilder.aiBudget.gatewayRoute=offline-fixture'],capture_output=True,text=True)
save_render(out/'external-ai-budget-enabled.yaml',result)
if result.returncode:failures.append('AI explicit allowance template')
# Background workers require a separately scoped credential and exact mesh identity.
allowance=['--set','worldbuilder.aiBudget.requestMicros=100','--set','worldbuilder.aiBudget.dailyMicros=1000','--set','worldbuilder.aiBudget.currency=TST','--set','worldbuilder.aiBudget.gatewayRoute=offline-fixture']
for chart in ['external','orchestrator']:
    base=['helm','template','homelab-'+chart,str(root/'helm/apps'/chart),'--kube-version','1.35.0','--set','global.allowMissingGeneratedSecretRefs=true','--set','worldbuilder.enabled=true','--set','worldbuilder.jobs.enabled=true']
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
    save_render(out/(chart+'-jobs-enabled.yaml'),result)
    if result.returncode:failures.append(chart+' jobs enabled template')
    if subprocess.run(base+['--set','worldbuilder.enabled=false'],capture_output=True).returncode==0:failures.append(chart+' jobs accepted disabled Worldbuilder')
# Dedicated database identities must remain unprivileged and the owner closed
# outside an explicit migration session. Exercise the opt-in CRD fields too.
roles=['helm','template','homelab-postgres',str(root/'helm/data/postgres'),'--set','worldbuilderRoles.enabled=true']
result=subprocess.run(roles,capture_output=True,text=True)
save_render(out/'postgres-managed-roles.yaml',result)
if result.returncode:failures.append('PostgreSQL managed roles template')
else:
    cluster=next(r for r in yaml.safe_load_all(result.stdout) if r and r.get('kind')=='Cluster')
    managed=cluster['spec']['managed']['roles']
    if {r['name'] for r in managed}!={'worldbuilder_owner','worldbuilder_external','worldbuilder_relay'}:
        failures.append('PostgreSQL dedicated identity names')
    for role in managed:
        if any(role[p] for p in ['inherit','superuser','createdb','createrole','replication','bypassrls']) or role['inRoles']:
            failures.append('PostgreSQL runtime privileges: '+role['name'])
        if role['login'] != (role['name']!='worldbuilder_owner'):
            failures.append('PostgreSQL default login: '+role['name'])
for key in ['ownerSecret','externalSecret','relaySecret']:
    if subprocess.run(roles+['--set','worldbuilderRoles.'+key+'='],capture_output=True).returncode==0:
        failures.append('PostgreSQL accepted missing '+key)
print(f'{len(checked)} base/overlay combinations checked; local artifacts: {out}')
print('Failures: '+(', '.join(failures) if failures else 'none'))
sys.exit(bool(failures))
