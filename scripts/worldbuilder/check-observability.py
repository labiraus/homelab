#!/usr/bin/env python3
"""Render and cross-check opt-in monitoring against a pinned upstream stack.
Writes public fixtures for disposable runtime acceptance; never contacts a cluster.
"""
import copy
import json
from pathlib import Path
import subprocess
import sys
import yaml

ROOT = Path(__file__).resolve().parents[2]
STACK_VERSION = '92.3.0'
NAMESPACE = 'homelab'
SCRAPER_NAMESPACE = 'istio-system'
SCRAPER = 'prometheus-kube-prometheus-prometheus'
MONITOR = {'worldbuilderMonitoring': {'enabled': True, 'labels': {'release': 'prometheus'},
    'scraperNamespace': SCRAPER_NAMESPACE, 'scraperPodLabels': {'prometheus': SCRAPER},
    'scraperPrincipal': f'cluster.local/ns/{SCRAPER_NAMESPACE}/sa/{SCRAPER}'}}
DASHBOARD = {'stack': {'chartVersion': STACK_VERSION}, 'worldbuilderObservability': {
    'enabled': True, 'datasourceUID': 'fixture-prom', 'externalNamespace': NAMESPACE}}

PERSISTENCE = {'stack': {'chartVersion': STACK_VERSION}, 'prometheusStorage': {
    'enabled': True, 'storageClassName': 'standard', 'size': '1Gi', 'retention': '1h'}}


def only(documents, kind, name=None):
    matches = [d for d in documents if d.get('kind') == kind and
               (name is None or d['metadata']['name'] == name)]
    assert len(matches) == 1, (kind, name, len(matches))
    return matches[0]


def main(output, stack_chart):
    output = Path(output)
    fixtures = output / 'observability'
    fixtures.mkdir()

    def render(chart, values, name, overlay='values.yaml', reject=False, release=None, emit=True):
        valuefile = fixtures / (name + '-values.yaml')
        valuefile.write_text(yaml.safe_dump(values))
        chartpath = ROOT / 'helm' / chart
        result = subprocess.run(['helm', 'template', release or 'homelab-' + chartpath.name,
            str(chartpath), '--kube-version', '1.35.0', '-f', str(chartpath / overlay),
            '-f', str(valuefile), '--set', 'global.allowMissingGeneratedSecretRefs=true'],
            capture_output=True, text=True)
        if reject:
            assert result.returncode != 0, name + ' unexpectedly accepted'
            return
        assert result.returncode == 0, result.stderr
        if emit: (output / (name + '-enabled.yaml')).write_text(result.stdout)
        if result.stderr: (output / (name + '.stderr')).write_text(result.stderr)
        return [d for d in yaml.safe_load_all(result.stdout) if d]

    for overlay in ('values.yaml', 'values-ghcr.yaml', 'values-ecr.yaml'):
        suffix = overlay.removesuffix('.yaml')
        default_external = render('apps/external', {}, 'default-external-' + suffix, overlay, emit=False)
        assert not any(d['kind'] == 'ServiceMonitor' or d['metadata']['name'].endswith('-worldbuilder-metrics') for d in default_external)
        default_stack = render('observability/prometheus', {}, 'default-stack-' + suffix, overlay, emit=False)
        assert not any(d['kind'] == 'ConfigMap' for d in default_stack)
        default_release = only(default_stack, 'HelmRelease')
        # The OCI wrapper already occupies flux-system/prometheus.
        assert default_release['metadata']['name'] == 'prometheus-stack'
        assert default_release['spec']['releaseName'] == 'prometheus'
        assert default_release['spec']['chart']['spec']['version'] == '*'
        assert set(default_release['spec']['values']['grafana']) == {'grafana.ini'}
        assert 'prometheus' not in default_release['spec']['values']
        persistent = render('observability/prometheus', PERSISTENCE, 'persistent-stack-' + suffix, overlay)
        storage_values = only(persistent, 'HelmRelease')['spec']['values']['prometheus']
        assert storage_values['prometheusSpec'] == {'enableAdminAPI': False, 'retention': '1h',
            'storageSpec': {'volumeClaimTemplate': {'spec': {'accessModes': ['ReadWriteOnce'],
                'storageClassName': 'standard', 'resources': {'requests': {'storage': '1Gi'}}}}}}
        assert not any(d['kind'] == 'ConfigMap' for d in persistent)
        combined = copy.deepcopy(DASHBOARD); combined['prometheusStorage'] = PERSISTENCE['prometheusStorage']
        combined_docs = render('observability/prometheus', combined, 'persistent-dashboard-' + suffix, overlay)
        assert only(combined_docs, 'HelmRelease')['spec']['values']['prometheus'] == storage_values
        assert only(combined_docs, 'ConfigMap')['data']['worldbuilder.json']
        external = render('apps/external', MONITOR, 'external-monitor-' + suffix, overlay)
        monitor = only(external, 'ServiceMonitor')
        service = only(external, 'Service')
        deployment = only(external, 'Deployment')
        assert monitor['spec']['namespaceSelector']['matchNames'] == [service['metadata']['namespace']]
        assert all(service['metadata']['labels'][k] == v for k, v in monitor['spec']['selector']['matchLabels'].items())
        assert service['spec']['selector'] == deployment['spec']['selector']['matchLabels']
        endpoint = monitor['spec']['endpoints'][0]
        port = next(p for p in service['spec']['ports'] if p['name'] == endpoint['port'])
        assert port['targetPort'] == deployment['spec']['template']['spec']['containers'][0]['ports'][0]['containerPort']
        policy = only(external, 'NetworkPolicy', service['metadata']['name'] + '-worldbuilder-metrics')['spec']
        assert policy['podSelector']['matchLabels'] == service['spec']['selector']
        peer = policy['ingress'][0]['from'][0]
        assert peer == {'namespaceSelector': {'matchLabels': {'kubernetes.io/metadata.name': SCRAPER_NAMESPACE}},
                        'podSelector': {'matchLabels': MONITOR['worldbuilderMonitoring']['scraperPodLabels']}}
        assert policy['ingress'][0]['ports'] == [{'protocol': 'TCP', 'port': port['targetPort']}]
        authz = only(external, 'AuthorizationPolicy', service['metadata']['name'] + '-worldbuilder-metrics')['spec']
        assert authz['action'] == 'DENY' and len(authz['rules']) == 3
        principal = MONITOR['worldbuilderMonitoring']['scraperPrincipal']
        assert authz['rules'][0]['from'][0]['source'] == {'notPrincipals': [principal]}
        assert authz['rules'][0]['to'][0]['operation']['paths'] == ['/metrics', '/metrics/*']
        assert authz['rules'][1]['from'][0]['source'] == {'principals': [principal]}
        assert authz['rules'][1]['to'][0]['operation']['notPaths'] == ['/metrics']
        assert authz['rules'][2]['to'][0]['operation']['notMethods'] == ['GET']
        assert only(external, 'PeerAuthentication')['spec']['mtls']['mode'] == 'STRICT'
        assert 'WORLDBUILDER_ENABLED' not in [e['name'] for e in deployment['spec']['template']['spec']['containers'][0]['env']]
        dashboards = render('observability/prometheus', DASHBOARD, 'worldbuilder-dashboard-' + suffix, overlay)
        dashboard_map = only(dashboards, 'ConfigMap')
        dashboard = json.loads(dashboard_map['data']['worldbuilder.json'])
        assert '__inputs' not in dashboard
        assert {p['datasource']['uid'] for p in dashboard['panels']} == {'fixture-prom'}
        assert dashboard['templating']['list'][0]['current']['value'] == NAMESPACE

    # Namespace/name overrides must still select this release, not an assumed name.
    renamed = copy.deepcopy(MONITOR)
    renamed.update(namespace='different-apps', fullnameOverride='different-external')
    renamed_docs = render('apps/external', renamed, 'external-monitor-renamed', release='different-release')
    renamed_monitor = only(renamed_docs, 'ServiceMonitor')
    assert renamed_monitor['spec']['namespaceSelector']['matchNames'] == ['different-apps']
    assert renamed_monitor['spec']['selector']['matchLabels']['app.kubernetes.io/instance'] == 'different-release'

    invalid = [{'worldbuilderMonitoring': {'enabled': True}}]
    for key, value in [('labels', {}), ('scraperPodLabels', {}), ('scraperNamespace', ''),
                       ('scraperPrincipal', ''), ('scraperPrincipal', '*'),
                       ('scraperPrincipal', 'cluster.local/ns/elsewhere/sa/prometheus')]:
        values = copy.deepcopy(MONITOR)
        values['worldbuilderMonitoring'][key] = value
        invalid.append(values)
    for role in ('browserPrincipal', 'mcp', 'jobs'):
        values = copy.deepcopy(MONITOR)
        values['worldbuilder'] = {role: principal if role == 'browserPrincipal' else {'principal': principal}}
        invalid.append(values)
    for change in [{'istio': {'peerAuthentication': {'mode': 'PERMISSIVE'}}},
                   {'networkPolicy': {'enabled': False}}]:
        values = copy.deepcopy(MONITOR); values['security'] = change; invalid.append(values)
    for n, values in enumerate(invalid): render('apps/external', values, f'reject-monitor-{n}', reject=True)
    for n, changes in enumerate([{'stack': {'chartVersion': '*'}}, {'stack': {'chartVersion': ''}},
        {'worldbuilderObservability': {'enabled': True, 'datasourceUID': '', 'externalNamespace': NAMESPACE}},
        {'worldbuilderObservability': {'enabled': True, 'datasourceUID': 'fixture-prom', 'externalNamespace': ''}}]):
        values = copy.deepcopy(DASHBOARD); values.update(changes)
        render('observability/prometheus', values, f'reject-dashboard-{n}', reject=True)

    storage_invalid = []
    for key, value in [('enabled', 'false'), ('storageClassName', ''), ('storageClassName', '*'), ('storageClassName', 'bad/class'),
                       ('storageClassName', 'bad..class'), ('storageClassName', 'a' * 254),
                       ('size', ''), ('size', '0Gi'), ('size', '-1Gi'), ('size', '1GB'),
                       ('retention', ''), ('retention', '0h'), ('retention', '-1d'), ('retention', 'forever')]:
        values = copy.deepcopy(PERSISTENCE); values['prometheusStorage'][key] = value
        storage_invalid.append(values)
    for value in ('*', '', '92.x'):
        values = copy.deepcopy(PERSISTENCE); values['stack']['chartVersion'] = value
        storage_invalid.append(values)
    for value in ('', 'BAD'):
        values = copy.deepcopy(PERSISTENCE); values['stack']['targetNamespace'] = value
        storage_invalid.append(values)
    for n, values in enumerate(storage_invalid):
        render('observability/prometheus', values, f'reject-storage-{n}', reject=True)
    storage_input = fixtures / 'storage-upstream-values.yaml'
    storage_input.write_text(yaml.safe_dump({'prometheus': storage_values}))
    result = subprocess.run(['helm', 'template', 'prometheus', str(stack_chart), '-n', SCRAPER_NAMESPACE,
        '--kube-version', '1.35.0', '-f', str(storage_input)], capture_output=True, text=True)
    assert result.returncode == 0, result.stderr
    persistent_prom = only([d for d in yaml.safe_load_all(result.stdout) if d], 'Prometheus')
    assert persistent_prom['spec']['storage'] == storage_values['prometheusSpec']['storageSpec']
    assert persistent_prom['spec']['retention'] == '1h' and persistent_prom['spec']['enableAdminAPI'] is False
    (output / 'tsdb-storage-upstream.template').write_text(yaml.safe_dump(persistent_prom))
    (fixtures / 'storage-inputs.json').write_text(json.dumps(PERSISTENCE, indent=2) + '\n')
    print(f'PASS: opt-in persistent storage, unchanged defaults, dashboard composition and {len(storage_invalid)} refusal cases')

    # The actual upstream chart must mount this ConfigMap and provider together.
    release = only(dashboards, 'HelmRelease')
    upstream_values = fixtures / 'upstream-values.yaml'
    upstream_values.write_text(yaml.safe_dump(release['spec']['values']))
    result = subprocess.run(['helm', 'template', 'prometheus', str(stack_chart), '-n', SCRAPER_NAMESPACE,
        '--kube-version', '1.35.0', '-f', str(upstream_values)], capture_output=True, text=True)
    assert result.returncode == 0, result.stderr
    (fixtures / 'stack.upstream').write_text(result.stdout)
    upstream = [d for d in yaml.safe_load_all(result.stdout) if d]
    grafana = only(upstream, 'Deployment', 'prometheus-grafana')
    spec = grafana['spec']['template']['spec']
    assert next(v for v in spec['volumes'] if v['name'] == 'dashboards-worldbuilder')['configMap']['name'] == dashboard_map['metadata']['name']
    container = next(c for c in spec['containers'] if c['name'] == 'grafana')
    assert next(v for v in container['volumeMounts'] if v['name'] == 'dashboards-worldbuilder')['mountPath'] == '/var/lib/grafana/dashboards/worldbuilder'
    assert any(v['mountPath'] == '/etc/grafana/provisioning/dashboards/worldbuilder.yaml' for v in container['volumeMounts'])
    config = only(upstream, 'ConfigMap', 'prometheus-grafana')
    provider = yaml.safe_load(config['data']['worldbuilder.yaml'])
    assert provider['providers'][0]['options']['path'] == '/var/lib/grafana/dashboards/worldbuilder'
    # Preserve the upstream's existing sidecar provider; no label selector replacement.
    assert any(v['name'] == 'sc-dashboard-provider' for v in spec['volumes'])
    prometheus = only(upstream, 'Prometheus')
    assert prometheus['metadata']['name'] == SCRAPER
    assert prometheus['spec']['serviceAccountName'] == SCRAPER
    assert prometheus['spec']['serviceMonitorSelector']['matchLabels'] == monitor['metadata']['labels']
    assert prometheus['spec']['serviceMonitorNamespaceSelector'] == {}
    selected = [grafana, config, prometheus]
    (output / 'observability-upstream.template').write_text(yaml.safe_dump_all(selected))
    (fixtures / 'dashboard.json').write_text(json.dumps(dashboard, indent=2) + '\n')
    (fixtures / 'provider.yaml').write_text(yaml.safe_dump(provider))
    (fixtures / 'monitor.json').write_text(json.dumps(monitor, indent=2) + '\n')
    # Runtime versions are the images actually selected by this pinned chart.
    (fixtures / 'images.json').write_text(json.dumps({'grafana': container['image'],
        'prometheus': prometheus['spec']['image']}, indent=2) + '\n')
    print(f'PASS: monitoring selectors/port/identity boundaries, dashboard binding, upstream mounts and {len(invalid)+4} refusal cases')


if __name__ == '__main__':
    main(*sys.argv[1:])
