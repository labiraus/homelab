import { randomUUID } from "node:crypto";

import type { Pool } from "pg";

import type {
	AuditRecord,
	ConversationRecord,
	CreateProposalRequest,
	FileProposalRecord,
	MemoryRecord,
	MessageRecord,
	ToolCallRecord,
} from "./assistant_types.js";

export async function listConversations(pool: Pool, userEmail: string): Promise<ConversationRecord[]> {
	const result = await pool.query<ConversationRecord>(
		`SELECT
			id,
			conversation_id AS "conversationId",
			user_email AS "userEmail",
			title,
			status,
			created_at AS "createdAt",
			updated_at AS "updatedAt"
		FROM assistant.conversations
		WHERE user_email = $1
		ORDER BY updated_at DESC
		LIMIT 50`,
		[userEmail],
	);
	return result.rows;
}

export async function ensureConversation(
	pool: Pool,
	userEmail: string,
	conversationId: string | undefined,
	title: string | undefined,
): Promise<ConversationRecord> {
	const trimmedID = conversationId?.trim() ?? "";
	if (trimmedID) {
		const existing = await getConversation(pool, userEmail, trimmedID);
		if (existing) {
			return existing;
		}
	}

	const nextID = trimmedID || `conv-${randomUUID()}`;
	const nextTitle = title?.trim() || "New conversation";
	const result = await pool.query<ConversationRecord>(
		`INSERT INTO assistant.conversations (conversation_id, user_email, title)
		VALUES ($1, $2, $3)
		ON CONFLICT (conversation_id) DO UPDATE
		SET updated_at = NOW()
		RETURNING
			id,
			conversation_id AS "conversationId",
			user_email AS "userEmail",
			title,
			status,
			created_at AS "createdAt",
			updated_at AS "updatedAt"`,
		[nextID, userEmail, nextTitle],
	);
	return requireRow(result.rows[0], "conversation");
}

export async function getConversation(
	pool: Pool,
	userEmail: string,
	conversationId: string,
): Promise<ConversationRecord | null> {
	const result = await pool.query<ConversationRecord>(
		`SELECT
			id,
			conversation_id AS "conversationId",
			user_email AS "userEmail",
			title,
			status,
			created_at AS "createdAt",
			updated_at AS "updatedAt"
		FROM assistant.conversations
		WHERE user_email = $1 AND conversation_id = $2`,
		[userEmail, conversationId],
	);
	return result.rows[0] ?? null;
}

export async function listMessages(pool: Pool, userEmail: string, conversationId: string): Promise<MessageRecord[]> {
	const result = await pool.query<MessageRecord>(
		`SELECT
			message_id AS "messageId",
			conversation_id AS "conversationId",
			role,
			content,
			citations,
			metadata,
			created_at AS "createdAt"
		FROM assistant.messages
		WHERE user_email = $1 AND conversation_id = $2
		ORDER BY created_at ASC`,
		[userEmail, conversationId],
	);
	return result.rows.map(normalizeMessage);
}

export async function insertMessage(
	pool: Pool,
	conversation: ConversationRecord,
	role: string,
	content: string,
	citations: unknown[] = [],
	metadata: Record<string, unknown> = {},
): Promise<MessageRecord> {
	const messageId = `msg-${randomUUID()}`;
	const result = await pool.query<MessageRecord>(
		`WITH inserted AS (
			INSERT INTO assistant.messages (
				message_id, conversation_pk, conversation_id, user_email, role, content, citations, metadata
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb)
			RETURNING
				message_id AS "messageId",
				conversation_id AS "conversationId",
				role,
				content,
				citations,
				metadata,
				created_at AS "createdAt"
		), touched AS (
			UPDATE assistant.conversations SET updated_at = NOW() WHERE id = $2
		)
		SELECT * FROM inserted`,
		[
			messageId,
			conversation.id,
			conversation.conversationId,
			conversation.userEmail,
			role,
			content,
			JSON.stringify(citations),
			JSON.stringify(metadata),
		],
	);
	return normalizeMessage(requireRow(result.rows[0], "message"));
}

export async function insertToolCall(pool: Pool, record: ToolCallRecord): Promise<ToolCallRecord> {
	const toolCallId = `tool-${randomUUID()}`;
	const result = await pool.query<{ createdAt: Date }>(
		`INSERT INTO assistant.tool_calls (
			tool_call_id, conversation_id, message_id, user_email, tool_name, arguments, result, is_error
		)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8)
		RETURNING created_at AS "createdAt"`,
		[
			toolCallId,
			emptyToNull(record.conversationId),
			emptyToNull(record.messageId),
			record.userEmail,
			record.toolName,
			JSON.stringify(record.arguments ?? {}),
			JSON.stringify(record.result ?? {}),
			record.isError,
		],
	);
	return {
		...record,
		toolCallId,
		createdAt: requireRow(result.rows[0], "tool call").createdAt,
	};
}

export async function listMemories(pool: Pool, userEmail: string, includeArchived: boolean): Promise<MemoryRecord[]> {
	const result = await pool.query<MemoryRecord & { archivedAtDate: Date | null }>(
		`SELECT
			memory_id AS "memoryId",
			user_email AS "userEmail",
			text,
			COALESCE(source_conversation_id, '') AS "sourceConversationId",
			status,
			created_at AS "createdAt",
			archived_at AS "archivedAtDate"
		FROM assistant.memories
		WHERE user_email = $1
		  AND ($2::boolean OR status = 'active')
		ORDER BY created_at DESC
		LIMIT 100`,
		[userEmail, includeArchived],
	);
	return result.rows.map((row) => ({
		memoryId: row.memoryId,
		userEmail: row.userEmail,
		text: row.text,
		sourceConversationId: row.sourceConversationId || undefined,
		status: row.status,
		createdAt: row.createdAt,
		archivedAt: row.archivedAtDate?.toISOString(),
	}));
}

export async function insertMemory(
	pool: Pool,
	userEmail: string,
	text: string,
	sourceConversationId: string | undefined,
): Promise<MemoryRecord> {
	const result = await pool.query<MemoryRecord>(
		`INSERT INTO assistant.memories (memory_id, user_email, text, source_conversation_id)
		VALUES ($1, $2, $3, $4)
		RETURNING
			memory_id AS "memoryId",
			user_email AS "userEmail",
			text,
			COALESCE(source_conversation_id, '') AS "sourceConversationId",
			status,
			created_at AS "createdAt"`,
		[`mem-${randomUUID()}`, userEmail, text, emptyToNull(sourceConversationId)],
	);
	const row = requireRow(result.rows[0], "memory");
	return { ...row, sourceConversationId: row.sourceConversationId || undefined };
}

export async function archiveMemory(pool: Pool, userEmail: string, memoryId: string): Promise<boolean> {
	const result = await pool.query(
		`UPDATE assistant.memories
		SET status = 'archived', archived_at = NOW()
		WHERE user_email = $1 AND memory_id = $2`,
		[userEmail, memoryId],
	);
	return result.rowCount === 1;
}

export async function insertProposal(
	pool: Pool,
	userEmail: string,
	request: CreateProposalRequest,
): Promise<FileProposalRecord> {
	const contentType = request.contentType?.trim() || "text/plain; charset=utf-8";
	const result = await pool.query<FileProposalRecord>(
		`INSERT INTO assistant.file_proposals (
			proposal_id, conversation_id, user_email, action, document_id, object_key, content_type, proposed_text, rationale
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING
			proposal_id AS "proposalId",
			conversation_id AS "conversationId",
			user_email AS "userEmail",
			action,
			COALESCE(document_id, '') AS "documentId",
			object_key AS "objectKey",
			content_type AS "contentType",
			proposed_text AS "proposedText",
			rationale,
			status,
			orchestrator_response AS "orchestratorResponse",
			created_at AS "createdAt"`,
		[
			`prop-${randomUUID()}`,
			request.conversationId,
			userEmail,
			request.action,
			emptyToNull(request.documentId),
			request.objectKey,
			contentType,
			request.proposedText,
			request.rationale ?? "",
		],
	);
	return normalizeProposal(requireRow(result.rows[0], "proposal"));
}

export async function getProposal(
	pool: Pool,
	userEmail: string,
	proposalId: string,
): Promise<FileProposalRecord | null> {
	const result = await pool.query<FileProposalRecord & { decidedAtDate: Date | null }>(
		`SELECT
			proposal_id AS "proposalId",
			conversation_id AS "conversationId",
			user_email AS "userEmail",
			action,
			COALESCE(document_id, '') AS "documentId",
			object_key AS "objectKey",
			content_type AS "contentType",
			proposed_text AS "proposedText",
			rationale,
			status,
			orchestrator_response AS "orchestratorResponse",
			created_at AS "createdAt",
			decided_at AS "decidedAtDate"
		FROM assistant.file_proposals
		WHERE user_email = $1 AND proposal_id = $2`,
		[userEmail, proposalId],
	);
	const row = result.rows[0];
	return row ? normalizeProposal(row) : null;
}

export async function listProposals(
	pool: Pool,
	userEmail: string,
	conversationId: string,
): Promise<FileProposalRecord[]> {
	const result = await pool.query<FileProposalRecord & { decidedAtDate: Date | null }>(
		`SELECT
			proposal_id AS "proposalId",
			conversation_id AS "conversationId",
			user_email AS "userEmail",
			action,
			COALESCE(document_id, '') AS "documentId",
			object_key AS "objectKey",
			content_type AS "contentType",
			proposed_text AS "proposedText",
			rationale,
			status,
			orchestrator_response AS "orchestratorResponse",
			created_at AS "createdAt",
			decided_at AS "decidedAtDate"
		FROM assistant.file_proposals
		WHERE user_email = $1 AND conversation_id = $2
		ORDER BY created_at DESC`,
		[userEmail, conversationId],
	);
	return result.rows.map(normalizeProposal);
}

export async function updateProposalDecision(
	pool: Pool,
	userEmail: string,
	proposalId: string,
	status: string,
	orchestratorResponse: Record<string, unknown>,
): Promise<FileProposalRecord> {
	const result = await pool.query<FileProposalRecord & { decidedAtDate: Date | null }>(
		`UPDATE assistant.file_proposals
		SET status = $3, orchestrator_response = $4::jsonb, decided_at = NOW()
		WHERE user_email = $1 AND proposal_id = $2
		RETURNING
			proposal_id AS "proposalId",
			conversation_id AS "conversationId",
			user_email AS "userEmail",
			action,
			COALESCE(document_id, '') AS "documentId",
			object_key AS "objectKey",
			content_type AS "contentType",
			proposed_text AS "proposedText",
			rationale,
			status,
			orchestrator_response AS "orchestratorResponse",
			created_at AS "createdAt",
			decided_at AS "decidedAtDate"`,
		[userEmail, proposalId, status, JSON.stringify(orchestratorResponse)],
	);
	return normalizeProposal(requireRow(result.rows[0], "proposal decision"));
}

export async function listAudits(pool: Pool, userEmail: string, conversationId: string): Promise<AuditRecord[]> {
	const args: unknown[] = [userEmail];
	let query = `SELECT
		audit_id AS "auditId",
		document_id AS "documentId",
		object_key AS "objectKey",
		action,
		actor_email AS "actorEmail",
		COALESCE(conversation_id, '') AS "conversationId",
		COALESCE(proposal_id, '') AS "proposalId",
		COALESCE(old_version_marker, '') AS "oldVersionMarker",
		COALESCE(new_version_marker, '') AS "newVersionMarker",
		COALESCE(reverted_to_version_marker, '') AS "revertedToVersionMarker",
		processing_version AS "processingVersion",
		metadata,
		created_at AS "createdAt"
	FROM rag.document_change_audits
	WHERE actor_email = $1`;
	if (conversationId.trim()) {
		args.push(conversationId.trim());
		query += " AND conversation_id = $2";
	}
	query += " ORDER BY created_at DESC LIMIT 100";

	const result = await pool.query<AuditRecord>(query, args);
	return result.rows.map((row) => ({
		...row,
		conversationId: row.conversationId || undefined,
		proposalId: row.proposalId || undefined,
		oldVersionMarker: row.oldVersionMarker || undefined,
		newVersionMarker: row.newVersionMarker || undefined,
		revertedToVersionMarker: row.revertedToVersionMarker || undefined,
		metadata: row.metadata ?? {},
	}));
}

function normalizeMessage(row: MessageRecord): MessageRecord {
	return {
		...row,
		citations: Array.isArray(row.citations) ? row.citations : [],
		metadata: row.metadata ?? {},
	};
}

function normalizeProposal(row: FileProposalRecord & { decidedAtDate?: Date | null }): FileProposalRecord {
	return {
		...row,
		documentId: row.documentId || undefined,
		orchestratorResponse: row.orchestratorResponse ?? {},
		decidedAt: row.decidedAtDate?.toISOString() ?? row.decidedAt,
	};
}

function emptyToNull(value: string | undefined): string | null {
	const trimmed = value?.trim() ?? "";
	return trimmed ? trimmed : null;
}

function requireRow<T>(row: T | undefined, label: string): T {
	if (!row) {
		throw new Error(`expected ${label} row`);
	}
	return row;
}
