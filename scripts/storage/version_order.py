"""Retained S3 listing evidence. Version IDs are opaque, never sort keys."""
from collections import defaultdict
from datetime import datetime, timezone
import hashlib
import os
from pathlib import Path
import re
from urllib.parse import unquote, unquote_plus
import uuid
import xml.etree.ElementTree as ET

FORMAT = "s3-list-object-versions-v1"
MAX_PAGE_BYTES = 8 * 1024**2
MAX_PAGES = 100000
FIELDS = ("key", "sourceVersion", "deleteMarker", "latest", "modified")
DIALECTS = ("s3", "minio-v2")
MINIO_CACHE_SUFFIX = re.compile(
    r"\[minio_cache:v2,(?:return:|id:[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12},p:[0-9]+,s:[0-9]+)\]")


def stamp(value):
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
        if parsed.tzinfo is None:
            raise ValueError("Missing timestamp timezone")
        return parsed.astimezone(timezone.utc).isoformat()
    except (ValueError, TypeError, AttributeError) as error:
        raise ValueError("Invalid listing timestamp") from error


def parse_page(body, bucket, max_keys, marker, dialect="s3"):
    if dialect not in DIALECTS:
        raise ValueError("Unsupported listing dialect")
    if not isinstance(body, bytes) or len(body) > MAX_PAGE_BYTES:
        raise ValueError("Listing page exceeds evidence bound")
    # UTF-8 only, no DTD or entity declarations. Predefined XML escapes remain valid.
    text = body.decode("utf-8", errors="strict")
    if "<!DOCTYPE" in text.upper() or "<!ENTITY" in text.upper() or "\x00" in text:
        raise ValueError("Unsafe listing XML")
    root = ET.fromstring(text)
    allowed_roots = ("{http://s3.amazonaws.com/doc/2006-03-01/}ListVersionsResult",
                     "{http://s3.amazonaws.com/doc/2006-03-01}ListVersionsResult")
    if root.tag not in allowed_roots:
        raise ValueError("Unexpected listing XML root")
    ns = root.tag.split("}")[0] + "}"

    def value(element, name, default=None):
        nodes = element.findall(ns + name)
        if len(nodes) > 1 or (not nodes and default is None):
            raise ValueError("Missing/duplicate listing field: " + name)
        if not nodes:
            return default
        if len(nodes[0]):
            raise ValueError("Nested listing scalar")
        return nodes[0].text or ""

    encoding = value(root, "EncodingType", "")
    if encoding not in ("", "url"):
        raise ValueError("Unsupported listing encoding")

    def key(value):
        # MinIO's explicit dialect uses form encoding, including + for spaces.
        # Keep the historical default for existing evidence and other providers.
        if encoding == "url":
            if re.search(r"%(?![0-9A-Fa-f]{2})", value):
                raise ValueError("Invalid percent-encoded listing key")
            decode = unquote_plus if dialect == "minio-v2" else unquote
            return decode(value, encoding="utf-8", errors="strict")
        return value

    if (value(root, "Name") != bucket or int(value(root, "MaxKeys")) != max_keys
            or value(root, "Prefix", "") or value(root, "Delimiter", "")
            or root.findall(ns + "CommonPrefixes")):
        raise ValueError("Listing scope differs from full-bucket request")
    if (key(value(root, "KeyMarker", "")), value(root, "VersionIdMarker", "")) != marker:
        raise ValueError("Listing marker echo differs from request")
    truncated = value(root, "IsTruncated")
    if truncated not in ("true", "false"):
        raise ValueError("Invalid listing truncation flag")
    records = []
    scalar_names = {"Name", "Prefix", "Delimiter", "MaxKeys", "KeyMarker", "VersionIdMarker",
                    "NextKeyMarker", "NextVersionIdMarker", "IsTruncated", "EncodingType"}
    for element in root:
        if element.tag in (ns + "Version", ns + "DeleteMarker"):
            latest = value(element, "IsLatest")
            identity = key(value(element, "Key")), value(element, "VersionId")
            if not all(identity) or latest not in ("true", "false"):
                raise ValueError("Invalid listing version identity/flag")
            records.append(dict(key=identity[0], sourceVersion=identity[1],
                                deleteMarker=element.tag == ns + "DeleteMarker", latest=latest == "true",
                                modified=stamp(value(element, "LastModified"))))
        elif element.tag not in {ns + name for name in scalar_names}:
            raise ValueError("Unknown listing element; refusing incomplete inventory")
    if len(records) > max_keys:
        raise ValueError("Listing exceeds requested page size")
    next_marker = key(value(root, "NextKeyMarker", "")), value(root, "NextVersionIdMarker", "")
    if truncated == "true":
        boundary = False
        if records:
            last = records[-1]
            boundary = next_marker == (last["key"], last["sourceVersion"])
            if dialect == "minio-v2" and next_marker[0].startswith(last["key"]):
                suffix = next_marker[0][len(last["key"]):]
                boundary |= bool(MINIO_CACHE_SUFFIX.fullmatch(suffix)) and next_marker[1] == last["sourceVersion"]
        if not boundary or next_marker == marker:
            raise ValueError("Truncated listing has an invalid continuation boundary")
    elif any(next_marker):
        raise ValueError("Final listing has unexpected continuation markers")
    return records, next_marker if truncated == "true" else None


def chronological(records):
    """Check provider order, then return oldest first per key.

    SeaweedFS puts null versions last regardless of their modification time.
    Place that version only when timestamps/latest flags prove one position.
    Non-null ties retain their mixed Version/DeleteMarker wire order.
    """
    groups, seen, previous_key = defaultdict(list), set(), None
    for record in records:
        identity = record["key"], record["sourceVersion"]
        if identity in seen or (previous_key is not None and record["key"].encode() < previous_key.encode()):
            raise ValueError("Duplicate version or non-monotonic listing keys")
        seen.add(identity)
        previous_key = record["key"]
        groups[record["key"]].append(record)
    result = []
    for versions in groups.values():
        null = next((r for r in versions if r["sourceVersion"] == "null"), None)
        ordered = [r for r in versions if r is not null]
        if any(datetime.fromisoformat(a["modified"]) < datetime.fromisoformat(b["modified"])
               for a, b in zip(ordered, ordered[1:])):
            raise ValueError("Provider listing chronology is inconsistent; inspect legacy pagination")
        if null is not None:
            moment = datetime.fromisoformat(null["modified"])
            tied = [r for r in ordered if datetime.fromisoformat(r["modified"]) == moment]
            if not tied:
                position = next((i for i, r in enumerate(ordered) if datetime.fromisoformat(r["modified"]) < moment), len(ordered))
            elif null["latest"]:
                position = 0  # IsLatest independently proves the newest position.
            elif len(tied) == 1 and tied[0]["latest"]:
                position = ordered.index(tied[0]) + 1  # The only tied peer is newer.
            else:
                raise ValueError("Null version tie has multiple possible positions; independent evidence required")
            ordered.insert(position, null)
        if any(datetime.fromisoformat(a["modified"]) < datetime.fromisoformat(b["modified"])
               for a, b in zip(ordered, ordered[1:])):
            raise ValueError("Null latest flag contradicts timestamp chronology")
        if sum(r["latest"] for r in ordered) != 1 or not ordered[0]["latest"]:
            raise ValueError("Latest version disagrees with listing chronology; inspect source/pagination")
        result.extend(reversed(ordered))
    return result


def capture(client, bucket, directory, label, max_keys, dialect="s3"):
    if (type(max_keys) is not int or not 1 <= max_keys <= 1000
            or label not in ("before", "after") or dialect not in DIALECTS):
        raise ValueError("Invalid listing evidence parameters")
    base = Path(directory) / "order" / label
    base.mkdir(parents=True, mode=0o700)
    marker, records, pages, seen = ("", ""), [], [], set()
    event_name, unique_id = "after-call.s3.ListObjectVersions", "archive-order-" + uuid.uuid4().hex
    captured = []

    def response(http_response, **kwargs):
        captured.append(http_response.content)

    client.meta.events.register(event_name, response, unique_id=unique_id)
    try:
        while True:
            if len(pages) >= MAX_PAGES or marker in seen:
                raise ValueError("Listing pagination did not terminate")
            seen.add(marker)
            captured.clear()
            request = dict(Bucket=bucket, MaxKeys=max_keys, EncodingType="url")
            if marker != ("", ""):
                request.update(KeyMarker=marker[0], VersionIdMarker=marker[1])
            client.list_object_versions(**request)
            if len(captured) != 1:
                raise ValueError("Expected exactly one raw listing response")
            body = captured[0]
            current, next_marker = parse_page(body, bucket, max_keys, marker, dialect)
            path = base / f"{len(pages):08d}.xml"
            with path.open("xb") as output:
                output.write(body)
                output.flush()
                os.fsync(output.fileno())
            pages.append(dict(file=path.relative_to(directory).as_posix(), size=len(body), sha256=hashlib.sha256(body).hexdigest()))
            records.extend(current)
            if next_marker is None:
                break
            marker = next_marker
    finally:
        client.meta.events.unregister(event_name, unique_id=unique_id)
    return chronological(records), dict(maxKeys=max_keys, pages=pages, dialect=dialect)


def verify(directory, bucket, evidence):
    if (not isinstance(evidence, dict) or evidence.get("format") != FORMAT
            or not isinstance(evidence.get("scans"), list) or len(evidence["scans"]) != 2):
        raise ValueError("Missing/unsupported listing evidence")
    scans = []
    dialects = [scan.get("dialect", "s3") for scan in evidence["scans"]]
    if any(d not in DIALECTS for d in dialects) or len(set(dialects)) != 1:
        raise ValueError("Listing evidence has unsupported or differing dialects")
    for label, scan in zip(("before", "after"), evidence["scans"]):
        max_keys, pages = scan["maxKeys"], scan["pages"]
        if type(max_keys) is not int or not 1 <= max_keys <= 1000 or not isinstance(pages, list) or not 1 <= len(pages) <= MAX_PAGES:
            raise ValueError("Invalid listing evidence bounds")
        marker, records, seen = ("", ""), [], set()
        for number, page in enumerate(pages):
            if marker is None or marker in seen:
                raise ValueError("Unexpected/repeated listing page")
            seen.add(marker)
            expected = f"order/{label}/{number:08d}.xml"
            path = (Path(directory) / page["file"]).resolve()
            if page["file"] != expected or not path.is_relative_to(Path(directory).resolve()):
                raise ValueError("Listing evidence path escapes archive")
            with path.open("rb") as stream:
                body = stream.read(MAX_PAGE_BYTES + 1)
            if len(body) > MAX_PAGE_BYTES or len(body) != page["size"] or hashlib.sha256(body).hexdigest() != page["sha256"]:
                raise ValueError("Listing evidence checksum/size mismatch")
            current, marker = parse_page(body, bucket, max_keys, marker, scan.get("dialect", "s3"))
            records.extend(current)
        if marker is not None:
            raise ValueError("Listing evidence is missing final pages")
        scans.append(chronological(records))
    if evidence["scans"][0]["maxKeys"] == evidence["scans"][1]["maxKeys"]:
        raise ValueError("Evidence requires two distinct pagination sizes")
    if scans[0] != scans[1]:
        raise ValueError("Source inventory/order changed between listing scans")
    return scans[0]
