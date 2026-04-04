import assert from "node:assert/strict";
import test from "node:test";

import { chunkText } from "../src/chunking.js";
import { toVectorLiteral } from "../src/db.js";

test("chunks text deterministically", () => {
	const chunks = chunkText("alpha beta gamma delta epsilon", {
		chunkSize: 12,
		chunkOverlap: 3,
	});

	assert.ok(chunks.length > 1);
	assert.equal(chunks[0]?.index, 0);
	assert.ok((chunks[0]?.tokenCount ?? 0) > 0);
});

test("serializes vectors for pgvector insertion", () => {
	assert.equal(toVectorLiteral([0.1, 0.2, 0.3]), "[0.1,0.2,0.3]");
});
