#!/usr/bin/env python3
"""Complete Docker execs by inspecting their IDs, not a truncated attach stream.

Used only by disposable kind/PostgreSQL fixtures. Non-exec commands pass to the
original Docker CLI. Execs require the exact generated name and ownership label
on a local Unix socket; input/output files stay inside the fixture and are removed.
"""
import base64
import http.client
import json
import os
import re
import socket
import sys
import struct
import time
from urllib.parse import quote
import uuid


class DockerConnection(http.client.HTTPConnection):
    def __init__(self, path):
        super().__init__('localhost', timeout=30)
        self.path = path

    def connect(self):
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.settimeout(self.timeout)
        self.sock.connect(self.path)


class FixtureDocker:
    def __init__(self, cluster, socket_path='/var/run/docker.sock'):
        if not re.fullmatch(r'wbctrl-[0-9a-f]{8}', cluster):
            raise ValueError('Expected a generated wbctrl-XXXXXXXX fixture name')
        self.cluster, self.socket_path = cluster, socket_path

    def request(self, method, path, data=None):
        connection = DockerConnection(self.socket_path)
        headers = {}
        body = None
        if data is not None:
            body = json.dumps(data).encode()
            headers = {'Content-Type': 'application/json'}
        connection.request(method, path, body, headers)
        response = connection.getresponse()
        if response.status >= 300:
            error = response.read(4096).decode(errors='replace')
            connection.close()
            raise RuntimeError(f'Docker {method} {path}: {response.status}: {error}')
        return connection, response

    def api(self, method, path, data=None):
        connection, response = self.request(method, path, data)
        try:
            body = response.read()
            return json.loads(body) if body else None
        finally:
            connection.close()

    def owned_node(self, container):
        if container != self.cluster + '-control-plane':
            raise ValueError('Refusing exec outside the owned fixture control-plane node')
        info = self.api('GET', '/containers/' + quote(container, safe='') + '/json')
        if (info['Config'].get('Labels') or {}).get('io.x-k8s.kind.cluster') != self.cluster:
            raise ValueError('Container does not have the expected kind ownership label')
        return info['Id']

    def start(self, container_id, command, privileged=False, env=None):
        identifier = self.api('POST', f'/containers/{container_id}/exec', {
            'Cmd': command, 'AttachStdin': False, 'AttachStdout': False, 'AttachStderr': False,
            'Privileged': privileged, 'Tty': False, 'Env': env or []})['Id']
        self.api('POST', f'/exec/{identifier}/start', {'Detach': True, 'Tty': False})
        return identifier

    def wait(self, identifier, timeout):
        deadline = time.monotonic() + timeout
        while True:
            state = self.api('GET', f'/exec/{identifier}/json')
            if not state['Running']:
                if not isinstance(state.get('ExitCode'), int):
                    raise RuntimeError('Terminal Docker exec has no exit code: ' + identifier)
                return state['ExitCode']
            if time.monotonic() >= deadline:
                raise TimeoutError(f'Docker exec {identifier} is still running; fixture teardown must stop it')
            time.sleep(0.1)

    def capture(self, node, command):
        """Read a bounded, already-complete file chunk through an attached exec."""
        identifier = self.api('POST', f'/containers/{node}/exec', {
            'Cmd': command, 'AttachStdout': True, 'AttachStderr': True, 'Tty': False})['Id']
        connection, response = self.request('POST', f'/exec/{identifier}/start', {'Detach': False, 'Tty': False})
        try:
            data = response.read()
        finally:
            connection.close()
        if self.wait(identifier, 30) != 0:
            raise RuntimeError('Fixture file reader failed')
        stdout = bytearray()
        while data:
            if len(data) < 8:
                raise RuntimeError('Truncated Docker stream header')
            channel, size = data[0], struct.unpack('>I', data[4:8])[0]
            if channel != 1 or len(data) < 8 + size:
                raise RuntimeError('Truncated or unexpected Docker output')
            stdout.extend(data[8:8 + size]); data = data[8 + size:]
        return bytes(stdout)

    def file(self, node, path, output):
        size = int(self.capture(node, ['stat', '-c', '%s', '--', path]))
        for offset in range(0, size, 16384):
            block = self.capture(node, ['dd', 'if=' + path, 'bs=16384',
                                        'skip=' + str(offset // 16384), 'count=1', 'status=none'])
            if len(block) != min(16384, size - offset):
                raise RuntimeError('Truncated fixture output chunk')
            output.write(block)

    def execute(self, container, command, stdin=None, stdout=None, stderr=None,
                privileged=False, env=None, timeout=600):
        node = self.owned_node(container)
        directory = '/tmp/wb-exec-' + uuid.uuid4().hex
        setup = self.start(node, ['/bin/sh', '-c',
            'umask 077; mkdir "$1" && : >"$1/stdin"', '--', directory])
        if self.wait(setup, 30) != 0:
            raise RuntimeError('Cannot create private fixture exec directory')
        # Some Docker proxies also acknowledge archive uploads without copying
        # them. Bounded argv chunks avoid both archive and stdin attach loss.
        if stdin is not None:
            while block := stdin.read(32768):
                upload = self.start(node, ['/bin/sh', '-c',
                    'printf %s "$1" | base64 -d >>"$2/stdin"', '--',
                    base64.b64encode(block).decode(), directory])
                if self.wait(upload, 30) != 0:
                    raise RuntimeError('Cannot upload fixture input')
        # Generated paths contain only fixed text and hex. Command arguments are
        # passed through "$@", never interpolated as shell source.
        wrapper = f'exec "$@" <{directory}/stdin >{directory}/stdout 2>{directory}/stderr'
        identifier = self.start(node, ['/bin/sh', '-c', wrapper, '--', *command], privileged, env)
        completed = False
        try:
            code = self.wait(identifier, timeout)
            completed = True
            self.file(node, directory + '/stdout', stdout if stdout is not None else sys.stdout.buffer)
            self.file(node, directory + '/stderr', stderr if stderr is not None else sys.stderr.buffer)
            return code
        finally:
            # Do not remove files still used by a running exec after a timeout.
            # The owning harness deletes its node in its own finally block.
            if completed:
                cleanup = self.start(node, ['/bin/rm', '-rf', '--', directory])
                if self.wait(cleanup, 30) != 0:
                    raise RuntimeError('Could not remove private fixture exec files')


class PostgresFixtureDocker(FixtureDocker):
    """The same completion transport, restricted to one labelled test database."""
    def __init__(self, container, socket_path='/var/run/docker.sock'):
        if not re.fullmatch(r'worldbuilder-test-[0-9]+-[0-9]+', container):
            raise ValueError('Expected a generated disposable PostgreSQL name')
        self.container, self.socket_path = container, socket_path

    def owned_node(self, container):
        if container != self.container:
            raise ValueError('Refusing exec outside the owned PostgreSQL fixture')
        info = self.api('GET', '/containers/' + quote(container, safe='') + '/json')
        if (info['Config'].get('Labels') or {}).get('homelab.worldbuilder.postgres-fixture') != self.container:
            raise ValueError('Container does not have the expected PostgreSQL ownership label')
        return info['Id']


def main(arguments):
    original = os.environ['WORLDBUILDER_DOCKER_CLI']
    if not arguments or arguments[0] != 'exec':
        os.execv(original, [original, *arguments])
    socket_url = os.environ.get('DOCKER_HOST', 'unix:///var/run/docker.sock')
    if not socket_url.startswith('unix:///'):
        raise ValueError('Disposable exec transport requires an explicit local Unix socket')
    interactive, privileged, env = False, False, []
    index = 1
    while index < len(arguments) and arguments[index].startswith('-'):
        flag = arguments[index]
        if flag in ('-i', '--interactive'): interactive = True
        elif flag == '--privileged': privileged = True
        elif flag in ('-e', '--env'):
            index += 1; env.append(arguments[index])
        else: raise ValueError('Unsupported fixture Docker exec flag: ' + flag)
        index += 1
    container, command = arguments[index], arguments[index + 1:]
    if not command: raise ValueError('Missing exec command')
    if os.environ.get('WORLDBUILDER_POSTGRES_CONTAINER'):
        docker = PostgresFixtureDocker(os.environ['WORLDBUILDER_POSTGRES_CONTAINER'], socket_url.removeprefix('unix://'))
    else:
        docker = FixtureDocker(os.environ['WORLDBUILDER_KIND_NAME'], socket_url.removeprefix('unix://'))
    return docker.execute(container, command, stdin=sys.stdin.buffer if interactive else None,
                          privileged=privileged, env=env)


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
