#!/usr/bin/env python3
"""Real disposable controller acceptance; never uses an operator kubeconfig.

Installs the pinned Prometheus Operator in a freshly generated kind cluster.
Only synthetic metrics and public chart inputs enter it. No production network,
mesh/admission claims, existing context, host mount or source data is used.
"""
import copy
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shlex
import shutil
import ssl
import subprocess
import sys
import time
from urllib.parse import urlencode
import uuid

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


class Fixture:
    def __init__(self, output):
        self.output = Path(output).resolve(); self.output.mkdir(parents=True, exist_ok=True)
        self.private = self.output / '.private'; self.private.mkdir(mode=0o700)
        self.name = 'wbctrl-' + uuid.uuid4().hex[:8]
        self.node = self.name + '-control-plane'
        self.docker = module('docker-exec').FixtureDocker(self.name)
        self.kind = module('install-kind').install(self.private / 'tools')
        self.cli = shutil.which('docker')
        if not self.cli: raise ValueError('Docker CLI required')
        bindir = self.private / 'bin'; bindir.mkdir()
        shim = bindir / 'docker'
        shim.write_text('#!/bin/sh\nexec ' + shlex.quote(sys.executable) + ' ' +
                        shlex.quote(str(HERE / 'docker-exec.py')) + ' "$@"\n')
        shim.chmod(0o755)
        self.env = dict(os.environ, PATH=str(bindir) + ':' + os.environ['PATH'],
            WORLDBUILDER_DOCKER_CLI=self.cli, WORLDBUILDER_KIND_NAME=self.name,
            KIND_EXPERIMENTAL_PROVIDER='docker', DOCKER_HOST='unix:///var/run/docker.sock',
            DOCKER_CONTEXT='', KUBECONFIG=str(self.private / 'kubeconfig'))
        shutil.copyfile(self.private / 'tools/kind.json', self.output / 'kind.json')
        cache = self.private / 'helm-cache'
        (cache / 'repository').mkdir(parents=True)
        self.helm_env = dict(os.environ, KUBECONFIG='/dev/null', HELM_CACHE_HOME=str(cache))

    def execute(self, command, data=None, check=True, timeout=300):
        out, err = io.BytesIO(), io.BytesIO()
        code = self.docker.execute(self.node, command,
            stdin=io.BytesIO(data) if data is not None else None,
            stdout=out, stderr=err, timeout=timeout)
        if check and code:
            raise RuntimeError(f'Fixture command failed ({code}): {command[:3]}: ' +
                               err.getvalue().decode(errors='replace')[-4000:])
        return code, out.getvalue(), err.getvalue()

    def kubectl(self, *args, data=None, check=True, timeout=300):
        code, out, err = self.execute(['kubectl', '--kubeconfig=/etc/kubernetes/admin.conf',
            '--request-timeout=240s', *args], data, check, timeout)
        return out.decode() if check else (code, out.decode(), err.decode())

    def apply(self, documents):
        self.kubectl('apply', '--server-side', '--field-manager=offline-acceptance', '-f', '-',
                     data=yaml.safe_dump_all(documents).encode())

    def create(self):
        config = self.private / 'kind.yaml'
        config.write_text(yaml.safe_dump({'kind': 'Cluster', 'apiVersion': 'kind.x-k8s.io/v1alpha4',
            'networking': {'apiServerAddress': '127.0.0.1', 'ipFamily': 'ipv4'},
            'nodes': [{'role': 'control-plane'}]}))
        with (self.output / 'kind.log').open('w') as log:
            subprocess.run([str(self.kind), 'create', 'cluster', '--name', self.name,
                '--image', module('install-kind').NODE, '--config', str(config),
                '--kubeconfig', self.env['KUBECONFIG'], '--wait', '180s', '--retain'],
                env=self.env, stdout=log, stderr=subprocess.STDOUT, check=True, timeout=480)
        self.kubectl('wait', 'nodes', '--all', '--for=condition=Ready', '--timeout=180s')
        self.kubectl('-n', 'kube-system', 'rollout', 'status', 'deployment/coredns', '--timeout=180s')
        if bundle := os.environ.get('WORLDBUILDER_TEST_CA_BUNDLE'):
            data = Path(bundle).read_bytes()
            if len(data) > 1024**2 or b'PRIVATE KEY' in data:
                raise ValueError('Fixture registry trust requires a bounded public CA bundle')
            ssl.create_default_context(cadata=data.decode())
            self.execute(['sh', '-c', 'cat >> /etc/ssl/certs/ca-certificates.crt && systemctl restart containerd'], data)
            (self.output / 'registry-trust.json').write_text(json.dumps({
                'sha256': hashlib.sha256(data).hexdigest(), 'tlsVerification': True}) + '\n')
        # These checks catch early-return Docker proxies before controller tests.
        before = time.monotonic()
        code, out, err = self.execute(['sh', '-c',
            'printf before; sleep 2; cat; printf late >&2; exit 7'], b'after\x00binary', check=False)
        assert (code, out, err) == (7, b'beforeafter\x00binary', b'late')
        assert time.monotonic() - before >= 2
        try: self.docker.owned_node('unowned-control-plane')
        except ValueError: pass
        else: raise AssertionError('Missing ownership refusal')
        info = self.docker.api('GET', '/containers/' + self.node + '/json')
        assert all(binding['HostIp'] == '127.0.0.1'
                   for binding in info['NetworkSettings']['Ports']['6443/tcp'])
        print('PASS: owned loopback kind cluster, Ready node/DNS and authoritative exec completion', flush=True)

    def close(self):
        # Explicit generated context applies to teardown too. Never lock/edit ~/.kube/config.
        try:
            self.docker.owned_node(self.node)
        except RuntimeError as error:
            if ': 404:' in str(error):
                shutil.rmtree(self.private)
                return
            raise
        with (self.output / 'cleanup.log').open('w') as log:
            subprocess.run([str(self.kind), 'delete', 'cluster', '--name', self.name,
                '--kubeconfig', self.env['KUBECONFIG']], env=self.env,
                stdout=log, stderr=subprocess.STDOUT, check=True, timeout=120)
        try: self.docker.owned_node(self.node)
        except RuntimeError as error:
            if ': 404:' not in str(error): raise
        else: raise AssertionError('Disposable node survived teardown')
        shutil.rmtree(self.private)


def wait_for(label, predicate, timeout=180):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if predicate():
            print('PASS: ' + label, flush=True)
            return
        time.sleep(2)
    raise TimeoutError(label)


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
        prometheusOperator={'admissionWebhooks': {'enabled': False},
                            'tls': {'enabled': False}, 'serviceMonitor': {'selfMonitor': False}},
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
    crds = [d for d in documents if d['kind'] == 'CustomResourceDefinition']
    resources = [d for d in documents if d['kind'] != 'CustomResourceDefinition']
    assert crds and any(d['kind'] == 'Prometheus' for d in resources)
    assert not any(d['kind'] in ('MutatingWebhookConfiguration', 'ValidatingWebhookConfiguration') for d in resources)
    f.apply([{'apiVersion': 'v1', 'kind': 'Namespace', 'metadata': {'name': n}}
             for n in (namespace, STACK_NAMESPACE, 'wrong-namespace')])
    f.apply(crds)
    for crd in crds:
        f.kubectl('wait', 'crd/' + crd['metadata']['name'], '--for=condition=Established', '--timeout=60s')
    f.apply(resources)
    f.kubectl('-n', STACK_NAMESPACE, 'rollout', 'status', 'deployment/' + OPERATOR, '--timeout=240s')
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
    for selector, label in [('app.kubernetes.io/name=kube-prometheus-stack-prometheus-operator', 'operator'),
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
            'operator and Prometheus replacement'],
        'exclusions': ['production readiness', 'Istio/NetworkPolicy enforcement', 'admission webhooks',
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
                               ('events', ['get', 'events', '-A', '--sort-by=.lastTimestamp'])]:
                code, out, err = f.kubectl(*args, check=False, timeout=30)
                (f.output / (name + '.log')).write_text(out + err)
        finally:
            f.close()
    print('PASS: disposable Prometheus controller acceptance and fixture teardown', flush=True)


if __name__ == '__main__':
    main()
