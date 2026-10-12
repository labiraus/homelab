#!/usr/bin/env python3
"""Refusal regressions for the public mesh artifact upload boundary."""
import base64
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('mesh', Path(__file__).with_name('test-mesh.py'))
mesh = importlib.util.module_from_spec(spec); spec.loader.exec_module(mesh)
KEY = '-----BEGIN PRIVATE KEY-----\nsynthetic-test-only\n-----END PRIVATE KEY-----'


class Artifacts(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix='worldbuilder-mesh-artifacts-')
        self.addCleanup(self.directory.cleanup)
        self.output = Path(self.directory.name)

    def test_redacted_failure_evidence_is_allowed(self):
        (self.output / 'render.yaml').write_text('kind: Secret\ndata:\n  ca.key: "<redacted>"\n')
        (self.output / 'mesh-result.json').write_text('{"status":"FAIL","ownedClusterRemoved":true}')
        (self.output / 'events.log').write_text('Fixture failed before readiness\n')
        mesh.public_artifacts(self.output)

    def test_both_secret_value_fields_require_redaction(self):
        for field in ('data', 'stringData'):
            with self.subTest(field=field):
                (self.output / 'render.yaml').write_text(json.dumps({'kind': 'Secret', field: {'key': 'synthetic'}}))
                with self.assertRaisesRegex(AssertionError, 'Unredacted Secret'):
                    mesh.public_artifacts(self.output)

    def test_encoded_key_outside_secret_is_refused(self):
        (self.output / 'unexpected.json').write_text(json.dumps({
            'users': [{'user': {'client-key-data': base64.b64encode(KEY.encode()).decode()}}]}))
        with self.assertRaisesRegex(AssertionError, 'Encoded private key'):
            mesh.public_artifacts(self.output)

    def test_plaintext_key_log_is_refused(self):
        (self.output / 'unexpected.log').write_text(KEY)
        with self.assertRaisesRegex(AssertionError, 'Private key'):
            mesh.public_artifacts(self.output)

    def test_missing_artifacts_are_not_success(self):
        with self.assertRaisesRegex(AssertionError, 'No mesh artifacts'):
            mesh.public_artifacts(self.output)


if __name__ == '__main__': unittest.main()
