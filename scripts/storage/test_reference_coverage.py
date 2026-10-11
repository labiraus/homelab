import copy
from collections import Counter
import hashlib
import json
from pathlib import Path
import tempfile
import subprocess
import sys
import unittest

import reference_coverage as coverage


class ReferenceCoverageTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.mapping = dict(schemaVersion=1, sourceEndpoint="http://source.invalid:9000",
                            destinationEndpoint="http://target.invalid:8333", sourceBucket="documents",
                            destinationBucket="documents", entries=[dict(key="../opaque/%2F key", sourceVersion="null",
                            destinationVersion="target-v1", aliases=["original-v1"], deleteMarker=False,
                            size=7, sha256=hashlib.sha256(b"fixture").hexdigest())])

    def save_export(self, refs):
        refs = [dict(table="rag.documents", locator={"id": 1}, **r) for r in refs]
        body = b"".join(coverage.encoded(r) for r in refs)
        (self.root / "references.jsonl").write_bytes(body)
        (self.root / "manifest.json").write_text(json.dumps(dict(schemaVersion=1,
            sourceEndpoint=self.mapping["sourceEndpoint"], sha256=hashlib.sha256(body).hexdigest(),
            extractorSHA256=hashlib.sha256(Path(coverage.__file__).read_bytes()).hexdigest(),
            referenceCounts=dict(Counter(r["kind"] for r in refs)), tableRows={t: 1 for t in coverage.REQUIRED},
            snapshot=dict(readOnly="on", isolation="repeatable read", tables=[dict(table=t) for t in coverage.REQUIRED]))))
        (self.root / "map.json").write_text(json.dumps(self.mapping))

    def check(self):
        return coverage.check(self.root, [self.root / "map.json"], self.root / "report.jsonl")

    def test_opaque_null_and_recovery_aliases(self):
        self.save_export(coverage.references(dict(bucket="documents", objectKey="../opaque/%2F key",
            metadata={"versionMarker": "null", "oldVersionMarker": "original-v1"}, processingVersion=19)))
        result = self.check()
        self.assertTrue(result["passed"])
        self.assertEqual(result["counts"], {"covered": 2})
        self.assertEqual((self.root / "report.jsonl").stat().st_mode & 0o777, 0o600)

    def test_missing_deleted_foreign_unversioned_and_missing_context(self):
        deleted = dict(self.mapping["entries"][0], sourceVersion="deleted", destinationVersion="target-delete",
                       deleteMarker=True, aliases=[])
        # Actual archive/import delete-marker entries have no body/size/hash.
        del deleted["size"], deleted["sha256"]
        self.mapping["entries"].append(deleted)
        refs = coverage.references({"bucket": "documents", "objectKey": "../opaque/%2F key", "items": [
            {"versionMarker": "missing"}, {"versionId": "deleted"}, {"version_marker": ""},
            {"bucket": "foreign", "objectKey": "../opaque/%2F key", "versionMarker": "null"},
            {"documentId": "different", "versionMarker": "null"}]})
        self.save_export(refs)
        result = self.check()
        self.assertFalse(result["passed"])
        self.assertEqual(result["counts"], dict(missing_version=1, delete_marker=1, invalid_version=1,
                                                unmapped_bucket=1, unresolved_context=1))

    def test_context_cannot_cross_siblings_or_conflicting_aliases(self):
        refs = coverage.references({"arguments": {"bucket": "documents", "objectKey": "key"},
                                    "result": {"versionMarker": "v1"}})
        self.assertIsNone(refs[0]["bucket"])
        refs = coverage.references(dict(bucket="documents", objectKey="k", nested=dict(bucket="other", versionId="v1")))
        self.assertIsNone(refs[0]["key"])
        refs = coverage.references(dict(bucket="documents", bucket_name="other", objectKey="k", versionId="v1"))
        self.assertIsNone(refs[0]["bucket"])

    def test_encoded_tool_results_nested_arrays_and_private_prose(self):
        value = [{"type": "text", "text": json.dumps({"bucket": "documents", "objectKey": "k",
                  "versionMarker": "v1", "body": "private story text"})}]
        refs = coverage.references(value)
        self.assertEqual(refs[0]["version"], "v1")
        self.assertIn("/$json/", refs[0]["path"])
        self.assertNotIn("private story", json.dumps(refs))
        self.assertEqual(coverage.references({"text": '{"versionMarker": bad}'})[0]["kind"], "unparsed_json")
        self.assertEqual(coverage.references({"text": '{"source_version": bad}'})[0]["kind"], "unparsed_json")
        self.assertEqual(coverage.references({"sourceUri": "https://store.invalid/k?versionId=v"})[0]["kind"], "versioned_uri")

    def test_duplicate_json_and_depth_are_not_silent(self):
        with self.assertRaises(ValueError):
            coverage.strict_json('{"versionMarker":"v1","versionMarker":"v2"}')
        with self.assertRaises(ValueError):
            coverage.references({"text": '{"versionMarker":"v1","versionMarker":"v2"}'})
        data = {}
        for _ in range(66):
            data = {"nested": data}
        with self.assertRaisesRegex(ValueError, "bound"):
            coverage.references(data)

    def test_map_collisions_identity_and_runtime_limits(self):
        for field, val in (("aliases", ["target-v1"]), ("aliases", ["null"]), ("size", -1),
                           ("size", True), ("sha256", "bad"), ("destinationVersion", "null")):
            mapping = copy.deepcopy(self.mapping)
            mapping["entries"][0][field] = val
            with self.subTest(field=field, value=val), self.assertRaises(ValueError):
                coverage.mapping_index(mapping, self.mapping["sourceEndpoint"])
        mapping = copy.deepcopy(self.mapping)
        mapping["entries"].append(mapping["entries"][0])
        with self.assertRaises(ValueError):
            coverage.mapping_index(mapping, self.mapping["sourceEndpoint"])
        with self.assertRaises(ValueError):
            coverage.mapping_index(self.mapping, "http://wrong.invalid")

    def test_tampered_reference_export_has_no_success_summary(self):
        self.save_export(coverage.references(dict(bucket="documents", objectKey="../opaque/%2F key", versionId="null")))
        with (self.root / "references.jsonl").open("ab") as file:
            file.write((self.root / "references.jsonl").read_bytes())
        with self.assertRaisesRegex(ValueError, "checksum"):
            self.check()
        self.assertNotIn('"kind":"summary"', (self.root / "report.jsonl").read_text())

    def test_optional_null_not_confused_with_literal_null(self):
        self.save_export(coverage.references(dict(bucket="documents", objectKey="../opaque/%2F key",
                                                  oldVersionMarker=None, versionMarker="null")))
        self.assertEqual(self.check()["counts"], dict(absent_optional_marker=1, covered=1))

    def test_cli_failure_exit_and_report_refuses_overwrite(self):
        self.save_export(coverage.references(dict(bucket="documents", objectKey="private-key", versionMarker="private-version")))
        command = [sys.executable, coverage.__file__, "check", str(self.root), "--map", str(self.root / "map.json"),
                   "--report", str(self.root / "report.jsonl")]
        result = subprocess.run(command, text=True, capture_output=True)
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertNotIn("private-key", result.stdout + result.stderr)
        first = (self.root / "report.jsonl").read_bytes()
        result = subprocess.run(command, text=True, capture_output=True)
        self.assertEqual(result.returncode, 1)
        self.assertEqual(first, (self.root / "report.jsonl").read_bytes())

    def test_reviewed_context_bound_to_export_without_rewriting_history(self):
        self.save_export(coverage.references(dict(documentId="historical", versionMarker="original-v1")))
        original = (self.root / "references.jsonl").read_bytes()
        self.assertFalse(self.check()["passed"])
        detail = json.loads((self.root / "report.jsonl").read_text().splitlines()[0])
        contexts = dict(schemaVersion=1, referenceSHA256=hashlib.sha256(original).hexdigest(),
                        reviewedBy="operator@example.test", entries=[dict(referenceID=detail["referenceID"],
                        bucket="documents", key="../opaque/%2F key", reason="Verified against the frozen source inventory")])
        path = self.root / "contexts.json"
        path.write_text(json.dumps(contexts))
        result = coverage.check(self.root, [self.root / "map.json"], self.root / "reviewed.jsonl", path)
        self.assertTrue(result["passed"])
        self.assertEqual(result["counts"], {"covered_with_reviewed_context": 1})
        self.assertEqual(original, (self.root / "references.jsonl").read_bytes())
        contexts["referenceSHA256"] = "0" * 64
        path.write_text(json.dumps(contexts))
        with self.assertRaisesRegex(ValueError, "bound"):
            coverage.check(self.root, [self.root / "map.json"], self.root / "stale.jsonl", path)
        contexts["referenceSHA256"] = hashlib.sha256(original).hexdigest()
        contexts["entries"][0]["referenceID"] = "0" * 64
        path.write_text(json.dumps(contexts))
        with self.assertRaisesRegex(ValueError, "absent"):
            coverage.check(self.root, [self.root / "map.json"], self.root / "unused.jsonl", path)

    def test_reviewed_context_cannot_reassign_known_bucket(self):
        self.save_export(coverage.references(dict(bucket="foreign", versionId="original-v1")))
        original = (self.root / "references.jsonl").read_bytes()
        path = self.root / "contexts.json"
        path.write_text(json.dumps(dict(schemaVersion=1, referenceSHA256=hashlib.sha256(original).hexdigest(),
            reviewedBy="operator", entries=[dict(referenceID=hashlib.sha256(original).hexdigest(),
            bucket="documents", key="../opaque/%2F key", reason="An incorrect guess")])) )
        with self.assertRaisesRegex(ValueError, "override"):
            coverage.check(self.root, [self.root / "map.json"], self.root / "conflicting.jsonl", path)


if __name__ == "__main__":
    unittest.main()
