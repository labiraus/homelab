#!/usr/bin/env python3
"""External authenticated SeaweedFS, owned by one disposable kind fixture."""
import importlib.util
import json
from pathlib import Path
import secrets
import subprocess

import boto3
from botocore.config import Config
import yaml

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
S3_IMAGE = 'chrislusf/seaweedfs:4.48@sha256:4e61d15fd35994cb1e43e1e553dff106794841fd9a99ade2fc8c8bfce4d7872d'


def module(name):
    spec = importlib.util.spec_from_file_location(name, HERE / (name + '.py'))
    result = importlib.util.module_from_spec(spec); spec.loader.exec_module(result)
    return result


common = module('kind-fixture')


class Storage:
    def __init__(self, fixture, consumer='cnpg'):
        if consumer not in ('cnpg', 'velero'): raise ValueError('Unknown disposable S3 consumer')
        self.consumer = consumer
        self.bucket = {'cnpg': 'cnpg-backups', 'velero': 'velero'}[consumer]
        self.f = fixture
        self.name = fixture.name + '-s3'
        self.identifier = None
        self.credentials = {key: secrets.token_hex(20) for key in ('admin', self.consumer)}

    def command(self, *args, check=True):
        return subprocess.run([self.f.cli, *args], env=self.f.env, check=check,
                              capture_output=True, text=True, timeout=180)

    def create(self):
        defaults = yaml.safe_load((ROOT / 'ansible/roles/seaweedfs/defaults/main.yml').read_text())
        target = defaults['seaweedfs_version']
        assert S3_IMAGE.startswith('chrislusf/seaweedfs:' + target + '@sha256:'), 'Update the fixture when the target SeaweedFS version changes'
        self.command('pull', S3_IMAGE)
        policies = {item['name']: item['actions'] for item in defaults['seaweedfs_identities']}
        assert self.bucket in defaults['seaweedfs_buckets']
        assert set(policies[self.consumer]) == {action + ':' + self.bucket for action in ('Read', 'Write', 'List', 'Tagging')}
        config = {'identities': [{'name': who,
            'credentials': [{'accessKey': 'fixture-' + who, 'secretKey': self.credentials[who]}],
            'actions': policies[who]} for who in ('admin', self.consumer)]}
        script = ('mkdir -p /data/master; printf %s "$FIXTURE_S3_CONFIG" >/tmp/s3.json; '
            'exec weed server -ip=127.0.0.1 -ip.bind=127.0.0.1 -dir=/data -master.dir=/data/master '
            '-master.volumeSizeLimitMB=1024 -master.telemetry=false -volume.max=32 -filer -s3 '
            '-s3.ip.bind=0.0.0.0 -s3.port=8333 -s3.config=/tmp/s3.json -s3.port.iceberg=0 -s3.port.lance=0')
        self.identifier = self.command('create', '--name', self.name, '--label',
            'homelab.offline-fixture=' + self.f.name, '--network', 'kind', '--entrypoint', '/bin/sh',
            '--workdir', '/data', '-e', 'FIXTURE_S3_CONFIG=' + json.dumps(config), S3_IMAGE, '-c', script).stdout.strip()
        self.command('start', self.identifier)
        info = self.inspect()
        assert not info['HostConfig']['PortBindings']
        self.endpoint = 'http://' + info['NetworkSettings']['Networks']['kind']['IPAddress'] + ':8333'
        common.wait_for('external S3 fixture requires authentication', lambda: self.f.execute(
            ['curl', '--silent', '--output', '/dev/null', '--write-out', '%{http_code}', self.endpoint],
            check=False)[1] == b'403', 60)
        client = boto3.client('s3', endpoint_url=self.endpoint, region_name='us-east-1',
            aws_access_key_id='fixture-admin', aws_secret_access_key=self.credentials['admin'],
            config=Config(signature_version='s3v4', s3={'addressing_style': 'path'}))
        url = client.generate_presigned_url('create_bucket', Params={'Bucket': self.bucket},
                                            ExpiresIn=300, HttpMethod='PUT')
        self.f.execute(['curl', '--fail', '--silent', '--show-error', '-X', 'PUT', url])
        (self.f.output / 'storage.json').write_text(json.dumps({'image': S3_IMAGE,
            'imageID': info['Image'], 'publishedPorts': False, 'externalToKubernetes': True}, indent=2) + '\n')

    def inspect(self):
        info = self.f.docker.api('GET', '/containers/' + self.identifier + '/json')
        if info['Config']['Labels'].get('homelab.offline-fixture') != self.f.name:
            raise ValueError('Refusing a storage container outside this fixture')
        return info

    def close(self):
        if self.identifier:
            self.inspect()
            result = self.command('logs', '--tail', '100', self.identifier, check=False)
            (self.f.output / 'storage.log').write_text(result.stdout + result.stderr)
            self.command('rm', '-f', '-v', self.identifier)
            try: self.inspect()
            except RuntimeError as error:
                if ': 404:' not in str(error): raise
            else: raise AssertionError('Storage fixture survived cleanup')
