#!/usr/bin/env python3
"""Exercise rendered monitoring with disposable services on an internal Docker network.
Usage: test-observability.py RENDER_DIR/observability ACTUAL_GO_METRICS_FILE
No published ports, cluster context, production endpoint or authentication bypass.
"""
import json
from pathlib import Path
import subprocess
import sys
import time
import uuid
import yaml

PYTHON_IMAGE = 'python:3.12.12-alpine3.22'


def command(*args, **kwargs):
    result = subprocess.run(list(args), capture_output=True, text=True, **kwargs)
    if result.returncode:
        raise RuntimeError(f'{args[0:3]} failed: {result.stderr[-3000:]} {result.stdout[-1000:]}')
    return result.stdout.strip()


def main(directory, metrics_file):
    directory = Path(directory).resolve()
    metrics = Path(metrics_file).read_text()
    assert 'external_ops_total{method="worldbuilder",state="call"} 6' in metrics
    assert 'private-world' not in metrics
    dashboard = json.loads((directory / 'dashboard.json').read_text())
    monitor = json.loads((directory / 'monitor.json').read_text())
    images = json.loads((directory / 'images.json').read_text())
    namespace = dashboard['templating']['list'][0]['current']['value']
    uid = dashboard['panels'][0]['datasource']['uid']
    queries = [p['targets'][0]['expr'].replace('$namespace', namespace) for p in dashboard['panels']]
    runtime = directory / 'runtime'
    runtime.mkdir()
    # Promtool checks the exact dashboard expressions, including unrelated scopes.
    series = []
    for state, step in [('call', 120), ('error', 6), ('conflict', 3), ('denied', 12), ('budget', 18), ('invalid', 24)]:
        series.append({'series': f'external_ops_total{{namespace="{namespace}",job="worldbuilder-external",method="worldbuilder",state="{state}"}}', 'values': f'0+{step}x10'})
    for suffix, step in [('sum', 240), ('count', 120)]:
        series.append({'series': f'external_processing_duration_seconds_{suffix}{{namespace="{namespace}",job="worldbuilder-external",method="worldbuilder"}}', 'values': f'0+{step}x10'})
    for different in [('elsewhere', 'worldbuilder-external', 'worldbuilder'), (namespace, 'unrelated', 'worldbuilder'), (namespace, 'worldbuilder-external', 'other')]:
        for metric, extra in [('external_ops_total', ',state="call"'), ('external_processing_duration_seconds_sum', ''), ('external_processing_duration_seconds_count', '')]:
            series.append({'series': f'{metric}{{namespace="{different[0]}",job="{different[1]}",method="{different[2]}"{extra}}}', 'values': '0+60000x10'})
    expected = [[{'labels': '{method="worldbuilder"}', 'value': 2}],
                [{'labels': '{method="worldbuilder"}', 'value': 0.1}],
                [{'labels': f'{{method="worldbuilder",state="{state}"}}', 'value': value}
                 for state, value in [('conflict', 0.05), ('denied', 0.2), ('budget', 0.3), ('invalid', 0.4)]],
                [{'labels': '{method="worldbuilder"}', 'value': 2}]]
    rule_tests = {'rule_files': [], 'evaluation_interval': '1m', 'tests': [
        {'interval': '1m', 'input_series': series, 'promql_expr_test': [
            {'expr': expr, 'eval_time': '10m', 'exp_samples': expected[i]} for i, expr in enumerate(queries)]},
        {'interval': '1m', 'input_series': [], 'promql_expr_test': [
            {'expr': expr, 'eval_time': '10m', 'exp_samples': []} for expr in queries]}]}
    (runtime / 'queries.yaml').write_text(yaml.safe_dump(rule_tests))
    (runtime / 'metrics.txt').write_text(metrics)
    (runtime / 'serve.py').write_text('''from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
class Handler(BaseHTTPRequestHandler):
 def do_GET(self):
  if self.path != '/metrics': self.send_error(404); return
  body=Path('/tmp/fixture/metrics.txt').read_bytes()
  self.send_response(200); self.send_header('Content-Type','text/plain; version=0.0.4'); self.end_headers(); self.wfile.write(body)
 def log_message(self,*args): pass
HTTPServer(('0.0.0.0',8080),Handler).serve_forever()
''')
    endpoint = monitor['spec']['endpoints'][0]
    def relabel(value):
        names = {'sourceLabels': 'source_labels', 'targetLabel': 'target_label'}
        return {names.get(k, k): v for k, v in value.items()}
    scrape = {'job_name': 'from-rendered-monitor', 'scrape_interval': '1s', 'scrape_timeout': '1s',
        'metrics_path': endpoint['path'], 'scheme': endpoint['scheme'], 'honor_labels': endpoint['honorLabels'],
        'sample_limit': monitor['spec']['sampleLimit'], 'label_limit': monitor['spec']['labelLimit'],
        'relabel_configs': [relabel(r) for r in endpoint['relabelings']],
        'metric_relabel_configs': [relabel(r) for r in endpoint['metricRelabelings']],
        'static_configs': [{'targets': ['metrics:8080'], 'labels': {'namespace': namespace}}]}
    (runtime / 'prometheus.yml').write_text(yaml.safe_dump({'global': {'scrape_interval': '1s'}, 'scrape_configs': [scrape]}))
    provisioning = runtime / 'provisioning'
    (provisioning / 'dashboards').mkdir(parents=True)
    (provisioning / 'datasources').mkdir()
    (provisioning / 'dashboards/worldbuilder.yaml').write_text((directory / 'provider.yaml').read_text())
    (provisioning / 'datasources/fixture.yaml').write_text(yaml.safe_dump({'apiVersion': 1, 'datasources': [
        {'name': 'Fixture Prometheus', 'uid': uid, 'type': 'prometheus', 'access': 'proxy', 'url': 'http://prometheus:9090'}]}))
    dashboards = runtime / 'dashboards/worldbuilder'
    dashboards.mkdir(parents=True)
    (dashboards / 'worldbuilder.json').write_text(json.dumps(dashboard))

    prefix = 'worldbuilder-observability-' + uuid.uuid4().hex[:10]
    network, containers = prefix, []
    created_network = False
    try:
        for image in [*images.values(), PYTHON_IMAGE]: command('docker', 'pull', image, timeout=240)
        def create(name, image, *args, networked=True, env=()):
            name = prefix + '-' + name
            options = ['--network', network if networked else 'none', '--name', name,
                       '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges']
            if networked: options += ['--network-alias', name.removeprefix(prefix + '-')]
            for value in env: options += ['-e', value]
            command('docker', 'create', *options, image, *args)
            containers.append(name)
            return name
        command('docker', 'network', 'create', '--internal', network)
        created_network = True
        assert json.loads(command('docker', 'network', 'inspect', network))[0]['Internal']
        # Promtool is shipped alongside the server in the exact selected image.
        checker = prefix + '-promtool'
        command('docker', 'create', '--name', checker, '--network', 'none', '--entrypoint', '/bin/promtool',
                images['prometheus'], 'test', 'rules', '/tmp/queries.yaml')
        containers.append(checker)
        command('docker', 'cp', str(runtime / 'queries.yaml'), checker + ':/tmp/queries.yaml')
        command('docker', 'start', '-a', checker, timeout=60)
        assert command('docker', 'inspect', '--format', '{{.State.ExitCode}}', checker) == '0'
        print('PASS: exact dashboard PromQL rates, means, namespace/job isolation and no-data behavior', flush=True)
        fixture = create('metrics', PYTHON_IMAGE, 'python3', '-u', '/tmp/fixture/serve.py')
        command('docker', 'cp', str(runtime), fixture + ':/tmp/fixture')
        prometheus = create('prometheus', images['prometheus'], '--config.file=/etc/prometheus/prometheus.yml',
                            '--storage.tsdb.path=/prometheus', '--storage.tsdb.retention.time=1h', '--log.level=warn')
        command('docker', 'cp', str(runtime / 'prometheus.yml'), prometheus + ':/etc/prometheus/prometheus.yml')
        grafana = create('grafana', images['grafana'], env=(
            'GF_SECURITY_ADMIN_USER=fixture', 'GF_SECURITY_ADMIN_PASSWORD=offline-fixture-only',
            'GF_ANALYTICS_REPORTING_ENABLED=false', 'GF_ANALYTICS_CHECK_FOR_UPDATES=false',
            'GF_ANALYTICS_CHECK_FOR_PLUGIN_UPDATES=false', 'GF_PLUGINS_PREINSTALL_DISABLED=true',
            'GF_AUTH_ANONYMOUS_ENABLED=false', 'GF_LOG_LEVEL=warn'))
        command('docker', 'cp', str(provisioning) + '/.', grafana + ':/etc/grafana/provisioning/')
        command('docker', 'cp', str(runtime / 'dashboards'), grafana + ':/var/lib/grafana/dashboards')
        command('docker', 'start', fixture, prometheus, grafana)
        for name in (fixture, prometheus, grafana):
            assert not json.loads(command('docker', 'inspect', name))[0]['HostConfig']['PortBindings']
        client = '''import base64,json,sys,urllib.request
request=urllib.request.Request(sys.argv[1])
if sys.argv[2]=='auth':request.add_header('Authorization','Basic '+base64.b64encode(b'fixture:offline-fixture-only').decode())
with urllib.request.urlopen(request,timeout=5) as response:print(json.dumps(json.load(response)))
'''
        def get(url, auth=False):
            return json.loads(command('docker', 'exec', '-i', fixture, 'python3', '-', url,
                                     'auth' if auth else 'anonymous', input=client, timeout=10))
        deadline = time.monotonic() + 90
        while True:
            try:
                observed = get('http://grafana:3000/api/dashboards/uid/worldbuilder-operations', True)
                assert observed['meta']['provisioned'] and observed['meta']['folderTitle'] == 'Worldbuilder'
                assert {p['datasource']['uid'] for p in observed['dashboard']['panels']} == {uid}
                health = get(f'http://grafana:3000/api/datasources/uid/{uid}/health', True)
                assert health['status'] == 'OK', health
                from urllib.parse import urlencode
                proxy = f'http://grafana:3000/api/datasources/proxy/uid/{uid}/api/v1/query?'
                result = get(proxy + urlencode({'query': f'external_ops_total{{namespace="{namespace}",job="worldbuilder-external",method="worldbuilder",state="call"}}'}), True)
                assert len(result['data']['result']) == 1
                assert float(result['data']['result'][0]['value'][1]) == 6
                dropped = get(proxy + urlencode({'query': 'go_goroutines'}), True)
                assert dropped['data']['result'] == [], dropped
                for expr in queries:
                    assert get(proxy + urlencode({'query': expr}), True)['status'] == 'success'
                break
            except (RuntimeError, AssertionError) as error:
                if time.monotonic() > deadline: raise RuntimeError('disposable observability readiness failed') from error
                time.sleep(2)
        try:
            get('http://grafana:3000/api/dashboards/uid/worldbuilder-operations')
        except RuntimeError as error:
            assert '401' in str(error), error
        else: raise AssertionError('anonymous dashboard API unexpectedly succeeded')
        result = {'result': 'passed', 'images': {image: json.loads(command('docker', 'image', 'inspect', image))[0]['RepoDigests']
                   for image in [*images.values(), PYTHON_IMAGE]}, 'namespace': namespace, 'datasourceUID': uid,
                   'checks': ['promql-numeric-scope-and-empty', 'actual-go-metrics', 'metric-filter', 'grafana-file-provisioning', 'datasource-proxy', 'anonymous-refusal']}
        (directory / 'runtime-result.json').write_text(json.dumps(result, indent=2) + '\n')
        print('PASS: actual Go metrics scraped/filtered; provisioned Grafana dashboard and datasource proxy; anonymous API refused', flush=True)
    finally:
        for name in reversed(containers):
            logs = subprocess.run(['docker', 'logs', name], capture_output=True, text=True)
            (runtime / (name + '.log')).write_text(logs.stdout + logs.stderr)
            command('docker', 'rm', '-f', name)
        if created_network: command('docker', 'network', 'rm', network)


if __name__ == '__main__':
    main(*sys.argv[1:])
