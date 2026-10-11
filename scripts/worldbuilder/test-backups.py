#!/usr/bin/env python3
"""Disposable CNPG base-backup/WAL/PITR acceptance against external SeaweedFS."""
import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import subprocess
import sys
import time

import yaml

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
NAMESPACE = 'fixture-postgres'
CHART_VERSION = '0.23.2'
PORTS = {'offline-source': 30432, 'offline-latest': 30433, 'offline-pitr': 30434}
TABLES = ('world', 'member', 'revision', 'record', 'record_identity', 'change_set',
          'change_review', 'audit', 'outbox', 'story_session', 'story_turn', 'story_outcome',
          'story_definition', 'generation_job', 'generation_outbox', 'inference_attempt', 'schema_version', 'superuser', 'account_audit')


def module(name):
    spec = importlib.util.spec_from_file_location(name, HERE / (name + '.py'))
    result = importlib.util.module_from_spec(spec); spec.loader.exec_module(result)
    return result


common = module('kind-fixture')


Storage = module('s3-fixture').Storage


class Database:
    def __init__(self, fixture, cluster, password):
        if cluster not in PORTS: raise ValueError('Unknown disposable database')
        self.f, self.cluster = fixture, cluster
        service = json.loads(fixture.kubectl('-n', NAMESPACE, 'get', 'service/' + cluster + '-rw', '-o', 'json'))
        fixture.apply([{'apiVersion': 'v1', 'kind': 'Service',
            'metadata': {'name': cluster + '-test', 'namespace': NAMESPACE},
            'spec': {'type': 'NodePort', 'selector': service['spec']['selector'],
                     'ports': [{'name': 'postgres', 'port': 5432, 'targetPort': 5432, 'nodePort': PORTS[cluster]}]}}])
        info = fixture.docker.api('GET', '/containers/' + fixture.node + '/json')
        bindings = info['NetworkSettings']['Ports'].get(str(PORTS[cluster]) + '/tcp') or []
        if getattr(fixture, 'ports', ()):
            assert len(bindings) == 1, 'Missing owned database port mapping'
        self.local = False
        self.env = {k: v for k, v in os.environ.items() if not k.startswith('PG')}
        self.env.update(PGPASSWORD=password, PGSSLMODE='require', PGCONNECT_TIMEOUT='2', LC_ALL='C',
            PGGSSENCMODE='disable', PGSSLCERTMODE='disable',
            PGSSLCERT=str(fixture.private / 'unused-client.crt'), PGSSLKEY=str(fixture.private / 'unused-client.key'),
            PGSSLROOTCERT=str(fixture.private / 'unused-root.crt'), PGSSLCRL=str(fixture.private / 'unused-root.crl'))
        self.args = ['psql', '-X', '-qAt', '-v', 'ON_ERROR_STOP=1', '-U', 'postgres', '-d', 'app']
        if shutil.which('psql') and len(bindings) == 1:
            assert bindings[0]['HostIp'] == '127.0.0.1'
            self.args += ['-h', '127.0.0.1', '-p', bindings[0]['HostPort']]
            for attempt in range(10):
                probe = subprocess.run(self.args + ['-c', 'SELECT 1'], env=self.env,
                                       capture_output=True, text=True, timeout=5)
                if probe.returncode == 0:
                    assert probe.stdout.strip() == '1'
                    self.local = True
                    break
                # Docker's listener can accept before kube-proxy installs a new
                # NodePort route. Retry a preflight EOF, but never hide a
                # persistent closed connection behind peer authentication.
                if 'server closed the connection unexpectedly' in probe.stderr:
                    if attempt == 9:
                        raise RuntimeError('Local psql connection kept closing: ' + probe.stderr[-1000:])
                    time.sleep(1)
                    continue
                if not re.search('connection refused|timeout expired|connection timed out|no route to host', probe.stderr, re.I):
                    raise RuntimeError('Local psql failed beyond a transport error: ' + probe.stderr[-1000:])
                if attempt >= 4: break
                time.sleep(0.5)
        print('Database transport for ' + cluster + ': ' +
              ('local psql over owned loopback port' if self.local else 'owned-node exec after unavailable local transport'), flush=True)

    def sql(self, text, check=True):
        if self.local:
            result = subprocess.run(self.args, env=self.env, input=text, capture_output=True, text=True, timeout=90)
            if check and result.returncode: raise RuntimeError(result.stderr[-3000:])
            return result.stdout.strip() if check else (result.returncode, result.stdout, result.stderr)
        cluster = json.loads(self.f.kubectl('-n', NAMESPACE, 'get', 'cluster/' + self.cluster, '-o', 'json'))
        primary = cluster['status']['currentPrimary']
        result = self.f.kubectl('-n', NAMESPACE, 'exec', '-i', primary, '-c', 'postgres', '--',
            'psql', '-X', '-qAt', '-U', 'postgres', '-d', 'app', '-v', 'ON_ERROR_STOP=1',
            data=text.encode(), check=check, timeout=90)
        return result.strip() if check else result

    def fingerprint(self):
        names = self.sql("SELECT tablename FROM pg_tables WHERE schemaname='worldbuilder' ORDER BY tablename").splitlines()
        assert set(names) == set(TABLES), names
        queries = ["SELECT '" + table + "',count(*),md5(string_agg(row_to_json(t)::text,E'\\n' ORDER BY row_to_json(t)::text)) FROM worldbuilder." + table + ' t' for table in TABLES]
        queries.append("SELECT 'auth.users',count(*),md5(string_agg(row_to_json(t)::text,E'\\n' ORDER BY row_to_json(t)::text)) FROM auth.users t")
        rows = self.sql(' UNION ALL '.join(queries)).splitlines()
        result = {}
        for row in rows:
            table, count, checksum = row.split('|')
            assert int(count) > 0 and re.fullmatch('[0-9a-f]{32}', checksum), row
            result[table] = {'rows': int(count), 'md5': checksum}
        return result

    def archive_wal(self):
        target = self.sql('SELECT pg_walfile_name(pg_current_wal_lsn());')
        assert re.fullmatch('[0-9A-F]{24}', target), target
        self.sql('SELECT pg_switch_wal();')
        common.wait_for('WAL containing fixture writes is archived', lambda:
            (value := self.sql('SELECT last_archived_wal FROM pg_stat_archiver;')) and
            bool(re.fullmatch('[0-9A-F]{24}', value)) and int(value, 16) >= int(target, 16), 180)

    def security(self):
        # Include column grants, not only the broader table ACLs. Never select password hashes.
        return json.loads(self.sql("""SELECT json_build_object(
          'roles',(SELECT json_agg(r ORDER BY rolname) FROM
            (SELECT rolname,rolsuper,rolinherit,rolcreaterole,rolcreatedb,rolcanlogin,rolreplication,rolbypassrls
             FROM pg_roles WHERE rolname IN ('worldbuilder_external','worldbuilder_relay')) r),
          'schema',(SELECT json_agg(json_build_object('name',nspname,'owner',pg_get_userbyid(nspowner),'acl',nspacl::text) ORDER BY nspname)
             FROM pg_namespace WHERE nspname IN ('worldbuilder','auth')),
          'relations',(SELECT json_agg(r ORDER BY relname) FROM
            (SELECT c.relname,c.relkind,pg_get_userbyid(c.relowner) AS owner,c.relacl::text,c.relrowsecurity,c.relforcerowsecurity
             FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
             WHERE n.nspname IN ('worldbuilder','auth') AND c.relkind IN ('r','S')) r),
          'columns',(SELECT json_agg(r ORDER BY relname,attnum) FROM
            (SELECT c.relname,a.attnum,a.attname,a.attacl::text FROM pg_attribute a
             JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace
             WHERE n.nspname IN ('worldbuilder','auth') AND c.relkind='r' AND a.attnum>0 AND NOT a.attisdropped) r),
          'functions',(SELECT json_agg(r ORDER BY name) FROM
            (SELECT p.oid::regprocedure::text AS name,pg_get_userbyid(p.proowner) AS owner,
                    p.proacl::text,pg_get_functiondef(p.oid) AS definition
             FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='worldbuilder') r),
          'triggers',(SELECT json_agg(r ORDER BY relname,tgname) FROM
            (SELECT c.relname,t.tgname,t.tgenabled,pg_get_triggerdef(t.oid) AS definition
             FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
             WHERE n.nspname='worldbuilder' AND NOT t.tgisinternal) r));"""))


def install(f, schemas, storage):
    selected = yaml.safe_load((ROOT / 'helm/bootstrap/flux-data/values.yaml').read_text())['cnpg-system']['version']
    assert selected in (CHART_VERSION, CHART_VERSION.rsplit('.', 1)[0] + '.x'), 'Review the CNPG comparison pin against the repository selection'
    render = subprocess.check_output(['helm', 'template', 'cnpg', 'cloudnative-pg', '--version', CHART_VERSION,
        '--repo', 'https://cloudnative-pg.github.io/charts', '--include-crds', '-n', 'cnpg-system'], env=f.helm_env)
    (f.output / 'operator.yaml').write_bytes(render)
    documents = [d for d in yaml.safe_load_all(render) if d]
    f.apply([{'apiVersion': 'v1', 'kind': 'Namespace', 'metadata': {'name': 'cnpg-system'}}])
    crds = [d for d in documents if d['kind'] == 'CustomResourceDefinition']
    assert crds
    f.apply(crds)
    for d in crds: f.kubectl('wait', 'crd/' + d['metadata']['name'], '--for=condition=Established', '--timeout=60s')
    f.apply([d for d in documents if d['kind'] != 'CustomResourceDefinition'])
    f.kubectl('-n', 'cnpg-system', 'rollout', 'status', 'deployment/cnpg-cloudnative-pg', '--timeout=240s')
    loader = module('validate-custom-resources').KubernetesLoader
    monitoring = [d for d in yaml.load_all((Path(schemas) / 'prometheus.yaml').read_text(), Loader=loader)
                  if d and d['kind'] == 'CustomResourceDefinition' and d['metadata']['name'] == 'podmonitors.monitoring.coreos.com']
    assert len(monitoring) == 1
    f.apply(monitoring)
    f.kubectl('wait', 'crd/podmonitors.monitoring.coreos.com', '--for=condition=Established', '--timeout=60s')
    values = {'namespace': NAMESPACE, 'cluster': {'name': 'offline-source', 'storageClass': 'standard', 'storageSize': '1Gi'},
        'backup': {'endpointURL': storage.endpoint, 'credentialsSecret': 'fixture-s3', 'destinationPath': 's3://cnpg-backups/drill'},
        'scheduledBackup': {'enabled': False}, 'networkPolicy': {'enabled': False}}
    inputs = f.output / 'postgres-inputs.yaml'; inputs.write_text(yaml.safe_dump(values))
    rendered = subprocess.check_output(['helm', 'template', 'offline-postgres', str(ROOT / 'helm/data/postgres'), '-f', str(inputs)], env=f.helm_env)
    documents = [d for d in yaml.safe_load_all(rendered) if d]
    cluster = next(d for d in documents if d['kind'] == 'Cluster')
    cluster['spec'].update(enableSuperuserAccess=True, superuserSecret={'name': 'fixture-admin'})
    password = secrets.token_hex(20)
    for name, kind, data in [('fixture-s3', 'Opaque', {'accessKey': 'fixture-cnpg', 'secretKey': storage.credentials['cnpg']}),
                            ('fixture-admin', 'kubernetes.io/basic-auth', {'username': 'postgres', 'password': password}),
                            ('fixture-denied', 'Opaque', {'accessKey': 'invalid-fixture-key', 'secretKey': secrets.token_hex(20)})]:
        documents.append({'apiVersion': 'v1', 'kind': 'Secret', 'metadata': {'name': name, 'namespace': NAMESPACE}, 'type': kind, 'stringData': data})
    f.apply([d for d in documents if d['kind'] == 'Namespace'])
    f.apply([d for d in documents if d['kind'] != 'Namespace'])
    f.kubectl('-n', NAMESPACE, 'wait', 'cluster/offline-source', '--for=condition=Ready', '--timeout=300s', timeout=330)
    invalid = copy.deepcopy(cluster); invalid['metadata']['name'] = 'offline-invalid'
    invalid['spec']['bootstrap']['recovery'] = {'source': 'missing'}
    code, _, error = f.kubectl('apply', '--dry-run=server', '-f', '-', data=yaml.safe_dump(invalid).encode(), check=False)
    assert code and 'Too many bootstrap types specified' in error, error
    valid = copy.deepcopy(cluster); valid['metadata']['name'] = 'offline-valid'
    f.kubectl('apply', '--dry-run=server', '-f', '-', data=yaml.safe_dump(valid).encode())
    f.kubectl('-n', 'cnpg-system', 'scale', 'deployment/cnpg-cloudnative-pg', '--replicas=0')
    try:
        f.kubectl('-n', 'cnpg-system', 'wait', 'pod', '-l', 'app.kubernetes.io/instance=cnpg', '--for=delete', '--timeout=90s')
        code, _, error = f.kubectl('apply', '--dry-run=server', '-f', '-', data=yaml.safe_dump(valid).encode(), check=False)
        assert code and 'failed calling webhook' in error, error
    finally:
        f.kubectl('-n', 'cnpg-system', 'scale', 'deployment/cnpg-cloudnative-pg', '--replicas=1')
        f.kubectl('-n', 'cnpg-system', 'rollout', 'status', 'deployment/cnpg-cloudnative-pg', '--timeout=180s')
    f.kubectl('apply', '--dry-run=server', '-f', '-', data=yaml.safe_dump(valid).encode())
    print('PASS: CNPG rejects invalid bootstrap, fails closed without its webhook, and recovers after operator restart', flush=True)
    (f.output / 'cluster.yaml').write_text(yaml.safe_dump(cluster))
    return cluster, password


def restore_spec(source, name, backup_id, denied=False, pitr=False):
    result = copy.deepcopy(source); result['metadata']['name'] = name
    result['spec'].pop('backup')
    store = copy.deepcopy(source['spec']['backup']['barmanObjectStore'])
    store['serverName'] = 'offline-source'
    if denied:
        for ref in store['s3Credentials'].values(): ref['name'] = 'fixture-denied'
    result['spec']['externalClusters'] = [{'name': 'source-archive', 'barmanObjectStore': store}]
    result['spec']['bootstrap'] = {'recovery': {'source': 'source-archive', 'recoveryTarget': {'backupID': backup_id}}}
    if pitr: result['spec']['bootstrap']['recovery']['recoveryTarget']['targetName'] = 'offline-safe-point'
    return result


def recovery(f, source, password):
    db = Database(f, 'offline-source', password)
    postgres_version = db.sql('SELECT version()')
    assert db.sql("SELECT extversion FROM pg_extension WHERE extname='vector'")
    migrations = sorted((ROOT / 'sql/worldbuilder').glob('[0-9][0-9][0-9]_up.sql'))
    assert [p.name for p in migrations] == [f'{n:03d}_up.sql' for n in range(1, len(migrations) + 1)]
    for migration in migrations: db.sql(migration.read_text())
    db.sql('CREATE ROLE worldbuilder_external NOINHERIT; CREATE ROLE worldbuilder_relay NOINHERIT;')
    db.sql((ROOT / 'sql/worldbuilder/grants.sql').read_text())
    db.sql((ROOT / 'sql/worldbuilder/auth-grants.sql').read_text())
    db.sql((HERE / 'backup-seed.sql').read_text())
    initial = db.fingerprint()
    security = db.security()
    assert len(security['roles']) == 2 and security['triggers']
    assert {s['name'] for s in security['schema']} == {'worldbuilder', 'auth'}
    assert all(s['acl'] for s in security['schema'])
    f.apply([{'apiVersion': 'postgresql.cnpg.io/v1', 'kind': 'Backup',
        'metadata': {'name': 'offline-base', 'namespace': NAMESPACE},
        'spec': {'method': 'barmanObjectStore', 'cluster': {'name': 'offline-source'}}}])
    def backup(): return json.loads(f.kubectl('-n', NAMESPACE, 'get', 'backup/offline-base', '-o', 'json')).get('status', {})
    common.wait_for('real CNPG base backup completed on external SeaweedFS', lambda: backup().get('phase') == 'completed', 300)
    backup_id = backup()['backupId']
    db.sql("UPDATE worldbuilder.world SET name='After base backup'; INSERT INTO worldbuilder.audit(world_id,actor,action,resource) SELECT id,'fixture','after-base','fixture' FROM worldbuilder.world;")
    before_target = db.fingerprint()
    db.sql("SELECT pg_create_restore_point('offline-safe-point');")
    db.archive_wal()
    db.sql("UPDATE worldbuilder.world SET name='After restore target'; INSERT INTO worldbuilder.audit(world_id,actor,action,resource) SELECT id,'fixture','after-target','fixture' FROM worldbuilder.world;")
    latest = db.fingerprint(); db.archive_wal()
    assert initial != before_target != latest
    f.kubectl('-n', NAMESPACE, 'delete', 'cluster/offline-source', '--wait=true')
    f.kubectl('-n', NAMESPACE, 'wait', 'pod', '-l', 'cnpg.io/cluster=offline-source', '--for=delete', '--timeout=120s')
    f.kubectl('-n', NAMESPACE, 'delete', 'pvc', '-l', 'cnpg.io/cluster=offline-source', '--wait=true', '--timeout=120s')
    assert not json.loads(f.kubectl('-n', NAMESPACE, 'get', 'pods,pvc', '-l', 'cnpg.io/cluster=offline-source', '-o', 'json'))['items']
    print('PASS: source database Pods and PVCs are absent before recovery', flush=True)
    f.apply([restore_spec(source, 'offline-denied', backup_id, denied=True)])
    def denied():
        state = json.loads(f.kubectl('-n', NAMESPACE, 'get', 'cluster/offline-denied', '-o', 'json'))
        assert not any(c['type'] == 'Ready' and c['status'] == 'True' for c in state.get('status', {}).get('conditions', []))
        pods = json.loads(f.kubectl('-n', NAMESPACE, 'get', 'pods', '-l', 'cnpg.io/cluster=offline-denied', '-o', 'json'))['items']
        for pod in pods:
            for item in pod.get('status', {}).get('containerStatuses', []) + pod.get('status', {}).get('initContainerStatuses', []):
                terminated = item.get('state', {}).get('terminated') or item.get('lastState', {}).get('terminated')
                if terminated and terminated['exitCode']:
                    _, out, err = f.kubectl('-n', NAMESPACE, 'logs', pod['metadata']['name'], '-c', item['name'], '--tail=80', check=False)
                    if re.search('AccessDenied|Forbidden|403|InvalidAccessKeyId|SignatureDoesNotMatch', out + err): return True
        return False
    common.wait_for('invalid S3 credentials refuse restore without a Ready database', denied, 180)
    f.kubectl('-n', NAMESPACE, 'delete', 'cluster/offline-denied', '--wait=true')
    results = {}
    for name, expected in [('offline-latest', latest), ('offline-pitr', before_target)]:
        f.apply([restore_spec(source, name, backup_id, pitr=name == 'offline-pitr')])
        f.kubectl('-n', NAMESPACE, 'wait', 'cluster/' + name, '--for=condition=Ready', '--timeout=360s', timeout=390)
        restored = Database(f, name, password)
        assert restored.sql('SELECT pg_is_in_recovery()') == 'f'
        actual = restored.fingerprint()
        assert actual == expected, (name, actual, expected)
        assert restored.security() == security, name + ' security/catalog state differs'
        code, _, error = restored.sql('UPDATE worldbuilder.revision SET actor=actor', check=False)
        assert code and 'immutable' in error
        code, _, error = restored.sql("SET ROLE worldbuilder_relay; UPDATE worldbuilder.world SET name='forbidden'", check=False)
        assert code and 'permission denied' in error
        assert restored.sql("SELECT has_table_privilege('worldbuilder_external','worldbuilder.world','SELECT')") == 't'
        code, _, error = restored.sql("SET ROLE worldbuilder_external; INSERT INTO worldbuilder.superuser(principal,granted_by) VALUES('forbidden@example.test','runtime')", check=False)
        assert code != 0 and 'permission denied' in error.lower(), error
        code, _, error = restored.sql("SET ROLE worldbuilder_external; UPDATE auth.users SET display_name='forbidden'", check=False)
        assert code != 0 and 'permission denied' in error.lower(), error
        previous = int(restored.sql('SELECT max(id) FROM worldbuilder.audit'))
        current = int(restored.sql("INSERT INTO worldbuilder.audit(world_id,actor,action,resource) SELECT id,'fixture','after-restore','fixture' FROM worldbuilder.world RETURNING id"))
        assert current > previous
        results[name] = {'tables': actual, 'transport': 'local-psql' if restored.local else 'owned-node-exec'}
        print('PASS: ' + name + ' restores all 19 populated Worldbuilder tables and auth users, grants, immutable triggers and a non-reusing audit sequence', flush=True)
    pods = json.loads(f.kubectl('get', 'pods', '-A', '-o', 'json'))['items']
    images = sorted({c['imageID'] for p in pods for c in p.get('status', {}).get('containerStatuses', []) if c.get('imageID')})
    (f.output / 'backup-result.json').write_text(json.dumps({'status': 'passed', 'cnpgChart': CHART_VERSION,
        'postgresVersion': postgres_version, 'observedImageIDs': images,
        'seedSHA256': hashlib.sha256((HERE / 'backup-seed.sql').read_bytes()).hexdigest(),
        'securitySHA256': hashlib.sha256(json.dumps(security, sort_keys=True).encode()).hexdigest(),
        'backupID': backup_id, 'results': results, 'sourceRemoved': True, 'invalidCredentialsRefused': True,
        'exclusions': ['production data/migration', 'Longhorn or hardware failure', 'Velero', 'mesh enforcement']}, indent=2) + '\n')


def main():
    if len(sys.argv) != 3: raise SystemExit('Usage: test-backups.py OUTPUT_DIRECTORY PUBLIC_SCHEMA_DIRECTORY')
    f = common.Fixture(sys.argv[1], ports=PORTS.values())
    storage = Storage(f)
    try:
        f.create(); storage.create()
        source, password = install(f, sys.argv[2], storage)
        recovery(f, source, password)
    finally:
        try:
            for name, args in [('pods', ['get', 'pods', '-A', '-o', 'wide']),
                               ('events', ['get', 'events', '-A', '--sort-by=.lastTimestamp']),
                               ('cnpg-state', ['-n', NAMESPACE, 'get', 'clusters,backups', '-o', 'json']),
                               ('operator', ['-n', 'cnpg-system', 'logs', 'deployment/cnpg-cloudnative-pg', '--tail=100'])]:
                code, out, err = f.kubectl(*args, check=False, timeout=30)
                (f.output / (name + '.log')).write_text(out + err)
        finally:
            try: storage.close()
            finally: f.close()
    print('PASS: physical backup/WAL/PITR acceptance and owned fixture teardown', flush=True)


if __name__ == '__main__': main()
