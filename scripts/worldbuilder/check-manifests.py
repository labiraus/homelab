#!/usr/bin/env python3
"""Local render/build checks. Never invokes cluster discovery, apply or reconciliation."""
import argparse, json, pathlib, subprocess, tempfile, sys
import yaml
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--output',type=pathlib.Path,help='New directory for render artifacts')
options=parser.parse_args()
root=pathlib.Path(__file__).resolve().parents[2]
out=options.output or pathlib.Path(tempfile.mkdtemp(prefix='worldbuilder-manifests-'))
if options.output:out.mkdir(parents=True)
(out/'dockerconfig.json').write_text('{"auths":{}}')
charts=['apps/external','apps/orchestrator','apps/processor','apps/mcp','apps/ui','data/postgres','bootstrap/flux-apps','bootstrap/flux-data','bootstrap/flux-infra','bootstrap/flux-observability','infra/vllm','infra/inference','infra/embeddings','infra/oauth2-proxy','infra/envoy-ai-gateway','observability/prometheus']
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
            if result.returncode:
                failures.append(name+' '+action)
                print(name+' '+action+': '+result.stdout+result.stderr, file=sys.stderr)
            elif action=='template':
                build=out/name;build.mkdir();(build/'resources.yaml').write_text(result.stdout)
                (build/'kustomization.yaml').write_text('resources:\n  - resources.yaml\n')
                result=subprocess.run(['kubectl','kustomize',str(build)],capture_output=True,text=True)
                (out/(name+'.kustomize')).write_text(result.stdout+result.stderr)
                if result.returncode:failures.append(name+' kustomize')
        checked.append(name)
# Render OpenSearch with the parent's actual values: parent arrays replace child
# defaults and previously removed the live relay's ingress grant on publication.
for overlay in ['values', 'values-ghcr', 'values-ecr']:
    parent=out/('bootstrap-flux-data-'+overlay+'.template')
    if not parent.exists(): continue
    resources=list(yaml.safe_load_all(parent.read_text()))
    releases=[r for r in resources if r and r.get('kind')=='HelmRelease' and r['metadata']['name']=='opensearch']
    if len(releases)!=1:
        failures.append('OpenSearch parent release missing: '+overlay)
        continue
    effective=out/('opensearch-parent-'+overlay+'.yaml')
    effective.write_text(yaml.safe_dump(releases[0]['spec'].get('values',{})))
    result=subprocess.run(['helm','template','data-opensearch',str(root/'helm/data/opensearch'),'-f',str(effective)],capture_output=True,text=True)
    save_render(out/('opensearch-composed-'+overlay+'.template'),result)
    if result.returncode:
        failures.append('OpenSearch composed render: '+overlay)
        continue
    policies=[r for r in yaml.safe_load_all(result.stdout) if r and r.get('kind')=='NetworkPolicy']
    clients=[peer for policy in policies for rule in policy['spec'].get('ingress',[]) if any(p.get('port')==9200 for p in rule.get('ports',[])) for peer in rule.get('from',[])]
    if not any(peer.get('namespaceSelector',{}).get('matchLabels',{}).get('kubernetes.io/metadata.name')=='homelab' and peer.get('podSelector',{}).get('matchLabels',{}).get('app.kubernetes.io/instance')=='homelab-orchestrator' for peer in clients):
        failures.append('OpenSearch composed ingress excludes relay: '+overlay)

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
# Flux passes parent values through JSON; large numbers can reach Helm as floats.
# Exercise the composed child, since --set integer fixtures miss this boundary.
for overlay in ['values', 'values-ghcr', 'values-ecr']:
    parent=out/('bootstrap-flux-apps-'+overlay+'.template')
    if not parent.exists(): continue
    release=next(r for r in yaml.safe_load_all(parent.read_text()) if r and r.get('kind')=='HelmRelease' and r['metadata']['name']=='external')
    values=release['spec'].get('values',{})
    if not values.get('worldbuilder',{}).get('aiSecret'): continue
    effective=out/('external-parent-'+overlay+'.json')
    effective.write_text(json.dumps(values))
    result=subprocess.run(['helm','template','homelab-external',str(root/'helm/apps/external'),'-f',str(effective),'--set','global.allowMissingGeneratedSecretRefs=true'],capture_output=True,text=True)
    save_render(out/('external-composed-'+overlay+'.template'),result)
    if result.returncode:
        failures.append('external composed AI render: '+overlay)
        continue
    deployment=next(r for r in yaml.safe_load_all(result.stdout) if r and r.get('kind')=='Deployment')
    env={item['name']:item.get('value') for item in deployment['spec']['template']['spec']['containers'][0]['env']}
    for field,suffix in [('dailyRequests','DAILY_REQUESTS'),('concurrent','CONCURRENT'),('requestMicros','REQUEST_MICROS'),('dailyMicros','DAILY_MICROS')]:
        if env.get('WORLDBUILDER_AI_'+suffix)!=str(values['worldbuilder']['aiBudget'][field]):
            failures.append('external composed AI integer setting: '+overlay+' '+field)
# Background workers require a separately scoped credential and exact mesh identity.
allowance=['--set','worldbuilder.aiBudget.requestMicros=100','--set','worldbuilder.aiBudget.dailyMicros=1000','--set','worldbuilder.aiBudget.currency=TST','--set','worldbuilder.aiBudget.gatewayRoute=offline-fixture']
# Semantic configuration is independently gated and never enables paid models.
for chart in ['external','orchestrator']:
    base=['helm','template','homelab-'+chart,str(root/'helm/apps'/chart),'--set','global.allowMissingGeneratedSecretRefs=true','--set','worldbuilder.enabled=true','--set','worldbuilder.semantic.enabled=true']
    if chart=='external': base+=['--set','worldbuilder.browserPrincipal=cluster.local/ns/homelab/sa/browser']+allowance
    if subprocess.run(base,capture_output=True).returncode==0: failures.append(chart+' semantic accepted missing profile')
    base+=['--set','worldbuilder.semantic.model=fixture-model','--set','worldbuilder.semantic.revision=immutable-fixture','--set','worldbuilder.semantic.dimensions=2']
    result=subprocess.run(base,capture_output=True,text=True)
    save_render(out/(chart+'-semantic-enabled.yaml'),result)
    if result.returncode: failures.append(chart+' semantic enabled template: '+result.stderr)
    else:
        resources=[r for r in yaml.safe_load_all(result.stdout) if r]
        env={v['name']:v.get('value') for r in resources if r.get('kind')=='Deployment' for c in r['spec']['template']['spec']['containers'] for v in c.get('env',[])}
        if env.get('WORLDBUILDER_EMBEDDING_DIMENSIONS')!='2': failures.append(chart+' semantic dimensions formatting')
        policy=next((r for r in resources if r.get('kind')=='NetworkPolicy' and r['metadata']['name'].endswith('-canon-embeddings')),None)
        if policy is None or policy['spec']['egress'][0]['to'][0]['podSelector']['matchLabels'].get('gateway.envoyproxy.io/owning-gateway-name')!='vllm-ai-gateway': failures.append(chart+' missing scoped semantic egress')
    for key,value in [('enabled','false'),('semantic.embeddingURL','https://provider.invalid/v1/embeddings'),('semantic.dimensions','0'),('semantic.revision','')]:
        if subprocess.run(base+['--set','worldbuilder.'+key+'='+value],capture_output=True).returncode==0: failures.append(chart+' semantic accepted invalid '+key)
    if chart=='external' and subprocess.run(base+['--set','worldbuilder.aiBudget.requestMicros=0'],capture_output=True).returncode==0: failures.append('semantic accepted missing interactive allowance')
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
# Native OAuth must remain explicitly gated, with a proxy-only consent path and
# a gateway/MCP-only token check. Rendering the parent provider catches list overrides.
native=['helm','template','homelab-external',str(root/'helm/apps/external'),'--set','global.allowMissingGeneratedSecretRefs=true','--set','nativeOAuth.enabled=true']
if subprocess.run(native,capture_output=True).returncode==0:failures.append('native OAuth accepted missing database Secret')
native+=['--set','nativeOAuth.databaseSecret=oauth-fixture']
for principal in ['', '*', 'cluster.local/ns/homelab/sa/*', 'cluster.local/ns/ingress/sa/gateway-istio']:
    if subprocess.run(native+['--set-string','nativeOAuth.browserPrincipal='+principal],capture_output=True).returncode==0:
        failures.append('native OAuth accepted unverified browser principal: '+principal)
for key in ['gatewayPrincipal','mcpPrincipal']:
    for invalid in ['', '*','cluster.local/ns/homelab/sa/homelab-oauth2-proxy']:
        if subprocess.run(native+['--set','nativeOAuth.browserPrincipal=cluster.local/ns/homelab/sa/homelab-oauth2-proxy','--set-string','nativeOAuth.'+key+'='+invalid],capture_output=True).returncode==0:
            failures.append('native OAuth accepted unverified '+key+': '+invalid)
result=subprocess.run(native+['--set','nativeOAuth.browserPrincipal=cluster.local/ns/homelab/sa/homelab-oauth2-proxy'],capture_output=True,text=True)
save_render(out/'external-native-oauth-enabled.yaml',result)
if result.returncode:failures.append('native OAuth enabled render')
else:
    resources=[r for r in yaml.safe_load_all(result.stdout) if r]
    boundary=next(r for r in resources if r['metadata']['name']=='homelab-external-native-oauth-boundaries')
    expected={('/api/mcp-oauth/authorize',('cluster.local/ns/homelab/sa/homelab-oauth2-proxy',)),('/api/mcp-oauth/check',('cluster.local/ns/ingress/sa/gateway-istio','cluster.local/ns/homelab/sa/homelab-mcp'))}
    actual={(rule['to'][0]['operation']['paths'][0],tuple(rule['from'][0]['source']['notPrincipals'])) for rule in boundary['spec']['rules']}
    if actual!=expected:failures.append('native OAuth identity boundary')
    allowance=next(r for r in resources if r['metadata']['name']=='homelab-external-native-oauth-gateway')
    preserved=[rule for rule in allowance['spec']['rules'] if 'from' not in rule]
    if len(preserved)!=1 or preserved[0]['to']!=[{'operation':{'ports':['8080'],'notPaths':['/.well-known/oauth-authorization-server','/mcp-oauth/*','/api/mcp-oauth/*']}}]:
        failures.append('native OAuth changed existing non-OAuth route access')
    if not any(rule.get('from')==[{'source':{'principals':['cluster.local/ns/homelab/sa/homelab-oauth2-proxy']}}] and rule['to']==[{'operation':{'ports':['8080'],'paths':['/api/mcp-oauth/authorize']}}] for rule in allowance['spec']['rules']):
        failures.append('native OAuth denied browser consent')
    route=next(r for r in resources if r['kind']=='HTTPRoute')
    public={match['path']['value'] for rule in route['spec']['rules'] for match in rule['matches']}
    if public!={'/.well-known/oauth-authorization-server','/mcp-oauth/register','/mcp-oauth/token','/mcp-oauth/revoke'}:
        failures.append('native OAuth exposes private consent/check directly')
result=subprocess.run(['helm','template','homelab-mcp',str(root/'helm/apps/mcp'),'--set','global.allowMissingGeneratedSecretRefs=true','--set','nativeOAuth.enabled=true'],capture_output=True,text=True)
save_render(out/'mcp-native-oauth-enabled.yaml',result)
if result.returncode:failures.append('native MCP enabled render')
else:
    policy=next(r for r in yaml.safe_load_all(result.stdout) if r and r.get('kind')=='AuthorizationPolicy' and r['spec'].get('action')=='CUSTOM')
    if policy['spec']['provider']['name']!='mcp-oauth':failures.append('native MCP lacks validated token provider')
parent=subprocess.run(['helm','template','homelab-bootstrap',str(root/'helm/bootstrap/flux-bootstrap'),'--set','image.repository=ghcr.io/labiraus/homelab'],capture_output=True,text=True)
if parent.returncode:failures.append('native OAuth bootstrap render')
else:
    release=next(r for r in yaml.safe_load_all(parent.stdout) if r and r.get('kind')=='HelmRelease' and r['metadata']['name']=='istio')
    values=out/'native-oauth-istio-parent-values.yaml';values.write_text(yaml.safe_dump(release['spec']['values']))
    result=subprocess.run(['helm','template','homelab-istio',str(root/'helm/bootstrap/istio'),'-f',str(values)],capture_output=True,text=True)
    save_render(out/'native-oauth-istio-composed.template',result)
    if result.returncode:failures.append('native OAuth composed Istio render')
    else:
        release=next(r for r in yaml.safe_load_all(result.stdout) if r and r.get('kind')=='HelmRelease' and r['metadata']['name']=='istiod')
        providers=release['spec']['values']['meshConfig']['extensionProviders']
        provider=next((p['envoyExtAuthzHttp'] for p in providers if p['name']=='mcp-oauth'),{})
        if provider.get('failOpen') is not False or provider.get('timeout')!='5s' or provider.get('statusOnError')!='503' or provider.get('pathPrefix')!='/api/mcp-oauth/check?' or provider.get('service')!='homelab-external.homelab.svc.cluster.local' or provider.get('headersToUpstreamOnAllow')!=['x-auth-request-email','x-auth-request-user','x-forwarded-email']:
            failures.append('native OAuth provider lost or trusts unvalidated identity')
        if not any(p['name']=='oauth2-proxy' for p in providers):failures.append('native OAuth removed website provider')

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
native_roles=['helm','template','homelab-postgres',str(root/'helm/data/postgres'),'--set','nativeOAuthRole.enabled=true']
if subprocess.run(native_roles,capture_output=True).returncode==0:failures.append('native OAuth role accepted missing password Secret')
result=subprocess.run(native_roles+['--set','nativeOAuthRole.passwordSecret=oauth-fixture'],capture_output=True,text=True)
save_render(out/'postgres-native-oauth-enabled.yaml',result)
if result.returncode:failures.append('native OAuth role render')
else:
    cluster=next(r for r in yaml.safe_load_all(result.stdout) if r and r.get('kind')=='Cluster')
    managed=cluster['spec']['managed']['roles']
    if len(managed)!=1 or managed[0]['name']!='mcp_oauth_runtime' or not managed[0]['login'] or managed[0]['inRoles'] or any(managed[0][p] for p in ['inherit','superuser','createdb','createrole','replication','bypassrls']):
        failures.append('native OAuth role isolation')
print(f'{len(checked)} base/overlay combinations checked; local artifacts: {out}')
print('Failures: '+(', '.join(failures) if failures else 'none'))
sys.exit(bool(failures))

subprocess.run([sys.executable,str(root/"scripts/inference/test-manifests.py"),"--output",str(out)],check=True)
