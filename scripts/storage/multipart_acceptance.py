"""Multipart scenarios for the fixed-loopback SeaweedFS harness only."""
import json
import resource
import shutil
from unittest.mock import patch


def archive_fixture(migration, directory, bucket, size):
    directory.mkdir()
    (directory / "objects").mkdir()
    path = directory / "objects/00000000.bin"
    with path.open("wb") as body:
        body.truncate(size)  # Sparse source avoids a second resident 5 GiB buffer.
        for number, offset in enumerate(range(0, size, migration.DEFAULT_PART_SIZE)):
            body.seek(offset)
            body.write(f"part-{number:06d}".encode()[:size-offset])
        body.seek(size - 17)
        body.write(b"Z" * 17)
    with path.open("rb") as body:
        checksum, actual_size = migration.digest(body)
    record = dict(key="multipart/large opaque.bin", sourceVersion="offline-source-v1", deleteMarker=False,
                  latest=True, modified="2026-10-11T00:00:00+00:00", file="objects/00000000.bin",
                  sha256=checksum, size=actual_size, metadata={"proof": "retained"},
                  tags=[dict(Key="label", Value="space / +")], headers={"ContentType": "application/octet-stream",
                  "CacheControl": "private, max-age=60", "ContentDisposition": 'attachment; filename="large.bin"',
                  "ContentLanguage": "en"})
    migration.write_json(directory / "manifest.json", dict(schemaVersion=1,
                         sourceEndpoint="http://offline-source.invalid:9000", sourceBucket=bucket, entries=[record]))
    return record


def uploads(client, bucket):
    return [u for page in client.get_paginator("list_multipart_uploads").paginate(Bucket=bucket)
            for u in page.get("Uploads", [])]


class LostReply:
    """Forward a real mutation, then simulate losing its successful response."""
    def __init__(self, client, method):
        self.client, self.method, self.result = client, method, None

    def __getattr__(self, name):
        target = getattr(self.client, name)
        if name != self.method:
            return target

        def lost(**kwargs):
            self.result = target(**kwargs)
            raise ConnectionError("Synthetic lost response after successful server write")
        return lost


def run(client, base, migration, large=False):
    for stage, method in (("part", "upload_part"), ("complete", "complete_multipart_upload"), ("initiate", "create_multipart_upload")):
        bucket = "multipart-failure-" + stage
        client.create_bucket(Bucket=bucket)
        client.put_bucket_versioning(Bucket=bucket, VersioningConfiguration={"Status": "Enabled"})
        directory = base / bucket
        record = archive_fixture(migration, directory, bucket, migration.MIN_PART_SIZE + 17)
        fault = LostReply(client, method)
        with patch.object(migration, "MULTIPART_THRESHOLD", 1), patch.object(migration, "DEFAULT_PART_SIZE", migration.MIN_PART_SIZE):
            try:
                migration.import_archive(fault, bucket, directory)
            except ConnectionError:
                pass
            else:
                raise AssertionError("Lost response was treated as completed migration")
        assert not (directory / "version-map.json").exists()
        events = [json.loads(line) for line in (directory / "multipart-journal.jsonl").read_text().splitlines()]
        failed = next(e for e in events if e["state"] == "failed")
        assert failed["stage"] == stage
        assert failed["completionUncertain"] == (stage == "complete")
        assert failed["initiationUncertain"] == (stage == "initiate")
        if stage == "initiate":
            assert uploads(client, bucket), "Unknown initiated upload was silently discarded"
            # Only this test knows the dropped ID. The migration must not guess it.
            client.abort_multipart_upload(Bucket=bucket, Key=record["key"], UploadId=fault.result["UploadId"])
        elif stage == "complete":
            body = client.get_object(Bucket=bucket, Key=record["key"], VersionId=fault.result["VersionId"])["Body"]
            try:
                assert migration.digest(body) == (record["sha256"], record["size"])
            finally:
                body.close()
            assert failed["completionUncertain"], "Abort erased evidence of a committed object"
        else:
            assert not migration.inventory(client, bucket)
        assert not uploads(client, bucket), "Known-ID cleanup left an unfinished upload"
        try:
            migration.import_archive(client, bucket, directory)
        except ValueError as error:
            assert "import evidence" in str(error)
        else:
            raise AssertionError("Uncertain archive was automatically retried")
    print("PASS: real multipart failure cleanup, unknown initiation and committed-but-unacknowledged completion", flush=True)
    if not large:
        return
    if shutil.disk_usage(base).free < 12 * 1024**3:
        raise RuntimeError("Large-object round trip requires 12 GiB free temporary disk; not skipped")
    bucket = "multipart-large"
    client.create_bucket(Bucket=bucket)
    client.put_bucket_versioning(Bucket=bucket, VersioningConfiguration={"Status": "Enabled"})
    directory = base / "large-archive"
    size = 5 * 1024**3 + 17
    record = archive_fixture(migration, directory, bucket, size)
    assert record["size"] > 5 * 1024**3
    print(f"Large-object fixture: {size} bytes with distinct part-boundary content", flush=True)
    mapping = migration.import_archive(client, bucket, directory)
    assert mapping["entries"][0]["size"] == size
    assert not uploads(client, bucket)
    events = [json.loads(line) for line in (directory / "multipart-journal.jsonl").read_text().splitlines()]
    parts = [e for e in events if e["state"] == "part_attempt"]
    assert len(parts) == 81 and sum(p["size"] for p in parts) == size
    assert [p["partNumber"] for p in parts] == list(range(1, 82))
    assert events[-1]["state"] == "complete_acknowledged"
    print("PASS: actual >5 GiB import, 81 streamed parts, immutable version, complete SHA-256/metadata/tags verification", flush=True)
    # Exercise the real export path too, not just a generated archive or HEAD size.
    recovered = base / "large-reexport"
    exported = migration.export_archive(client, bucket, recovered)
    migration.check_archive(recovered)
    assert len(exported["entries"]) == 1
    assert (exported["entries"][0]["sha256"], exported["entries"][0]["size"]) == (record["sha256"], size)
    assert exported["entries"][0]["sourceVersion"] == mapping["entries"][0]["destinationVersion"]
    peak_kib = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss
    assert peak_kib < 256 * 1024, f"Client memory not bounded: {peak_kib} KiB"
    print(f"PASS: >5 GiB S3 re-export/archive validation; Python peak RSS {peak_kib} KiB (<256 MiB)", flush=True)
