import type { IncomingMessage, ServerResponse } from "node:http";

import type { Pool } from "pg";

import {
	archiveMemory,
	ensureConversation,
	getConversation,
	getProposal,
	insertMemory,
	insertMessage,
	insertProposal,
	insertToolCall,
	listAudits,
	listConversations,
	listMemories,
	listMessages,
	listProposals,
	updateProposalDecision,
} from "./assistant_db.js";
import { runAssistantChat } from "./assistant_llm.js";
import type { ChatRequest, CreateMemoryRequest, CreateProposalRequest, FileProposalRecord } from "./assistant_types.js";

const assistantBodyLimit = 2 << 20;

export function canHandleAssistantRequest(pathname: string): boolean {
	return pathname === "/assistant" || pathname.startsWith("/assistant/");
}

export async function handleAssistantRequest(
	pool: Pool | undefined,
	request: IncomingMessage,
	response: ServerResponse,
): Promise<void> {
	if (!pool) {
		writeJSON(response, 503, { error: "postgres is not initialized" });
		return;
	}

	const userEmail = authenticatedEmail(request);
	if (!userEmail) {
		writeJSON(response, 401, { error: "authenticated email is required" });
		return;
	}

	const url = new URL(request.url ?? "/", "http://processor.local");
	const pathname = url.pathname;
	try {
		if (pathname === "/assistant/conversations") {
			await handleConversations(pool, userEmail, request, response);
			return;
		}
		if (pathname.startsWith("/assistant/conversations/")) {
			await handleConversationDetail(pool, userEmail, pathname, request, response);
			return;
		}
		if (pathname === "/assistant/chat") {
			await handleChat(pool, userEmail, request, response);
			return;
		}
		if (pathname === "/assistant/memories") {
			await handleMemories(pool, userEmail, url, request, response);
			return;
		}
		if (pathname.startsWith("/assistant/memories/")) {
			await handleMemoryAction(pool, userEmail, pathname, request, response);
			return;
		}
		if (pathname === "/assistant/proposals") {
			await handleProposals(pool, userEmail, url, request, response);
			return;
		}
		if (pathname.startsWith("/assistant/proposals/")) {
			await handleProposalAction(pool, userEmail, pathname, request, response);
			return;
		}
		if (pathname === "/assistant/audits") {
			await handleAudits(pool, userEmail, url, request, response);
			return;
		}
		writeJSON(response, 404, { error: "assistant route not found" });
	} catch (error) {
		console.error(error);
		writeJSON(response, 500, { error: "assistant request failed" });
	}
}

async function handleConversations(
	pool: Pool,
	userEmail: string,
	request: IncomingMessage,
	response: ServerResponse,
): Promise<void> {
	if (request.method === "GET") {
		writeJSON(response, 200, { conversations: await listConversations(pool, userEmail) });
		return;
	}
	if (request.method === "POST") {
		const body = await readJSONBody<{ title?: string }>(request);
		const conversation = await ensureConversation(pool, userEmail, "", body.title);
		writeJSON(response, 201, { conversation });
		return;
	}
	response.writeHead(405).end();
}

async function handleConversationDetail(
	pool: Pool,
	userEmail: string,
	pathname: string,
	request: IncomingMessage,
	response: ServerResponse,
): Promise<void> {
	if (request.method !== "GET") {
		response.writeHead(405).end();
		return;
	}
	const conversationId = pathValue(pathname, "/assistant/conversations/");
	if (!conversationId) {
		writeJSON(response, 400, { error: "conversationId is required" });
		return;
	}
	const conversation = await getConversation(pool, userEmail, conversationId);
	if (!conversation) {
		writeJSON(response, 404, { error: "conversation not found" });
		return;
	}
	const [messages, proposals, audits] = await Promise.all([
		listMessages(pool, userEmail, conversationId),
		listProposals(pool, userEmail, conversationId),
		listAudits(pool, userEmail, conversationId),
	]);
	writeJSON(response, 200, { conversation, messages, proposals, audits });
}

async function handleChat(pool: Pool, userEmail: string, request: IncomingMessage, response: ServerResponse): Promise<void> {
	if (request.method !== "POST") {
		response.writeHead(405).end();
		return;
	}
	const body = await readJSONBody<ChatRequest>(request);
	const message = body.message?.trim() ?? "";
	if (!message) {
		writeJSON(response, 400, { error: "message is required" });
		return;
	}

	const conversation = await ensureConversation(pool, userEmail, body.conversationId, body.title);
	const userMessage = await insertMessage(pool, conversation, "user", message);
	const memories = await listMemories(pool, userEmail, false);
	const result = await runAssistantChat(userEmail, conversation, message, memories);
	const reply = await insertMessage(pool, conversation, "assistant", result.content, result.citations, result.metadata);
	const toolCalls = [];
	for (const toolCall of result.toolCalls) {
		toolCalls.push(
			await insertToolCall(pool, {
				...toolCall,
				conversationId: conversation.conversationId,
				messageId: reply.messageId,
				userEmail,
			}),
		);
	}
	writeJSON(response, 200, { conversation, userMessage, reply, toolCalls });
}

async function handleMemories(
	pool: Pool,
	userEmail: string,
	url: URL,
	request: IncomingMessage,
	response: ServerResponse,
): Promise<void> {
	if (request.method === "GET") {
		writeJSON(response, 200, {
			memories: await listMemories(pool, userEmail, url.searchParams.get("includeArchived") === "true"),
		});
		return;
	}
	if (request.method === "POST") {
		const body = await readJSONBody<CreateMemoryRequest>(request);
		const text = body.text?.trim() ?? "";
		if (!text) {
			writeJSON(response, 400, { error: "text is required" });
			return;
		}
		const memory = await insertMemory(pool, userEmail, text, body.sourceConversationId);
		writeJSON(response, 201, { memory });
		return;
	}
	response.writeHead(405).end();
}

async function handleMemoryAction(
	pool: Pool,
	userEmail: string,
	pathname: string,
	request: IncomingMessage,
	response: ServerResponse,
): Promise<void> {
	if (request.method !== "POST") {
		response.writeHead(405).end();
		return;
	}
	const memoryId = pathValue(pathname, "/assistant/memories/", "/archive");
	if (!memoryId) {
		writeJSON(response, 404, { error: "memory action not found" });
		return;
	}
	const archived = await archiveMemory(pool, userEmail, memoryId);
	if (!archived) {
		writeJSON(response, 404, { error: "memory not found" });
		return;
	}
	writeJSON(response, 200, { status: "archived" });
}

async function handleProposals(
	pool: Pool,
	userEmail: string,
	url: URL,
	request: IncomingMessage,
	response: ServerResponse,
): Promise<void> {
	if (request.method === "GET") {
		const conversationId = url.searchParams.get("conversationId")?.trim() ?? "";
		if (!conversationId) {
			writeJSON(response, 400, { error: "conversationId is required" });
			return;
		}
		writeJSON(response, 200, { proposals: await listProposals(pool, userEmail, conversationId) });
		return;
	}
	if (request.method === "POST") {
		const body = await readJSONBody<CreateProposalRequest>(request);
		const validation = validateProposal(body);
		if (validation) {
			writeJSON(response, 400, { error: validation });
			return;
		}
		const proposal = await insertProposal(pool, userEmail, body);
		writeJSON(response, 201, { proposal });
		return;
	}
	response.writeHead(405).end();
}

async function handleProposalAction(
	pool: Pool,
	userEmail: string,
	pathname: string,
	request: IncomingMessage,
	response: ServerResponse,
): Promise<void> {
	if (request.method !== "POST") {
		response.writeHead(405).end();
		return;
	}
	const approve = pathname.endsWith("/approve");
	const reject = pathname.endsWith("/reject");
	const proposalId = pathValue(pathname, "/assistant/proposals/", approve ? "/approve" : reject ? "/reject" : "");
	if (!proposalId || (!approve && !reject)) {
		writeJSON(response, 404, { error: "proposal action not found" });
		return;
	}
	const proposal = await getProposal(pool, userEmail, proposalId);
	if (!proposal) {
		writeJSON(response, 404, { error: "proposal not found" });
		return;
	}
	if (proposal.status !== "pending") {
		writeJSON(response, 409, { error: "proposal has already been decided" });
		return;
	}
	if (reject) {
		const body: { reason?: string } = await readJSONBody<{ reason?: string }>(request).catch(() => ({}));
		const updated = await updateProposalDecision(pool, userEmail, proposalId, "rejected", {
			reason: body.reason?.trim() || "rejected",
		});
		writeJSON(response, 200, { proposal: updated });
		return;
	}

	const orchestratorResponse = await approveProposal(proposal);
	const updated = await updateProposalDecision(pool, userEmail, proposalId, "approved", orchestratorResponse);
	writeJSON(response, 200, { proposal: updated });
}

async function handleAudits(
	pool: Pool,
	userEmail: string,
	url: URL,
	request: IncomingMessage,
	response: ServerResponse,
): Promise<void> {
	if (request.method !== "GET") {
		response.writeHead(405).end();
		return;
	}
	writeJSON(response, 200, {
		audits: await listAudits(pool, userEmail, url.searchParams.get("conversationId") ?? ""),
	});
}

async function approveProposal(proposal: FileProposalRecord): Promise<Record<string, unknown>> {
	const baseURL = (process.env.ORCHESTRATOR_BASE_URL ?? "").trim().replace(/\/+$/, "");
	if (!baseURL) {
		throw new Error("ORCHESTRATOR_BASE_URL is not configured");
	}
	const path = proposal.action === "edit" ? "/documents/edit-text" : "/documents/create-text";
	if (proposal.action !== "edit" && proposal.action !== "create") {
		throw new Error(`unsupported proposal action ${proposal.action}`);
	}
	const response = await fetch(`${baseURL}${path}`, {
		method: "POST",
		headers: {
			Accept: "application/json",
			"Content-Type": "application/json",
			"X-Forwarded-Email": proposal.userEmail,
			UserID: proposal.userEmail,
		},
		body: JSON.stringify({
			documentId: proposal.documentId,
			objectKey: proposal.objectKey,
			contentType: proposal.contentType,
			text: proposal.proposedText,
			actorEmail: proposal.userEmail,
			conversationId: proposal.conversationId,
			proposalId: proposal.proposalId,
			metadata: {
				assistantProposalId: proposal.proposalId,
				assistantRationale: proposal.rationale,
			},
		}),
	});
	const decoded = (await response.json().catch(() => ({}))) as Record<string, unknown>;
	decoded.statusCode = response.status;
	if (!response.ok) {
		throw new Error(`orchestrator returned status ${response.status}`);
	}
	return decoded;
}

async function readJSONBody<T>(request: IncomingMessage): Promise<T> {
	let size = 0;
	const chunks: Buffer[] = [];
	for await (const chunk of request) {
		const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
		size += buffer.length;
		if (size > assistantBodyLimit) {
			throw new Error("request body is too large");
		}
		chunks.push(buffer);
	}
	const raw = Buffer.concat(chunks).toString("utf8").trim();
	if (!raw) {
		return {} as T;
	}
	return JSON.parse(raw) as T;
}

function authenticatedEmail(request: IncomingMessage): string {
	return (
		stringHeader(request, "x-forwarded-email") ||
		stringHeader(request, "x-auth-request-email") ||
		stringHeader(request, "userid")
	)
		.trim()
		.toLowerCase();
}

function stringHeader(request: IncomingMessage, name: string): string {
	const value = request.headers[name];
	return Array.isArray(value) ? value[0] ?? "" : value ?? "";
}

function writeJSON(response: ServerResponse, status: number, payload: unknown): void {
	const body = JSON.stringify(payload);
	response.writeHead(status, {
		"Content-Type": "application/json",
		"Content-Length": Buffer.byteLength(body),
	});
	response.end(body);
}

function pathValue(pathname: string, prefix: string, suffix = ""): string {
	if (!pathname.startsWith(prefix)) {
		return "";
	}
	let value = pathname.slice(prefix.length);
	if (suffix) {
		if (!value.endsWith(suffix)) {
			return "";
		}
		value = value.slice(0, -suffix.length);
	}
	return decodeURIComponent(value).trim();
}

function validateProposal(request: CreateProposalRequest): string {
	const action = request.action?.trim() ?? "";
	if (action !== "create" && action !== "edit") {
		return "action must be create or edit";
	}
	if (!request.conversationId?.trim()) {
		return "conversationId is required";
	}
	if (!request.objectKey?.trim()) {
		return "objectKey is required";
	}
	if (!request.proposedText?.trim()) {
		return "proposedText is required";
	}
	if (action === "edit" && !request.documentId?.trim()) {
		return "documentId is required for edits";
	}
	return "";
}
