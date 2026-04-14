import assert from "node:assert/strict";
import test from "node:test";

import { buildLifecycleEvent, DOCUMENT_EVENT_SUBJECTS } from "../src/events.js";

test("builds lifecycle events from the document payload", () => {
	const event = buildLifecycleEvent(DOCUMENT_EVENT_SUBJECTS.processorFailed, {
		documentId: "doc-1",
		bucket: "documents",
		objectKey: "reports/doc-1.txt",
		sourceUri: "s3://documents/reports/doc-1.txt",
		contentType: "text/plain",
		requestedAt: "2026-04-14T10:00:00Z",
		processingVersion: 3,
	});

	assert.equal(event.subject, "documents.events.processor.failed");
	assert.equal(event.documentId, "doc-1");
	assert.equal(event.objectKey, "reports/doc-1.txt");
	assert.equal(event.processingVersion, 3);
	assert.match(event.occurredAt, /^\d{4}-\d{2}-\d{2}T/);
});
