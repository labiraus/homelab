#!/usr/bin/env python3
"""Refuse a live/unowned container before any canon-index fixture command."""
import importlib.util
from pathlib import Path
import unittest
from unittest.mock import Mock

spec = importlib.util.spec_from_file_location('fixture', Path(__file__).with_name('test-canon-index.py'))
fixture = importlib.util.module_from_spec(spec); spec.loader.exec_module(fixture)


class Ownership(unittest.TestCase):
    def test_invalid_name_and_role_do_not_contact_docker(self):
        with self.assertRaises(ValueError): fixture.OwnedDocker('production')
        docker = fixture.OwnedDocker('wbindex-123456abcdef')
        docker.api = Mock()
        for container in ('production', 'wbindex-123456abcdef-unexpected', 'wbindex-123456abcdee-search'):
            with self.subTest(container=container), self.assertRaises(ValueError): docker.owned_node(container)
        docker.api.assert_not_called()

    def test_exact_name_requires_label_and_isolated_network(self):
        docker = fixture.OwnedDocker('wbindex-123456abcdef')
        data = {'Id': 'immutable-id', 'Config': {'Labels': {}}, 'HostConfig': {'NetworkMode': 'none'}}
        docker.api = Mock(return_value=data)
        with self.assertRaisesRegex(ValueError, 'label'): docker.owned_node(docker.name + '-search')
        data['Config']['Labels'][fixture.LABEL] = docker.name
        data['HostConfig']['NetworkMode'] = 'host'
        with self.assertRaisesRegex(ValueError, 'network'): docker.owned_node(docker.name + '-search')
        data['HostConfig']['NetworkMode'] = 'none'
        self.assertEqual(docker.owned_node(docker.name + '-search'), 'immutable-id')


if __name__ == '__main__': unittest.main()
