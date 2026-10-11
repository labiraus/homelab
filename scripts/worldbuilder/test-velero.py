#!/usr/bin/env python3
"""Real disposable Velero/Kopia restore on external SeaweedFS; no LAN context."""
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import secrets
import subprocess
import sys

import yaml

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
NAMESPACE = 'fixture-velero'
SOURCE, TARGET, DENIED = 'fixture-data', 'fixture-restored', 'fixture-denied'
CHART_VERSION = '12.2.1'
PLUGIN_IMAGE = 'velero/velero-plugin-for-aws:v1.14.4'
PYTHON_IMAGE = 'python:3.12.12-alpine3.22'


def module(name):
    spec = importlib.util.spec_from_file_location(name, HERE / (name + '.py'))
    result = importlib.util.module_from_spec(spec); spec.loader.exec_module(result)
    return result


common = module('kind-fixture')
Storage = module('s3-fixture').Storage


def get(f, resource, namespace=NAMESPACE):
    return json.loads(f.kubectl('-n', namespace, 'get', resource, '-o', 'json'))


def completed(obj):
    status = obj.get('status', {})
    phase = status.get('phase')
    if phase in ('Failed', 'PartiallyFailed', 'FailedValidation'):
        raise AssertionError((obj['kind'], obj['metadata']['name'], status))
    if phase != 'Completed': return False
    assert not status.get('errors') and not status.get('warnings'), status
    return True


def volume(f, namespace):
    pvc = get(f, 'pvc/fixture-files', namespace)
    pv = get(f, 'pv/' + pvc['spec']['volumeName'])
    assert pv['spec'].get('local', {}).get('path') and 'hostPath' not in pv['spec'], pv['spec']
    assert pv['spec']['claimRef']['uid'] == pvc['metadata']['uid']
    return pvc, pv


def credential_refused(obj):
    status = obj.get('status', {})
    assert status.get('phase') != 'Completed', status
    if status.get('phase') not in ('Failed', 'FailedValidation'): return False
    reason = ' '.join(status.get('validationErrors', [])) + ' ' + status.get('failureReason', '')
    assert re.search('BSL default is unavailable|403|AccessDenied|Forbidden|InvalidAccessKeyId', reason), status
    return True


def app_pod(f, namespace):
    items = json.loads(f.kubectl('-n', namespace, 'get', 'pods', '-l', 'fixture=files', '-o', 'json'))['items']
    assert len(items) == 1, (namespace, len(items))
    return items[0]['metadata']['name']


def files(f, namespace, operation):
    output = f.kubectl('-n', namespace, 'exec', '-i', app_pod(f, namespace), '-c', 'files', '--',
        'python3', '-', operation, data=(HERE / 'velero-files.py').read_bytes())
    return json.loads(output) if operation == 'manifest' else None


def resource_state(f, namespace):
    """Compare portable application content, including mounted credentials and RBAC."""
    fields = {'configmap/fixture-config': ('data', 'binaryData', 'immutable'),
              'secret/fixture-secret': ('data', 'type', 'immutable'),
              'serviceaccount/fixture-reader': ('automountServiceAccountToken', 'imagePullSecrets'),
              'role/fixture-reader': ('rules',), 'rolebinding/fixture-reader': ('roleRef', 'subjects'),
              'deployment/fixture-files': ('spec',), 'service/fixture-files': ('spec',),
              'pvc/fixture-files': ('spec',)}
    result = {}
    for resource, keys in fields.items():
        obj = get(f, resource, namespace)
        value = {key: obj[key] for key in keys if key in obj}
        if resource.startswith('service/'):
            for key in ('clusterIP', 'clusterIPs'): value['spec'].pop(key, None)
        if resource.startswith('pvc/'): value['spec'].pop('volumeName', None)
        if resource.startswith('rolebinding/'):
            for subject in value['subjects']:
                assert subject['namespace'] == namespace
                subject['namespace'] = '<restored-namespace>'
        result[resource] = value
    return result


def install(f, storage):
    values = yaml.safe_load((ROOT / 'values/velero-seaweedfs-values.yaml').read_text())
    assert values['deployNodeAgent'] is True and values['snapshotsEnabled'] is False
    location = values['configuration']['backupStorageLocation']
    assert len(location) == 1 and location[0]['bucket'] == storage.bucket == 'velero'
    assert location[0]['provider'] == 'aws' and location[0]['config']['s3ForcePathStyle'] == 'true'
    assert location[0]['config']['insecureSkipTLSVerify'] == 'false'
    values['credentials']['existingSecret'] = 'fixture-cloud'
    location[0]['config']['s3Url'] = storage.endpoint
    location[0]['validationFrequency'] = '3s'
    values['initContainers'] = [{'name': 'aws-plugin', 'image': PLUGIN_IMAGE,
        'volumeMounts': [{'mountPath': '/target', 'name': 'plugins'}]}]
    # Original chart CRDs are applied explicitly below; Helm hook Jobs are not simulated.
    values['upgradeCRDs'] = False
    f.apply([{'apiVersion': 'v1', 'kind': 'Namespace', 'metadata': {'name': NAMESPACE}}])
    credentials = '[default]\naws_access_key_id=fixture-velero\naws_secret_access_key=' + storage.credentials['velero'] + '\n'
    for name, data in [('fixture-cloud', {'cloud': credentials}),
                       ('fixture-denied-cloud', {'cloud': '[default]\naws_access_key_id=invalid\naws_secret_access_key=invalid\n'}),
                       ('velero-repo-credentials', {'repository-password': secrets.token_hex(32)})]:
        f.apply([{'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {'name': name, 'namespace': NAMESPACE}, 'stringData': data}])
    path = f.output / 'velero-values.yaml'; path.write_text(yaml.safe_dump(values))
    render = subprocess.check_output(['helm', 'template', 'velero', 'velero', '--repo',
        'https://vmware-tanzu.github.io/helm-charts', '--version', CHART_VERSION, '--include-crds',
        '-n', NAMESPACE, '-f', str(path)], env=f.helm_env)
    (f.output / 'velero.yaml').write_bytes(render)
    documents = [d for d in yaml.safe_load_all(render) if d]
    crds = [d for d in documents if d['kind'] == 'CustomResourceDefinition']
    assert crds and not any(d['kind'] == 'VolumeSnapshotLocation' for d in documents)
    f.apply(crds)
    for d in crds: f.kubectl('wait', 'crd/' + d['metadata']['name'], '--for=condition=Established', '--timeout=60s')
    f.apply([d for d in documents if d['kind'] != 'CustomResourceDefinition'])
    for resource in ('deployment/velero', 'daemonset/node-agent'):
        f.kubectl('-n', NAMESPACE, 'rollout', 'status', resource, '--timeout=240s')
    common.wait_for('Velero/node-agent Ready and authenticated backup storage Available', lambda:
        get(f, 'backupstoragelocation/default').get('status', {}).get('phase') == 'Available')


def seed(f):
    def metadata(name): return {'name': name, 'namespace': SOURCE}
    f.apply([{'apiVersion': 'v1', 'kind': 'Namespace', 'metadata': {'name': SOURCE}},
        {'apiVersion': 'v1', 'kind': 'ConfigMap', 'metadata': metadata('fixture-config'),
         'immutable': True, 'data': {'note': 'Preserve configuration Ω'}},
        {'apiVersion': 'v1', 'kind': 'Secret', 'metadata': metadata('fixture-secret'),
         'immutable': True, 'stringData': {'synthetic': 'fixture-only-no-real-credentials'}},
        {'apiVersion': 'v1', 'kind': 'ServiceAccount', 'metadata': metadata('fixture-reader'),
         'automountServiceAccountToken': False},
        {'apiVersion': 'rbac.authorization.k8s.io/v1', 'kind': 'Role', 'metadata': metadata('fixture-reader'),
         'rules': [{'apiGroups': [''], 'resources': ['configmaps'], 'resourceNames': ['fixture-config'], 'verbs': ['get']}]},
        {'apiVersion': 'rbac.authorization.k8s.io/v1', 'kind': 'RoleBinding', 'metadata': metadata('fixture-reader'),
         'roleRef': {'apiGroup': 'rbac.authorization.k8s.io', 'kind': 'Role', 'name': 'fixture-reader'},
         'subjects': [{'kind': 'ServiceAccount', 'name': 'fixture-reader', 'namespace': SOURCE}]},
        {'apiVersion': 'v1', 'kind': 'PersistentVolumeClaim', 'metadata': {
            **metadata('fixture-files'), 'annotations': {'volumeType': 'local'}},
         'spec': {'storageClassName': 'standard', 'accessModes': ['ReadWriteOnce'], 'resources': {'requests': {'storage': '1Gi'}}}},
        {'apiVersion': 'apps/v1', 'kind': 'Deployment', 'metadata': metadata('fixture-files'),
         'spec': {'replicas': 1, 'selector': {'matchLabels': {'fixture': 'files'}}, 'template': {
            'metadata': {'labels': {'fixture': 'files'}}, 'spec': {'serviceAccountName': 'fixture-reader',
                'automountServiceAccountToken': False, 'containers': [{'name': 'files', 'image': PYTHON_IMAGE,
                    # Startup never recreates the payload: it must come from the backup.
                    'command': ['python3', '-m', 'http.server', '8080', '--directory', '/data'],
                    'readinessProbe': {'httpGet': {'path': '/', 'port': 8080}, 'periodSeconds': 2},
                    'volumeMounts': [{'name': 'data', 'mountPath': '/data'},
                                     {'name': 'config', 'mountPath': '/config', 'readOnly': True},
                                     {'name': 'secret', 'mountPath': '/secret', 'readOnly': True}]}],
                'volumes': [{'name': 'data', 'persistentVolumeClaim': {'claimName': 'fixture-files'}},
                            {'name': 'config', 'configMap': {'name': 'fixture-config'}},
                            {'name': 'secret', 'secret': {'secretName': 'fixture-secret'}}]}}}},
        {'apiVersion': 'v1', 'kind': 'Service', 'metadata': metadata('fixture-files'), 'spec': {
            'selector': {'fixture': 'files'}, 'ports': [{'name': 'http', 'port': 8080, 'targetPort': 8080}]}}])
    f.kubectl('-n', SOURCE, 'rollout', 'status', 'deployment/fixture-files', '--timeout=180s')
    # The destination namespace must keep its freshly generated cluster CA.
    f.kubectl('-n', SOURCE, 'label', 'configmap/kube-root-ca.crt', 'velero.io/exclude-from-backup=true')
    files(f, SOURCE, 'seed')
    manifest = files(f, SOURCE, 'manifest')
    assert manifest['payload.bin']['bytes'] == 16 * 1024 * 1024 and len(manifest) == 8
    return manifest, resource_state(f, SOURCE), volume(f, SOURCE)


def restore_spec(name, target):
    if name not in ('fixture-restore', 'fixture-denied') or target not in (TARGET, DENIED):
        raise ValueError('Unknown disposable restore')
    return {'apiVersion': 'velero.io/v1', 'kind': 'Restore', 'metadata': {'name': name, 'namespace': NAMESPACE},
            'spec': {'backupName': 'fixture-full', 'namespaceMapping': {SOURCE: target}}}


def restart(f):
    for selector, resource in [('name=velero', 'deployment/velero'), ('name=node-agent', 'daemonset/node-agent')]:
        before = json.loads(f.kubectl('-n', NAMESPACE, 'get', 'pods', '-l', selector, '-o', 'json'))['items']
        assert len(before) == 1, (selector, len(before))
        uid = before[0]['metadata']['uid']
        f.kubectl('-n', NAMESPACE, 'delete', 'pod/' + before[0]['metadata']['name'], '--wait=true')
        f.kubectl('-n', NAMESPACE, 'rollout', 'status', resource, '--timeout=180s')
        def ready():
            pods = json.loads(f.kubectl('-n', NAMESPACE, 'get', 'pods', '-l', selector, '-o', 'json'))['items']
            return len(pods) == 1 and pods[0]['metadata']['uid'] != uid and any(
                c['type'] == 'Ready' and c['status'] == 'True' for c in pods[0].get('status', {}).get('conditions', []))
        common.wait_for(resource + ' has a new Ready Pod', ready)


def credential_ref(f, name):
    f.kubectl('-n', NAMESPACE, 'patch', 'backupstoragelocation/default', '--type=merge', '-p',
              json.dumps({'spec': {'credential': {'name': name, 'key': 'cloud'}}}))


def recovery(f):
    manifest, resources, (pvc, pv) = seed(f)
    f.apply([{'apiVersion': 'velero.io/v1', 'kind': 'Backup', 'metadata': {'name': 'fixture-full', 'namespace': NAMESPACE},
              'spec': {'includedNamespaces': [SOURCE], 'storageLocation': 'default',
                       'defaultVolumesToFsBackup': True, 'snapshotVolumes': False, 'ttl': '1h0m0s'}}])
    common.wait_for('Velero Backup completes without errors or warnings', lambda: completed(get(f, 'backup/fixture-full')), 360)
    pvbs = get(f, 'podvolumebackups')['items']
    assert len(pvbs) == 1 and completed(pvbs[0]), pvbs
    assert pvbs[0]['spec']['volume'] == 'data' and pvbs[0]['spec']['pod']['namespace'] == SOURCE
    assert pvbs[0]['status'].get('snapshotID'), pvbs[0]['status']
    f.kubectl('delete', 'namespace/' + SOURCE, '--wait=true', '--timeout=120s')
    f.kubectl('wait', 'pv/' + pv['metadata']['name'], '--for=delete', '--timeout=120s')
    assert f.execute(['test', '-e', pv['spec']['local']['path']], check=False)[0] == 1
    print('PASS: source namespace, PVC/PV and original data directory are absent', flush=True)

    credential_ref(f, 'fixture-denied-cloud')
    def unavailable():
        status = get(f, 'backupstoragelocation/default').get('status', {})
        return status.get('phase') == 'Unavailable' and bool(re.search('403|AccessDenied|Forbidden|InvalidAccessKeyId', status.get('message', '')))
    common.wait_for('bad credentials make backup storage Unavailable with an S3 denial', unavailable)
    f.apply([restore_spec('fixture-denied', DENIED)])
    common.wait_for('restore with bad S3 credentials fails', lambda:
        credential_refused(get(f, 'restore/fixture-denied')))
    assert not json.loads(f.kubectl('-n', DENIED, 'get', 'pods,pvc', '-o', 'json'))['items']
    credential_ref(f, 'fixture-cloud')
    restart(f)
    common.wait_for('backup storage is Available after controller/node-agent replacement', lambda:
        get(f, 'backupstoragelocation/default').get('status', {}).get('phase') == 'Available')
    f.apply([restore_spec('fixture-restore', TARGET)])
    common.wait_for('mapped namespace restore completes without errors or warnings', lambda:
        completed(get(f, 'restore/fixture-restore')), 360)
    f.kubectl('-n', TARGET, 'rollout', 'status', 'deployment/fixture-files', '--timeout=180s')
    pvrs = get(f, 'podvolumerestores')['items']
    assert len(pvrs) == 1 and completed(pvrs[0]), pvrs
    assert pvrs[0]['spec']['volume'] == 'data' and pvrs[0]['spec']['pod']['namespace'] == TARGET
    newpvc, newpv = volume(f, TARGET)
    assert newpvc['metadata']['uid'] != pvc['metadata']['uid']
    assert newpv['metadata']['uid'] != pv['metadata']['uid'] and newpv['spec']['local']['path'] != pv['spec']['local']['path']
    actual = files(f, TARGET, 'manifest')
    assert actual == manifest, (actual, manifest)
    assert resource_state(f, TARGET) == resources, 'Restored application resources differ'
    pod = app_pod(f, TARGET)
    assert f.kubectl('-n', TARGET, 'exec', pod, '-c', 'files', '--', 'cat', '/config/note') == 'Preserve configuration Ω'
    assert f.kubectl('-n', TARGET, 'exec', pod, '-c', 'files', '--', 'cat', '/secret/synthetic') == 'fixture-only-no-real-credentials'
    actor = 'system:serviceaccount:' + TARGET + ':fixture-reader'
    assert f.kubectl('-n', TARGET, 'auth', 'can-i', 'get', 'configmap/fixture-config', '--as', actor).strip() == 'yes'
    for namespace, resource in [(TARGET, 'secret/fixture-secret'), (SOURCE, 'configmap/fixture-config')]:
        code, out, _ = f.kubectl('-n', namespace, 'auth', 'can-i', 'get', resource, '--as', actor, check=False)
        assert code and out.strip() == 'no'
    address = get(f, 'service/fixture-files', TARGET)['spec']['clusterIP']
    def serving():
        code, out, _ = f.execute(['curl', '--fail', '--silent', '--connect-timeout', '2', '--max-time', '3',
                                  'http://' + address + ':8080/readme.txt'], check=False)
        return code == 0 and out.decode() == 'Restored fixture Ω\nSecond line\n'
    common.wait_for('restored Service serves the expected file', serving, 30)
    pods = json.loads(f.kubectl('get', 'pods', '-A', '-o', 'json'))['items']
    images = sorted({c['imageID'] for p in pods for c in p.get('status', {}).get('containerStatuses', []) +
                     p.get('status', {}).get('initContainerStatuses', []) if c.get('imageID')})
    (f.output / 'velero-result.json').write_text(json.dumps({'status': 'passed', 'chartVersion': CHART_VERSION,
        'pluginImage': PLUGIN_IMAGE, 'observedImageIDs': images, 'files': actual,
        'resourcesSHA256': hashlib.sha256(json.dumps(resources, sort_keys=True).encode()).hexdigest(),
        'sourceRemoved': True, 'invalidCredentialsRefused': True, 'controllerReplacement': True,
        'exclusions': ['production backup/data coverage', 'CSI/Longhorn snapshots', 'power-loss durability',
                       'live database file consistency', 'mesh enforcement']}, indent=2) + '\n')
    print('PASS: file contents/metadata, mounted configuration/Secrets, mapped RBAC and Service recovered on distinct storage', flush=True)


def main():
    if len(sys.argv) != 2: raise SystemExit('Usage: test-velero.py OUTPUT_DIRECTORY')
    f = common.Fixture(sys.argv[1]); storage = Storage(f, consumer='velero')
    try:
        f.create(); storage.create(); install(f, storage); recovery(f)
    finally:
        try:
            for name, args in [('pods', ['get', 'pods', '-A', '-o', 'wide']),
                              ('events', ['get', 'events', '-A', '--sort-by=.lastTimestamp']),
                              ('velero-state', ['-n', NAMESPACE, 'get', 'backups,restores,podvolumebackups,podvolumerestores,backupstoragelocations', '-o', 'json']),
                              ('velero-controller', ['-n', NAMESPACE, 'logs', 'deployment/velero', '--tail=120']),
                              ('velero-node-agent', ['-n', NAMESPACE, 'logs', 'daemonset/node-agent', '--tail=120']),
                              ('backup', ['-n', NAMESPACE, 'exec', 'deployment/velero', '--', '/velero', '--namespace', NAMESPACE, 'backup', 'logs', 'fixture-full']),
                              ('restore', ['-n', NAMESPACE, 'exec', 'deployment/velero', '--', '/velero', '--namespace', NAMESPACE, 'restore', 'logs', 'fixture-restore'])]:
                _, out, err = f.kubectl(*args, check=False, timeout=30)
                (f.output / (name + '.log')).write_text(out + err)
        finally:
            try: storage.close()
            finally: f.close()
    print('PASS: Velero file/resource recovery and owned fixture teardown', flush=True)


if __name__ == '__main__': main()
