#!/usr/bin/env python3
"""Refuse incomplete recovery evidence and unintended fixture scopes."""
import importlib.util
from pathlib import Path
import unittest
from unittest.mock import Mock, patch

spec = importlib.util.spec_from_file_location('velero', Path(__file__).with_name('test-velero.py'))
velero = importlib.util.module_from_spec(spec); spec.loader.exec_module(velero)


class VeleroTests(unittest.TestCase):
    def status(self, **values):
        return {'kind': 'Restore', 'metadata': {'name': 'fixture'}, 'status': values}

    def test_incomplete_and_partial_restores_cannot_pass(self):
        for phase in ('New', 'InProgress', 'Finalizing'):
            self.assertFalse(velero.completed(self.status(phase=phase)))
        for phase in ('Failed', 'PartiallyFailed', 'FailedValidation'):
            with self.subTest(phase=phase), self.assertRaises(AssertionError):
                velero.completed(self.status(phase=phase))

    def test_completed_restore_with_warnings_or_errors_is_refused(self):
        for values in ({'warnings': 1}, {'errors': 1}, {'warnings': 1, 'errors': 1}):
            with self.subTest(values=values), self.assertRaises(AssertionError):
                velero.completed(self.status(phase='Completed', **values))
        self.assertTrue(velero.completed(self.status(phase='Completed', warnings=0, errors=0)))

    def test_unsupported_hostpath_is_not_volume_backup_evidence(self):
        pvc = {'metadata': {'uid': 'owned-pvc'}, 'spec': {'volumeName': 'owned-pv'}}
        for storage in ({'hostPath': {'path': '/fixture'}}, {'local': {'path': '/fixture'}, 'hostPath': {}}):
            with patch.object(velero, 'get', side_effect=[pvc, {'spec': storage}]), self.assertRaises(AssertionError):
                velero.volume(Mock(), velero.SOURCE)

    def test_wrong_pvc_binding_is_refused(self):
        pvc = {'metadata': {'uid': 'owned-pvc'}, 'spec': {'volumeName': 'owned-pv'}}
        pv = {'spec': {'local': {'path': '/fixture'}, 'claimRef': {'uid': 'different-pvc'}}}
        with patch.object(velero, 'get', side_effect=[pvc, pv]), self.assertRaises(AssertionError):
            velero.volume(Mock(), velero.SOURCE)

    def test_restore_scope_cannot_select_an_unowned_namespace(self):
        with self.assertRaises(ValueError): velero.restore_spec('fixture-restore', 'production')
        with self.assertRaises(ValueError): velero.restore_spec('arbitrary', velero.TARGET)
        self.assertEqual(velero.restore_spec('fixture-restore', velero.TARGET)['spec']['namespaceMapping'],
                         {velero.SOURCE: velero.TARGET})

    def test_unknown_storage_consumer_is_refused_before_access(self):
        fixture = Mock()
        with self.assertRaises(ValueError): velero.Storage(fixture, consumer='production')
        fixture.assert_not_called()

    def test_unrelated_restore_failure_cannot_prove_credential_refusal(self):
        with self.assertRaises(AssertionError):
            velero.credential_refused(self.status(phase='Failed', failureReason='invalid namespace mapping'))
        with self.assertRaises(AssertionError): velero.credential_refused(self.status(phase='Completed'))
        self.assertFalse(velero.credential_refused(self.status(phase='InProgress')))
        self.assertTrue(velero.credential_refused(self.status(phase='FailedValidation',
            validationErrors=['The BSL default is unavailable, cannot retrieve the backup'])))


if __name__ == '__main__': unittest.main()
