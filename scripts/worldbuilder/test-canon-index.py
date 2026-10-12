#!/usr/bin/env python3
"""Real canon projection/discovery with owned offline PostgreSQL, NATS and OpenSearch.

All containers use network=none. Only generated loopback Docker-exec bridges
reach them; operator endpoints, credentials and Docker remote contexts are unused.
"""
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import shutil
import socketserver
import subprocess
import sys
import threading
import time
import urllib.request
import uuid

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
spec = importlib.util.spec_from_file_location('transport', HERE / 'docker-exec.py')
transport = importlib.util.module_from_spec(spec); spec.loader.exec_module(transport)
IMAGES = {
    'postgres': 'postgres:16.13@sha256:5d143123fdf80462d1778cd4f24b9f7ca13c87174bca19141fb194c5a1ebca59',
    'nats': 'nats:2.11.9-alpine@sha256:777787bbb4c7c4083ec2213b0b8b14151617610a3e0d85776bd995cd46599c73',
    'search': 'opensearchproject/opensearch:3.9.0@sha256:adfa61f85025d06b4aeb562e7e74fde7e31c437039c93c3862c17e9acebd6c7c'}
LABEL = 'homelab.worldbuilder.index-fixture'


class OwnedDocker(transport.FixtureDocker):
    def __init__(self, name):
        if not re.fullmatch(r'wbindex-[0-9a-f]{12}', name):
            raise ValueError('Expected generated canon-index fixture name')
        self.name, self.socket_path = name, '/var/run/docker.sock'

    def owned_node(self, container):
        if container not in [self.name + '-' + role for role in IMAGES]:
            raise ValueError('Refusing container outside canon-index fixture')
        info = self.api('GET', '/containers/' + container + '/json')
        if (info['Config'].get('Labels') or {}).get(LABEL) != self.name:
            raise ValueError('Canon-index fixture ownership label mismatch')
        if info['HostConfig']['NetworkMode'] != 'none':
            raise ValueError('Canon-index fixture must have no external network')
        return info['Id']


class Bridge(socketserver.ThreadingTCPServer):
    allow_reuse_address = False
    daemon_threads = True

    def __init__(self, docker, container, port, cli, environment):
        self.docker, self.container, self.cli, self.environment = docker, container, cli, environment
        self.processes, self.lock = set(), threading.Lock()
        if port not in (5432, 4222, 9200): raise ValueError('Unexpected fixture port')
        self.command = ['nc', '127.0.0.1', str(port)] if port == 4222 else [
            'bash', '-c', 'exec 3<>/dev/tcp/127.0.0.1/' + str(port) +
            '; cat <&3 & receiver=$!; cat >&3; kill "$receiver" 2>/dev/null || true; wait "$receiver" 2>/dev/null || true']
        super().__init__(('127.0.0.1', 0), Handler)
        self.thread = threading.Thread(target=self.serve_forever, daemon=True)
        self.thread.start()

    def close(self):
        self.shutdown(); self.server_close()
        with self.lock:
            for process in self.processes:
                process.terminate()
        self.thread.join(timeout=5)


class Handler(socketserver.BaseRequestHandler):
    def handle(self):
        s = self.server
        identifier = s.docker.owned_node(s.container)
        # This is the original streaming CLI only for a binary loopback bridge.
        # Finite commands below use inspected exec IDs and verified output.
        with subprocess.Popen([s.cli, 'exec', '-i', identifier, *s.command], env=s.environment,
                              stdin=self.request, stdout=self.request, stderr=subprocess.DEVNULL) as process:
            with s.lock: s.processes.add(process)
            try: process.wait(timeout=300)
            except subprocess.TimeoutExpired: process.kill(); process.wait()
            finally:
                with s.lock: s.processes.discard(process)


def wait_for(label, probe, seconds=120):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        if probe(): return
        time.sleep(1)
    raise TimeoutError(label)


def main(output):
    output = Path(output).resolve(); output.mkdir(parents=True, exist_ok=False)
    name = 'wbindex-' + uuid.uuid4().hex[:12]
    docker = OwnedDocker(name)
    cli = shutil.which('docker')
    assert cli, 'Docker CLI required'
    environment = {k: v for k, v in os.environ.items() if not k.startswith(('DOCKER_', 'OPENSEARCH_', 'WORLDBUILDER_TEST_'))}
    environment['DOCKER_HOST'] = 'unix:///var/run/docker.sock'
    containers, bridges, volumes = [], [], set()
    report = {'status': 'FAIL', 'fixture': name, 'ownedContainersRemoved': False}

    def command(*args, timeout=120):
        result = subprocess.run([cli, *args], env=environment, capture_output=True, text=True, timeout=timeout)
        if result.returncode:
            raise RuntimeError('Docker ' + args[0] + ' failed: ' + result.stderr[-2000:] + result.stdout[-1000:])
        return result.stdout

    def execute(role, args, data=None):
        stdout, stderr = io.BytesIO(), io.BytesIO()
        code = docker.execute(name + '-' + role, args, stdin=io.BytesIO(data) if data is not None else None,
                              stdout=stdout, stderr=stderr, timeout=90)
        return code, stdout.getvalue(), stderr.getvalue()

    try:
        for role, image in IMAGES.items():
            print('Fetching pinned ' + role + ' image', flush=True)
            # The pinned OpenSearch image contains a roughly 1.1 GiB compressed
            # layer; allow a bounded cold download independently of readiness.
            command('pull', image, timeout=1800)
        for role, image in IMAGES.items():
            print('Preparing isolated ' + role, flush=True)
            options = ['--network', 'none', '--name', name + '-' + role, '--label', LABEL + '=' + name,
                       '--security-opt', 'no-new-privileges', '--memory', '2g' if role == 'search' else '512m']
            args = []
            if role == 'postgres': options += ['-e', 'POSTGRES_PASSWORD=offline-index-only', '-e', 'POSTGRES_DB=canon_test']
            if role == 'nats': args = ['-js', '-sd', '/tmp/jetstream']
            if role == 'search':
                options += ['-e', 'discovery.type=single-node', '-e', 'DISABLE_INSTALL_DEMO_CONFIG=true',
                            '-e', 'DISABLE_SECURITY_PLUGIN=true', '-e', 'OPENSEARCH_JAVA_OPTS=-Xms512m -Xmx512m']
            command('create', *options, image, *args)
            containers.append(name + '-' + role)
            command('start', containers[-1])
            info = json.loads(command('inspect', containers[-1]))[0]
            volumes.update(mount['Name'] for mount in info['Mounts'] if mount['Type'] == 'volume')
        report['images'] = {role: json.loads(command('inspect', name + '-' + role))[0]['Image'] for role in IMAGES}
        wait_for('PostgreSQL final TCP server', lambda: execute('postgres', ['pg_isready', '-h', '127.0.0.1', '-U', 'postgres', '-d', 'canon_test'])[0] == 0)
        code, _, error = execute('postgres', ['createdb', '-U', 'postgres', 'canon_transport'])
        assert code == 0, error.decode()[-2000:]
        for database in ('canon_test', 'canon_transport'):
            for migration in sorted((ROOT / 'sql/worldbuilder').glob('[0-9][0-9][0-9]_up.sql')):
                code, _, error = execute('postgres', ['psql', '-X', '-U', 'postgres', '-d', database, '-v', 'ON_ERROR_STOP=1'], migration.read_bytes())
                assert code == 0, (migration.name, error.decode()[-2000:])
        addresses = {}
        for role, port in [('postgres', 5432), ('nats', 4222), ('search', 9200)]:
            bridge = Bridge(docker, name + '-' + role, port, cli, environment)
            bridges.append(bridge); addresses[role] = '127.0.0.1:' + str(bridge.server_address[1])
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

        def search_ready():
            try:
                with opener.open('http://' + addresses['search'], timeout=3) as response:
                    return json.load(response)['version']['number'] == '3.9.0'
            except (OSError, ValueError): return False

        wait_for('OpenSearch actual HTTP version through owned bridge', search_ready, 180)
        environment.update({'WORLDBUILDER_TEST_DATABASE_URL': 'postgres://postgres:offline-index-only@' + addresses['postgres'] + '/canon_test?sslmode=disable&connect_timeout=5',
                            'WORLDBUILDER_TEST_NATS_URL': 'nats://' + addresses['nats'],
                            'WORLDBUILDER_TEST_OPENSEARCH_URL': 'http://' + addresses['search']})
        cases = [('pkg/canonsearch', '.', '^TestVectorAgainstDisposableOpenSearch$', ['TestVectorAgainstDisposableOpenSearch']),
                 ('external', './worldbuilder', '^Test(Discovery|Search|Hybrid)', ['TestDiscoveryAgainstDisposableOpenSearch', 'TestHybridSearchActualIndexAuthorizationAllowanceAndEvidence']),
                 ('orchestrator', '.', '^TestCanonLiveTransportAndIndex$', ['TestCanonLiveTransportAndIndex'])]
        for app, package, pattern, required_cases in cases:
            print('Running real ' + app + ' index checks', flush=True)
            if app == 'orchestrator':
                environment['WORLDBUILDER_TEST_DATABASE_URL'] = environment['WORLDBUILDER_TEST_DATABASE_URL'].replace('/canon_test?', '/canon_transport?')
            log = output / (app.replace('/', '-') + '.log')
            with log.open('w') as stream:
                result = subprocess.run(['go', 'test', '-v', '-race', '-count=1', '-timeout=180s', '-run', pattern, package],
                                        cwd=ROOT / 'apps' / app, env=environment, stdout=stream, stderr=subprocess.STDOUT, timeout=600)
            evidence = log.read_text()
            assert result.returncode == 0 and '--- SKIP:' not in evidence and all(('--- PASS: ' + case + ' (') in evidence for case in required_cases), (app, evidence[-5000:])
        report['checksPassed'] = True
    finally:
        cleanup_errors = []
        for bridge in reversed(bridges): bridge.close()
        for container in reversed(containers):
            try:
                identifier = docker.owned_node(container)
                log = subprocess.run([cli, 'logs', identifier], env=environment, capture_output=True, text=True, timeout=30)
                (output / (container.removeprefix(name + '-') + '-container.log')).write_text(log.stdout + log.stderr)
                command('rm', '-fv', identifier)
            except Exception as error: cleanup_errors.append(str(error))
        remaining = command('ps', '-aq', '--filter', 'label=' + LABEL + '=' + name).strip()
        retained_volumes = volumes.intersection(command('volume', 'ls', '--format', '{{.Name}}').splitlines())
        report['ownedContainersRemoved'] = not remaining and not cleanup_errors and not retained_volumes
        report['ownedVolumesRemoved'] = not retained_volumes
        if report.get('checksPassed') and report['ownedContainersRemoved']: report['status'] = 'PASS'
        if cleanup_errors: report['cleanupErrors'] = cleanup_errors
        (output / 'index-result.json').write_text(json.dumps(report, indent=2) + '\n')
        print(json.dumps(report), flush=True)
        assert report['ownedContainersRemoved'], 'Owned fixture cleanup failed'


if __name__ == '__main__': main(sys.argv[1])
