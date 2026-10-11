import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import tempfile
import threading
import unittest
from unittest.mock import Mock, patch
from urllib.parse import parse_qs, quote, urlsplit
import xml.etree.ElementTree as ET

import boto3
from botocore.config import Config
import migrate_versions as migration
import version_order as order


def record(version, latest=False, delete=False, key="folder/café + %2F", modified="2026-10-11T00:00:00+00:00"):
    return dict(key=key, sourceVersion=version, latest=latest, deleteMarker=delete, modified=modified)


def wire(records, size, marker=("", ""), truncated=False, encoded=True):
    root = ET.Element("ListVersionsResult", xmlns="http://s3.amazonaws.com/doc/2006-03-01/")

    def add(parent, name, text):
        ET.SubElement(parent, name).text = str(text)

    encode = (lambda value: quote(value, safe="")) if encoded else (lambda value: value)
    for name, text in (("Name", "documents"), ("MaxKeys", size), ("KeyMarker", encode(marker[0])),
                       ("VersionIdMarker", marker[1]), ("IsTruncated", str(truncated).lower())):
        add(root, name, text)
    if encoded:
        add(root, "EncodingType", "url")
    for item in records:
        child = ET.SubElement(root, "DeleteMarker" if item["deleteMarker"] else "Version")
        for name, text in (("Key", encode(item["key"])), ("VersionId", item["sourceVersion"]),
                           ("IsLatest", str(item["latest"]).lower()), ("LastModified", item["modified"])):
            add(child, name, text)
    if truncated:
        add(root, "NextKeyMarker", encode(records[-1]["key"]))
        add(root, "NextVersionIdMarker", records[-1]["sourceVersion"])
    return ET.tostring(root, encoding="utf-8", xml_declaration=True)


class VersionOrderTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.records = [record("a+%2F", True), record("z-delete", delete=True), record("m-old")]

    def evidence(self, records=None):
        records = self.records if records is None else records
        scans = []
        for label, size in (("before", 2), ("after", 3)):
            marker, pages = ("", ""), []
            for start in range(0, max(len(records), 1), size):
                batch = records[start:start+size]
                body = wire(batch, size, marker, start + size < len(records))
                path = self.directory / f"order/{label}/{len(pages):08d}.xml"
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(body)
                pages.append(dict(file=path.relative_to(self.directory).as_posix(), size=len(body), sha256=hashlib.sha256(body).hexdigest()))
                if batch:
                    marker = batch[-1]["key"], batch[-1]["sourceVersion"]
            scans.append(dict(maxKeys=size, pages=pages))
        return dict(format=order.FORMAT, scans=scans)

    def test_interleaved_ties_cross_pages_without_sorting_opaque_ids(self):
        evidence = self.evidence()
        self.assertEqual(order.verify(self.directory, "documents", evidence), list(reversed(self.records)))

    def test_null_latest_and_middle_use_only_distinct_timestamps(self):
        newer = record("new", modified="2026-10-11T00:00:03+00:00")
        older = record("old", modified="2026-10-11T00:00:01+00:00")
        for timestamp, latest, expected in (("04", True, ["old", "new", "null"]),
                                            ("02", False, ["old", "null", "new"])):
            newer["latest"] = not latest
            null = record("null", latest=latest, modified=f"2026-10-11T00:00:{timestamp}+00:00")
            self.assertEqual([r["sourceVersion"] for r in order.chronological([newer, older, null])], expected)
        self.assertEqual(order.chronological([record("null", True)]), [record("null", True)])

    def test_null_tie_remains_unprovable(self):
        with self.assertRaisesRegex(ValueError, "Null version shares"):
            order.chronological([record("new", True), record("null")])

    def test_legacy_pagination_latest_duplicates_and_key_order_refused(self):
        for values in ([record("old"), record("new", True, modified="2026-10-11T00:00:01+00:00")],
                       [record("old")], [record("same", True), record("same")],
                       [record("a", True, key="z"), record("b", True, key="a")],
                       [record("first", True), record("second", True)]):
            with self.subTest(values=values), self.assertRaises(ValueError):
                order.chronological(values)

    def test_encoded_and_literal_keys_markers_and_version_ids(self):
        for encoded in (True, False):
            body = wire(self.records[:1], 1, truncated=True, encoded=encoded)
            values, marker = order.parse_page(body, "documents", 1, ("", ""))
            self.assertEqual(values, self.records[:1])
            self.assertEqual(marker, (self.records[0]["key"], "a+%2F"))
            next_body = wire(self.records[1:2], 1, marker, encoded=encoded)
            self.assertEqual(order.parse_page(next_body, "documents", 1, marker)[0], self.records[1:2])

    def test_timestamp_offsets_normalize_before_comparing_scans(self):
        entry = record("x", True, modified="2026-10-11T01:00:00+01:00")
        values, _ = order.parse_page(wire([entry], 1), "documents", 1, ("", ""))
        self.assertEqual(values[0]["modified"], "2026-10-11T00:00:00+00:00")
        with self.assertRaises(ValueError):
            order.stamp("2026-10-11T00:00:00")

    def test_malformed_scope_xml_and_pagination_are_refused(self):
        original = wire(self.records[:1], 1, truncated=True)
        for body in (original.replace(b"<Name>documents", b"<Name>wrong"),
                     original.replace(b"<MaxKeys>1", b"<MaxKeys>2"),
                     original.replace(b"<IsTruncated>true", b"<IsTruncated>yes"),
                     original.replace(b"<NextVersionIdMarker>a+%2F", b"<NextVersionIdMarker>wrong"),
                     original.replace(b"<IsLatest>true", b"<IsLatest>yes"),
                     original.replace(b"<EncodingType>url", b"<EncodingType>unknown"),
                     original.replace(b"</ListVersionsResult>", b"<CommonPrefixes/></ListVersionsResult>"),
                     original.replace(b"</ListVersionsResult>", b"<Name>documents</Name></ListVersionsResult>"),
                     original.replace(b"</ListVersionsResult>", b"<UnknownVersion/></ListVersionsResult>"),
                     original.replace(b"<KeyMarker />", b"<KeyMarker>wrong</KeyMarker>"),
                     original.replace(b"folder%2F", b"folder%ZZ"),
                     b'<!DOCTYPE x [<!ENTITY e "x">]>' + original):
            with self.subTest(body=body), self.assertRaises((ValueError, ET.ParseError)):
                order.parse_page(body, "documents", 1, ("", ""))
        with self.assertRaises(ValueError):
            order.parse_page(b"x" * (order.MAX_PAGE_BYTES + 1), "documents", 1, ("", ""))

    def test_missing_trailing_page_or_extra_final_page_refused(self):
        for operation in (lambda e: e["scans"][0]["pages"].pop(),
                          lambda e: e["scans"][1]["pages"].append(e["scans"][1]["pages"][0])):
            evidence = self.evidence()
            operation(evidence)
            with self.assertRaises(ValueError):
                order.verify(self.directory, "documents", evidence)

    def test_corrupt_or_escaping_evidence_refused(self):
        for name, value in (("file", "../outside.xml"), ("sha256", "0"*64), ("size", 0)):
            evidence = self.evidence()
            evidence["scans"][0]["pages"][0][name] = value
            with self.assertRaises(ValueError):
                order.verify(self.directory, "documents", evidence)

    def test_changed_source_refused(self):
        evidence = self.evidence()
        page = evidence["scans"][1]["pages"][0]
        body = (self.directory / page["file"]).read_bytes().replace(b"m-old", b"changed")
        (self.directory / page["file"]).write_bytes(body)
        page.update(size=len(body), sha256=hashlib.sha256(body).hexdigest())
        with self.assertRaisesRegex(ValueError, "changed"):
            order.verify(self.directory, "documents", evidence)

    def test_identical_scan_page_sizes_refused(self):
        evidence = self.evidence([])
        evidence["scans"][1]["maxKeys"] = 2
        page = evidence["scans"][1]["pages"][0]
        body = wire([], 2)
        (self.directory / page["file"]).write_bytes(body)
        page.update(size=len(body), sha256=hashlib.sha256(body).hexdigest())
        with self.assertRaisesRegex(ValueError, "distinct pagination"):
            order.verify(self.directory, "documents", evidence)

    def test_archive_tie_reordering_refused_before_any_destination_request(self):
        # Delete markers need no bodies; both kinds share the same chronology checker.
        values = [dict(r, deleteMarker=True) for r in self.records]
        evidence = self.evidence(values)
        manifest = dict(schemaVersion=2, sourceEndpoint="http://source.invalid:9000", sourceBucket="documents",
                        orderEvidence=evidence, entries=list(reversed(values)))
        path = self.directory / "manifest.json"
        path.write_text(json.dumps(manifest))
        self.assertEqual(migration.check_archive(self.directory), manifest)
        manifest["entries"][0], manifest["entries"][1] = manifest["entries"][1], manifest["entries"][0]
        path.write_text(json.dumps(manifest))
        client = Mock()
        with self.assertRaisesRegex(ValueError, "retained listing"):
            migration.import_archive(client, "documents", self.directory)
        self.assertEqual(client.mock_calls, [])

    def test_empty_bucket_has_complete_two_scan_evidence(self):
        self.assertEqual(order.verify(self.directory, "documents", self.evidence([])), [])

    def test_destination_intermediate_reorder_refused_even_when_latest_matches(self):
        values = list(reversed(self.records))
        mapping = dict(destinationEndpoint="http://destination.invalid:8333", destinationBucket="documents",
                       entries=[dict(r, destinationVersion=r["sourceVersion"], sourceVersion="old-" + r["sourceVersion"])
                                for r in values])
        values[0], values[1] = values[1], values[0]
        client = Mock()
        client.meta.endpoint_url = mapping["destinationEndpoint"]
        with patch.object(order, "capture", return_value=(values, {})), self.assertRaisesRegex(ValueError, "historical version order"):
            migration.verify_copy(client, "documents", mapping)
        client.get_object.assert_not_called()

    def test_real_sdk_captures_raw_mixed_xml_and_exact_marker_requests(self):
        records, requests = self.records, []

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                query = parse_qs(urlsplit(self.path).query, keep_blank_values=True)
                requests.append(query)
                size = int(query["max-keys"][0])
                marker = query.get("key-marker", [""])[0], query.get("version-id-marker", [""])[0]
                start = 0 if marker == ("", "") else next(i + 1 for i, r in enumerate(records)
                         if (r["key"], r["sourceVersion"]) == marker)
                body = wire(records[start:start+size], size, marker, start + size < len(records))
                self.send_response(200)
                self.send_header("Content-Type", "application/xml")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, *args):
                pass

        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            client = boto3.client("s3", endpoint_url=f"http://127.0.0.1:{server.server_port}", region_name="us-east-1",
                                  aws_access_key_id="synthetic", aws_secret_access_key="synthetic-secret",
                                  config=Config(retries={"max_attempts": 0}, proxies={}, s3={"addressing_style": "path"}))
            scans = []
            for label, size in (("before", 2), ("after", 3)):
                values, scan = order.capture(client, "documents", self.directory, label, size)
                self.assertEqual(values, list(reversed(records)))
                scans.append(scan)
            self.assertEqual(order.verify(self.directory, "documents", dict(format=order.FORMAT, scans=scans)), list(reversed(records)))
            self.assertEqual(requests[1]["key-marker"], [records[1]["key"]])
            self.assertEqual(requests[1]["version-id-marker"], [records[1]["sourceVersion"]])
            self.assertTrue(all(r["encoding-type"] == ["url"] for r in requests))
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=5)


if __name__ == "__main__":
    unittest.main()
