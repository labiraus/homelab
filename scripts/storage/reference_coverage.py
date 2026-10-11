#!/usr/bin/env python3
"""Read-only PostgreSQL reference inventory and offline S3 version-map coverage.

Credentials use libpq's normal environment/service/password-file workflow.
Exports contain object identities, not document bodies; keep them private.
"""
import argparse
from collections import Counter
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
from urllib.parse import urlsplit

MAX_LINE = 16 * 1024 * 1024
MAX_MAP = 32 * 1024 * 1024  # Matches the deployed Go resolver.
MARKERS = {"versionmarker", "oldversionmarker", "newversionmarker",
           "revertedtoversionmarker", "versionid", "sourceversion", "destinationversion"}
CONTEXT = {"bucket", "bucketname", "objectkey", "key", "documentid", "sourceuri"}
REQUIRED = {"rag.documents", "rag.document_change_audits", "rag.document_lifecycle_events",
            "rag.chunks", "assistant.messages", "assistant.tool_calls", "assistant.file_proposals"}
KINDS = {"version", "absent", "unversioned_inventory", "unparsed_json", "versioned_uri"}


class EvidenceError(ValueError):
    """Fixed operator-facing diagnostics which never interpolate private data."""


def normal(key):
    return key.replace("_", "").lower()


def strict_json(raw):
    def pairs(items):
        out = {}
        for key, value in items:
            if key in out:
                raise EvidenceError("Duplicate JSON key")
            out[key] = value
        return out
    return json.loads(raw, object_pairs_hook=pairs,
                      parse_constant=lambda _: (_ for _ in ()).throw(EvidenceError("Non-finite JSON")))


def origin(value):
    u = urlsplit(value)
    if u.scheme not in ("http", "https") or not u.hostname or u.username or u.password or u.query or u.fragment or u.path not in ("", "/"):
        raise EvidenceError("S3 endpoint must be an HTTP(S) origin without credentials")
    return value.rstrip("/")


def encoded(value):
    return (json.dumps(value, ensure_ascii=False, separators=(",", ":")) + "\n").encode()


def private_file(path):
    return os.fdopen(os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "wb")


def bounded_file(path, limit):
    with Path(path).open("rb") as file:
        body = file.read(limit+1)
    if len(body) > limit:
        raise EvidenceError("Evidence file exceeds its size bound")
    return body


def discover_sql():
    # Only catalog-controlled identifiers are interpolated by PostgreSQL format
    # with %I/%L. Stored JSON is output data and never passed to \gexec.
    markers = ",".join("'" + x + "'" for x in sorted(MARKERS))
    context = ",".join("'" + x + "'" for x in sorted(CONTEXT))
    catalog = f"""
WITH columns AS (
 SELECT n.nspname, c.relname, c.oid, a.attname, a.attnum,
   t.typname IN ('json','jsonb') OR bt.typname IN ('json','jsonb') AS is_json,
   lower(replace(a.attname,'_','')) IN ({markers}) AS is_marker,
   lower(replace(a.attname,'_','')) IN ({context}) AS is_context,
   EXISTS(SELECT FROM pg_index i WHERE i.indrelid=c.oid AND i.indisprimary AND a.attnum=ANY(i.indkey)) AS is_pk
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
 JOIN pg_type t ON t.oid=a.atttypid LEFT JOIN pg_type bt ON bt.oid=t.typbasetype
 WHERE c.relkind IN ('r','p') AND NOT c.relispartition
   AND n.nspname NOT LIKE 'pg\\_%' AND n.nspname<>'information_schema'
), tables AS (
 SELECT nspname, relname, oid,
   json_agg(attname ORDER BY attnum) FILTER(WHERE is_json OR is_marker OR is_context OR is_pk) AS selected,
   string_agg(format('%L,t.%I',attname,attname),',' ORDER BY attnum)
     FILTER(WHERE is_json OR is_marker OR is_context OR is_pk) AS arguments,
   string_agg(format('%L,t.%I',attname,attname),',' ORDER BY attnum) FILTER(WHERE is_pk) AS identity
 FROM columns GROUP BY nspname,relname,oid HAVING bool_or(is_json OR is_marker)
)
"""
    return """\\set ON_ERROR_STOP on
\\set FETCH_COUNT 100
BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL search_path=pg_catalog;
SET LOCAL row_security=off;
SET LOCAL standard_conforming_strings=on;
SET LOCAL statement_timeout='5min';
SET LOCAL lock_timeout='5s';
""" + catalog + """
SELECT json_build_object('kind','header','snapshot',pg_current_snapshot()::text,'capturedAt',clock_timestamp(),
 'readOnly',current_setting('transaction_read_only'),'isolation',current_setting('transaction_isolation'),
 'tables',coalesce(json_agg(json_build_object('table',nspname||'.'||relname,'columns',selected)
 ORDER BY nspname,relname),'[]')) FROM tables;
""" + catalog + """
SELECT format('SELECT json_build_object(''kind'',''row'',''table'',%L,''locator'',json_build_object(%s),''data'',json_build_object(%s)) FROM %I.%I t',
 nspname||'.'||relname,coalesce(identity,'''tableOid'',t.tableoid::text,''ctid'',t.ctid::text'),arguments,nspname,relname),
 format('SELECT json_build_object(''kind'',''count'',''table'',%L,''rows'',count(*)) FROM %I.%I',nspname||'.'||relname,nspname,relname)
FROM tables ORDER BY nspname,relname
\\gexec
COMMIT;
SELECT '{"kind":"complete"}';
"""


def references(data):
    """Recognise structured marker contracts; never infer identity from current rows.

    Context is inherited inside an object, but a different document identity
    clears inherited bucket/key. Siblings cannot supply each other's context.
    """
    found, visited = [], 0

    def visit(value, context, path, depth):
        nonlocal visited
        visited += 1
        if depth > 64 or visited > 100000:
            raise EvidenceError("Reference nesting/node bound exceeded")
        if isinstance(value, dict):
            ctx = dict(context)
            fields = {}
            for key, item in value.items():
                fields.setdefault(normal(key), []).append(item)
            docs = fields.get("documentid", [])
            if docs and (len(docs) != 1 or docs[0] != ctx.get("documentId")):
                ctx = {"documentId": docs[0] if len(docs) == 1 else None}
            for target, aliases in (("bucket", ("bucket", "bucketname")), ("key", ("objectkey", "key"))):
                values = [v for name in aliases for v in fields.get(name, [])]
                if values:
                    selected = values[0] if all(isinstance(v, str) and v and v == values[0] for v in values) else None
                    if target == "bucket" and selected != ctx.get("bucket"):
                        ctx.pop("key", None)
                    ctx[target] = selected
            for key, item in value.items():
                pointer = path + "/" + key.replace("~", "~0").replace("/", "~1")
                if normal(key) in MARKERS:
                    # SQL NULL optional audit edges do not identify a version.
                    # Empty strings and malformed non-null fields remain failures.
                    if item is None:
                        found.append(dict(path=pointer, kind="absent", bucket=ctx.get("bucket"), key=ctx.get("key")))
                    else:
                        found.append(dict(path=pointer, kind="version", bucket=ctx.get("bucket"), key=ctx.get("key"),
                                          version=item if isinstance(item, str) else None))
                else:
                    visit(item, ctx, pointer, depth+1)
        elif isinstance(value, list):
            for i, item in enumerate(value):
                visit(item, context, path + "/" + str(i), depth+1)
        elif isinstance(value, str):
            trimmed = value.strip()
            if trimmed.startswith(("{", "[")):
                try:
                    nested = strict_json(trimmed)
                except json.JSONDecodeError:
                    # JSON-looking strings with reference syntax are unresolved,
                    # not silently treated as successfully scanned prose.
                    if any(marker in normal(trimmed) for marker in MARKERS):
                        found.append(dict(path=path, kind="unparsed_json"))
                else:
                    visit(nested, context, path + "/$json", depth+1)
            elif re.search(r'[?&]versionId=', value, re.I):
                # Do not normalize opaque object keys or assume HTTP URL origins.
                found.append(dict(path=path, kind="versioned_uri"))

    visit(data, {}, "", 0)
    return found


def export(directory, source_endpoint, psql="psql"):
    source_endpoint = origin(source_endpoint)
    directory = Path(directory)
    directory.mkdir(mode=0o700)  # Exclusive: never merge with an earlier export.
    counts, expected, ref_counts = Counter(), {}, Counter()
    digest = hashlib.sha256()
    header, complete = None, False
    output_path = directory / "references.jsonl"
    with tempfile.TemporaryFile() as sql_input, private_file(directory / "psql.log") as errors, private_file(output_path) as output:
        sql_input.write(discover_sql().encode()); sql_input.seek(0)
        env = dict(os.environ, PGCLIENTENCODING="UTF8", PGAPPNAME="storage-reference-export")
        command = [psql, "-X", "-qAt", "-w", "-v", "ON_ERROR_STOP=1", "-f", "-"]
        process = subprocess.Popen(command, stdin=sql_input, stdout=subprocess.PIPE, stderr=errors, env=env)
        try:
            while raw := process.stdout.readline(MAX_LINE+1):
                if len(raw) > MAX_LINE or not raw.endswith(b"\n"):
                    raise EvidenceError("Reference row exceeds 16 MiB or is truncated")
                item = strict_json(raw)
                if complete:
                    raise EvidenceError("Unexpected data after snapshot completion")
                if item["kind"] == "header":
                    if header is not None or item["readOnly"] != "on" or item["isolation"] != "repeatable read":
                        raise EvidenceError("Invalid snapshot contract")
                    header = item
                    tables = [t["table"] for t in header["tables"]]
                    if len(set(tables)) != len(tables) or not REQUIRED.issubset(tables):
                        raise EvidenceError("Required document reference tables missing")
                elif header is None:
                    raise EvidenceError("Missing snapshot header")
                elif item["kind"] == "row":
                    table = item["table"]
                    if table not in tables or table in expected:
                        raise EvidenceError("Unexpected snapshot row")
                    counts[table] += 1
                    refs = references(item["data"])
                    for ref in refs:
                        # Current inventory lacking a marker is not version-pinned.
                        if table == "rag.documents" and ref["path"] == "/version_marker" and ref["kind"] == "absent":
                            ref["kind"] = "unversioned_inventory"
                        ref_counts[ref["kind"]] += 1
                        record = dict(table=table, locator=item["locator"], **ref)
                        line = encoded(record)
                        if len(line) > MAX_LINE:
                            raise EvidenceError("Extracted reference exceeds size bound")
                        output.write(line); digest.update(line)
                elif item["kind"] == "count":
                    if item["table"] not in tables or item["table"] in expected:
                        raise EvidenceError("Duplicate or unknown table count")
                    expected[item["table"]] = item["rows"]
                elif item["kind"] == "complete":
                    complete = True
                else:
                    raise EvidenceError("Unexpected snapshot output")
            if process.wait() != 0:
                raise EvidenceError("PostgreSQL reference export failed (permissions, RLS, schema or query error); no manifest sealed")
            if not complete or set(expected) != set(tables) or any(counts[t] != expected[t] for t in tables):
                raise EvidenceError("Incomplete snapshot or inconsistent table counts")
            output.flush(); os.fsync(output.fileno())
        finally:
            if process.poll() is None:
                process.kill()
            process.wait()
            process.stdout.close()
    manifest = dict(schemaVersion=1, sourceEndpoint=source_endpoint, snapshot=header,
                    tableRows=expected, referenceCounts=dict(ref_counts), sha256=digest.hexdigest(),
                    extractorSHA256=hashlib.sha256(Path(__file__).read_bytes()).hexdigest())
    with private_file(directory / "manifest.json") as output:
        output.write(encoded(manifest)); output.flush(); os.fsync(output.fileno())
    return manifest


def mapping_index(mapping, source_endpoint):
    if (mapping.get("schemaVersion") != 1 or origin(mapping["sourceEndpoint"]) != source_endpoint
            or origin(mapping["destinationEndpoint"]) == source_endpoint
            or not isinstance(mapping.get("sourceBucket"), str) or not mapping["sourceBucket"]
            or mapping["sourceBucket"] != mapping.get("destinationBucket") or not isinstance(mapping.get("entries"), list)):
        raise EvidenceError("Map schema, source origin or bucket does not match runtime contract")
    index, destinations = {}, set()
    for entry in mapping["entries"]:
        key, version, dest = (entry.get(f) for f in ("key", "sourceVersion", "destinationVersion"))
        aliases = entry.get("aliases", [])
        if (not all(isinstance(s, str) and s for s in (key, version, dest)) or dest == "null"
                or type(entry.get("deleteMarker")) is not bool or type(entry.get("size", 0)) is not int or entry.get("size", 0) < 0
                or not isinstance(aliases, list) or (key, dest) in destinations
                or (not entry["deleteMarker"] and not re.fullmatch(r'[0-9a-fA-F]{64}', entry.get("sha256", "")))):
            raise EvidenceError("Invalid/duplicate version map entry")
        for alias in [version] + aliases:
            if not isinstance(alias, str) or not alias or (key, alias) in index:
                raise EvidenceError("Duplicate or ambiguous historical version alias")
            index[key, alias] = entry
        destinations.add((key, dest))
    if set(index).intersection(destinations):
        raise EvidenceError("Source/alias/destination version collision")
    return mapping["sourceBucket"], index


def check(directory, map_paths, report_path, contexts_path=None):
    directory = Path(directory)
    manifest = strict_json(bounded_file(directory / "manifest.json", MAX_LINE))
    if (manifest.get("schemaVersion") != 1 or manifest.get("extractorSHA256") != hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
            or manifest["snapshot"]["readOnly"] != "on" or manifest["snapshot"]["isolation"] != "repeatable read"
            or not REQUIRED.issubset(manifest["tableRows"])
            or {t["table"] for t in manifest["snapshot"]["tables"]} != set(manifest["tableRows"])
            or any(type(v) is not int or v < 0 for v in manifest["tableRows"].values())
            or any(type(v) is not int or v < 0 for v in manifest["referenceCounts"].values())):
        raise EvidenceError("Unsupported reference export")
    source = origin(manifest["sourceEndpoint"])
    contexts, contexts_hash, used_contexts = {}, None, set()
    if contexts_path is not None:
        body = bounded_file(contexts_path, MAX_MAP)
        reviewed = strict_json(body)
        if (reviewed.get("schemaVersion") != 1 or reviewed.get("referenceSHA256") != manifest["sha256"]
                or not isinstance(reviewed.get("reviewedBy"), str) or not reviewed["reviewedBy"].strip()
                or not isinstance(reviewed.get("entries"), list)):
            raise EvidenceError("Reviewed contexts are not bound to this export")
        for entry in reviewed["entries"]:
            identifier = entry.get("referenceID", "")
            if (not re.fullmatch(r'[0-9a-f]{64}', identifier) or identifier in contexts
                    or not all(isinstance(entry.get(k), str) and entry[k] for k in ("bucket", "key"))
                    or not isinstance(entry.get("reason"), str) or not entry["reason"].strip()):
                raise EvidenceError("Invalid or duplicate reviewed context")
            contexts[identifier] = entry
        contexts_hash = hashlib.sha256(body).hexdigest()
    indexes, maps = {}, []
    for path in map_paths:
        body = bounded_file(path, MAX_MAP)
        bucket, index = mapping_index(strict_json(body), source)
        if bucket in indexes:
            raise EvidenceError("Multiple maps for the same source bucket")
        indexes[bucket] = index
        maps.append(dict(bucket=bucket, sha256=hashlib.sha256(body).hexdigest()))
    digest, counts, kinds = hashlib.sha256(), Counter(), Counter()
    # Report streamed detail to a private file, with a final summary only after
    # complete export checksum verification. A failed report has no summary.
    with private_file(report_path) as report, (directory / "references.jsonl").open("rb") as refs:
        while raw := refs.readline(MAX_LINE+1):
            if len(raw) > MAX_LINE or not raw.endswith(b"\n"):
                raise EvidenceError("Reference export truncated or oversized")
            digest.update(raw)
            ref = strict_json(raw)
            if (ref["table"] not in manifest["tableRows"] or not isinstance(ref.get("locator"), dict)
                    or not isinstance(ref.get("path"), str) or ref.get("kind") not in KINDS):
                raise EvidenceError("Invalid reference origin")
            identifier = hashlib.sha256(raw).hexdigest()
            resolved = dict(ref)
            if identifier in contexts:
                supplied = contexts[identifier]
                if (ref["kind"] != "version" or not isinstance(ref.get("version"), str) or not ref["version"]
                        or (ref.get("bucket") and ref.get("key"))
                        or any(ref.get(k) and ref[k] != supplied[k] for k in ("bucket", "key"))):
                    raise EvidenceError("Reviewed context cannot override known identity or invalid versions")
                resolved.update(bucket=supplied["bucket"], key=supplied["key"])
                used_contexts.add(identifier)
            kinds[ref["kind"]] += 1
            if ref["kind"] == "absent":
                status = "absent_optional_marker"
            elif ref["kind"] != "version":
                status = ref["kind"]
            elif not isinstance(ref.get("version"), str) or not ref["version"]:
                status = "invalid_version"
            elif not resolved.get("bucket") or not resolved.get("key"):
                status = "unresolved_context"
            elif resolved["bucket"] not in indexes:
                status = "unmapped_bucket"
            else:
                entry = indexes[resolved["bucket"]].get((resolved["key"], resolved["version"]))
                status = "missing_version" if entry is None else "delete_marker" if entry["deleteMarker"] else "covered"
                if status == "covered" and identifier in used_contexts:
                    status = "covered_with_reviewed_context"
            counts[status] += 1
            detail = dict(kind="reference", referenceID=identifier, status=status, reference=ref)
            if identifier in used_contexts:
                detail["reviewedContext"] = contexts[identifier]
            report.write(encoded(detail))
        if digest.hexdigest() != manifest["sha256"] or dict(kinds) != manifest["referenceCounts"]:
            raise EvidenceError("Reference export checksum/count mismatch")
        if used_contexts != set(contexts):
            raise EvidenceError("Reviewed context references are absent from this export")
        passed = all(status in ("covered", "covered_with_reviewed_context", "absent_optional_marker") for status in counts)
        summary = dict(kind="summary", schemaVersion=1, passed=passed, counts=dict(counts),
                       sourceEndpoint=source, referenceSHA256=manifest["sha256"], maps=maps,
                       tableRows=manifest["tableRows"], snapshot=manifest["snapshot"], contextsSHA256=contexts_hash)
        report.write(encoded(summary)); report.flush(); os.fsync(report.fileno())
    return summary


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    exp = sub.add_parser("export")
    exp.add_argument("directory", type=Path)
    exp.add_argument("--source-endpoint", required=True)
    chk = sub.add_parser("check")
    chk.add_argument("directory", type=Path)
    chk.add_argument("--map", action="append", required=True, type=Path)
    chk.add_argument("--report", required=True, type=Path)
    chk.add_argument("--contexts", type=Path, help="Reviewed, export-bound missing bucket/key evidence")
    args = parser.parse_args()
    try:
        if args.command == "export":
            result = export(args.directory, args.source_endpoint)
            print(f"Exported {sum(result['tableRows'].values())} rows from {len(result['tableRows'])} tables")
        else:
            result = check(args.directory, args.map, args.report, args.contexts)
            print(json.dumps({"passed": result["passed"], "counts": result["counts"]}))
            return 0 if result["passed"] else 2
    except (ValueError, OSError, KeyError, TypeError, RecursionError) as error:
        # Do not echo database values, DSNs, nested JSON or secrets in CLI logs.
        detail = str(error) if isinstance(error, EvidenceError) else type(error).__name__
        print("Reference operation failed; no completed acceptance result. " + detail)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
