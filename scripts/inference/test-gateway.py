#!/usr/bin/env python3
"""Real Envoy/Cilium controllers; synthetic providers and temporary credentials. No AWS calls."""
import importlib.util
import json
import pathlib
import subprocess
import sys
import time
import yaml

ROOT = pathlib.Path(__file__).resolve().parents[2]
HERE = ROOT / 'scripts/worldbuilder'
def load(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
    return m
mesh = load('mesh', HERE / 'test-mesh.py')
common = mesh.common

def main():
    if len(sys.argv) != 3:
        raise SystemExit('Usage: test-gateway.py OUTPUT_DIRECTORY PUBLIC_SCHEMA_DIRECTORY')
    f = common.Fixture(sys.argv[1])
    schemas = pathlib.Path(sys.argv[2])
    try:
        f.create(cni_installer=mesh.install_cilium)
        for ns in ['homelab', 'envoy-gateway-system', 'envoy-ai-gateway-system']:
            f.apply([{'apiVersion': 'v1', 'kind': 'Namespace', 'metadata': {'name': ns}}])
        values = yaml.safe_load((ROOT / 'helm/infra/envoy-ai-gateway/values.yaml').read_text())
        # Include every controller-owned CRD from the pinned chart; no operator context.
        gateway_values=f.private/'gateway-values.yaml';gateway_values.write_text(yaml.safe_dump(values['envoyGateway']['values']))
        command=['helm','template','envoy-gateway','oci://docker.io/envoyproxy/gateway-helm','--version','v1.7.0','--namespace','envoy-gateway-system','--include-crds','-f',str(gateway_values)]
        eg=[d for d in yaml.safe_load_all(subprocess.check_output(command,env=f.helm_env,text=True)) if d]
        crds = [d for d in eg if d['kind'] == 'CustomResourceDefinition']
        f.apply(crds)
        for d in crds:
            f.kubectl('wait', 'crd/' + d['metadata']['name'], '--for=condition=Established', '--timeout=60s')
        f.apply([d for d in eg if d['kind'] != 'CustomResourceDefinition'])
        ai_crds = [d for d in yaml.safe_load_all((schemas / 'ai-gateway.yaml').read_text()) if d and d['kind'] == 'CustomResourceDefinition']
        f.apply(ai_crds)
        for d in ai_crds:
            f.kubectl('wait', 'crd/' + d['metadata']['name'], '--for=condition=Established', '--timeout=60s')
        # mesh.render passes versions only with a repo; OCI references carry explicit --version below.
        command = ['helm', 'template', 'envoy-ai-gateway', 'oci://docker.io/envoyproxy/ai-gateway-helm', '--version', 'v1.0.0', '--namespace', 'envoy-ai-gateway-system']
        ai = list(yaml.safe_load_all(subprocess.check_output(command, env=f.helm_env, text=True)))
        f.apply([d for d in ai if d])
        for ns in ['envoy-gateway-system', 'envoy-ai-gateway-system']:
            f.kubectl('-n', ns, 'rollout', 'status', 'deployment', '--timeout=600s', timeout=630)
        f.apply([{'apiVersion': 'gateway.networking.k8s.io/v1', 'kind': 'GatewayClass', 'metadata': {'name': 'envoy-ai-gateway'}, 'spec': {'controllerName': 'gateway.envoyproxy.io/gatewayclass-controller'}}])
        cert, key = f.private/'tls.crt', f.private/'tls.key'
        subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1', '-subj', '/CN=bedrock-runtime.eu-west-1.amazonaws.com', '-addext', 'subjectAltName=DNS:bedrock-runtime.eu-west-1.amazonaws.com', '-keyout', str(key), '-out', str(cert)], check=True, capture_output=True)
        code = (ROOT/'scripts/inference/gateway-fixture.py').read_text()
        for ns in ['homelab', 'envoy-gateway-system']:
            f.apply([{'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {'name': 'inference-fixture', 'namespace': ns}, 'data': {'fixture.py': code, 'ca.crt': cert.read_text()}},
                     {'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {'name': 'inference-bedrock-certificate', 'namespace': ns}, 'stringData': {'tls.crt': cert.read_text(), 'tls.key': key.read_text()}}])
        pod = {'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': 'provider', 'namespace': 'homelab', 'labels': {'fixture': 'provider'}}, 'spec': {'containers': [{'name': 'provider', 'image': mesh.PYTHON_IMAGE, 'command': ['python','/fixture/fixture.py','provider'], 'volumeMounts': [{'name':'code','mountPath':'/fixture'}, {'name':'identity','mountPath':'/identity'}]}], 'volumes': [{'name':'code','configMap':{'name':'inference-fixture'}}, {'name':'identity','secret':{'secretName':'inference-bedrock-certificate'}}]}}
        f.apply([pod, {'apiVersion':'v1','kind':'Service','metadata':{'name':'homelab-vllm','namespace':'homelab'}, 'spec':{'selector':{'fixture':'provider'},'ports':[{'name':'local','port':8000},{'name':'cloud','port':8443}]}}])
        f.kubectl('-n','homelab','wait','pod/provider','--for=condition=Ready','--timeout=300s')
        manifest = load('inference_renders', ROOT/'scripts/inference/test-manifests.py')
        cloud = yaml.safe_load((ROOT/'helm/infra/inference/values.yaml').read_text())
        cloud['cloud'].update(enabled=True, roleArn='arn:aws:iam::123456789012:role/fixture', profileArn='arn:aws:rolesanywhere:eu-west-1:123456789012:profile/fixture',trustAnchorArn='arn:aws:rolesanywhere:eu-west-1:123456789012:trust-anchor/fixture',helperImage=mesh.PYTHON_IMAGE)
        cloud['databaseSecret']='fixture'
        for m in cloud['config']['models']:
            if m['provider']=='bedrock':m.update(input_micros_per_million=1,output_micros_per_million=1,price_version='fixture',price_valid_until='2099-01-01T00:00:00Z')
        _, docs = manifest.render('infra/inference',cloud)
        selected=[]
        for d in docs:
            if d['kind'] in ['Deployment','Service','ServiceAccount','ConfigMap'] or (d['kind']=='NetworkPolicy' and d['metadata']['name']=='inference'):continue
            if d['kind']=='EnvoyProxy':
                spec=d['spec']['provider']['kubernetes']['envoyDeployment']['patch']['value']['spec']['template']['spec']
                helper=spec['containers'][0];helper['command']=['python','/fixture/fixture.py','credentials'];helper['args']=[]
                helper['securityContext']['readOnlyRootFilesystem']=False
                helper['volumeMounts'].append({'name':'fixture-code','mountPath':'/fixture'})
                spec['volumes'].append({'name':'fixture-code','configMap':{'name':'inference-fixture'}})
            if d['kind']=='Backend' and d['metadata']['name']=='inference-bedrock':d['spec']['endpoints']=[{'fqdn':{'hostname':'homelab-vllm.homelab.svc.cluster.local','port':8443}}]
            if d['kind']=='BackendTLSPolicy':
                d['spec']['validation'].pop('wellKnownCACertificates')
                d['spec']['validation']['caCertificateRefs']=[{'group':'','kind':'ConfigMap','name':'inference-fixture'}]
            selected.append(d)
        f.apply(selected)
        f.kubectl('-n','homelab','wait','gateway/inference-upstream','--for=condition=Programmed','--timeout=600s',timeout=630)
        for name,label in [('allowed','inference'),('denied','untrusted')]:
            f.apply([{'apiVersion':'v1','kind':'Pod','metadata':{'name':name,'namespace':'homelab','labels':{'app.kubernetes.io/name':label}},'spec':{'containers':[{'name':'client','image':mesh.PYTHON_IMAGE,'command':['sleep','3600']}]}}])
            f.kubectl('-n','homelab','wait','pod/'+name,'--for=condition=Ready','--timeout=300s')
        def probe(model,client='allowed',success=True):
            program='import json,urllib.request; r=urllib.request.Request("http://inference-upstream.envoy-gateway-system.svc.cluster.local/v1/chat/completions",data=json.dumps({"model":'+repr(model)+',"messages":[{"role":"user","content":"hi"}],"max_tokens":10}).encode(),headers={"Content-Type":"application/json"}); print(urllib.request.urlopen(r,timeout=8).read().decode())'
            result=f.kubectl('-n','homelab','exec',client,'--','python','-c',program,check=False,timeout=25)
            if success:
                assert result[0]==0,result
                return json.loads(result[1])['choices'][0]['message']['content']
            assert result[0]!=0,'forbidden request succeeded'
        def ready():
            try:return probe('Qwen/Qwen2.5-3B-Instruct-AWQ')=='blue'
            except (AssertionError,ValueError):return False
        common.wait_for('local provider through actual AI gateway',ready,180)
        # Programmed/local routing can precede the controller's helper-injection rollout.
        f.kubectl('-n','envoy-gateway-system','rollout','status','deployment', '--timeout=300s', timeout=330)
        probe('Qwen/Qwen2.5-3B-Instruct-AWQ','denied',False)
        # Prove the actual translator preserves the schema-tool response contract.
        payload = {'model':'eu.amazon.nova-2-lite-v1:0','messages':[{'role':'user','content':'hi'}],'max_tokens':10,'tools':[{'type':'function','function':{'name':'structured_response','parameters':{'type':'object','properties':{'answer':{'type':'string'}},'required':['answer']}}}],'tool_choice':'required'}
        program = 'import json,urllib.request; r=urllib.request.Request("http://inference-upstream.envoy-gateway-system.svc.cluster.local/v1/chat/completions",data='+repr(json.dumps(payload).encode())+',headers={"Content-Type":"application/json"}); print(urllib.request.urlopen(r,timeout=8).read().decode())'
        structured = json.loads(f.kubectl('-n','homelab','exec','allowed','--','python','-c',program,timeout=25))
        call = structured['choices'][0]['message']['tool_calls'][0]
        assert call['function']['name'] == 'structured_response' and json.loads(call['function']['arguments']) == {'answer':'blue'}
        first=probe('eu.amazon.nova-2-lite-v1:0');time.sleep(6);second=probe('eu.amazon.nova-2-lite-v1:0')
        assert first!=second and first.startswith('offline-session-') and second.startswith('offline-session-')
        pods=[p for p in json.loads(f.kubectl('-n','envoy-gateway-system','get','pods','-l','gateway.envoyproxy.io/owning-gateway-name=inference-upstream','-o','json'))['items'] if not p['metadata'].get('deletionTimestamp')]
        assert len(pods)==1
        f.kubectl('-n','envoy-gateway-system','exec',pods[0]['metadata']['name'],'-c','roles-anywhere','--','python','-c','open("/tmp/deny","w").close()')
        time.sleep(9);probe('eu.amazon.nova-2-lite-v1:0',success=False)
        assert probe('Qwen/Qwen2.5-3B-Instruct-AWQ')=='blue'
        (f.output/'acceptance.json').write_text(json.dumps({'controllers':True,'networkBypassRefused':True,'syntheticCredentialRefresh':True,'expiredCredentialsRefused':True,'localIndependent':True,'schemaToolTranslation':True,'realAWS':False},indent=2)+'\n')
    finally:
        try:
            # Only synthetic credentials exist in this fixture. Capture consuming-process
            # diagnostics before teardown; never retain certificates or full rendered YAML.
            for ns in ['envoy-gateway-system', 'envoy-ai-gateway-system', 'homelab']:
                code, out, _ = f.kubectl('-n', ns, 'get', 'pods', '-o', 'json', check=False, timeout=30)
                if code == 0:
                    for pod in json.loads(out)['items']:
                        for container in pod['spec']['containers'] + pod['spec'].get('initContainers', []):
                            name = pod['metadata']['name']
                            _, log, err = f.kubectl('-n', ns, 'logs', name, '-c', container['name'], '--tail=120', check=False, timeout=30)
                            (f.output / (ns + '-' + name + '-' + container['name'] + '.log')).write_text(log + err)
            for name,args in [('pods',['get','pods','-A','-o','wide']),('events',['get','events','-A','--sort-by=.lastTimestamp'])]:
                _,out,err=f.kubectl(*args,check=False,timeout=30);(f.output/(name+'.log')).write_text(out+err)
        finally:f.close()
    print('PASS: real gateway controllers, Cilium isolation, synthetic credential renewal/refusal and owned cleanup')

if __name__=='__main__':main()
