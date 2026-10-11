"""Explicit disposable/local-psql integration; never run against an operator DSN."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

import reference_coverage as coverage

REPO = Path(__file__).resolve().parents[2]


@unittest.skipUnless(os.environ.get("STORAGE_REFERENCE_DATABASE_TEST") == "1", "run test-reference-postgres.sh")
class PostgreSQLReferenceTests(unittest.TestCase):
    @classmethod
    def sql(cls, query, reader=False):
        env = dict(os.environ)
        env.update(PGUSER="reference_reader" if reader else "postgres",
                   PGPASSWORD="offline-reader-only" if reader else "offline-reference-only")
        result = subprocess.run(["psql", "-XqAtw", "-v", "ON_ERROR_STOP=1"], input=query,
                                text=True, capture_output=True, env=env, timeout=30)
        if result.returncode:
            raise AssertionError(result.stderr)  # Synthetic local fixture only.
        return result.stdout.strip()

    @classmethod
    def setUpClass(cls):
        if os.environ.get("PGHOST") != "127.0.0.1" or os.environ.get("PGDATABASE") != "storage_reference_test":
            raise RuntimeError("Disposable loopback database required")
        # Use the actual document DDL through chunks. The following pgvector-only
        # derived embedding table has no storage references and is not needed.
        rag = (REPO / "sql/rag/schema.pgsql").read_text()
        prefix, separator, _ = rag.partition("CREATE TABLE IF NOT EXISTS rag.embeddings")
        if not separator or "CREATE EXTENSION IF NOT EXISTS vector;" not in prefix:
            raise RuntimeError("Re-audit document schema fixture boundary")
        cls.sql(prefix.replace("CREATE EXTENSION IF NOT EXISTS vector;", "", 1))
        cls.sql((REPO / "sql/assistant/schema.pgsql").read_text())
        cls.sql("CREATE ROLE reference_reader LOGIN PASSWORD 'offline-reader-only'; "
                "ALTER ROLE reference_reader SET default_transaction_read_only=on; "
                "GRANT USAGE ON SCHEMA rag,assistant,public TO reference_reader; "
                "GRANT SELECT ON ALL TABLES IN SCHEMA rag,assistant TO reference_reader;")

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.sql("DROP TABLE IF EXISTS public.extra_references; DROP DOMAIN IF EXISTS public.reference_payload; "
                 "TRUNCATE rag.documents,rag.document_change_audits,assistant.conversations,assistant.file_proposals CASCADE;")
        self.sql("""
INSERT INTO rag.documents(document_id,bucket_name,object_key,source_uri,version_marker,metadata)
 VALUES ('retained','documents','opaque/%2F key','s3://documents/opaque/%2F key','v1','{"privateNote":"DO-NOT-EXPORT"}'),
 ('removed','documents','deleted-inventory','s3://documents/deleted-inventory','old','{}');
INSERT INTO rag.document_change_audits(audit_id,document_pk,document_id,bucket_name,object_key,action,actor_email,old_version_marker,new_version_marker)
 SELECT 'historic',id,document_id,bucket_name,object_key,'edit','private@example.test','old','old' FROM rag.documents WHERE document_id='removed';
DELETE FROM rag.documents WHERE document_id='removed';
INSERT INTO rag.chunks(document_pk,chunk_index,chunk_text,token_count,chunk_metadata)
 SELECT id,0,'PRIVATE-DOCUMENT-BODY',1,'{"bucket":"documents","objectKey":"opaque/%2F key","versionId":"null"}' FROM rag.documents;
INSERT INTO assistant.conversations(conversation_id,user_email) VALUES ('conversation','private@example.test');
INSERT INTO assistant.messages(message_id,conversation_pk,conversation_id,user_email,role,content,citations,metadata)
 SELECT 'message',id,conversation_id,user_email,'assistant','PRIVATE-ASSISTANT-BODY',
 '[{"processingVersion":100,"objectKey":"opaque/%2F key"}]',
 '{"history":{"bucket":"documents","objectKey":"opaque/%2F key","versionMarker":"original"}}' FROM assistant.conversations;
INSERT INTO assistant.tool_calls(tool_call_id,conversation_id,user_email,tool_name,result)
 VALUES ('tool','conversation','private@example.test','history',
 '{"content":[{"type":"text","text":"{\\"bucket\\":\\"documents\\",\\"objectKey\\":\\"opaque/%2F key\\",\\"versionMarker\\":\\"v1\\"}"}]}');
INSERT INTO assistant.file_proposals(proposal_id,conversation_id,user_email,action,document_id,object_key,proposed_text,orchestrator_response)
 VALUES ('proposal','conversation','private@example.test','edit','retained','opaque/%2F key','PRIVATE-PROPOSED-BODY',
 '{"bucket":"documents","objectKey":"opaque/%2F key","versionMarker":"v1"}');
""")
        self.mapping = dict(schemaVersion=1, sourceEndpoint="http://source.invalid:9000", destinationEndpoint="http://target.invalid:8333",
                            sourceBucket="documents", destinationBucket="documents", entries=[
            dict(key="opaque/%2F key", sourceVersion="v1", aliases=["original", "null"], destinationVersion="new-v1",
                 deleteMarker=False, size=7, sha256=hashlib.sha256(b"fixture").hexdigest()),
            dict(key="deleted-inventory", sourceVersion="old", destinationVersion="new-old", deleteMarker=False,
                 size=7, sha256=hashlib.sha256(b"fixture").hexdigest())])
        self.map_path = self.root / "map.json"
        self.map_path.write_text(json.dumps(self.mapping))

    def export(self, name="export"):
        with patch.dict(os.environ, PGUSER="reference_reader", PGPASSWORD="offline-reader-only"):
            return coverage.export(self.root / name, self.mapping["sourceEndpoint"])

    def fingerprint(self):
        # Original stored rows, not the export's filtered projection.
        queries = [f"SELECT md5(coalesce(string_agg(to_jsonb(t)::text,',' ORDER BY id),'')) FROM {table} t;"
                   for table in sorted(coverage.REQUIRED)]
        return self.sql("\n".join(queries))

    def test_actual_schemas_deleted_inventory_and_private_projection(self):
        before = self.fingerprint()
        env = dict(os.environ, PGUSER="reference_reader", PGPASSWORD="offline-reader-only")
        result = subprocess.run([sys.executable, coverage.__file__, "export", str(self.root / "export"),
                                 "--source-endpoint", self.mapping["sourceEndpoint"]], env=env,
                                text=True, capture_output=True, timeout=30)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        manifest = json.loads((self.root / "export/manifest.json").read_bytes())
        self.assertEqual(before, self.fingerprint())
        result = coverage.check(self.root / "export", [self.map_path], self.root / "report.jsonl")
        self.assertTrue(result["passed"], result["counts"])
        self.assertEqual(result["counts"]["covered"], 7)
        self.assertEqual(self.sql("SELECT document_pk IS NULL FROM rag.document_change_audits"), "t")
        self.assertEqual(manifest["tableRows"]["rag.documents"], 1)
        self.assertEqual(manifest["snapshot"]["readOnly"], "on")
        body = (self.root / "export/references.jsonl").read_text()
        for private in ("PRIVATE-", "DO-NOT-EXPORT", "private@example.test"):
            self.assertNotIn(private, body)
        self.assertIn("deleted-inventory", body)
        self.assertEqual((self.root / "export/manifest.json").stat().st_mode & 0o777, 0o600)

    def test_discovered_new_json_domain_and_explicit_marker_columns(self):
        self.sql("CREATE DOMAIN public.reference_payload AS jsonb; "
                 "CREATE TABLE public.extra_references(id int PRIMARY KEY,bucket text,object_key text,version_id text,details public.reference_payload); "
                 "GRANT SELECT ON public.extra_references TO reference_reader; "
                 "INSERT INTO public.extra_references VALUES(1,'documents','opaque/%2F key','v1','{\"nested\":{\"versionMarker\":\"original\"}}');")
        manifest = self.export()
        self.assertEqual(manifest["tableRows"]["public.extra_references"], 1)
        result = coverage.check(self.root / "export", [self.map_path], self.root / "report.jsonl")
        self.assertTrue(result["passed"])
        self.assertEqual(result["counts"]["covered"], 9)

    def test_null_inventory_and_unresolved_saved_tool_reference_fail(self):
        self.sql("UPDATE rag.documents SET version_marker=NULL; "
                 "UPDATE assistant.tool_calls SET arguments='{\"documentId\":\"retained\",\"versionMarker\":\"v1\"}';")
        self.export()
        result = coverage.check(self.root / "export", [self.map_path], self.root / "report.jsonl")
        self.assertFalse(result["passed"])
        self.assertEqual(result["counts"]["unversioned_inventory"], 1)
        self.assertEqual(result["counts"]["unresolved_context"], 1)

    def test_rls_and_missing_grants_never_seal_partial_export(self):
        self.sql("CREATE TABLE public.extra_references(id int PRIMARY KEY,data jsonb); "
                 "INSERT INTO public.extra_references VALUES(1,'{}'); "
                 "GRANT SELECT ON public.extra_references TO reference_reader; "
                 "ALTER TABLE public.extra_references ENABLE ROW LEVEL SECURITY; "
                 "CREATE POLICY hidden ON public.extra_references USING(false);")
        with self.assertRaises(ValueError):
            self.export("rls")
        self.assertFalse((self.root / "rls/manifest.json").exists())
        self.sql("ALTER TABLE public.extra_references DISABLE ROW LEVEL SECURITY; "
                 "REVOKE SELECT ON public.extra_references FROM reference_reader;")
        with self.assertRaises(ValueError):
            self.export("permissions")
        self.assertFalse((self.root / "permissions/manifest.json").exists())

    def test_duplicate_native_json_keys_refuse_incomplete_evidence(self):
        self.sql("CREATE TABLE public.extra_references(id int PRIMARY KEY,data json); "
                 "GRANT SELECT ON public.extra_references TO reference_reader; "
                 "INSERT INTO public.extra_references VALUES(1,'{\"versionMarker\":\"v1\",\"versionMarker\":\"v2\"}');")
        with self.assertRaisesRegex(ValueError, "Duplicate JSON"):
            self.export()
        self.assertFalse((self.root / "export/manifest.json").exists())

    def test_concurrent_write_is_outside_the_consistent_snapshot(self):
        # Enough data to backpressure psql's pipe while its read transaction is
        # open. No giant output file: non-reference payloads are never retained.
        self.sql("INSERT INTO rag.documents(document_id,bucket_name,object_key,source_uri,version_marker,metadata) "
                 "SELECT 'bulk-'||i,'documents','bulk-'||i,'s3://documents/bulk-'||i,'v1',jsonb_build_object('padding',repeat('x',8000)) FROM generate_series(1,2000) i;")
        original = coverage.references
        written = False

        def concurrent(data):
            nonlocal written
            if not written:
                written = True
                self.sql("INSERT INTO rag.documents(document_id,bucket_name,object_key,source_uri,version_marker) "
                         "VALUES('late','documents','late','s3://documents/late','late-version');")
            return original(data)

        with patch.object(coverage, "references", concurrent):
            manifest = self.export()
        self.assertTrue(written)
        self.assertEqual(manifest["tableRows"]["rag.documents"], 2001)
        self.assertEqual(self.sql("SELECT count(*) FROM rag.documents"), "2002")
        self.assertNotIn("late-version", (self.root / "export/references.jsonl").read_text())


if __name__ == "__main__":
    unittest.main()
