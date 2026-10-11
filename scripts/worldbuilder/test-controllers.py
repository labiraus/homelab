#!/usr/bin/env python3
"""Real disposable controller acceptance; never uses an operator kubeconfig.

Installs the pinned Prometheus Operator in a freshly generated kind cluster.
Only synthetic metrics and public chart inputs enter it. No production network,
live admission claims, existing context, host mount or source data is used.
"""
import base64
import copy
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
from urllib.parse import urlencode

import yaml

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
STACK_VERSION = '92.3.0'
STACK_NAMESPACE = 'istio-system'
PROMETHEUS = 'prometheus-kube-prometheus-prometheus'
OPERATOR = 'prometheus-kube-prometheus-operator'
PYTHON_IMAGE = 'python:3.12.12-alpine3.22'


def module(name):
    spec = importlib.util.spec_from_file_location(name, HERE / (name + '.py'))
    result = importlib.util.module_from_spec(spec); spec.loader.exec_module(result)
    return result


common = module('kind-fixture')
Fixture, wait_for = common.Fixture, common.wait_for


def operator_selector(f):
    deployment = json.loads(f.kubectl('-n', STACK_NAMESPACE, 'get', 'deployment/' + OPERATOR, '-o', 'json'))
    return ','.join(k + '=' + v for k, v in deployment['spec']['selector']['matchLabels'].items())


def install_stack(f, documents):
    crds = [d for d in documents if d['kind'] == 'CustomResourceDefinition']
    resources = [d for d in documents if d['kind'] != 'CustomResourceDefinition']
    hooks = [d for d in resources if 'helm.sh/hook' in (d['metadata'].get('annotations') or {})]
    assert crds and any(d['kind'] == 'Prometheus' for d in resources)
    assert {d['kind'] for d in resources} >= {'MutatingWebhookConfiguration', 'ValidatingWebhookConfiguration'}
    f.apply(crds)
    for crd in crds:
        f.kubectl('wait', 'crd/' + crd['metadata']['name'], '--for=condition=Established', '--timeout=60s')
    f.apply([d for d in hooks if d['kind'] != 'Job'])
    jobs = [d for d in hooks if d['kind'] == 'Job']
    create = [d for d in jobs if d['metadata']['name'].endswith('-admission-create')]
    patch = [d for d in jobs if d['metadata']['name'].endswith('-admission-patch')]
    assert len(create) == len(patch) == 1 and len(jobs) == 2
    f.apply(create)
    f.kubectl('-n', STACK_NAMESPACE, 'wait', 'job/' + create[0]['metadata']['name'],
              '--for=condition=Complete', '--timeout=180s')
    f.apply([d for d in resources if d not in hooks])
    f.apply(patch)
    f.kubectl('-n', STACK_NAMESPACE, 'wait', 'job/' + patch[0]['metadata']['name'],
              '--for=condition=Complete', '--timeout=180s')
    f.kubectl('-n', STACK_NAMESPACE, 'rollout', 'status', 'deployment/' + OPERATOR, '--timeout=240s')


def admission_drill(f):
    """Exercise actual semantic admission, TLS trust, and fail-closed recovery."""
    rule = {'apiVersion': 'monitoring.coreos.com/v1', 'kind': 'PrometheusRule',
            'metadata': {'name': 'fixture-rule', 'namespace': STACK_NAMESPACE},
            'spec': {'groups': [{'name': 'fixture', 'rules': [{'alert': 'OfflineRule',
                     'expr': 'up == 0', 'annotations': {'summary': 42}}]}]}}

    def dry(obj, check=True):
        return f.kubectl('apply', '--dry-run=server', '-f', '-', '-o', 'json',
                         data=json.dumps(obj).encode(), check=check, timeout=45)

    def ready(label):
        wait_for(label, lambda: dry(rule, False)[0] == 0, 90)
        assert json.loads(dry(rule))['spec']['groups'][0]['rules'][0]['annotations']['summary'] == '42'

    configurations = {}
    for kind in ('validatingwebhookconfiguration', 'mutatingwebhookconfiguration'):
        key = kind + '/prometheus-kube-prometheus-admission'
        obj = json.loads(f.kubectl('get', key, '-o', 'json'))
        assert obj['webhooks'] and all(w['failurePolicy'] == 'Fail' and w['clientConfig'].get('caBundle')
                                       for w in obj['webhooks'])
        configurations[key] = obj
    ready('real admission accepts and mutates a valid PrometheusRule')
    bad = copy.deepcopy(rule); bad['spec']['groups'][0]['rules'][0]['expr'] = 'sum('
    code, out, err = dry(bad, False)
    assert code and 'could not parse expression' in err, (code, out, err)
    alert = {'apiVersion': 'monitoring.coreos.com/v1alpha1', 'kind': 'AlertmanagerConfig',
             'metadata': {'name': 'fixture-alertmanager', 'namespace': STACK_NAMESPACE},
             'spec': {'route': {'receiver': 'sink'}, 'receivers': [{'name': 'sink'}]}}
    dry(alert)
    bad = copy.deepcopy(alert); bad['spec']['route']['receiver'] = 'missing'
    code, out, err = dry(bad, False)
    assert code and 'receiver' in err.lower() and 'missing' in err, (code, out, err)
    print('PASS: invalid PromQL and missing Alertmanager receiver are refused', flush=True)

    # A different, valid CA isolates trust failure from malformed input or server downtime.
    other = base64.b64encode(json.loads(f.kubectl('-n', STACK_NAMESPACE, 'get',
        'configmap/kube-root-ca.crt', '-o', 'json'))['data']['ca.crt'].encode()).decode()
    for key, obj in configurations.items():
        original = obj['webhooks'][0]['clientConfig']['caBundle']
        assert other != original

        def trust(value):
            f.kubectl('patch', key, '--type=json', '-p', json.dumps([{
                'op': 'replace', 'path': '/webhooks/0/clientConfig/caBundle', 'value': value}]))

        def untrusted():
            code, out, err = dry(rule, False)
            return code != 0 and 'certificate signed by unknown authority' in err

        try:
            trust(other)
            wait_for(key + ' refuses incorrect certificate trust', untrusted, 60)
        finally:
            trust(original)
        ready(key + ' recovers with the original certificate trust')

    selector = operator_selector(f)
    before = json.loads(f.kubectl('-n', STACK_NAMESPACE, 'get', 'pods', '-l', selector, '-o', 'json'))['items']
    assert len(before) == 1
    old_uid = before[0]['metadata']['uid']
    f.kubectl('-n', STACK_NAMESPACE, 'scale', 'deployment/' + OPERATOR, '--replicas=0')
    try:
        f.kubectl('-n', STACK_NAMESPACE, 'wait', 'pod', '-l', selector, '--for=delete', '--timeout=90s')
        code, out, err = dry(rule, False)
        assert code and 'failed calling webhook' in err, (code, out, err)
        print('PASS: unavailable webhook refuses valid input', flush=True)
    finally:
        f.kubectl('-n', STACK_NAMESPACE, 'scale', 'deployment/' + OPERATOR, '--replicas=1')
        f.kubectl('-n', STACK_NAMESPACE, 'rollout', 'status', 'deployment/' + OPERATOR, '--timeout=240s')
    ready('operator restart restores actual admission and mutation')
    after = json.loads(f.kubectl('-n', STACK_NAMESPACE, 'get', 'pods', '-l', selector, '-o', 'json'))['items']
    assert len(after) == 1 and after[0]['metadata']['uid'] != old_uid
    dry(alert)


def monitor_drill(f, fixture_directory, metrics):
    directory = Path(fixture_directory)
    monitor = json.loads((directory / 'monitor.json').read_text())
    namespace = monitor['metadata']['namespace']
    endpoint = monitor['spec']['endpoints'][0]
    # Only the disposable scrape cadence changes; selectors/filtering/limits are source renders.
    endpoint['interval'] = '2s'; endpoint['scrapeTimeout'] = '1s'
    chartdir = f.output / 'chart'
    subprocess.run(['helm', 'pull', 'kube-prometheus-stack', '--version', STACK_VERSION,
        '--repo', 'https://prometheus-community.github.io/helm-charts', '--untar',
        '--untardir', str(chartdir)], env=f.helm_env, check=True, timeout=120)
    values = {key: {'enabled': False} for key in ('alertmanager', 'grafana', 'nodeExporter',
        'kubeStateMetrics', 'kubeApiServer', 'kubelet', 'kubeControllerManager', 'coreDns',
        'kubeDns', 'kubeEtcd', 'kubeScheduler', 'kubeProxy')}
    values.update(defaultRules={'create': False},
        prometheusOperator={'admissionWebhooks': {'enabled': True, 'failurePolicy': 'Fail',
                                'patch': {'ttlSecondsAfterFinished': 1800}},
                            'tls': {'enabled': True}, 'serviceMonitor': {'selfMonitor': False}},
        prometheus={'serviceMonitor': {'selfMonitor': False}, 'prometheusSpec': {
            'retention': '1h', 'resources': {'requests': {'memory': '128Mi', 'cpu': '50m'},
                                          'limits': {'memory': '512Mi'}},
            'scrapeInterval': '2s', 'evaluationInterval': '2s'}})
    valuefile = f.output / 'stack-values.yaml'; valuefile.write_text(yaml.safe_dump(values))
    rendered = subprocess.check_output(['helm', 'template', 'prometheus',
        str(chartdir / 'kube-prometheus-stack'), '-n', STACK_NAMESPACE,
        '--kube-version', '1.35.0', '--include-crds', '-f', str(valuefile)], env=f.helm_env)
    (f.output / 'stack.yaml').write_bytes(rendered)
    # The upstream CRD enums can contain an unquoted '='. Use the repository safe loader.
    loader = module('validate-custom-resources').KubernetesLoader
    documents = [d for d in yaml.load_all(rendered, Loader=loader) if d]
    f.apply([{'apiVersion': 'v1', 'kind': 'Namespace', 'metadata': {'name': n}}
             for n in (namespace, STACK_NAMESPACE, 'wrong-namespace')])
    install_stack(f, documents)
    admission_drill(f)
    wait_for('operator creates the Prometheus Pod', lambda: bool(json.loads(f.kubectl(
        '-n', STACK_NAMESPACE, 'get', 'pods', '-l', 'prometheus=' + PROMETHEUS, '-o', 'json'))['items']))
    f.kubectl('-n', STACK_NAMESPACE, 'wait', 'pod', '-l', 'prometheus=' + PROMETHEUS,
              '--for=condition=Ready', '--timeout=240s')
    labels = monitor['spec']['selector']['matchLabels']
    serve = '''from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
class Handler(BaseHTTPRequestHandler):
 def do_GET(self):
  if self.path != '/metrics': self.send_error(404); return
  self.send_response(200); self.send_header('Content-Type','text/plain; version=0.0.4'); self.end_headers()
  self.wfile.write(Path('/fixture/metrics.txt').read_bytes())
 def log_message(self,*args): pass
HTTPServer(('0.0.0.0',8080),Handler).serve_forever()
'''
    metric_text = Path(metrics).read_text() + '\noffline_unrelated_metric 99\nexternal_ops_total{method="unrelated",state="call"} 99\n'
    app = [{'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {'name': 'metrics', 'namespace': namespace},
            'data': {'serve.py': serve, 'metrics.txt': metric_text}},
           {'apiVersion': 'apps/v1', 'kind': 'Deployment', 'metadata': {'name': 'metrics', 'namespace': namespace},
            'spec': {'replicas': 1, 'selector': {'matchLabels': {'fixture': 'metrics'}},
                'template': {'metadata': {'labels': {'fixture': 'metrics'}}, 'spec': {
                    'automountServiceAccountToken': False,
                    'containers': [{'name': 'metrics', 'image': PYTHON_IMAGE,
                        'command': ['python3', '/fixture/serve.py'], 'ports': [{'name': 'http', 'containerPort': 8080}],
                        'readinessProbe': {'httpGet': {'path': '/metrics', 'port': 'http'}, 'periodSeconds': 2},
                        'resources': {'requests': {'cpu': '10m', 'memory': '16Mi'}, 'limits': {'memory': '64Mi'}},
                        'volumeMounts': [{'name': 'fixture', 'mountPath': '/fixture', 'readOnly': True}]}],
                    'volumes': [{'name': 'fixture', 'configMap': {'name': 'metrics'}}]}}}},
           {'apiVersion': 'v1', 'kind': 'Service', 'metadata': {'name': 'metrics', 'namespace': namespace, 'labels': labels},
            'spec': {'selector': {'fixture': 'metrics'}, 'ports': [{'name': endpoint['port'], 'port': 80, 'targetPort': 'http'}]}}]
    f.apply(app)
    f.kubectl('-n', namespace, 'rollout', 'status', 'deployment/metrics', '--timeout=180s')
    f.apply([monitor])

    def api(path):
        code, out, _ = f.kubectl('get', '--raw', '/api/v1/namespaces/' + STACK_NAMESPACE +
            '/services/http:' + PROMETHEUS + ':9090/proxy/api/v1/' + path, check=False)
        if code: return None
        value = json.loads(out)
        return value.get('data') if value.get('status') == 'success' else None

    def targets():
        data = api('targets')
        if data is None: return None
        return [x for x in data['activeTargets'] if x['labels'].get('job') == 'worldbuilder-external']

    def healthy():
        data = targets()
        return data is not None and len(data) == 1 and data[0]['health'] == 'up' and data[0]['labels']['namespace'] == namespace

    wait_for('operator discovers exactly one healthy rendered ServiceMonitor target', healthy)
    def query(expression):
        data = api('query?' + urlencode({'query': expression}))
        return None if data is None else data['result']

    expression = 'external_ops_total{job="worldbuilder-external",method="worldbuilder",state="call"}'
    wait_for('actual Go metric value survives operator discovery and metric filtering',
             lambda: (r := query(expression)) is not None and len(r) == 1 and float(r[0]['value'][1]) == 6)
    assert query('offline_unrelated_metric') == []
    assert query('external_ops_total{method="unrelated"}') == []
    # Wrong monitor labels, Service labels, namespace and named port cannot add targets.
    invalids = []
    for suffix in ('monitor-label', 'service-label', 'namespace', 'port'):
        item = copy.deepcopy(monitor); item['metadata']['name'] += '-' + suffix
        if suffix == 'monitor-label': item['metadata']['labels'] = {'release': 'unselected'}
        elif suffix == 'service-label': item['spec']['selector']['matchLabels'] = {'fixture': 'absent'}
        elif suffix == 'namespace': item['spec']['namespaceSelector']['matchNames'] = ['wrong-namespace']
        else: item['spec']['endpoints'][0]['port'] = 'absent'
        invalids.append(item)
    f.apply(invalids)
    def configured():
        config = api('status/config')
        if config is None: return False
        text = config['yaml']
        return all(item['metadata']['name'] in text for item in invalids[1:]) and invalids[0]['metadata']['name'] not in text
    wait_for('operator reconciles negative selector fixtures', configured)
    assert healthy(), targets()
    # Update and delete must reach live discovery, rather than leave stale targets.
    f.kubectl('-n', namespace, 'label', 'service/metrics', 'app.kubernetes.io/instance=unselected', '--overwrite')
    wait_for('Service label change removes the target', lambda: targets() == [])
    f.kubectl('-n', namespace, 'label', 'service/metrics',
              'app.kubernetes.io/instance=' + labels['app.kubernetes.io/instance'], '--overwrite')
    wait_for('Service label restoration rediscovers one target', healthy)
    f.kubectl('-n', namespace, 'delete', 'servicemonitor', monitor['metadata']['name'])
    wait_for('ServiceMonitor deletion removes discovery configuration', lambda: targets() == [])
    f.apply([monitor])
    wait_for('ServiceMonitor recreation recovers scraping', healthy)
    for selector, label in [(operator_selector(f), 'operator'),
                            ('prometheus=' + PROMETHEUS, 'Prometheus')]:
        before = json.loads(f.kubectl('-n', STACK_NAMESPACE, 'get', 'pods', '-l', selector, '-o', 'json'))['items']
        assert len(before) == 1, (label, len(before))
        old_uid = before[0]['metadata']['uid']
        f.kubectl('-n', STACK_NAMESPACE, 'delete', 'pod', before[0]['metadata']['name'], '--wait=true')
        def replaced():
            items = json.loads(f.kubectl('-n', STACK_NAMESPACE, 'get', 'pods', '-l', selector, '-o', 'json'))['items']
            return len(items) == 1 and items[0]['metadata']['uid'] != old_uid and any(
                c['type'] == 'Ready' and c['status'] == 'True' for c in items[0].get('status', {}).get('conditions', []))
        wait_for(label + ' Pod replacement becomes Ready', replaced, 240)
        wait_for(label + ' restart preserves exactly one healthy target', healthy)
    assert query('offline_unrelated_metric') == []
    pods = json.loads(f.kubectl('get', 'pods', '-A', '-o', 'json'))['items']
    images = sorted({c['imageID'] for p in pods for c in p.get('status', {}).get('containerStatuses', []) if c.get('imageID')})
    (f.output / 'controller-result.json').write_text(json.dumps({
        'status': 'passed', 'stackVersion': STACK_VERSION, 'observedImageIDs': images, 'tests': [
            'kind Ready/DNS/exec transport', 'operator reconciliation', 'Go metrics and filtering',
            'monitor/Service/namespace/port isolation', 'Service change recovery', 'monitor delete/recreate',
            'operator and Prometheus replacement', 'real certificate hook Jobs',
            'PrometheusRule annotation mutation and semantic refusal', 'AlertmanagerConfig receiver refusal',
            'validating and mutating TLS trust refusal/recovery', 'unavailable admission refuses valid input',
            'fresh operator UID and real admission recovery'],
        'exclusions': ['production readiness', 'Istio/NetworkPolicy enforcement', 'installed admission',
                       'CNPG/Velero restore', 'persistent TSDB recovery']}, indent=2) + '\n')


def main():
    if len(sys.argv) != 4:
        raise SystemExit('Usage: test-controllers.py OUTPUT_DIRECTORY OBSERVABILITY_FIXTURES GO_METRICS')
    f = Fixture(sys.argv[1])
    try:
        f.create()
        monitor_drill(f, sys.argv[2], sys.argv[3])
    finally:
        try:
            for name, args in [('pods', ['get', 'pods', '-A', '-o', 'wide']),
                               ('events', ['get', 'events', '-A', '--sort-by=.lastTimestamp']),
                               ('operator', ['-n', STACK_NAMESPACE, 'logs', 'deployment/' + OPERATOR, '--tail=200']),
                               ('webhooks', ['get', 'validatingwebhookconfiguration,mutatingwebhookconfiguration', '-o', 'json'])]:
                code, out, err = f.kubectl(*args, check=False, timeout=30)
                (f.output / (name + '.log')).write_text(out + err)
        finally:
            f.close()
    print('PASS: disposable Prometheus controller acceptance and fixture teardown', flush=True)


if __name__ == '__main__':
    main()
