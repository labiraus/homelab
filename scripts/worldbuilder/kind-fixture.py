#!/usr/bin/env python3
"""Owned disposable kind lifecycle and transport shared by controller drills."""
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
import uuid
import yaml

HERE = Path(__file__).resolve().parent


def module(name):
    spec = importlib.util.spec_from_file_location(name, HERE / (name + '.py'))
    result = importlib.util.module_from_spec(spec); spec.loader.exec_module(result)
    return result


class Fixture:
    def __init__(self, output, ports=()):
        self.ports = tuple(ports)
        if len(set(self.ports)) != len(self.ports) or any(type(p) is not int or not 30000 <= p <= 32767 for p in self.ports):
            raise ValueError("Fixture ports must be distinct Kubernetes NodePorts")
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
            KIND_EXPERIMENTAL_PROVIDER='docker', KIND_EXPERIMENTAL_DOCKER_NETWORK='kind',
            DOCKER_HOST='unix:///var/run/docker.sock',
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

    def create(self, cni_installer=None):
        config = self.private / 'kind.yaml'
        networking = {'apiServerAddress': '127.0.0.1', 'ipFamily': 'ipv4'}
        if cni_installer is not None:
            if not callable(cni_installer): raise ValueError('Fixture CNI installer must be callable')
            networking['disableDefaultCNI'] = True
        config.write_text(yaml.safe_dump({'kind': 'Cluster', 'apiVersion': 'kind.x-k8s.io/v1alpha4',
            'networking': networking,
            'nodes': [{'role': 'control-plane', 'extraPortMappings': [
                {'containerPort': port, 'hostPort': 0, 'listenAddress': '127.0.0.1', 'protocol': 'TCP'}
                for port in self.ports]}]}))
        with (self.output / 'kind.log').open('w') as log:
            subprocess.run([str(self.kind), 'create', 'cluster', '--name', self.name,
                '--image', module('install-kind').NODE, '--config', str(config),
                '--kubeconfig', self.env['KUBECONFIG'], '--wait', '0s' if cni_installer else '180s', '--retain'],
                env=self.env, stdout=log, stderr=subprocess.STDOUT, check=True, timeout=480)
        if bundle := os.environ.get('WORLDBUILDER_TEST_CA_BUNDLE'):
            data = Path(bundle).read_bytes()
            if len(data) > 1024**2 or b'PRIVATE KEY' in data:
                raise ValueError('Fixture registry trust requires a bounded public CA bundle')
            ssl.create_default_context(cadata=data.decode())
            self.execute(['sh', '-c', 'cat >> /etc/ssl/certs/ca-certificates.crt && systemctl restart containerd'], data)
            (self.output / 'registry-trust.json').write_text(json.dumps({
                'sha256': hashlib.sha256(data).hexdigest(), 'tlsVerification': True}) + '\n')
        if cni_installer is not None:
            # kind creates CoreDNS before a custom CNI can make the node Ready.
            # Its default progress deadline otherwise expires during cold CNI
            # downloads and rollout fails immediately when scheduling resumes.
            self.kubectl('-n', 'kube-system', 'patch', 'deployment/coredns', '--type=merge',
                         '-p', '{"spec":{"progressDeadlineSeconds":1800}}')
            cni_installer(self)
        self.kubectl('wait', 'nodes', '--all', '--for=condition=Ready', '--timeout=180s')
        self.kubectl('-n', 'kube-system', 'rollout', 'status', 'deployment/coredns', '--timeout=180s')
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
                   for port in (6443, *self.ports)
                   for binding in info['NetworkSettings']['Ports'][str(port) + '/tcp'])
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


