#!/usr/bin/env python3
"""Actual PVC/WAL and snapshot recovery in a newly owned offline kind cluster."""
import copy
import hashlib
import importlib.util
import json
from pathlib import Path, PurePosixPath
import re
import sys
import time
from urllib.parse import urlencode

import yaml

HERE = Path(__file__).resolve().parent


def module(name):
    spec = importlib.util.spec_from_file_location(name, HERE / (name + '.py'))
    result = importlib.util.module_from_spec(spec); spec.loader.exec_module(result)
    return result


controllers = module('test-controllers')
snapshot = module('tsdb-snapshot')
Fixture, wait_for = controllers.Fixture, controllers.wait_for
NAMESPACE, PROMETHEUS = controllers.STACK_NAMESPACE, controllers.PROMETHEUS
SELECTOR = 'prometheus=' + PROMETHEUS
RESTORED = 'tsdb-restored'
HISTORY = '{job="tsdb-fixture",__name__=~"external_ops_total|external_processing_duration_seconds_.*|tsdb_fixture_value"}[20m]'


class Recovery:
    def __init__(self, fixture):
        self.f = fixture

    def get(self, resource):
        return json.loads(self.f.kubectl('-n', NAMESPACE, 'get', resource, '-o', 'json'))

    def pods(self, selector):
        return json.loads(self.f.kubectl('-n', NAMESPACE, 'get', 'pods', '-l', selector, '-o', 'json'))['items']

    def matrix(self, service=PROMETHEUS, end=None):
        query = {'query': HISTORY}
        if end is not None: query['time'] = end
        code, out, err = self.f.kubectl('get', '--raw', '/api/v1/namespaces/' + NAMESPACE +
            '/services/http:' + service + ':9090/proxy/api/v1/query?' + urlencode(query), check=False, timeout=30)
        if code: return None
        value = json.loads(out)
        assert value['status'] == 'success' and value['data']['resultType'] == 'matrix', value
        return sorted(value['data']['result'], key=lambda r: json.dumps(r['metric'], sort_keys=True))

    def phase_samples(self, phase):
        data = self.matrix()
        return data is not None and any(r['metric'].get('phase') == phase and len(r['values']) >= 5 for r in data)

    def stop_metrics(self):
        self.f.kubectl('-n', NAMESPACE, 'scale', 'deployment/tsdb-metrics', '--replicas=0')
        self.f.kubectl('-n', NAMESPACE, 'wait', 'pod', '-l', 'fixture=tsdb-metrics', '--for=delete', '--timeout=90s')
        previous, observations = None, 0

        def stable():
            nonlocal previous, observations
            data = self.matrix()
            observations = observations + 1 if data and data == previous else 0
            previous = data
            return observations >= 3

        wait_for('stopped source has quiescent historical samples', stable, 30)

    def volume(self, name):
        pvc = self.get('pvc/' + name)
        pv = self.get('pv/' + pvc['spec']['volumeName'])
        path = pv['spec'].get('local', {}).get('path')
        assert pvc['status']['phase'] == 'Bound' and pv['spec']['claimRef']['uid'] == pvc['metadata']['uid']
        assert path and path.startswith('/var/local-path-provisioner/') and '..' not in PurePosixPath(path).parts
        assert 'hostPath' not in pv['spec']
        return pvc, pv, path

    def loader(self, claim):
        self.f.apply([{'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': 'tsdb-loader', 'namespace': NAMESPACE},
            'spec': {'automountServiceAccountToken': False, 'containers': [{'name': 'loader',
                'image': controllers.PYTHON_IMAGE, 'command': ['sleep', '3600'],
                'volumeMounts': [{'name': 'data', 'mountPath': '/data'}]}],
                'volumes': [{'name': 'data', 'persistentVolumeClaim': {'claimName': claim}}]}}])
        self.f.kubectl('-n', NAMESPACE, 'wait', 'pod/tsdb-loader', '--for=condition=Ready', '--timeout=180s')

    def unload(self):
        self.f.kubectl('-n', NAMESPACE, 'delete', 'pod/tsdb-loader', '--wait=true')

    def load_snapshot(self, data, manifest):
        snapshot.verify_snapshot(data, manifest)
        # Validate the same bounded archive again in the loader before touching the fresh PVC.
        script = (HERE / 'tsdb-snapshot.py').read_text() + '''
import sys
from pathlib import Path
payload = sys.stdin.buffer.read(MAX_ARCHIVE + 1)
expected = json.loads(sys.argv[1])
verify_snapshot(payload, expected)
root = Path('/data')
assert not [p for p in root.iterdir() if p.name != 'lost+found'], 'restore requires an empty PVC'
with tarfile.open(fileobj=io.BytesIO(payload), mode='r:gz') as archive:
    archive.extractall(root, filter='data')
actual = {str(p.relative_to(root)): {'bytes': p.stat().st_size, 'sha256': hashlib.sha256(p.read_bytes()).hexdigest()}
          for p in root.rglob('*') if p.is_file()}
assert actual == expected['files'], 'restored snapshot file mismatch'
print(json.dumps(actual))
'''
        files = json.loads(self.f.kubectl('-n', NAMESPACE, 'exec', '-i', 'tsdb-loader', '--', 'python3', '-c',
            script, json.dumps(manifest), data=data))
        assert files == manifest['files']

    def recover(self, directory, metrics):
        f = self.f
        inputs = yaml.safe_load((Path(directory) / 'storage-upstream-values.yaml').read_text())
        prepared = inputs['prometheus']['prometheusSpec']
        assert set(prepared) == {'enableAdminAPI', 'retention', 'storageSpec'} and prepared['retention'] == '1h'
        assert prepared['enableAdminAPI'] is False and prepared['storageSpec']['volumeClaimTemplate']['spec']['storageClassName'] == 'standard'
        values = controllers.stack_values()
        spec = values['prometheus']['prometheusSpec']; spec.update(copy.deepcopy(prepared))
        # Admin API and local volume annotation are exclusively disposable fixture settings.
        spec['enableAdminAPI'] = True
        claim = spec['storageSpec']['volumeClaimTemplate']
        claim['metadata'] = {'annotations': {'volumeType': 'local'}}
        spec['additionalScrapeConfigs'] = [{'job_name': 'tsdb-fixture', 'scrape_interval': '1s',
            'static_configs': [{'targets': ['tsdb-metrics.' + NAMESPACE + '.svc:8080']}]}]
        f.apply([{'apiVersion': 'v1', 'kind': 'Namespace', 'metadata': {'name': NAMESPACE}}])
        controllers.install_stack(f, controllers.render_stack(f, values))
        wait_for('Prometheus storage Pod exists', lambda: bool(self.pods(SELECTOR)))
        f.kubectl('-n', NAMESPACE, 'wait', 'pod', '-l', SELECTOR, '--for=condition=Ready', '--timeout=240s')
        source = self.pods(SELECTOR)[0]
        container = next(c for c in source['spec']['containers'] if c['name'] == 'prometheus')
        mount = next(v for v in container['volumeMounts'] if v['mountPath'] == '/prometheus')
        claimname = next(v['persistentVolumeClaim']['claimName'] for v in source['spec']['volumes'] if v['name'] == mount['name'])
        assert mount.get('subPath') == 'prometheus-db', mount
        original, pv, path = self.volume(claimname)
        data_path = path + '/' + mount['subPath']
        image = next(c['imageID'] for c in source['status']['containerStatuses'] if c['name'] == 'prometheus')
        assert re.fullmatch(r'quay.io/prometheus/prometheus@sha256:[0-9a-f]{64}', image), image
        self.metrics(metrics)
        wait_for('actual Go metrics and phase samples are stored', lambda: self.phase_samples('before'))
        self.stop_metrics()
        end = time.time(); before = self.matrix(end=end)
        assert len(before) > 3 and all(len(row['values']) >= 5 for row in before)
        assert any(r['metric']['__name__'] == 'external_ops_total' and r['metric'].get('method') == 'worldbuilder'
                   and r['metric'].get('state') == 'call' and all(float(v[1]) == 6 for v in r['values']) for r in before)
        # Prove these are head/WAL samples, not already compacted historical blocks.
        code, listing, _ = f.execute(['find', data_path, '-maxdepth', '2', '-type', 'f'])
        names = listing.decode().splitlines()
        assert any('/wal/' in n for n in names) and not any(n.endswith('/meta.json') for n in names), names
        source = self.pods(SELECTOR)[0]
        f.kubectl('-n', NAMESPACE, 'delete', 'pod', source['metadata']['name'], '--wait=true')
        wait_for('source Pod replacement has a new UID', lambda: bool(self.pods(SELECTOR)) and
                 self.pods(SELECTOR)[0]['metadata']['uid'] != source['metadata']['uid'])
        f.kubectl('-n', NAMESPACE, 'wait', 'pod', '-l', SELECTOR, '--for=condition=Ready', '--timeout=240s')
        wait_for('same-PVC WAL recovery preserves exact sample history', lambda: self.matrix(end=end) == before)
        assert self.volume(claimname)[0]['metadata']['uid'] == original['metadata']['uid']
        pod = self.pods(SELECTOR)[0]['metadata']['name']
        wal_log = f.kubectl('-n', NAMESPACE, 'logs', pod, '-c', 'prometheus')
        assert 'WAL replay completed' in wal_log
        (f.output / 'source-wal.log').write_text(wal_log)
        service = self.get('service/' + PROMETHEUS)
        url = 'http://' + service['spec']['clusterIP'] + ':9090/api/v1/admin/tsdb/snapshot?skip_head=false'
        _, output, _ = f.execute(['curl', '--fail', '--silent', '--show-error', '--max-time', '30', '-X', 'POST', url])
        response = json.loads(output); assert response['status'] == 'success'
        name = response['data']['name']; assert re.fullmatch(r'[0-9A-Za-z-]+', name)
        _, archive, _ = f.execute(['tar', '-C', data_path + '/snapshots/' + name, '-czf', '-', '.'], timeout=60)
        manifest = snapshot.inspect_snapshot(archive)
        (f.output / 'snapshot-manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
        print('PASS: real snapshot includes head samples and has complete block/index/chunk manifest', flush=True)
        f.kubectl('-n', NAMESPACE, 'set', 'env', 'deployment/tsdb-metrics', 'PHASE=after')
        f.kubectl('-n', NAMESPACE, 'scale', 'deployment/tsdb-metrics', '--replicas=1')
        wait_for('source contains samples written after the snapshot', lambda: self.phase_samples('after'))
        self.stop_metrics()
        f.kubectl('-n', NAMESPACE, 'delete', 'deployment/tsdb-metrics', 'service/tsdb-metrics', '--wait=true')
        f.kubectl('-n', NAMESPACE, 'delete', 'prometheus/' + PROMETHEUS, '--wait=true')
        f.kubectl('-n', NAMESPACE, 'wait', 'pod', '-l', SELECTOR, '--for=delete', '--timeout=90s')
        f.kubectl('-n', NAMESPACE, 'delete', 'pvc/' + claimname, '--wait=true')
        f.kubectl('wait', 'pv/' + pv['metadata']['name'], '--for=delete', '--timeout=120s')
        assert f.execute(['test', '-e', path], check=False)[0] == 1
        print('PASS: source workload, PVC, PV and original data path removed', flush=True)
        f.apply([{'apiVersion': 'v1', 'kind': 'PersistentVolumeClaim',
            'metadata': {'name': RESTORED, 'namespace': NAMESPACE, **claim['metadata']}, 'spec': claim['spec']}])
        self.loader(RESTORED)
        restored, newpv, newpath = self.volume(RESTORED)
        assert newpath != path and restored['metadata']['uid'] != original['metadata']['uid'] and newpv['metadata']['uid'] != pv['metadata']['uid']
        self.load_snapshot(archive, manifest); self.unload()
        self.restored_resources(image)
        self.start_restored()
        wait_for('restored labels, timestamps and values exactly match the original history',
                 lambda: self.matrix(RESTORED, end) == before)
        assert self.matrix(RESTORED) == before, 'snapshot unexpectedly contains later samples'
        # The restored server has no scrape jobs or admin API, so it cannot regenerate history.
        restored_service = self.get('service/' + RESTORED)
        admin_url = 'http://' + restored_service['spec']['clusterIP'] + ':9090/api/v1/admin/tsdb/snapshot'
        _, output, _ = f.execute(['curl', '--silent', '--show-error', '--max-time', '10', '-X', 'POST', admin_url])
        refusal = json.loads(output); assert refusal['status'] == 'error' and 'disabled' in refusal['error']
        # Corrupt an actual restored block and require Prometheus itself to reject it.
        f.kubectl('-n', NAMESPACE, 'delete', 'pod/' + RESTORED, '--wait=true')
        self.loader(RESTORED)
        index = next(name for name in manifest['files'] if name.endswith('/index'))
        f.kubectl('-n', NAMESPACE, 'exec', 'tsdb-loader', '--', 'python3', '-c',
            'import sys; from pathlib import Path; p=Path("/data")/sys.argv[1]; b=p.read_bytes(); p.write_bytes(b"BAD!"+b[4:])', index)
        self.unload(); f.apply([self.restored_pod])

        def corrupt_refused():
            states = self.get('pod/' + RESTORED).get('status', {}).get('containerStatuses', [])
            return any(s.get('state', {}).get('terminated', {}).get('exitCode', 0) != 0 for s in states)

        wait_for('Prometheus refuses a corrupt snapshot index', corrupt_refused, 90)
        error = f.kubectl('-n', NAMESPACE, 'logs', 'pod/' + RESTORED)
        assert 'invalid magic' in error.lower(), error[-3000:]
        (f.output / 'corrupt-index.log').write_text(error)
        f.kubectl('-n', NAMESPACE, 'delete', 'pod/' + RESTORED, '--wait=true')
        # A fresh PVC prevents the failed attempt's WAL or runtime files from contributing to recovery.
        f.kubectl('-n', NAMESPACE, 'delete', 'pvc/' + RESTORED, '--wait=true')
        f.kubectl('wait', 'pv/' + newpv['metadata']['name'], '--for=delete', '--timeout=120s')
        assert f.execute(['test', '-e', newpath], check=False)[0] == 1
        f.apply([{'apiVersion': 'v1', 'kind': 'PersistentVolumeClaim',
            'metadata': {'name': RESTORED, 'namespace': NAMESPACE, **claim['metadata']}, 'spec': claim['spec']}])
        self.loader(RESTORED)
        repaired, repaired_pv, repaired_path = self.volume(RESTORED)
        assert repaired['metadata']['uid'] not in (original['metadata']['uid'], restored['metadata']['uid'])
        assert repaired_path not in (path, newpath) and repaired_pv['metadata']['uid'] != newpv['metadata']['uid']
        self.load_snapshot(archive, manifest); self.unload(); self.start_restored()
        wait_for('known snapshot recovers after corrupt-data refusal', lambda: self.matrix(RESTORED) == before)
        assert self.get('pod/' + RESTORED)['status']['containerStatuses'][0]['imageID'] == image
        allpods = json.loads(f.kubectl('get', 'pods', '-A', '-o', 'json'))['items']
        images = sorted({c['imageID'] for p in allpods for c in p.get('status', {}).get('containerStatuses', []) if c.get('imageID')})
        (f.output / 'tsdb-result.json').write_text(json.dumps({'status': 'passed', 'stackVersion': controllers.STACK_VERSION,
            'observedImageIDs': images, 'sourcePVC': original['metadata']['uid'], 'restoredPVC': restored['metadata']['uid'],
            'repairedPVC': repaired['metadata']['uid'], 'prometheusImage': image, 'snapshot': name, 'snapshotSHA256': hashlib.sha256(archive).hexdigest(), 'history': before,
            'tests': ['source-rendered PVC/retention', 'actual Go metrics', 'quiescent fixed-window history',
                      'same-PVC WAL replay', 'snapshot including head', 'source data removal', 'distinct-volume restore',
                      'file hashes and exact samples', 'later samples absent', 'disabled restored admin API',
                      'actual corrupt-index refusal and clean recovery'],
            'exclusions': ['production storage/backup readiness', 'hardware or node loss', 'Longhorn durability',
                           'capacity/retention sizing', 'cross-version compatibility', 'scheduled backup transport']}, indent=2) + '\n')

    def metrics(self, filename):
        metrics = Path(filename).read_text()
        assert 'external_ops_total{method="worldbuilder",state="call"} 6' in metrics
        serve = '''from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
import os
class Handler(BaseHTTPRequestHandler):
 def do_GET(self):
  if self.path != '/metrics': self.send_error(404); return
  self.send_response(200); self.send_header('Content-Type','text/plain; version=0.0.4'); self.end_headers()
  self.wfile.write(Path('/fixture/metrics.txt').read_bytes())
  self.wfile.write(('\\ntsdb_fixture_value{phase="'+os.environ['PHASE']+'"} 7\\n').encode())
 def log_message(self,*args): pass
HTTPServer(('0.0.0.0',8080),Handler).serve_forever()
'''
        self.f.apply([{'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {'name': 'tsdb-metrics', 'namespace': NAMESPACE},
            'data': {'metrics.txt': metrics, 'serve.py': serve}},
            {'apiVersion': 'apps/v1', 'kind': 'Deployment', 'metadata': {'name': 'tsdb-metrics', 'namespace': NAMESPACE},
             'spec': {'replicas': 1, 'selector': {'matchLabels': {'fixture': 'tsdb-metrics'}}, 'template': {
                'metadata': {'labels': {'fixture': 'tsdb-metrics'}}, 'spec': {'automountServiceAccountToken': False,
                    'containers': [{'name': 'metrics', 'image': controllers.PYTHON_IMAGE,
                        'command': ['python3', '/fixture/serve.py'], 'env': [{'name': 'PHASE', 'value': 'before'}],
                        'volumeMounts': [{'name': 'fixture', 'mountPath': '/fixture', 'readOnly': True}]}],
                    'volumes': [{'name': 'fixture', 'configMap': {'name': 'tsdb-metrics'}}]}}}},
            {'apiVersion': 'v1', 'kind': 'Service', 'metadata': {'name': 'tsdb-metrics', 'namespace': NAMESPACE},
             'spec': {'selector': {'fixture': 'tsdb-metrics'}, 'ports': [{'port': 8080}]}}])

    def restored_resources(self, image):
        self.restored_pod = {'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': RESTORED, 'namespace': NAMESPACE,
            'labels': {'fixture': RESTORED}}, 'spec': {'restartPolicy': 'Never', 'automountServiceAccountToken': False,
                'securityContext': {'runAsUser': 65534, 'runAsGroup': 65534, 'fsGroup': 65534}, 'containers': [{
                    'name': 'prometheus', 'image': image, 'args': ['--config.file=/config/prometheus.yml',
                        '--storage.tsdb.path=/prometheus', '--storage.tsdb.retention.time=1h'],
                    'volumeMounts': [{'name': 'data', 'mountPath': '/prometheus'}, {'name': 'config', 'mountPath': '/config'}],
                    'readinessProbe': {'httpGet': {'path': '/-/ready', 'port': 9090}, 'periodSeconds': 2}}],
                'volumes': [{'name': 'data', 'persistentVolumeClaim': {'claimName': RESTORED}},
                            {'name': 'config', 'configMap': {'name': 'restored-config'}}]}}
        self.f.apply([{'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': {'name': 'restored-config', 'namespace': NAMESPACE},
            'data': {'prometheus.yml': 'scrape_configs: []\n'}},
            {'apiVersion': 'v1', 'kind': 'Service', 'metadata': {'name': RESTORED, 'namespace': NAMESPACE},
             'spec': {'selector': {'fixture': RESTORED}, 'ports': [{'name': 'http', 'port': 9090}]}}])

    def start_restored(self):
        self.f.apply([self.restored_pod])
        self.f.kubectl('-n', NAMESPACE, 'wait', 'pod/' + RESTORED, '--for=condition=Ready', '--timeout=180s')


def main():
    if len(sys.argv) != 4: raise SystemExit('Usage: test-tsdb.py OUTPUT_DIRECTORY OBSERVABILITY_FIXTURES GO_METRICS')
    f = Fixture(sys.argv[1])
    try:
        f.create(); Recovery(f).recover(sys.argv[2], sys.argv[3])
    finally:
        try:
            for name, args in [('pods', ['get', 'pods', '-A', '-o', 'wide']),
                               ('events', ['get', 'events', '-A', '--sort-by=.lastTimestamp']),
                               ('restored', ['-n', NAMESPACE, 'logs', 'pod/' + RESTORED]),
                               ('volumes', ['get', 'pv,pvc', '-A', '-o', 'json'])]:
                _, out, err = f.kubectl(*args, check=False, timeout=30)
                (f.output / (name + '.log')).write_text(out + err)
        finally: f.close()
    print('PASS: persistent TSDB recovery and owned fixture teardown', flush=True)


if __name__ == '__main__': main()
