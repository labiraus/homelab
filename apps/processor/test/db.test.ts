import assert from "node:assert/strict";
import test from "node:test";

import { resolveClaimResult } from "../src/db.js";
import type { DocumentState } from "../src/types.js";

function buildState(overrides: Partial<DocumentState> = {}): DocumentState {
	return {
		documentPk: 42,
		status: "pending",
		desiredProcessingVersion: 2,
		currentProcessingVersion: 1,
		...overrides,
	};
}

test("retries when the document row is not visible yet", () => {
	assert.deepEqual(resolveClaimResult(null, 2), {
		kind: "retry",
		reason: "document row is not visible yet",
	});
});

test("retries when the committed document version lags the event", () => {
	assert.deepEqual(resolveClaimResult(buildState({ desiredProcessingVersion: 1 }), 2), {
		kind: "retry",
		reason: "document version is not committed yet",
	});
});

test("noops when the event is stale", () => {
	assert.deepEqual(resolveClaimResult(buildState({ desiredProcessingVersion: 3 }), 2), {
		kind: "noop",
		reason: "document was superseded by a newer version",
	});
});

test("noops when the version is already processed", () => {
	assert.deepEqual(resolveClaimResult(buildState({ currentProcessingVersion: 2, status: "processed" }), 2), {
		kind: "noop",
		reason: "document version is already processed",
	});
});

test("retries when another worker already claimed the document", () => {
	assert.deepEqual(resolveClaimResult(buildState({ status: "processing" }), 2), {
		kind: "retry",
		reason: "document is already being processed",
	});
});

test("claims a pending document when versions match", () => {
	assert.deepEqual(resolveClaimResult(buildState(), 2), {
		kind: "claimed",
		documentPk: 42,
		reason: "document claimed for processing",
	});
});
