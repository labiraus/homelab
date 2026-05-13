package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"pkg/postgresutil"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func listConversations(ctx context.Context, userEmail string) ([]conversationRecord, error) {
	rows, err := postgresutil.Query(ctx, `SELECT id, conversation_id, user_email, title, status, created_at, updated_at
FROM assistant.conversations
WHERE user_email = $1
ORDER BY updated_at DESC
LIMIT 50`, userEmail)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	conversations := []conversationRecord{}
	for rows.Next() {
		var row conversationRecord
		if err := rows.Scan(&row.ID, &row.ConversationID, &row.UserEmail, &row.Title, &row.Status, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		conversations = append(conversations, row)
	}
	return conversations, rows.Err()
}

func ensureConversation(ctx context.Context, userEmail string, conversationID string, title string) (conversationRecord, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID != "" {
		conversation, found, err := getConversation(ctx, userEmail, conversationID)
		if err != nil || found {
			return conversation, err
		}
	}

	if conversationID == "" {
		conversationID = "conv-" + uuid.NewString()
	}
	if strings.TrimSpace(title) == "" {
		title = "New conversation"
	}

	var row conversationRecord
	err := postgresutil.QueryRow(ctx, `INSERT INTO assistant.conversations (conversation_id, user_email, title)
VALUES ($1, $2, $3)
ON CONFLICT (conversation_id) DO UPDATE
SET updated_at = NOW()
RETURNING id, conversation_id, user_email, title, status, created_at, updated_at`, conversationID, userEmail, title).
		Scan(&row.ID, &row.ConversationID, &row.UserEmail, &row.Title, &row.Status, &row.CreatedAt, &row.UpdatedAt)
	return row, err
}

func getConversation(ctx context.Context, userEmail string, conversationID string) (conversationRecord, bool, error) {
	var row conversationRecord
	err := postgresutil.QueryRow(ctx, `SELECT id, conversation_id, user_email, title, status, created_at, updated_at
FROM assistant.conversations
WHERE user_email = $1 AND conversation_id = $2`, userEmail, conversationID).
		Scan(&row.ID, &row.ConversationID, &row.UserEmail, &row.Title, &row.Status, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return conversationRecord{}, false, nil
		}
		return conversationRecord{}, false, err
	}
	return row, true, nil
}

func listMessages(ctx context.Context, userEmail string, conversationID string) ([]messageRecord, error) {
	rows, err := postgresutil.Query(ctx, `SELECT message_id, conversation_id, role, content, citations::text, metadata::text, created_at
FROM assistant.messages
WHERE user_email = $1 AND conversation_id = $2
ORDER BY created_at ASC`, userEmail, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []messageRecord{}
	for rows.Next() {
		var row messageRecord
		var citationsRaw string
		var metadataRaw string
		if err := rows.Scan(&row.MessageID, &row.ConversationID, &row.Role, &row.Content, &citationsRaw, &metadataRaw, &row.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(citationsRaw), &row.Citations)
		_ = json.Unmarshal([]byte(metadataRaw), &row.Metadata)
		normalizeMessageJSON(&row)
		messages = append(messages, row)
	}
	return messages, rows.Err()
}

func insertMessage(ctx context.Context, conversation conversationRecord, role string, content string, citations []any, metadata map[string]any) (messageRecord, error) {
	if citations == nil {
		citations = []any{}
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	citationsBytes, err := json.Marshal(citations)
	if err != nil {
		return messageRecord{}, err
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return messageRecord{}, err
	}

	messageID := "msg-" + uuid.NewString()
	var row messageRecord
	var citationsRaw string
	var metadataRaw string
	err = postgresutil.QueryRow(ctx, `WITH inserted AS (
	INSERT INTO assistant.messages (
		message_id, conversation_pk, conversation_id, user_email, role, content, citations, metadata
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb)
	RETURNING message_id, conversation_id, role, content, citations::text, metadata::text, created_at
), touched AS (
	UPDATE assistant.conversations SET updated_at = NOW() WHERE id = $2
)
SELECT message_id, conversation_id, role, content, citations::text, metadata::text, created_at FROM inserted`,
		messageID, conversation.ID, conversation.ConversationID, conversation.UserEmail, role, content, string(citationsBytes), string(metadataBytes)).
		Scan(&row.MessageID, &row.ConversationID, &row.Role, &row.Content, &citationsRaw, &metadataRaw, &row.CreatedAt)
	if err != nil {
		return messageRecord{}, err
	}
	if err := decodeJSONField(citationsRaw, &row.Citations); err != nil {
		return messageRecord{}, err
	}
	if err := decodeJSONField(metadataRaw, &row.Metadata); err != nil {
		return messageRecord{}, err
	}
	normalizeMessageJSON(&row)
	return row, err
}

func insertToolCall(ctx context.Context, record toolCallRecord) (toolCallRecord, error) {
	record.ToolCallID = "tool-" + uuid.NewString()
	argsBytes, err := json.Marshal(record.Arguments)
	if err != nil {
		return toolCallRecord{}, err
	}
	resultBytes, err := json.Marshal(record.Result)
	if err != nil {
		return toolCallRecord{}, err
	}
	err = postgresutil.QueryRow(ctx, `INSERT INTO assistant.tool_calls (
	tool_call_id, conversation_id, message_id, user_email, tool_name, arguments, result, is_error
) VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8)
RETURNING created_at`, record.ToolCallID, nullIfEmpty(record.ConversationID), nullIfEmpty(record.MessageID), record.UserEmail, record.ToolName, string(argsBytes), string(resultBytes), record.IsError).
		Scan(&record.CreatedAt)
	return record, err
}

func listMemories(ctx context.Context, userEmail string, includeArchived bool) ([]memoryRecord, error) {
	query := `SELECT memory_id, user_email, text, COALESCE(source_conversation_id, ''), status, created_at, archived_at
FROM assistant.memories
WHERE user_email = $1`
	args := []any{userEmail}
	if !includeArchived {
		query += ` AND status = 'active'`
	}
	query += ` ORDER BY created_at DESC LIMIT 100`
	rows, err := postgresutil.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	memories := []memoryRecord{}
	for rows.Next() {
		var row memoryRecord
		var archivedAt *time.Time
		if err := rows.Scan(&row.MemoryID, &row.UserEmail, &row.Text, &row.SourceConversationID, &row.Status, &row.CreatedAt, &archivedAt); err != nil {
			return nil, err
		}
		if archivedAt != nil {
			row.ArchivedAt = archivedAt.UTC().Format(time.RFC3339)
		}
		memories = append(memories, row)
	}
	return memories, rows.Err()
}

func insertMemory(ctx context.Context, userEmail string, text string, sourceConversationID string) (memoryRecord, error) {
	var row memoryRecord
	err := postgresutil.QueryRow(ctx, `INSERT INTO assistant.memories (memory_id, user_email, text, source_conversation_id)
VALUES ($1, $2, $3, $4)
RETURNING memory_id, user_email, text, COALESCE(source_conversation_id, ''), status, created_at`,
		"mem-"+uuid.NewString(), userEmail, text, nullIfEmpty(sourceConversationID)).
		Scan(&row.MemoryID, &row.UserEmail, &row.Text, &row.SourceConversationID, &row.Status, &row.CreatedAt)
	return row, err
}

func archiveMemory(ctx context.Context, userEmail string, memoryID string) (bool, error) {
	tag, err := postgresutil.Exec(ctx, `UPDATE assistant.memories
SET status = 'archived', archived_at = NOW()
WHERE user_email = $1 AND memory_id = $2`, userEmail, memoryID)
	return tag.RowsAffected() > 0, err
}

func insertProposal(ctx context.Context, userEmail string, request createProposalRequest) (fileProposalRecord, error) {
	var row fileProposalRecord
	contentType := strings.TrimSpace(request.ContentType)
	if contentType == "" {
		contentType = "text/plain; charset=utf-8"
	}
	var orchestratorResponseRaw string
	err := postgresutil.QueryRow(ctx, `INSERT INTO assistant.file_proposals (
	proposal_id, conversation_id, user_email, action, document_id, object_key, content_type, proposed_text, rationale
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING proposal_id, conversation_id, user_email, action, COALESCE(document_id, ''), object_key, content_type, proposed_text, rationale, status, orchestrator_response::text, created_at`,
		"prop-"+uuid.NewString(), request.ConversationID, userEmail, request.Action, nullIfEmpty(request.DocumentID), request.ObjectKey, contentType, request.ProposedText, request.Rationale).
		Scan(&row.ProposalID, &row.ConversationID, &row.UserEmail, &row.Action, &row.DocumentID, &row.ObjectKey, &row.ContentType, &row.ProposedText, &row.Rationale, &row.Status, &orchestratorResponseRaw, &row.CreatedAt)
	if err != nil {
		return fileProposalRecord{}, err
	}
	if err := decodeJSONField(orchestratorResponseRaw, &row.OrchestratorResponse); err != nil {
		return fileProposalRecord{}, err
	}
	normalizeProposalJSON(&row)
	return row, nil
}

func getProposal(ctx context.Context, userEmail string, proposalID string) (fileProposalRecord, bool, error) {
	var row fileProposalRecord
	var orchestratorResponseRaw string
	var decidedAt *time.Time
	err := postgresutil.QueryRow(ctx, `SELECT proposal_id, conversation_id, user_email, action, COALESCE(document_id, ''), object_key, content_type, proposed_text, rationale, status, orchestrator_response::text, created_at, decided_at
FROM assistant.file_proposals
WHERE user_email = $1 AND proposal_id = $2`, userEmail, proposalID).
		Scan(&row.ProposalID, &row.ConversationID, &row.UserEmail, &row.Action, &row.DocumentID, &row.ObjectKey, &row.ContentType, &row.ProposedText, &row.Rationale, &row.Status, &orchestratorResponseRaw, &row.CreatedAt, &decidedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fileProposalRecord{}, false, nil
		}
		return fileProposalRecord{}, false, err
	}
	if err := decodeJSONField(orchestratorResponseRaw, &row.OrchestratorResponse); err != nil {
		return fileProposalRecord{}, false, err
	}
	normalizeProposalJSON(&row)
	setOptionalTime(decidedAt, &row.DecidedAt)
	return row, true, nil
}

func listProposals(ctx context.Context, userEmail string, conversationID string) ([]fileProposalRecord, error) {
	rows, err := postgresutil.Query(ctx, `SELECT proposal_id, conversation_id, user_email, action, COALESCE(document_id, ''), object_key, content_type, proposed_text, rationale, status, orchestrator_response::text, created_at, decided_at
FROM assistant.file_proposals
WHERE user_email = $1 AND conversation_id = $2
ORDER BY created_at DESC`, userEmail, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	proposals := []fileProposalRecord{}
	for rows.Next() {
		var row fileProposalRecord
		var orchestratorResponseRaw string
		var decidedAt *time.Time
		if err := rows.Scan(&row.ProposalID, &row.ConversationID, &row.UserEmail, &row.Action, &row.DocumentID, &row.ObjectKey, &row.ContentType, &row.ProposedText, &row.Rationale, &row.Status, &orchestratorResponseRaw, &row.CreatedAt, &decidedAt); err != nil {
			return nil, err
		}
		if err := decodeJSONField(orchestratorResponseRaw, &row.OrchestratorResponse); err != nil {
			return nil, err
		}
		normalizeProposalJSON(&row)
		setOptionalTime(decidedAt, &row.DecidedAt)
		proposals = append(proposals, row)
	}
	return proposals, rows.Err()
}

func updateProposalDecision(ctx context.Context, userEmail string, proposalID string, status string, orchestratorResponse map[string]any) (fileProposalRecord, error) {
	body, err := json.Marshal(orchestratorResponse)
	if err != nil {
		return fileProposalRecord{}, err
	}
	var row fileProposalRecord
	var orchestratorResponseRaw string
	var decidedAt *time.Time
	err = postgresutil.QueryRow(ctx, `UPDATE assistant.file_proposals
SET status = $3, orchestrator_response = $4::jsonb, decided_at = NOW()
WHERE user_email = $1 AND proposal_id = $2
RETURNING proposal_id, conversation_id, user_email, action, COALESCE(document_id, ''), object_key, content_type, proposed_text, rationale, status, orchestrator_response::text, created_at, decided_at`,
		userEmail, proposalID, status, string(body)).
		Scan(&row.ProposalID, &row.ConversationID, &row.UserEmail, &row.Action, &row.DocumentID, &row.ObjectKey, &row.ContentType, &row.ProposedText, &row.Rationale, &row.Status, &orchestratorResponseRaw, &row.CreatedAt, &decidedAt)
	if err != nil {
		return fileProposalRecord{}, err
	}
	if err := decodeJSONField(orchestratorResponseRaw, &row.OrchestratorResponse); err != nil {
		return fileProposalRecord{}, err
	}
	normalizeProposalJSON(&row)
	setOptionalTime(decidedAt, &row.DecidedAt)
	return row, nil
}

func listAudits(ctx context.Context, userEmail string, conversationID string) ([]auditRecord, error) {
	query := `SELECT audit_id, document_id, object_key, action, actor_email, COALESCE(conversation_id, ''), COALESCE(proposal_id, ''), COALESCE(old_version_marker, ''), COALESCE(new_version_marker, ''), COALESCE(reverted_to_version_marker, ''), processing_version, metadata::text, created_at
FROM rag.document_change_audits
WHERE actor_email = $1`
	args := []any{userEmail}
	if strings.TrimSpace(conversationID) != "" {
		query += ` AND conversation_id = $2`
		args = append(args, conversationID)
	}
	query += ` ORDER BY created_at DESC LIMIT 100`
	rows, err := postgresutil.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	audits := []auditRecord{}
	for rows.Next() {
		var row auditRecord
		var metadataRaw string
		if err := rows.Scan(&row.AuditID, &row.DocumentID, &row.ObjectKey, &row.Action, &row.ActorEmail, &row.ConversationID, &row.ProposalID, &row.OldVersionMarker, &row.NewVersionMarker, &row.RevertedToVersionMarker, &row.ProcessingVersion, &metadataRaw, &row.CreatedAt); err != nil {
			return nil, err
		}
		if err := decodeJSONField(metadataRaw, &row.Metadata); err != nil {
			return nil, err
		}
		audits = append(audits, row)
	}
	return audits, rows.Err()
}

func decodeJSONField(raw string, target any) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), target)
}

func normalizeMessageJSON(message *messageRecord) {
	if message.Citations == nil {
		message.Citations = []any{}
	}
	if message.Metadata == nil {
		message.Metadata = map[string]any{}
	}
}

func normalizeProposalJSON(proposal *fileProposalRecord) {
	if proposal.OrchestratorResponse == nil {
		proposal.OrchestratorResponse = map[string]any{}
	}
}

func setOptionalTime(value *time.Time, target *string) {
	if value != nil {
		*target = value.UTC().Format(time.RFC3339)
	}
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func requirePostgres() error {
	if postgresutil.Query == nil || postgresutil.QueryRow == nil || postgresutil.Exec == nil {
		return fmt.Errorf("postgres is not initialized")
	}
	return nil
}
