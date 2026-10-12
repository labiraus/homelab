#!/usr/bin/env python3
"""Disposable source-policy Cilium/Istio acceptance; never uses a LAN context."""
import base64
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys

import yaml

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
CILIUM_VERSION = '1.20.2'
ISTIO_VERSION = '1.30.5'
PYTHON_IMAGE = 'python:3.12.12-alpine3.22'


def module(name):
    spec = importlib.util.spec_from_file_location(name, HERE / (name + '.py'))
    result = importlib.util.module_from_spec(spec); spec.loader.exec_module(result)
    return result


common = module('kind-fixture')


def render(f, name, chart, namespace, values, repo=None, version=None):
    inputs = f.output / (name + '-values.yaml')
    inputs.write_text(yaml.safe_dump(values))
    command = ['helm', 'template', name, str(chart), '--namespace', namespace,
               '--kube-version', '1.35.0', '--include-crds', '-f', str(inputs)]
    if repo: command += ['--repo', repo, '--version', version]
    result = subprocess.run(command, env=f.helm_env, capture_output=True, text=True,
                            check=True, timeout=120)
    (f.output / (name + '.yaml')).write_text(result.stdout)
    return [d for d in yaml.safe_load_all(result.stdout) if d]


def install_cilium(f):
    values = {'ipam': {'mode': 'kubernetes'}, 'kubeProxyReplacement': False,
              'operator': {'replicas': 1}, 'image': {'pullPolicy': 'IfNotPresent'}}
    docs = render(f, 'cilium', 'cilium', 'kube-system', values,
                  'https://helm.cilium.io', CILIUM_VERSION)
    f.apply(docs)
    f.kubectl('-n', 'kube-system', 'rollout', 'status', 'ds/cilium', '--timeout=900s', timeout=930)
    f.kubectl('-n', 'kube-system', 'rollout', 'status', 'deployment/cilium-operator', '--timeout=180s')
    f.kubectl('-n', 'kube-system', 'exec', 'ds/cilium', '-c', 'cilium-agent', '--', 'cilium-dbg', 'status', '--brief')
    print('PASS: real Cilium CNI and operator on the owned kind node', flush=True)


def install_istio(f, schemas):
    gateway = [d for d in yaml.safe_load_all((schemas / 'envoy-gatewayapi-crds.yaml').read_text()) if d]
    f.apply(gateway)
    for crd in gateway:
        if crd['kind'] == 'CustomResourceDefinition':
            f.kubectl('wait', 'crd/' + crd['metadata']['name'], '--for=condition=Established', '--timeout=60s')
    f.apply([{'apiVersion': 'v1', 'kind': 'Namespace', 'metadata': {'name': 'istio-system'}}])
    repo = 'https://istio-release.storage.googleapis.com/charts'
    base = render(f, 'istio-base', 'base', 'istio-system', {'defaultRevision': 'default'}, repo, ISTIO_VERSION)
    f.apply(base)
    values = yaml.safe_load((ROOT / 'helm/bootstrap/istio/values.yaml').read_text())
    values['pilot'].update(autoscaleEnabled=False, replicaCount=1,
                           resources={'requests': {'cpu': '100m', 'memory': '256Mi'},
                                      'limits': {'memory': '768Mi'}})
    values['global'] = {'proxy': {'resources': {'requests': {'cpu': '10m', 'memory': '64Mi'},
                                             'limits': {'memory': '256Mi'}}}}
    f.apply(render(f, 'istiod', 'istiod', 'istio-system', values, repo, ISTIO_VERSION))
    f.kubectl('-n', 'istio-system', 'rollout', 'status', 'deployment/istiod', '--timeout=300s', timeout=330)
    istio_admission(f)
    print('PASS: real Istio control plane with source extension providers', flush=True)


def istio_admission(f):
    policy = {'apiVersion': 'security.istio.io/v1', 'kind': 'PeerAuthentication',
              'metadata': {'name': 'fixture-admission', 'namespace': 'istio-system'},
              'spec': {'selector': {'matchLabels': {'app': 'unused-fixture'}}, 'mtls': {'mode': 'STRICT'}}}
    def available():
        hooks = json.loads(f.kubectl('get', 'validatingwebhookconfiguration/istiod-default-validator',
                                    '-o', 'json', timeout=25))['webhooks']
        assert len(hooks) == 1 and hooks[0]['name'] == 'validation.istio.io'
        hook = hooks[0]
        assert not hook.get('namespaceSelector') and not hook.get('objectSelector')
        assert any('security.istio.io' in r['apiGroups'] and '*' in r['resources'] and
                   'CREATE' in r['operations'] for r in hook['rules'])
        if hook['failurePolicy'] != 'Fail' or not hook['clientConfig'].get('caBundle'): return False
        return f.kubectl('apply', '--dry-run=server', '-f', '-', data=yaml.safe_dump(policy).encode(),
                         check=False, timeout=25)[0] == 0
    common.wait_for('real Istio admission accepts a valid policy after rollout', available, 90)


def diagnostics(f):
    for name, args in [('pods', ['get', 'pods', '-A', '-o', 'wide']),
                       ('events', ['get', 'events', '-A', '--sort-by=.lastTimestamp']),
                       ('cilium', ['-n', 'kube-system', 'logs', 'ds/cilium', '-c', 'cilium-agent', '--tail=100']),
                       ('cilium-operator', ['-n', 'kube-system', 'logs', 'deployment/cilium-operator', '--tail=100']),
                       ('istiod', ['-n', 'istio-system', 'logs', 'deployment/istiod', '--tail=100']),
                       ('gateway', ['-n', 'ingress', 'logs', 'deployment/gateway-istio', '--tail=100']),
                       ('external-proxy', ['-n', 'homelab', 'logs', 'external', '-c', 'istio-proxy', '--tail=100'])]:
        try:
            code, out, err = f.kubectl(*args, check=False, timeout=25)
            (f.output / (name + '.log')).write_text(out + err)
        except Exception as error:
            (f.output / (name + '.log')).write_text(type(error).__name__)
    try:
        pods = json.loads(f.kubectl('get', 'pods', '-A', '-o', 'json', timeout=25))['items']
        images = {p['metadata']['namespace'] + '/' + p['metadata']['name']: [
            {key: c.get(key) for key in ('name', 'image', 'imageID', 'restartCount')}
            for c in p['status'].get('initContainerStatuses', []) + p['status'].get('containerStatuses', [])]
            for p in pods}
        (f.output / 'images.json').write_text(json.dumps(images, indent=2) + '\n')
    except Exception as error:
        (f.output / 'images.log').write_text(type(error).__name__)


def get(f, resource, namespace='homelab'):
    return json.loads(f.kubectl('-n', namespace, 'get', resource, '-o', 'json'))


def pod(name, namespace, account, labels, role='client', injected=True, ports=(8080,)):
    return {'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': name, 'namespace': namespace,
            'labels': labels, 'annotations': {'sidecar.istio.io/inject': str(injected).lower()}},
            'spec': {'serviceAccountName': account, 'containers': [{'name': 'fixture', 'image': PYTHON_IMAGE,
                'command': ['python3', '/fixture/mesh-fixture.py', 'serve', role, *map(str, ports)],
                'resources': {'requests': {'cpu': '10m', 'memory': '24Mi'}, 'limits': {'memory': '64Mi'}},
                'readinessProbe': {'httpGet': {'path': '/health', 'port': ports[0]}, 'periodSeconds': 2},
                'volumeMounts': [{'name': 'code', 'mountPath': '/fixture', 'readOnly': True}]}],
                'volumes': [{'name': 'code', 'configMap': {'name': 'mesh-fixture'}}]}}


def probe(f, name, path, namespace='homelab', host='homelab-external.homelab.svc.cluster.local',
          port=80, headers=None, method='GET', certificate=None):
    config = dict(host=host, port=port, path=path, headers=headers or {}, method=method)
    if certificate: config.update(certificate=certificate, hostname='mcp.labiraus.com')
    raw = f.kubectl('-n', namespace, 'exec', '-i', name, '-c', 'fixture', '--',
                   'python3', '/fixture/mesh-fixture.py', 'request', data=json.dumps(config).encode(), timeout=20)
    result = json.loads(raw)
    if not hasattr(f, 'probes'): f.probes = []
    f.probes.append(dict(client=namespace + '/' + name, host=host, port=port, method=method,
                         path=path, tls=bool(certificate), result=result))
    (f.output / 'probes.json').write_text(json.dumps(f.probes, indent=2) + '\n')
    return result


def expect(f, name, path, status, server=None, **kwargs):
    result = probe(f, name, path, **kwargs)
    assert result['status'] == status, (name, path, status, result)
    if server:
        body = json.loads(result['body'])
        assert body['fixture'] == 'worldbuilder-mesh' and body['server'] == server and body['path'] == path, result
    if status == 403: assert 'RBAC: access denied' in result['body'], result
    return result


def source_fixtures(f):
    browser = 'cluster.local/ns/homelab/sa/homelab-oauth2-proxy'
    external = {'global': {'allowMissingGeneratedSecretRefs': True}, 'worldbuilder': {
        'enabled': True, 'browserPrincipal': browser, 'aiSecret': 'synthetic-ai',
        'aiBudget': {'requestMicros': 100, 'dailyMicros': 1000, 'currency': 'TST', 'gatewayRoute': 'fixture'},
        'mcp': {'enabled': True, 'delegationSecret': 'synthetic-mcp', 'principal': 'cluster.local/ns/homelab/sa/homelab-mcp'},
        'jobs': {'enabled': True, 'workerSecret': 'synthetic-worker', 'principal': 'cluster.local/ns/homelab/sa/homelab-orchestrator'}},
        'nativeOAuth': {'enabled': True, 'databaseSecret': 'synthetic-oauth', 'browserPrincipal': browser},
        'worldbuilderMonitoring': {'enabled': True, 'labels': {'release': 'fixture'},
            'scraperNamespace': 'observability', 'scraperPodLabels': {'app': 'fixture-scraper'},
            'scraperPrincipal': 'cluster.local/ns/observability/sa/prometheus'}}
    mcp = {'global': {'allowMissingGeneratedSecretRefs': True}, 'nativeOAuth': {'enabled': True},
           'worldbuilder': {'enabled': True, 'delegationSecret': 'synthetic-mcp',
                           'trustedEdgePrincipals': ['cluster.local/ns/ingress/sa/gateway-istio']}}
    documents = []
    for app, values in [('external', external), ('mcp', mcp)]:
        subprocess.run(['helm', 'dependency', 'build', str(ROOT / 'helm/apps' / app)],
                       env=f.helm_env, check=True, capture_output=True, timeout=90)
        documents.extend(render(f, 'homelab-' + app, ROOT / 'helm/apps' / app, 'homelab', values))
    namespaces = ('homelab', 'ingress', 'outside', 'data', 'observability', 'clients')
    f.apply([{'apiVersion': 'v1', 'kind': 'Namespace', 'metadata': {'name': ns,
             'labels': {'istio-injection': 'enabled' if ns in ('homelab', 'ingress', 'outside', 'observability') else 'disabled'}}}
             for ns in namespaces])
    f.apply([{'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {'name': 'mesh-fixture', 'namespace': ns},
              'data': {'mesh-fixture.py': (HERE / 'mesh-fixture.py').read_text()}} for ns in namespaces])
    selected = [d for d in documents if d['kind'] in ('NetworkPolicy', 'PeerAuthentication', 'AuthorizationPolicy',
                                                    'Service', 'ServiceAccount', 'HTTPRoute')]
    assert len([d for d in selected if d['kind'] == 'NetworkPolicy']) >= 5
    f.apply(selected)
    app_labels = {d['metadata']['name']: d['spec']['selector']['matchLabels']
                  for d in documents if d['kind'] == 'Deployment'}
    proxy_labels = {'app.kubernetes.io/instance': 'homelab-oauth2-proxy', 'app.kubernetes.io/name': 'oauth2-proxy'}
    fixtures = [
        pod('external', 'homelab', 'homelab-external', app_labels['homelab-external'], 'external'),
        pod('mcp', 'homelab', 'homelab-mcp', app_labels['homelab-mcp'], 'mcp'),
        pod('browser', 'homelab', 'homelab-oauth2-proxy', proxy_labels),
        pod('worker', 'homelab', 'homelab-orchestrator', {'app.kubernetes.io/instance': 'homelab-orchestrator', 'app.kubernetes.io/name': 'orchestrator'}),
        pod('processor', 'homelab', 'homelab-processor', {'app.kubernetes.io/instance': 'homelab-processor'}),
        pod('spoof-browser', 'homelab', 'wrong', proxy_labels),
        pod('unlabelled', 'homelab', 'homelab-oauth2-proxy', {'app': 'unlabelled'}),
        pod('plaintext', 'homelab', 'homelab-oauth2-proxy', proxy_labels, injected=False),
        pod('outsider', 'outside', 'homelab-oauth2-proxy', proxy_labels),
        pod('scraper', 'observability', 'prometheus', {'app': 'fixture-scraper'}),
        pod('spoof-scraper', 'observability', 'wrong', {'app': 'fixture-scraper'}),
        pod('web-client', 'clients', 'default', {'app': 'web-client'}, injected=False),
        pod('database', 'data', 'default', {'cnpg.io/cluster': 'app-db'}, 'database', False, (5432, 5433)),
        pod('wrong-database', 'data', 'default', {'app': 'wrong-database'}, 'database', False, (5432,)),
        pod('foreign-database', 'outside', 'default', {'cnpg.io/cluster': 'app-db'}, 'database', False, (5432,)),
    ]
    accounts = {(p['metadata']['namespace'], p['spec']['serviceAccountName']) for p in fixtures}
    f.apply([{'apiVersion': 'v1', 'kind': 'ServiceAccount', 'metadata': {'name': name, 'namespace': ns}}
             for ns, name in sorted(accounts) if name != 'default'])
    f.apply(fixtures)
    for p in fixtures:
        f.kubectl('-n', p['metadata']['namespace'], 'wait', 'pod/' + p['metadata']['name'],
                  '--for=condition=Ready', '--timeout=600s', timeout=630)
    (f.output / 'source-policy-hashes.json').write_text(json.dumps({
        name: hashlib.sha256((f.output / (name + '.yaml')).read_bytes()).hexdigest()
        for name in ('homelab-external', 'homelab-mcp')}, indent=2) + '\n')
    return fixtures


def identity_matrix(f, fixtures):
    common.wait_for('source browser route is available with real mTLS',
                    lambda: probe(f, 'browser', '/api/worlds')['status'] == 200, 90)
    paths = {'browser': '/api/worlds', 'mcp': '/api/worldbuilder-tools/read', 'worker': '/api/worldbuilder-jobs/run'}
    for client in ('browser', 'mcp', 'worker', 'processor', 'spoof-browser'):
        for owner, path in paths.items():
            for method in ('GET', 'POST'):
                expect(f, client, path, 200 if client == owner else 403,
                       'external' if client == owner else None, method=method)
    forged = {'X-Forwarded-Client-Cert': 'URI=spiffe://cluster.local/ns/homelab/sa/homelab-oauth2-proxy',
              'X-Auth-Request-Email': 'operator@example.invalid'}
    expect(f, 'spoof-browser', '/api/worlds', 403, headers=forged)
    for name, namespace in [('unlabelled', 'homelab'), ('outsider', 'outside')]:
        expect(f, name, '/health', 0, namespace=namespace)
    result = probe(f, 'plaintext', '/health')
    assert result['status'] == 0, ('STRICT mTLS accepted plaintext', result)
    expect(f, 'scraper', '/metrics', 200, 'external', namespace='observability')
    expect(f, 'scraper', '/api/worlds', 403, namespace='observability')
    expect(f, 'scraper', '/metrics', 403, namespace='observability', method='POST')
    expect(f, 'spoof-scraper', '/metrics', 403, namespace='observability')
    expect(f, 'browser', '/metrics', 403)
    expect(f, 'browser', '/api/mcp-oauth/authorize', 200, 'external')
    expect(f, 'mcp', '/api/mcp-oauth/authorize', 403)
    expect(f, 'browser', '/api/mcp-oauth/check', 403)
    expect(f, 'mcp', '/api/mcp-oauth/check', 401)
    for name, namespace, port, status in [('database', 'data', 5432, 200), ('database', 'data', 5433, 0),
                                         ('wrong-database', 'data', 5432, 0), ('foreign-database', 'outside', 5432, 0)]:
        address = get(f, 'pod/' + name, namespace)['status']['podIP']
        # Prove the destination and every tested port work before interpreting
        # the source workload's refusal as network-policy enforcement.
        expect(f, 'web-client', '/health', 200, 'database', namespace='clients', host=address, port=port)
        expect(f, 'external', '/health', status, 'database' if status == 200 else None, host=address, port=port)
    expect(f, 'browser', '/api/worlds', 200, 'external')
    print('PASS: source network, STRICT mTLS, browser/MCP/worker/metrics identities and spoof refusal', flush=True)


def certificate_identities(f, fixtures):
    identities = {}
    script = """import json,urllib.request
with urllib.request.urlopen('http://127.0.0.1:15000/certs',timeout=3) as r:d=json.load(r)
def leaves(x):
 if isinstance(x,dict):
  for v in x.values():yield from leaves(v)
 elif isinstance(x,list):
  for v in x:yield from leaves(v)
 elif isinstance(x,str) and x.startswith('spiffe://'):yield x
print(json.dumps(sorted(set(leaves(d)))))
"""
    for p in fixtures:
        if p['metadata']['annotations']['sidecar.istio.io/inject'] != 'true': continue
        ns, name = p['metadata']['namespace'], p['metadata']['name']
        value = json.loads(f.kubectl('-n', ns, 'exec', '-i', name, '-c', 'fixture', '--',
                                    'python3', '-', data=script.encode(), timeout=15))
        expected = f"spiffe://cluster.local/ns/{ns}/sa/{p['spec']['serviceAccountName']}"
        assert value == [expected], (name, value, expected)
        identities[ns + '/' + name] = value[0]
    (f.output / 'identities.json').write_text(json.dumps(identities, indent=2) + '\n')
    print('PASS: actual workload certificate SANs match exact namespace/service-account identities', flush=True)


def create_certificate(f, name):
    cert, key = f.private / (name + '.crt'), f.private / (name + '.key')
    subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
                    '-subj', '/CN=mcp.labiraus.com', '-addext', 'subjectAltName=DNS:mcp.labiraus.com',
                    '-keyout', str(key), '-out', str(cert)], check=True, capture_output=True, timeout=20)
    return cert, key


def install_gateway(f):
    cert, key = create_certificate(f, 'gateway')
    f.apply([{'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {'name': 'mcp-labiraus-com-tls', 'namespace': 'ingress'},
              'type': 'kubernetes.io/tls', 'data': {'tls.crt': base64.b64encode(cert.read_bytes()).decode(),
                                                'tls.key': base64.b64encode(key.read_bytes()).decode()}}])
    documents = render(f, 'fixture-gateway', ROOT / 'helm/bootstrap/gateway', 'ingress', {})
    gateway = next(d for d in documents if d['kind'] == 'Gateway' and d['metadata']['name'] == 'gateway')
    # The actual source listeners/routes/TLS stay intact; this owned fixture has
    # no external load balancer and reaches the generated ClusterIP internally.
    gateway['metadata'].setdefault('annotations', {})['networking.istio.io/service-type'] = 'ClusterIP'
    f.apply([gateway])
    f.kubectl('-n', 'ingress', 'wait', 'gateway/gateway', '--for=condition=Programmed', '--timeout=300s', timeout=330)
    f.kubectl('-n', 'ingress', 'rollout', 'status', 'deployment/gateway-istio', '--timeout=180s')
    service = get(f, 'service/gateway-istio', 'ingress')
    assert service['spec']['type'] == 'ClusterIP'
    return service['spec']['clusterIP'], cert.read_text()


def gateway_matrix(f, address, certificate):
    kwargs = dict(namespace='clients', host=address, port=443, certificate=certificate)
    authorized = {'Authorization': 'Bearer synthetic-native-client', 'X-Auth-Request-Email': 'forged@example.invalid'}
    common.wait_for('actual TLS Gateway authenticates through the source private authority route',
                    lambda: probe(f, 'web-client', '/mcp', headers=authorized, **kwargs)['status'] == 200, 120)
    unrelated, _ = create_certificate(f, 'unrelated-trust')
    untrusted = probe(f, 'web-client', '/mcp', headers=authorized,
                      **dict(kwargs, certificate=unrelated.read_text()))
    assert untrusted == {'status': 0, 'error': 'SSLCertVerificationError'}, untrusted
    for path, method in (('/mcp', 'GET'), ('/mcp', 'POST'), ('/sse', 'GET'),
                         ('/messages', 'GET'), ('/messages', 'POST'),
                         ('/message', 'GET'), ('/message', 'POST')):
        for headers in ({}, {'Authorization': 'Bearer invalid'}, {'X-Auth-Request-Email': 'forged@example.invalid'}):
            expect(f, 'web-client', path, 401, **kwargs, headers=headers, method=method)
        response = expect(f, 'web-client', path, 200, 'mcp', **kwargs, headers=authorized, method=method)
        assert json.loads(response['body'])['email'] == 'member@example.invalid', response
    for path in ('/api/worlds', '/api/worldbuilder-tools/read', '/api/worldbuilder-jobs/run', '/api/mcp-oauth/check'):
        result = probe(f, 'web-client', path, headers=authorized, **kwargs)
        assert result['status'] in (403, 404), (path, result)
    expect(f, 'browser', '/mcp', 0, host='homelab-mcp.homelab.svc.cluster.local')
    print('PASS: source TLS Gateway and all MCP paths enforce CUSTOM auth and hide private routes', flush=True)


def restart(f, resource, namespace, selector=None):
    obj = get(f, resource, namespace)
    match = selector or obj['spec']['selector']['matchLabels']
    labels = ','.join(k + '=' + v for k, v in match.items())
    before = get(f, 'pods', namespace)  # Filter the exact owning Deployment/DaemonSet selector below.
    pods = [p for p in before['items'] if all(p['metadata'].get('labels', {}).get(k) == v for k, v in match.items())]
    assert len(pods) == 1, (resource, len(pods))
    old = pods[0]['metadata']['uid']
    f.kubectl('-n', namespace, 'delete', 'pod/' + pods[0]['metadata']['name'], '--wait=true', '--timeout=90s')
    f.kubectl('-n', namespace, 'rollout', 'status', resource, '--timeout=180s')
    common.wait_for(resource + ' has a Ready replacement identity', lambda: any(
        p['metadata']['uid'] != old and any(c['type'] == 'Ready' and c['status'] == 'True' for c in p['status'].get('conditions', []))
        for p in json.loads(f.kubectl('-n', namespace, 'get', 'pods', '-l', labels, '-o', 'json'))['items']), 90)


def recovery_matrix(f, fixtures, address, certificate):
    kwargs = dict(namespace='clients', host=address, port=443, certificate=certificate,
                  headers={'Authorization': 'Bearer synthetic-native-client'})
    external = next(p for p in fixtures if p['metadata']['name'] == 'external')
    old_uid = get(f, 'pod/external')['metadata']['uid']
    f.kubectl('-n', 'homelab', 'delete', 'pod/external', '--wait=true', '--timeout=90s')
    common.wait_for('missing authorization backend refuses previously valid callers',
                    lambda: probe(f, 'web-client', '/mcp', **kwargs)['status'] == 503, 45)
    f.apply([external])
    f.kubectl('-n', 'homelab', 'wait', 'pod/external', '--for=condition=Ready', '--timeout=180s')
    assert get(f, 'pod/external')['metadata']['uid'] != old_uid
    common.wait_for('replacement authority restores real Gateway requests',
                    lambda: probe(f, 'web-client', '/mcp', **kwargs)['status'] == 200, 90)
    for namespace, resource in [('istio-system', 'deployment/istiod'), ('kube-system', 'ds/cilium'),
                                ('ingress', 'deployment/gateway-istio')]:
        restart(f, resource, namespace)
        if resource == 'deployment/istiod': istio_admission(f)
        common.wait_for(resource + ' preserves actual Gateway traffic',
                        lambda: probe(f, 'web-client', '/mcp', **kwargs)['status'] == 200, 90)
        expect(f, 'spoof-browser', '/api/worlds', 403)
        expect(f, 'unlabelled', '/health', 0)
        expect(f, 'web-client', '/mcp', 401, namespace='clients', host=address, port=443, certificate=certificate)
    print('PASS: authorization outage fails closed; authority, Istio, Cilium and Gateway replacement recover', flush=True)


def main():
    output, schemas = Path(sys.argv[1]), Path(sys.argv[2])
    f = common.Fixture(output)
    report = {'cilium': CILIUM_VERSION, 'istio': ISTIO_VERSION, 'status': 'RUNNING'}
    try:
        f.create(cni_installer=install_cilium)
        install_istio(f, schemas)
        fixtures = source_fixtures(f)
        certificate_identities(f, fixtures)
        identity_matrix(f, fixtures)
        address, certificate = install_gateway(f)
        gateway_matrix(f, address, certificate)
        recovery_matrix(f, fixtures, address, certificate)
        report.update(status='PASS', sourcePolicy=True, realCertificateIdentities=True,
                      gatewayTLSVerified=True, credentialRefusal=True, replacementRecovery=True)
    except BaseException:
        report['status'] = 'FAIL'
        raise
    finally:
        diagnostics(f)
        try:
            f.close()
            report['ownedClusterRemoved'] = True
        except BaseException:
            report.update(status='FAIL', ownedClusterRemoved=False)
            raise
        finally:
            (f.output / 'mesh-result.json').write_text(json.dumps(report, indent=2) + '\n')
    print('PASS: disposable source-policy mesh acceptance and teardown', flush=True)


if __name__ == '__main__': main()
