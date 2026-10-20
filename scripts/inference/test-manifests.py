#!/usr/bin/env python3
"""Render inference boundaries, including the real credential sidecar contract."""
import argparse
import json
import pathlib
import subprocess
import tempfile
import yaml

ROOT = pathlib.Path(__file__).resolve().parents[2]

def render(chart, values=None, ok=True):
    with tempfile.TemporaryDirectory(prefix='inference-render-') as d:
        p = pathlib.Path(d) / 'values.json'
        p.write_text(json.dumps(values or {}))
        result = subprocess.run(['helm', 'template', 'homelab-' + chart.split('/')[-1],
                                 str(ROOT / 'helm' / chart), '-f', str(p),
                                 '--set', 'global.allowMissingGeneratedSecretRefs=true'], text=True, capture_output=True)
    if not ok:
        assert result.returncode, 'unsafe configuration rendered successfully'
        return None, []
    assert result.returncode == 0, result.stderr
    return result.stdout, [r for r in yaml.safe_load_all(result.stdout) if r]

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--output', type=pathlib.Path)
    args = parser.parse_args()
    text, local = render('infra/inference')
    assert not any(r['kind'] in ('GatewayConfig', 'BackendSecurityPolicy') for r in local)
    cfg = json.loads(next(r for r in local if r['kind'] == 'ConfigMap')['data']['config.json'])
    assert cfg['cloud_enabled'] is False and cfg['monthly_micros'] == 100000000
    render('infra/inference', {'cloud': {'enabled': True}}, ok=False)
    cloud = yaml.safe_load((ROOT / 'helm/infra/inference/values.yaml').read_text())
    cloud['cloud'].update(enabled=True, roleArn='arn:aws:iam::123456789012:role/fixture',
                         profileArn='arn:aws:rolesanywhere:eu-west-1:123456789012:profile/fixture',
                         trustAnchorArn='arn:aws:rolesanywhere:eu-west-1:123456789012:trust-anchor/fixture',
                         helperImage='fixture/credential-helper:1.8.5')
    cloud['databaseSecret'] = 'fixture-inference-db'
    for m in cloud['config']['models']:
        if m['provider'] == 'bedrock':
            m.update(input_micros_per_million=1000000, output_micros_per_million=2000000,
                     price_version='offline-fixture-only', price_valid_until='2099-01-01T00:00:00Z')
    cloud['monitoring'].update(enabled=True, dashboard=True)
    cloud_text, resources = render('infra/inference', cloud)
    proxy = next(r for r in resources if r['kind'] == 'EnvoyProxy')
    pod = proxy['spec']['provider']['kubernetes']['envoyDeployment']['patch']['value']['spec']['template']['spec']
    helper = next(c for c in pod['containers'] if c['name'] == 'roles-anywhere')
    assert helper['args'][0] == 'serve' and '--private-key' in helper['args']
    assert not helper.get('ports'), 'credentials must never have a Service or public listener'
    cfg = next(r for r in resources if r['kind'] == 'GatewayConfig')
    assert cfg['spec']['extProc']['kubernetes']['env'][0]['value'] == 'http://127.0.0.1:9911'
    boundary = next(r for r in resources if r['kind'] == 'NetworkPolicy' and r['metadata']['name'] == 'inference-upstream')
    assert boundary['spec']['ingress'][0]['ports'][0]['port'] == 10080
    peers = boundary['spec']['ingress'][0]['from']
    assert len(peers) == 1 and peers[0]['podSelector']['matchLabels']['app.kubernetes.io/name'] == 'inference'
    assert not any(r['kind'] in ('HTTPRoute', 'Ingress') for r in resources)
    assert any(r['kind'] == 'PrometheusRule' for r in resources)
    assert any(r['kind'] == 'ConfigMap' and r['metadata']['name'] == 'inference-dashboard' for r in resources)
    cloud['cloud']['region'] = 'us-east-1'
    render('infra/inference', cloud, ok=False)
    vtext, vllm = render('infra/vllm', {'aiGateway': {'chatEnabled': False}, 'persistence': {'cache': {'enabled': True}}, 'security': {'networkPolicy': {'gatewayOnly': True}}})
    assert not any(r['kind'] == 'AIGatewayRoute' for r in vllm), 'legacy chat bypass remains'
    assert any(r['kind'] == 'PersistentVolumeClaim' for r in vllm)
    deployment = next(r for r in vllm if r['kind'] == 'Deployment')
    assert '--revision' in deployment['spec']['template']['spec']['containers'][0]['args']
    policy = next(r for r in vllm if r['kind'] == 'NetworkPolicy')
    assert len(policy['spec']['ingress']) == 1
    assert policy['spec']['ingress'][0]['from'][0]['podSelector']['matchLabels']['gateway.envoyproxy.io/owning-gateway-name'] == 'inference-upstream'
    for parent in ('flux-apps', 'flux-infra', 'flux-data'):
        overlay = yaml.safe_load((ROOT / 'helm/bootstrap' / parent / 'values-inference.yaml').read_text())
        overlay.update(image={'repository': 'ghcr.io/labiraus/homelab'})
        ptext, releases = render('bootstrap/' + parent, overlay)
        for release in releases:
            if release['kind'] != 'HelmRelease':
                continue
            name = release['metadata']['name']
            chart = {'external': 'apps/external', 'processor': 'apps/processor', 'vllm': 'infra/vllm', 'inference': 'infra/inference', 'postgres': 'data/postgres'}.get(name)
            if not chart:
                continue
            composed, docs = render(chart, release['spec'].get('values', {}))
            if name == 'postgres':
                db = next(r for r in docs if r['kind'] == 'Cluster')
                roles = {r['name']: r for r in db['spec']['managed']['roles']}
                assert roles['inference_runtime']['login'] and not roles['inference_owner']['login']
                policy = next(r for r in docs if r['kind'] == 'NetworkPolicy')
                assert any(p.get('podSelector', {}).get('matchLabels', {}).get('app.kubernetes.io/name') == 'inference' for p in policy['spec']['ingress'][0]['from'])
            if name == 'external':
                env = next(r for r in docs if r['kind'] == 'Deployment')['spec']['template']['spec']['containers'][0]['env']
                assert {e['name']: e.get('value') for e in env}['WORLDBUILDER_AI_DAILY_REQUESTS'] == '0'
            if args.output:
                (args.output / ('inference-composed-' + name + '-enabled.yaml')).write_text(composed)
    if args.output:
        (args.output / 'inference-local-enabled.yaml').write_text(text)
        (args.output / 'inference-cloud-enabled.yaml').write_text(cloud_text)
        (args.output / 'inference-vllm-enabled.yaml').write_text(vtext)
    print('PASS: local/cloud render, credential sidecar, restricted gateways, persistent cache and composed activation')

if __name__ == '__main__':
    main()
