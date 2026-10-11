#!/usr/bin/env python3
"""Offline-verifiable, version-preserving archive/copy. Never deletes source data.

Use a quiesced source and an empty, version-enabled destination. AWS credentials
come from the normal SDK chain; archives contain private data, never credentials.
"""
import argparse
import copy
from collections import defaultdict
import hashlib
import io
import json
import os
import tempfile
from pathlib import Path
from urllib.parse import urlencode, urlsplit

import boto3
from botocore.config import Config
import version_order

HEADERS = ("ContentType", "CacheControl", "ContentDisposition", "ContentEncoding", "ContentLanguage")
MIB = 1024**2
MULTIPART_THRESHOLD = 64 * MIB
DEFAULT_PART_SIZE = 64 * MIB
MIN_PART_SIZE = 5 * MIB
MAX_PART_SIZE = 5 * 1024**3
MAX_PARTS = 10000
READ_SIZE = MIB


def multipart_part_size(size):
    if type(size) is not int or size < 0 or size > MAX_PART_SIZE * MAX_PARTS:
        raise ValueError("Object size exceeds multipart protocol bounds")
    # Round up to MiB so every non-final part respects the minimum and no
    # object requires more than 10,000 parts. This is not a provider capacity claim.
    return max(DEFAULT_PART_SIZE, ((size + MAX_PARTS * MIB - 1) // (MAX_PARTS * MIB)) * MIB)


class FilePart(io.RawIOBase):
    """Seekable window for SDK signing/checksums; never buffers a whole part."""
    def __init__(self, file, offset, size):
        self.file, self.offset, self.size, self.position = file, offset, size, 0

    def readable(self):
        return True

    def seekable(self):
        return True

    def tell(self):
        return self.position

    def seek(self, offset, whence=io.SEEK_SET):
        bases = {io.SEEK_SET: 0, io.SEEK_CUR: self.position, io.SEEK_END: self.size}
        if whence not in bases:
            raise ValueError("Invalid seek mode")
        position = bases[whence] + offset
        if not 0 <= position <= self.size:
            raise ValueError("Seek outside multipart file window")
        self.position = position
        return position

    def read(self, size=-1):
        if self.closed:
            raise ValueError("Read from closed multipart window")
        if size is None or size < 0:
            raise io.UnsupportedOperation("Unbounded multipart reads are forbidden")
        size = min(size, READ_SIZE, self.size - self.position)
        if not size:
            return b""
        self.file.seek(self.offset + self.position)
        body = self.file.read(size)
        if len(body) != size:
            raise ValueError("Archive body changed or truncated during multipart upload")
        self.position += len(body)
        return body


def journal_event(output, **event):
    output.write(json.dumps(event) + "\n")
    output.flush()
    os.fsync(output.fileno())


def multipart_upload(client, request, record, file, journal):
    """One sequential upload; failures preserve uncertainty and abort known IDs."""
    identity = dict(bucket=request["Bucket"], key=request["Key"], sourceVersion=record["sourceVersion"])
    upload_id, stage = None, "initiate"

    def event(state, **detail):
        journal_event(journal, **identity, state=state, **detail)

    def recovery_event(state, **detail):
        # A full/broken journal must not prevent an attempted known-ID abort.
        try:
            event(state, **detail)
        except Exception:
            pass

    try:
        part_size = multipart_part_size(record["size"])
        event("initiate_attempt", size=record["size"], sha256=record["sha256"], partSize=part_size)
        response = client.create_multipart_upload(**request, **record["headers"], Metadata=record["metadata"],
                                                 Tagging=urlencode({t["Key"]: t["Value"] for t in record["tags"]}))
        candidate = response.get("UploadId")
        if not isinstance(candidate, str) or not candidate:
            raise ValueError("Multipart initiation returned no upload ID")
        upload_id = candidate
        event("initiated", uploadId=upload_id)
        stage, parts = "part", []
        for number, offset in enumerate(range(0, record["size"], part_size), start=1):
            size = min(part_size, record["size"] - offset)
            event("part_attempt", uploadId=upload_id, partNumber=number, offset=offset, size=size)
            with FilePart(file, offset, size) as body:
                response = client.upload_part(**request, UploadId=upload_id, PartNumber=number, Body=body, ContentLength=size)
            etag = response.get("ETag")
            if not isinstance(etag, str) or not etag:
                raise ValueError("Multipart part returned no ETag")
            parts.append(dict(PartNumber=number, ETag=etag))
            event("part_acknowledged", uploadId=upload_id, partNumber=number, etag=etag)
        stage = "complete"
        event("complete_attempt", uploadId=upload_id, parts=len(parts))
        response = client.complete_multipart_upload(**request, UploadId=upload_id, MultipartUpload={"Parts": parts})
        version = response.get("VersionId")
        if not isinstance(version, str) or not version or version == "null":
            raise ValueError("Multipart completion returned no immutable version ID")
        event("complete_acknowledged", uploadId=upload_id, destinationVersion=version)
        return response
    except BaseException as error:
        recovery_event("failed", uploadId=upload_id, stage=stage, errorType=type(error).__name__,
                       initiationUncertain=stage == "initiate", completionUncertain=stage == "complete")
        if upload_id:
            recovery_event("abort_attempt", uploadId=upload_id)
            try:
                client.abort_multipart_upload(**request, UploadId=upload_id)
            except Exception as abort_error:
                recovery_event("abort_unconfirmed", uploadId=upload_id, errorType=type(abort_error).__name__)
            else:
                recovery_event("abort_acknowledged", uploadId=upload_id)
        raise


def endpoint(value):
    parsed = urlsplit(value)
    if (parsed.scheme not in ("http", "https") or not parsed.hostname or parsed.username
            or parsed.password or parsed.query or parsed.fragment or parsed.path not in ("", "/")):
        raise ValueError("Endpoint must be an HTTP(S) origin without credentials or a path")
    return value.rstrip("/")


def inventory(client, bucket, validate_order=True):
    records = []
    for page in client.get_paginator("list_object_versions").paginate(Bucket=bucket):
        for kind in ("Versions", "DeleteMarkers"):
            for item in page.get(kind, []):
                records.append(dict(key=item["Key"], sourceVersion=item["VersionId"],
                                    deleteMarker=kind == "DeleteMarkers", latest=item["IsLatest"],
                                    modified=item["LastModified"].isoformat()))
    records.sort(key=lambda r: (r["key"], r["modified"], r["sourceVersion"]))
    groups = defaultdict(list)
    seen = set()
    for record in records:
        identity = record["key"], record["sourceVersion"]
        if identity in seen:
            raise ValueError("Duplicate version in inventory; freeze writers before export")
        seen.add(identity)
        groups[record["key"]].append(record)
    for versions in groups.values() if validate_order else []:
        if len({r["modified"] for r in versions}) != len(versions):
            raise ValueError("Ambiguous equal version timestamps; resolve source ordering before migration")
        if sum(r["latest"] for r in versions) != 1 or not versions[-1]["latest"]:
            raise ValueError("Inconsistent latest version; freeze writers before export")
    return records


def digest(stream, output=None):
    checksum, size = hashlib.sha256(), 0
    while chunk := stream.read(1024 * 1024):
        checksum.update(chunk)
        size += len(chunk)
        if output is not None:
            output.write(chunk)
    return checksum.hexdigest(), size


def write_json(path, value):
    with path.open("x", encoding="utf-8") as output:
        json.dump(value, output, ensure_ascii=False, indent=2)
        output.write("\n")
        output.flush()
        os.fsync(output.fileno())


def export_archive(client, bucket, directory, page_sizes=(1000, 127), listing_dialect="s3"):
    directory = Path(directory)
    directory.mkdir(mode=0o700)  # Never merge/overwrite an existing export.
    (directory / "objects").mkdir(mode=0o700)
    if len(page_sizes) != 2 or page_sizes[0] == page_sizes[1]:
        raise ValueError("Export requires distinct listing page sizes")
    records, before = version_order.capture(client, bucket, directory, "before", page_sizes[0], listing_dialect)
    original = json.loads(json.dumps(records))
    for number, record in enumerate(records):
        if record["deleteMarker"]:
            continue
        request = dict(Bucket=bucket, Key=record["key"], VersionId=record["sourceVersion"])
        response = client.get_object(**request)
        try:
            if any(response.get(name) for name in ("ObjectLockMode", "ObjectLockRetainUntilDate",
                                                    "ObjectLockLegalHoldStatus", "SSECustomerAlgorithm")):
                raise ValueError("Object lock/customer encryption needs a dedicated migration")
            if response.get("StorageClass", "STANDARD") != "STANDARD":
                raise ValueError("Non-standard storage class needs a dedicated migration")
            record["headers"] = {name: response[name] for name in HEADERS if name in response}
            if "Expires" in response:
                record["headers"]["Expires"] = response["Expires"].isoformat()
            record["metadata"] = response.get("Metadata", {})
            record["tags"] = client.get_object_tagging(**request).get("TagSet", [])
            record["file"] = f"objects/{number:08d}.bin"
            with (directory / record["file"]).open("xb") as output:
                record["sha256"], record["size"] = digest(response["Body"], output)
            if record["size"] != response["ContentLength"]:
                raise ValueError("Truncated source body")
        finally:
            response["Body"].close()
    final, after = version_order.capture(client, bucket, directory, "after", page_sizes[1], listing_dialect)
    if final != original:
        raise ValueError("Source changed during export; incomplete archive must not be imported")
    manifest = dict(schemaVersion=2, sourceEndpoint=endpoint(client.meta.endpoint_url),
                    sourceBucket=bucket, entries=records,
                    orderEvidence=dict(format=version_order.FORMAT, scans=[before, after]))
    write_json(directory / "manifest.json", manifest)
    return manifest


def check_archive(directory):
    directory = Path(directory)
    manifest = json.loads((directory / "manifest.json").read_text())
    if manifest.get("schemaVersion") not in (1, 2) or not isinstance(manifest.get("entries"), list):
        raise ValueError("Unsupported archive format")
    endpoint(manifest["sourceEndpoint"])
    if manifest["schemaVersion"] == 2:
        proven = version_order.verify(directory, manifest["sourceBucket"], manifest.get("orderEvidence"))
        declared = [{field: record[field] for field in version_order.FIELDS} for record in manifest["entries"]]
        if declared != proven:
            raise ValueError("Archive versions/order differ from retained listing evidence")
    identities, timestamps, groups = set(), set(), defaultdict(list)
    for record in manifest["entries"]:
        identity = record["key"], record["sourceVersion"]
        stamp = record["key"], record["modified"]
        if (not all(isinstance(v, str) and v for v in identity) or identity in identities
                or (manifest["schemaVersion"] == 1 and stamp in timestamps)):
            raise ValueError("Invalid/duplicate source version or ambiguous ordering")
        identities.add(identity)
        timestamps.add(stamp)
        groups[record["key"]].append(record)
        if not isinstance(record["deleteMarker"], bool) or not isinstance(record["latest"], bool):
            raise ValueError("Invalid version flags")
        if not record["deleteMarker"]:
            multipart_part_size(record["size"])
            body = (directory / record["file"]).resolve()
            if body.parent != (directory / "objects").resolve():
                raise ValueError("Archive object escapes objects directory")
            with body.open("rb") as stream:
                checksum, size = digest(stream)
            if checksum != record["sha256"] or size != record["size"]:
                raise ValueError("Archive content checksum/size mismatch")
            if set(record["headers"]) - set((*HEADERS, "Expires")):
                raise ValueError("Unsupported object headers")
    for versions in groups.values():
        if manifest["schemaVersion"] == 1 and versions != sorted(versions, key=lambda r: r["modified"]):
            raise ValueError("Versions must be oldest first")
        if sum(r["latest"] for r in versions) != 1 or not versions[-1]["latest"]:
            raise ValueError("Invalid latest version")
    return manifest


def compose_history(manifest, prior):
    """Carry original aliases through a later migration/recovery to a fresh store."""
    if (prior.get("schemaVersion") != 1 or prior["destinationEndpoint"] != manifest["sourceEndpoint"]
            or prior["destinationBucket"] != manifest["sourceBucket"]):
        raise ValueError("Prior mapping does not describe this archive's source")
    manifest = copy.deepcopy(manifest)
    current = {(r["key"], r["sourceVersion"]): r for r in manifest["entries"]}
    identities = dict(current)
    seen = set()
    for old in prior["entries"]:
        identity = old["key"], old["destinationVersion"]
        if identity in seen or identity not in current:
            raise ValueError("Prior history is missing or duplicated in recovery archive")
        seen.add(identity)
        record = current[identity]
        if (old["deleteMarker"] != record["deleteMarker"] or
                (not record["deleteMarker"] and (old["sha256"], old["size"]) != (record["sha256"], record["size"]))):
            raise ValueError("Prior history content differs from recovery archive")
        aliases = [old["sourceVersion"], *old.get("aliases", [])]
        for alias in aliases:
            if not isinstance(alias, str) or not alias:
                raise ValueError("Invalid historical version alias")
            key = record["key"], alias
            if key in identities and identities[key] is not record:
                raise ValueError("Historical alias collides with another version")
            identities[key] = record
        record["aliases"] = sorted(set(aliases) - {record["sourceVersion"]})
    return manifest


def import_archive(client, bucket, directory, prior_map=None):
    directory = Path(directory)
    manifest = check_archive(directory)  # Validate every body before the first write.
    if any((directory / name).exists() for name in ("import-journal.jsonl", "multipart-journal.jsonl", "version-map.json")):
        raise ValueError("Archive already has import evidence; inspect it instead of retrying")
    if prior_map is not None:
        manifest = compose_history(manifest, prior_map)
    destination = endpoint(client.meta.endpoint_url)
    if destination == manifest["sourceEndpoint"] and bucket == manifest["sourceBucket"]:
        raise ValueError("Source and destination must differ")
    if client.get_bucket_versioning(Bucket=bucket).get("Status") != "Enabled":
        raise ValueError("Destination versioning must be enabled")
    if inventory(client, bucket):
        raise ValueError("Destination must be empty, including old versions/delete markers")
    for page in client.get_paginator("list_multipart_uploads").paginate(Bucket=bucket):
        if page.get("Uploads"):
            raise ValueError("Destination has unfinished multipart uploads; inspect prior writes")
    retries = client.meta.config.retries
    if not isinstance(retries, dict) or not (retries.get("total_max_attempts") == 1 or
                                           ("total_max_attempts" not in retries and retries.get("max_attempts") == 0)):
        raise ValueError("Migration mutations require SDK retries disabled")
    # Journal proves which writes happened if import is interrupted. Never resume
    # automatically: an acknowledged write can be lost before its journal entry.
    result = dict(schemaVersion=1, sourceEndpoint=manifest["sourceEndpoint"],
                  destinationEndpoint=destination, sourceBucket=manifest["sourceBucket"],
                  destinationBucket=bucket, entries=[])
    journal = directory / "import-journal.jsonl"
    multipart_journal = directory / "multipart-journal.jsonl"
    with journal.open("x") as output, multipart_journal.open("x") as multipart_output:
        journal_event(multipart_output, state="import_started", destinationEndpoint=destination, destinationBucket=bucket)
        for record in manifest["entries"]:
            request = dict(Bucket=bucket, Key=record["key"])
            if record["deleteMarker"]:
                response = client.delete_object(**request)
                if response.get("DeleteMarker") is not True:
                    raise ValueError("Destination did not create a delete marker")
            else:
                with (directory / record["file"]).open("rb") as body:
                    if record["size"] >= MULTIPART_THRESHOLD:
                        response = multipart_upload(client, request, record, body, multipart_output)
                    else:
                        response = client.put_object(**request, Body=body, **record["headers"],
                                                     Metadata=record["metadata"],
                                                     Tagging=urlencode({t["Key"]: t["Value"] for t in record["tags"]}))
            version = response.get("VersionId")
            if not version or version == "null":
                raise ValueError("Destination returned no immutable version ID")
            mapped = dict(record, destinationVersion=version)
            result["entries"].append(mapped)
            output.write(json.dumps(mapped) + "\n")
            output.flush()
            os.fsync(output.fileno())
    verify_copy(client, bucket, result)
    write_json(directory / "version-map.json", result)
    return result


def verify_copy(client, bucket, mapping):
    if (endpoint(client.meta.endpoint_url) != mapping["destinationEndpoint"]
            or bucket != mapping["destinationBucket"]):
        raise ValueError("Mapping destination does not match client")
    with tempfile.TemporaryDirectory(prefix="storage-order-verify-") as temporary:
        listed, _ = version_order.capture(client, bucket, Path(temporary), "before", 1000)
    actual = {(r["key"], r["sourceVersion"], r["deleteMarker"], r["latest"]) for r in listed}
    expected = {(r["key"], r["destinationVersion"], r["deleteMarker"], r["latest"])
                for r in mapping["entries"]}
    if len(actual) != len(mapping["entries"]) or actual != expected:
        raise ValueError("Destination versions/delete markers/latest state do not match")
    ordered = [(r["key"], r["sourceVersion"]) for r in listed]
    required = [(r["key"], r["destinationVersion"]) for r in
                sorted(mapping["entries"], key=lambda r: r["key"].encode())]
    if ordered != required:
        raise ValueError("Destination historical version order differs from archive")
    sources = {(r["key"], version) for r in mapping["entries"]
               for version in (r["sourceVersion"], *r.get("aliases", []))}
    for record in mapping["entries"]:
        if (record["key"], record["destinationVersion"]) in sources:
            raise ValueError("Source/destination version collision requires manual resolution")
        if record["deleteMarker"]:
            continue
        request = dict(Bucket=bucket, Key=record["key"], VersionId=record["destinationVersion"])
        response = client.get_object(**request)
        try:
            checksum, size = digest(response["Body"])
            if (checksum, size) != (record["sha256"], record["size"]):
                raise ValueError("Destination content checksum/size mismatch")
            headers = {name: response[name] for name in HEADERS if name in response}
            if "Expires" in response:
                headers["Expires"] = response["Expires"].isoformat()
            if headers != record["headers"] or response.get("Metadata", {}) != record["metadata"]:
                raise ValueError("Destination metadata differs")
            tags = client.get_object_tagging(**request).get("TagSet", [])
            if sorted(tags, key=lambda t: t["Key"]) != sorted(record["tags"], key=lambda t: t["Key"]):
                raise ValueError("Destination tags differ")
        finally:
            response["Body"].close()


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=("export", "check", "import", "verify"))
    parser.add_argument("archive", type=Path)
    parser.add_argument("--endpoint")
    parser.add_argument("--bucket")
    parser.add_argument("--profile")
    parser.add_argument("--region", default="us-east-1")
    parser.add_argument("--source-listing-dialect", choices=version_order.DIALECTS,
                        help="Export only: select minio-v2 for verified MinIO form encoding/cache markers")
    parser.add_argument("--writers-quiesced", action="store_true")
    parser.add_argument("--prior-map", type=Path,
                        help="For recovery to a fresh store, compose the prior verified version map")
    args = parser.parse_args()
    if args.source_listing_dialect and args.operation != "export":
        parser.error("--source-listing-dialect applies only to export; checks use retained evidence")
    if args.prior_map and args.operation != "import":
        parser.error("--prior-map applies only to import")
    if args.operation == "check":
        check_archive(args.archive)
    else:
        if not args.endpoint or not args.bucket:
            parser.error("--endpoint and --bucket are required")
        if args.operation in ("export", "import") and not args.writers_quiesced:
            parser.error("Freeze source/destination writers and supply --writers-quiesced")
        session = boto3.Session(profile_name=args.profile)
        client = session.client("s3", endpoint_url=endpoint(args.endpoint), region_name=args.region,
                                config=Config(signature_version="s3v4", s3={"addressing_style": "path"},
                                              retries={"max_attempts": 0}, connect_timeout=5, read_timeout=120))
        if args.operation == "export":
            export_archive(client, args.bucket, args.archive, listing_dialect=args.source_listing_dialect or "s3")
        elif args.operation == "import":
            prior = json.loads(args.prior_map.read_text()) if args.prior_map else None
            import_archive(client, args.bucket, args.archive, prior)
        else:
            verify_copy(client, args.bucket, json.loads((args.archive / "version-map.json").read_text()))
    print(f"PASS: {args.operation}")


if __name__ == "__main__":
    main()
