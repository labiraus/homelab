import assert from "node:assert/strict";
import { test } from "node:test";

import { runAssistantChat } from "../src/assistant_llm.js";
import type { ConversationRecord } from "../src/assistant_types.js";

test("assistant chat records context tool failure and falls back when model is unavailable", async () => {
	const previousFetch = globalThis.fetch;
	globalThis.fetch = (async () => {
		throw new Error("network unavailable");
	}) as typeof fetch;
	try {
		const conversation: ConversationRecord = {
			id: 1,
			conversationId: "conv-test",
			userEmail: "user@example.com",
			title: "Test",
			status: "active",
			createdAt: new Date("2026-01-01T00:00:00Z"),
			updatedAt: new Date("2026-01-01T00:00:00Z"),
		};

		const result = await runAssistantChat("user@example.com", conversation, "where are the notes?", []);

		assert.match(result.content, /could not retrieve/i);
		assert.equal(result.toolCalls.length, 1);
		assert.equal(result.toolCalls[0]?.toolName, "documents.context");
		assert.equal(result.toolCalls[0]?.isError, true);
		assert.equal(result.metadata.langGraphWorkflow, "assistant-chat");
		assert.ok(result.metadata.llmError);
	} finally {
		globalThis.fetch = previousFetch;
	}
});
