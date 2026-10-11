#!/usr/bin/env python3
"""Refusal boundaries for the physical restore harness; no cluster required."""
import importlib.util
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

spec = importlib.util.spec_from_file_location('backups', Path(__file__).with_name('test-backups.py'))
backups = importlib.util.module_from_spec(spec); spec.loader.exec_module(backups)


class BackupTests(unittest.TestCase):
    def fixture(self):
        f = Mock()
        f.node = 'wbctrl-1234abcd-control-plane'; f.ports = (30432,)
        f.private = Path('/tmp/unused-backup-fixture')
        f.kubectl.return_value = '{"spec":{"selector":{"cnpg.io/cluster":"offline-source"}}}'
        f.docker.api.return_value = {'NetworkSettings': {'Ports': {'30432/tcp': [{'HostIp': '127.0.0.1', 'HostPort': '45678'}]}}}
        return f

    def test_unrecognised_database_is_refused_before_cluster_access(self):
        f = self.fixture()
        with self.assertRaises(ValueError): backups.Database(f, 'production', 'synthetic')
        f.kubectl.assert_not_called()

    def test_bad_password_does_not_fall_back_to_peer_authentication(self):
        with patch.object(backups.shutil, 'which', return_value='/usr/bin/psql'), patch.object(backups.subprocess, 'run',
                return_value=SimpleNamespace(returncode=2, stdout='', stderr='FATAL: password authentication failed')):
            with self.assertRaisesRegex(RuntimeError, 'beyond a transport error'):
                backups.Database(self.fixture(), 'offline-source', 'synthetic')

    def test_unreachable_loopback_allows_only_owned_exec_fallback(self):
        with patch.object(backups.shutil, 'which', return_value='/usr/bin/psql'), patch.object(backups.time, 'sleep'), patch.object(backups.subprocess, 'run',
                return_value=SimpleNamespace(returncode=2, stdout='', stderr='connection refused')) as run:
            database = backups.Database(self.fixture(), 'offline-source', 'synthetic')
            self.assertFalse(database.local)
            self.assertEqual(run.call_count, 5)

    def test_published_database_port_must_be_loopback(self):
        f = self.fixture(); f.docker.api.return_value['NetworkSettings']['Ports']['30432/tcp'][0]['HostIp'] = '0.0.0.0'
        with patch.object(backups.shutil, 'which', return_value='/usr/bin/psql'):
            with self.assertRaises(AssertionError): backups.Database(f, 'offline-source', 'synthetic')

    def test_missing_or_empty_tables_cannot_pass_fingerprinting(self):
        db = object.__new__(backups.Database)
        db.sql = Mock(return_value='world')
        with self.assertRaises(AssertionError): db.fingerprint()
        db.sql = Mock(side_effect=['\n'.join(backups.TABLES), 'world|0|'])
        with self.assertRaises(AssertionError): db.fingerprint()

    def test_restore_reads_source_archive_without_writing_to_it(self):
        source = {'metadata': {'name': 'offline-source'}, 'spec': {'bootstrap': {'initdb': {}}, 'backup': {
            'barmanObjectStore': {'destinationPath': 's3://cnpg-backups/drill', 'endpointURL': 'http://fixture:8333',
                                 's3Credentials': {'accessKeyId': {'name': 'good', 'key': 'accessKey'}}}}}}
        target = backups.restore_spec(source, 'offline-pitr', 'backup-id', pitr=True)
        self.assertNotIn('backup', target['spec'])
        self.assertEqual(target['spec']['externalClusters'][0]['barmanObjectStore']['serverName'], 'offline-source')
        self.assertEqual(target['spec']['bootstrap']['recovery']['recoveryTarget'], {'backupID': 'backup-id', 'targetName': 'offline-safe-point'})
        denied = backups.restore_spec(source, 'offline-denied', 'backup-id', denied=True)
        self.assertEqual(denied['spec']['externalClusters'][0]['barmanObjectStore']['s3Credentials']['accessKeyId']['name'], 'fixture-denied')
        self.assertEqual(source['spec']['backup']['barmanObjectStore']['s3Credentials']['accessKeyId']['name'], 'good')
        self.assertIn('initdb', source['spec']['bootstrap'])

    def test_kind_rejects_invalid_or_duplicate_database_ports(self):
        for ports in [(5432,), (30432, 30432), (True,)]:
            with self.subTest(ports=ports), self.assertRaises(ValueError):
                backups.common.Fixture('/tmp/unused-invalid-backup-fixture', ports=ports)


if __name__ == '__main__': unittest.main()
