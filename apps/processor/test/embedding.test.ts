import assert from "node:assert/strict";
import test from "node:test";

import { embedText, fetchEmbedding } from "../src/embedding.js";

test("builds normalized local embeddings", () => {
	const vector = embedText("Astra keeps field notes");

	assert.equal(vector.length, 384);
	const magnitude = Math.sqrt(vector.reduce((sum, value) => sum + value * value, 0));
	assert.ok(Math.abs(magnitude - 1) < 0.000001);
});

test("uses local embeddings when no endpoint is configured", async () => {
	const embedding = await fetchEmbedding("", "local-embeddings", "Astra keeps field notes");

	assert.equal(embedding.model, "local-embeddings");
	assert.equal(embedding.vector.length, 384);
});

test("requires an endpoint for non-local models", async () => {
	await assert.rejects(
		fetchEmbedding("", "remote-model", "Astra keeps field notes"),
		/EMBEDDING_ENDPOINT is required/,
	);
});
