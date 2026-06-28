import { Annotation, END, START, StateGraph } from "@langchain/langgraph";

import type {
	AssistantRunResult,
	ConversationRecord,
	DocumentContextResult,
	MemoryRecord,
	ToolCallRecord,
} from "./assistant_types.js";

interface AssistantGraphState {
	userEmail: string;
	conversation: ConversationRecord;
	message: string;
	memories: MemoryRecord[];
	contextResult: DocumentContextResult;
	content: string;
	metadata: Record<string, unknown>;
}

const AssistantState = Annotation.Root({
	userEmail: Annotation<string>(),
	conversation: Annotation<ConversationRecord>(),
	message: Annotation<string>(),
	memories: Annotation<MemoryRecord[]>(),
	contextResult: Annotation<DocumentContextResult>(),
	content: Annotation<string>(),
	metadata: Annotation<Record<string, unknown>>(),
});

export async function runAssistantChat(
	userEmail: string,
	conversation: ConversationRecord,
	message: string,
	memories: MemoryRecord[],
): Promise<AssistantRunResult> {
	const graph = new StateGraph(AssistantState)
		.addNode("retrieve", async (state: AssistantGraphState) => {
			const contextResult = await callDocumentsContext(state.userEmail, state.message);
			return {
				contextResult,
				metadata: {
					model: llmModel(),
					langGraphWorkflow: "assistant-chat",
					contextToolUsed: true,
					...(contextResult.isError ? { contextError: contextResult.error } : {}),
				},
			};
		})
		.addNode("generate", async (state: AssistantGraphState) => {
			let content = "";
			const metadata = { ...(state.metadata ?? {}) };
			try {
				content = await callLLM(state);
			} catch (error) {
				metadata.llmError = formatError(error);
			}
			if (!content.trim()) {
				content = fallbackReply(state.contextResult);
			}
			return { content, metadata };
		})
		.addEdge(START, "retrieve")
		.addEdge("retrieve", "generate")
		.addEdge("generate", END)
		.compile();

	const result = await graph.invoke({
		userEmail,
		conversation,
		message,
		memories,
		contextResult: { context: "", citations: [], raw: {}, isError: false },
		content: "",
		metadata: {},
	});

	const contextResult = result.contextResult;
	const toolCall: ToolCallRecord = {
		toolName: "documents.context",
		userEmail,
		arguments: {
			query: message,
			limit: 4,
			maxChars: 5000,
		},
		result: contextResult.isError ? { error: contextResult.error ?? "context request failed" } : contextResult.raw,
		isError: contextResult.isError,
	};

	return {
		content: result.content,
		citations: contextResult.citations,
		metadata: result.metadata,
		toolCalls: [toolCall],
	};
}

async function callDocumentsContext(userEmail: string, query: string): Promise<DocumentContextResult> {
	const baseURL = externalBaseURL();
	if (!baseURL) {
		return { context: "", citations: [], raw: {}, isError: true, error: "EXTERNAL_BASE_URL is not configured" };
	}

	const controller = new AbortController();
	const timeout = setTimeout(() => controller.abort(), contextTimeoutSeconds() * 1000);
	try {
		const response = await fetch(`${baseURL}/api/documents/context`, {
			method: "POST",
			headers: {
				Accept: "application/json",
				"Content-Type": "application/json",
				"X-Forwarded-Email": userEmail,
				UserID: userEmail,
			},
			body: JSON.stringify({ query, limit: 4, maxChars: 5000 }),
			signal: controller.signal,
		});
		const raw = (await response.json().catch(() => ({}))) as Record<string, unknown>;
		if (!response.ok) {
			return {
				context: "",
				citations: [],
				raw,
				isError: true,
				error: `external context returned status ${response.status}`,
			};
		}
		return {
			context: stringValue(raw.context),
			citations: Array.isArray(raw.citations) ? raw.citations : [],
			raw,
			isError: false,
		};
	} catch (error) {
		return { context: "", citations: [], raw: {}, isError: true, error: formatError(error) };
	} finally {
		clearTimeout(timeout);
	}
}

async function callLLM(state: AssistantGraphState): Promise<string> {
	const baseURL = llmBaseURL();
	if (!baseURL) {
		throw new Error("AI_GATEWAY_BASE_URL or LLM_BASE_URL is not configured");
	}

	const response = await fetch(`${baseURL}/chat/completions`, {
		method: "POST",
		headers: {
			Accept: "application/json",
			"Content-Type": "application/json",
		},
		body: JSON.stringify({
			model: llmModel(),
			temperature: 0.2,
			max_tokens: llmMaxTokens(),
			messages: [
				{ role: "system", content: assistantSystemPrompt(state.memories, state.contextResult) },
				{ role: "user", content: state.message },
			],
			metadata: {
				user_email: state.userEmail,
				conversation_id: state.conversation.conversationId,
			},
		}),
	});
	const raw = (await response.json().catch(() => ({}))) as {
		choices?: Array<{ message?: { content?: string } }>;
		error?: { message?: string };
	};
	if (!response.ok) {
		throw new Error(raw.error?.message || `LLM returned status ${response.status}`);
	}
	return raw.choices?.[0]?.message?.content?.trim() ?? "";
}

function assistantSystemPrompt(memories: MemoryRecord[], contextResult: DocumentContextResult): string {
	const lines = [
		"You are the Labiraus assistant. Answer using the provided Labiraus RAG context when it is relevant.",
		"Cite document references that appear in the context. Do not claim file changes have been made unless the user approved a persisted proposal.",
		"",
	];
	if (memories.length > 0) {
		lines.push("User-approved memories for this authenticated user:");
		for (const memory of memories) {
			lines.push(`- ${memory.text}`);
		}
		lines.push("");
	}
	if (contextResult.context) {
		lines.push("Labiraus RAG context:", contextResult.context);
	} else if (contextResult.isError) {
		lines.push(`Labiraus RAG context could not be loaded: ${contextResult.error ?? "unknown error"}`);
	} else {
		lines.push("No Labiraus RAG context was found for this request.");
	}
	return lines.join("\n");
}

function fallbackReply(contextResult: DocumentContextResult): string {
	if (contextResult.isError) {
		return "I could not retrieve the Labiraus RAG context for that request. The conversation was saved, but the answer should be retried after the document context tool is healthy.";
	}
	if (!contextResult.context.trim()) {
		return "I searched the Labiraus documents but did not find relevant context for that request.";
	}
	return `I searched the Labiraus documents and found this relevant context:\n\n${contextResult.context}`;
}

function externalBaseURL(): string {
	return trimTrailingSlash(process.env.EXTERNAL_BASE_URL ?? "http://homelab-external.homelab.svc.cluster.local");
}

function llmBaseURL(): string {
	return trimTrailingSlash(process.env.AI_GATEWAY_BASE_URL ?? process.env.LLM_BASE_URL ?? "");
}

function llmModel(): string {
	return process.env.AI_CHAT_MODEL?.trim() || process.env.LLM_MODEL?.trim() || "Qwen/Qwen2.5-0.5B-Instruct";
}

function llmMaxTokens(): number {
	const value = Number(process.env.AI_CHAT_MAX_TOKENS ?? process.env.LLM_MAX_TOKENS ?? "768");
	return Number.isFinite(value) && value > 0 ? value : 768;
}

function contextTimeoutSeconds(): number {
	const value = Number(process.env.EXTERNAL_CONTEXT_TIMEOUT_SECONDS ?? process.env.MCP_CONTEXT_TIMEOUT_SECONDS ?? "5");
	return Number.isFinite(value) && value > 0 ? value : 5;
}

function trimTrailingSlash(value: string): string {
	return value.trim().replace(/\/+$/, "");
}

function stringValue(value: unknown): string {
	return typeof value === "string" ? value : "";
}

function formatError(error: unknown): string {
	return error instanceof Error ? error.message : String(error);
}
