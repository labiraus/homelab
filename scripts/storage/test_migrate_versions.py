import copy
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock

spec = importlib.util.spec_from_file_location("migration", Path(__file__).with_name("migrate_versions.py"))
migration = importlib.util.module_from_spec(spec)
spec.loader.exec_module(migration)


class ArchiveTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        (self.directory / "objects").mkdir()
        (self.directory / "objects/00000000.bin").write_bytes(b"original")
        self.manifest = dict(schemaVersion=1, sourceEndpoint="http://source.invalid:9000", sourceBucket="documents",
                             entries=[dict(key="../opaque/object key", sourceVersion="null", deleteMarker=False,
                                           latest=True, modified="2026-10-11T00:00:00+00:00", file="objects/00000000.bin",
                                           sha256=hashlib.sha256(b"original").hexdigest(), size=8,
                                           headers={"ContentType": "text/plain"}, metadata={}, tags=[])])
        self.save()

    def save(self):
        (self.directory / "manifest.json").write_text(json.dumps(self.manifest))

    def test_valid_null_version_and_opaque_key(self):
        self.assertEqual(migration.check_archive(self.directory), self.manifest)

    def test_tamper_prevents_any_destination_request(self):
        (self.directory / "objects/00000000.bin").write_bytes(b"tampered")
        client = Mock()
        with self.assertRaisesRegex(ValueError, "checksum"):
            migration.import_archive(client, "documents", self.directory)
        self.assertEqual(client.mock_calls, [])

    def test_rejects_invalid_archive_before_writes(self):
        for field, value in (("file", "../secret"), ("size", -1), ("sha256", "0" * 64),
                             ("headers", {"ACL": "public-read"}), ("latest", False)):
            with self.subTest(field=field):
                original = copy.deepcopy(self.manifest)
                self.manifest["entries"][0][field] = value
                self.save()
                with self.assertRaises(ValueError):
                    migration.check_archive(self.directory)
                self.manifest = original

    def test_duplicate_version_rejected(self):
        self.manifest["entries"].append(copy.deepcopy(self.manifest["entries"][0]))
        self.save()
        with self.assertRaises(ValueError):
            migration.check_archive(self.directory)

    def test_ambiguous_source_order_rejected(self):
        client = Mock()
        stamp = datetime(2026, 10, 11, tzinfo=timezone.utc)
        client.get_paginator.return_value.paginate.return_value = [{"Versions": [
            dict(Key="key", VersionId="v1", IsLatest=False, LastModified=stamp),
            dict(Key="key", VersionId="v2", IsLatest=True, LastModified=stamp),
        ]}]
        with self.assertRaisesRegex(ValueError, "Ambiguous"):
            migration.inventory(client, "documents")

    def test_nonempty_destination_refused_without_writes(self):
        client = Mock()
        client.meta.endpoint_url = "http://destination.invalid:8333"
        client.get_bucket_versioning.return_value = {"Status": "Enabled"}
        client.get_paginator.return_value.paginate.return_value = [{"DeleteMarkers": [
            dict(Key="deleted", VersionId="delete-version", IsLatest=True,
                 LastModified=datetime(2026, 10, 11, tzinfo=timezone.utc)),
        ]}]
        with self.assertRaisesRegex(ValueError, "Destination must be empty"):
            migration.import_archive(client, "documents", self.directory)
        client.put_object.assert_not_called()
        client.delete_object.assert_not_called()

    def test_source_destination_identity_refused(self):
        client = Mock()
        client.meta.endpoint_url = self.manifest["sourceEndpoint"]
        with self.assertRaisesRegex(ValueError, "must differ"):
            migration.import_archive(client, "documents", self.directory)
        self.assertEqual(client.mock_calls, [])

    def test_endpoint_rejects_credentials_and_path(self):
        for value in ("file:///tmp", "http://user:secret@example.com", "http://example.com/bucket", "http://example.com?key=x"):
            with self.assertRaises(ValueError):
                migration.endpoint(value)

    def test_recovery_composes_original_and_intermediate_references(self):
        prior = dict(schemaVersion=1, destinationEndpoint=self.manifest["sourceEndpoint"],
                     destinationBucket="documents", entries=[dict(self.manifest["entries"][0],
                     sourceVersion="original-id", destinationVersion="null", aliases=["earliest-id"])])
        composed = migration.compose_history(self.manifest, prior)
        self.assertEqual(composed["entries"][0]["aliases"], ["earliest-id", "original-id"])
        self.assertNotIn("aliases", self.manifest["entries"][0])
        for field, value in (("destinationVersion", "missing-id"), ("sha256", "0" * 64)):
            broken = copy.deepcopy(prior)
            broken["entries"][0][field] = value
            with self.assertRaises(ValueError):
                migration.compose_history(self.manifest, broken)


if __name__ == "__main__":
    unittest.main()
