import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { Pool } from "pg";
import { claimDocumentForProcessing, markDocumentIndexed, markDocumentPendingWithError } from "../src/db.js";
import type { DocumentClaimResult, DocumentEvent } from "../src/types.js";

const dsn = process.env.DOCUMENT_CLAIMS_TEST_DATABASE_URL;
test("document ownership survives retries and recovers after the owning database session dies", { skip: !dsn }, async () => {
	const address = new URL(dsn!);
	assert.equal(address.hostname, "127.0.0.1");
	assert.equal(address.pathname, "/document_claims_test");
	const owner = "document-claim-" + randomUUID();
	const first = new Pool({ connectionString: dsn, application_name: owner });
	const second = new Pool({ connectionString: dsn, application_name: owner + "-second" });
	const claims: DocumentClaimResult[] = [];
	try {
		const schema = await readFile(new URL("../../../sql/rag/schema.pgsql", import.meta.url), "utf8");
		await first.query(schema.replace("CREATE EXTENSION IF NOT EXISTS vector;", "").split("CREATE TABLE IF NOT EXISTS rag.document_lifecycle_events")[0]!);
		const id = randomUUID();
		await first.query("INSERT INTO rag.documents(document_id,source_uri,desired_processing_version) VALUES($1,'s3://documents/owned.txt',2)", [id]);
		const event: DocumentEvent = { documentId: id, bucket: "documents", objectKey: "owned.txt", sourceUri: "s3://documents/owned.txt", contentType: "text/plain", processingVersion: 2, requestedAt: new Date().toISOString() };
		const claim = await claimDocumentForProcessing(first, event); claims.push(claim);
		assert.equal(claim.kind, "claimed");
		assert.equal((await claimDocumentForProcessing(second, event)).kind, "retry");
		const aborted = new Promise<void>((resolve) => claim.signal!.addEventListener("abort", () => resolve(), { once: true }));
		await second.query("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE application_name=$1", [owner]);
		await Promise.race([aborted, new Promise((_, reject) => setTimeout(() => reject(new Error("lost owner did not abort")), 5000).unref())]);
		assert.equal(claim.signal!.aborted, true);
		const recovered = await claimDocumentForProcessing(second, event); claims.push(recovered);
		assert.equal(recovered.kind, "claimed", "durable processing status must not strand work after owner death");
		await markDocumentIndexed(second, recovered.documentPk!, event);
		await recovered.release!();
		assert.equal((await claimDocumentForProcessing(second, event)).kind, "noop", "completed generation must stay deduplicated");
		await second.query("UPDATE rag.documents SET status='pending',desired_processing_version=3 WHERE document_id=$1", [id]);
		assert.equal((await claimDocumentForProcessing(second, event)).kind, "noop", "superseded delivery must be acknowledged");
		const newer = await claimDocumentForProcessing(second, { ...event, processingVersion: 3 }); claims.push(newer);
		assert.equal(newer.kind, "claimed");
		assert.equal((await claimDocumentForProcessing(first, { ...event, processingVersion: 3 })).kind, "retry");
		await markDocumentIndexed(second, newer.documentPk!, { ...event, processingVersion: 3 });
		await markDocumentIndexed(second, newer.documentPk!, event);
		await markDocumentPendingWithError(second, event, "late superseded worker");
		await markDocumentPendingWithError(second, { ...event, processingVersion: 3 }, "late completed duplicate");
		const row = (await second.query("SELECT status,current_processing_version,desired_processing_version FROM rag.documents WHERE document_id=$1", [id])).rows[0];
		assert.deepEqual(row, { status: "processed", current_processing_version: 3, desired_processing_version: 3 });
		assert.equal((await second.query("SELECT last_error FROM rag.documents WHERE document_id=$1", [id])).rows[0].last_error, null);
	} finally {
		for (const claim of claims) await claim.release?.();
		await first.end(); await second.end();
	}
});
