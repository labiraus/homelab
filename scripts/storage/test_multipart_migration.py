import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock, patch

import migrate_versions as migration


class MultipartMigrationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name)
        self.file = (self.path / "body").open("w+b")
        self.addCleanup(self.file.close)
        self.file.write(b"prefix-" + b"content" * 1000000 + b"-tail")
        self.file.flush()
        self.record = dict(size=self.file.tell(), sourceVersion="old-v1", sha256="a" * 64,
                           headers={"ContentType": "application/octet-stream"}, metadata={"origin": "kept"},
                           tags=[{"Key": "label", "Value": "spaces / +"}])
        self.journal = (self.path / "journal.jsonl").open("w+")
        self.addCleanup(self.journal.close)
        self.client = Mock()
        self.client.create_multipart_upload.return_value = {"UploadId": "upload"}
        self.client.upload_part.return_value = {"ETag": '"part-etag"'}
        self.client.complete_multipart_upload.return_value = {"VersionId": "new-v1"}

    def events(self):
        self.journal.flush()
        return [json.loads(line) for line in (self.path / "journal.jsonl").read_text().splitlines()]

    def upload(self):
        with patch.object(migration, "DEFAULT_PART_SIZE", migration.MIN_PART_SIZE):
            return migration.multipart_upload(self.client, dict(Bucket="documents", Key="opaque/%2F key"),
                                              self.record, self.file, self.journal)

    def test_part_windows_are_bounded_rewindable_and_exact(self):
        with migration.FilePart(self.file, 7, 7000000) as part:
            self.assertEqual(part.read(7), b"content")
            self.assertEqual(part.seek(-7, io.SEEK_END), 6999993)
            self.assertEqual(part.read(100), b"content")
            self.assertEqual(part.read(1), b"")
            part.seek(0)
            self.assertEqual(len(part.read(1000000000)), migration.READ_SIZE)
            with self.assertRaises(io.UnsupportedOperation):
                part.read()
            for offset in (-1, 7000001):
                with self.assertRaises(ValueError):
                    part.seek(offset)

    def test_protocol_part_sizing_and_final_part(self):
        for size in (0, 1, 5*1024**3+17, 5*1024**4, migration.MAX_PART_SIZE*migration.MAX_PARTS):
            part = migration.multipart_part_size(size)
            self.assertLessEqual((size + part - 1)//part, 10000)
            self.assertGreaterEqual(part, migration.MIN_PART_SIZE)
            self.assertLessEqual(part, migration.MAX_PART_SIZE)
        for size in (-1, True, 1.5, migration.MAX_PART_SIZE*migration.MAX_PARTS+1):
            with self.assertRaises(ValueError):
                migration.multipart_part_size(size)

    def test_upload_streams_exact_parts_and_preserves_metadata(self):
        chunks, lengths = [], []

        def uploaded(**kwargs):
            part = kwargs["Body"]
            lengths.append(kwargs["ContentLength"])
            # Signing/checksum rewind must not read bytes outside this part.
            checksum, size = migration.digest(part)
            self.assertEqual(size, kwargs["ContentLength"])
            part.seek(0)
            content = bytearray()
            while piece := part.read(65536):
                content.extend(piece)
            chunks.append(bytes(content))
            return {"ETag": checksum}

        self.client.upload_part.side_effect = uploaded
        self.assertEqual(self.upload()["VersionId"], "new-v1")
        self.file.seek(0)
        self.assertEqual(b"".join(chunks), self.file.read())
        self.assertEqual(lengths[0], migration.MIN_PART_SIZE)
        self.assertLess(lengths[1], migration.MIN_PART_SIZE)
        created = self.client.create_multipart_upload.call_args.kwargs
        self.assertEqual(created["Metadata"], self.record["metadata"])
        self.assertEqual(created["Tagging"], "label=spaces+%2F+%2B")
        self.assertEqual(self.events()[-1]["state"], "complete_acknowledged")
        self.client.abort_multipart_upload.assert_not_called()

    def test_part_failure_aborts_known_upload_and_never_completes(self):
        self.client.upload_part.side_effect = ConnectionError("lost part response")
        with self.assertRaises(ConnectionError):
            self.upload()
        self.client.abort_multipart_upload.assert_called_once()
        self.client.complete_multipart_upload.assert_not_called()
        failed = next(e for e in self.events() if e["state"] == "failed")
        self.assertFalse(failed["completionUncertain"])
        self.assertEqual(self.events()[-1]["state"], "abort_acknowledged")

    def test_lost_completion_retains_uncertainty_even_after_abort_ack(self):
        self.client.complete_multipart_upload.side_effect = ConnectionError("lost completion response")
        with self.assertRaises(ConnectionError):
            self.upload()
        states = [e["state"] for e in self.events()]
        self.assertIn("complete_attempt", states)
        self.assertNotIn("complete_acknowledged", states)
        self.assertTrue(next(e for e in self.events() if e["state"] == "failed")["completionUncertain"])

    def test_lost_initiation_cannot_guess_an_upload_to_abort(self):
        self.client.create_multipart_upload.side_effect = ConnectionError("lost initiation response")
        with self.assertRaises(ConnectionError):
            self.upload()
        self.client.abort_multipart_upload.assert_not_called()
        self.assertTrue(self.events()[-1]["initiationUncertain"])
        self.assertIsNone(self.events()[-1]["uploadId"])

    def test_abort_failure_retains_original_error_and_journal(self):
        self.client.upload_part.side_effect = ConnectionError("original failure")
        self.client.abort_multipart_upload.side_effect = RuntimeError("cleanup unavailable")
        with self.assertRaisesRegex(ConnectionError, "original failure"):
            self.upload()
        self.assertEqual(self.events()[-1]["state"], "abort_unconfirmed")

    def test_journal_disk_failure_after_initiation_still_aborts(self):
        real = migration.journal_event

        def failing(output, **event):
            if event["state"] != "initiate_attempt":
                raise OSError("disk full")
            real(output, **event)

        with patch.object(migration, "journal_event", failing), self.assertRaises(OSError):
            self.upload()
        self.client.abort_multipart_upload.assert_called_once()
        self.client.upload_part.assert_not_called()

    def test_truncated_file_fails_before_sending_incomplete_part(self):
        self.file.truncate(1)
        self.client.upload_part.side_effect = lambda **kw: migration.digest(kw["Body"])
        with self.assertRaisesRegex(ValueError, "truncated"):
            self.upload()
        self.client.abort_multipart_upload.assert_called_once()


if __name__ == "__main__":
    unittest.main()
