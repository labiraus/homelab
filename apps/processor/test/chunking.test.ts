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

test("keeps dominant segment metadata on chunked text", () => {
	const chunks = chunkText(
		[
			{
				text: "Campaign Notes",
				metadata: { sourceType: "html", title: "Campaign Notes" },
			},
			{
				text: "Tower Entrance\nBrass door and rune lock.",
				metadata: { sourceType: "html", title: "Campaign Notes", headingPath: ["Tower Entrance"] },
			},
		],
		{
			chunkSize: 80,
			chunkOverlap: 10,
		},
	);

	assert.equal(chunks.length, 1);
	assert.deepEqual(chunks[0]?.metadata?.headingPath, ["Tower Entrance"]);
	assert.equal(chunks[0]?.metadata?.title, "Campaign Notes");
});

test("serializes vectors for pgvector insertion", () => {
	assert.equal(toVectorLiteral([0.1, 0.2, 0.3]), "[0.1,0.2,0.3]");
});
