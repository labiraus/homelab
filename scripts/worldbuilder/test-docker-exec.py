#!/usr/bin/env python3
"""Refusal tests for the fixture-only Docker transport; no daemon required."""
import importlib.util
import io
from pathlib import Path
import unittest
from unittest.mock import Mock, patch

spec = importlib.util.spec_from_file_location('transport', Path(__file__).with_name('docker-exec.py'))
transport = importlib.util.module_from_spec(spec); spec.loader.exec_module(transport)


class TransportTests(unittest.TestCase):
    def setUp(self):
        self.docker = transport.FixtureDocker('wbctrl-1234abcd')

    def test_refuses_non_fixture_names_without_contacting_docker(self):
        with self.assertRaises(ValueError): transport.FixtureDocker('production')
        self.docker.api = Mock()
        with self.assertRaises(ValueError): self.docker.owned_node('production-control-plane')
        self.docker.api.assert_not_called()

    def test_exact_name_still_requires_kind_ownership_label(self):
        self.docker.api = Mock(return_value={'Id': 'immutable-id', 'Config': {'Labels': {'io.x-k8s.kind.cluster': 'different'}}})
        with self.assertRaises(ValueError): self.docker.owned_node('wbctrl-1234abcd-control-plane')

    def test_uses_immutable_container_id_after_ownership_check(self):
        self.docker.api = Mock(return_value={'Id': 'immutable-id', 'Config': {'Labels': {'io.x-k8s.kind.cluster': 'wbctrl-1234abcd'}}})
        self.assertEqual(self.docker.owned_node('wbctrl-1234abcd-control-plane'), 'immutable-id')

    def test_postgres_requires_exact_name_and_ownership_label(self):
        with self.assertRaises(ValueError): transport.PostgresFixtureDocker('production')
        docker = transport.PostgresFixtureDocker('worldbuilder-test-123-456')
        docker.api = Mock(return_value={'Id': 'database-id', 'Config': {'Labels': {}}})
        with self.assertRaises(ValueError): docker.owned_node('another-container')
        docker.api.assert_not_called()
        with self.assertRaises(ValueError): docker.owned_node('worldbuilder-test-123-456')
        docker.api.return_value['Config']['Labels']['homelab.worldbuilder.postgres-fixture'] = 'worldbuilder-test-123-456'
        self.assertEqual(docker.owned_node('worldbuilder-test-123-456'), 'database-id')

    def test_waits_until_actual_failure_even_after_early_client_exit(self):
        self.docker.api = Mock(side_effect=[{'Running': True, 'ExitCode': None}, {'Running': False, 'ExitCode': 7}])
        with patch.object(transport.time, 'sleep'):
            self.assertEqual(self.docker.wait('exec-id', 5), 7)
        self.assertEqual(self.docker.api.call_count, 2)

    def test_timeout_does_not_become_success(self):
        self.docker.api = Mock(return_value={'Running': True, 'ExitCode': None})
        with self.assertRaisesRegex(TimeoutError, 'still running'):
            self.docker.wait('exec-id', 0)

    def test_missing_terminal_code_is_not_success(self):
        self.docker.api = Mock(return_value={'Running': False, 'ExitCode': None})
        with self.assertRaises(RuntimeError): self.docker.wait('exec-id', 0)

    def test_truncated_output_is_refused(self):
        self.docker.capture = Mock(side_effect=[b'100', b'short'])
        with self.assertRaisesRegex(RuntimeError, 'Truncated'):
            self.docker.file('node', '/fixture/output', io.BytesIO())

    def test_complete_binary_output_crosses_chunk_boundary(self):
        expected = b'\x00' * 16384 + b'late'
        self.docker.capture = Mock(side_effect=[str(len(expected)).encode(), expected[:16384], expected[16384:]])
        out = io.BytesIO(); self.docker.file('node', '/fixture/output', out)
        self.assertEqual(out.getvalue(), expected)


if __name__ == '__main__': unittest.main()
