package main

import "time"

type errorResponse struct {
	Error string `json:"error"`
}

type conversationRecord struct {
	ID             int64     `json:"-"`
	ConversationID string    `json:"conversationId"`
	UserEmail      string    `json:"userEmail"`
	Title          string    `json:"title"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type messageRecord struct {
	MessageID      string         `json:"messageId"`
	ConversationID string         `json:"conversationId"`
	Role           string         `json:"role"`
	Content        string         `json:"content"`
	Citations      []any          `json:"citations"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"createdAt"`
}

type memoryRecord struct {
	MemoryID             string    `json:"memoryId"`
	UserEmail            string    `json:"userEmail"`
	Text                 string    `json:"text"`
	SourceConversationID string    `json:"sourceConversationId,omitempty"`
	Status               string    `json:"status"`
	CreatedAt            time.Time `json:"createdAt"`
	ArchivedAt           string    `json:"archivedAt,omitempty"`
}

type fileProposalRecord struct {
	ProposalID           string         `json:"proposalId"`
	ConversationID       string         `json:"conversationId"`
	UserEmail            string         `json:"userEmail"`
	Action               string         `json:"action"`
	DocumentID           string         `json:"documentId,omitempty"`
	ObjectKey            string         `json:"objectKey"`
	ContentType          string         `json:"contentType"`
	ProposedText         string         `json:"proposedText"`
	Rationale            string         `json:"rationale"`
	Status               string         `json:"status"`
	OrchestratorResponse map[string]any `json:"orchestratorResponse,omitempty"`
	CreatedAt            time.Time      `json:"createdAt"`
	DecidedAt            string         `json:"decidedAt,omitempty"`
}

type chatRequest struct {
	ConversationID string `json:"conversationId,omitempty"`
	Message        string `json:"message"`
	Title          string `json:"title,omitempty"`
}

type chatResponse struct {
	Conversation conversationRecord `json:"conversation"`
	UserMessage  messageRecord      `json:"userMessage"`
	Reply        messageRecord      `json:"reply"`
	ToolCalls    []toolCallRecord   `json:"toolCalls"`
}

type createConversationRequest struct {
	Title string `json:"title,omitempty"`
}

type createMemoryRequest struct {
	Text                 string `json:"text"`
	SourceConversationID string `json:"sourceConversationId,omitempty"`
}

type createProposalRequest struct {
	ConversationID string `json:"conversationId"`
	Action         string `json:"action"`
	DocumentID     string `json:"documentId,omitempty"`
	ObjectKey      string `json:"objectKey"`
	ContentType    string `json:"contentType,omitempty"`
	ProposedText   string `json:"proposedText"`
	Rationale      string `json:"rationale,omitempty"`
}

type toolCallRecord struct {
	ToolCallID     string         `json:"toolCallId"`
	ConversationID string         `json:"conversationId,omitempty"`
	MessageID      string         `json:"messageId,omitempty"`
	UserEmail      string         `json:"userEmail"`
	ToolName       string         `json:"toolName"`
	Arguments      map[string]any `json:"arguments"`
	Result         map[string]any `json:"result"`
	IsError        bool           `json:"isError"`
	CreatedAt      time.Time      `json:"createdAt"`
}

type auditRecord struct {
	AuditID                 string         `json:"auditId"`
	DocumentID              string         `json:"documentId"`
	ObjectKey               string         `json:"objectKey"`
	Action                  string         `json:"action"`
	ActorEmail              string         `json:"actorEmail"`
	ConversationID          string         `json:"conversationId,omitempty"`
	ProposalID              string         `json:"proposalId,omitempty"`
	OldVersionMarker        string         `json:"oldVersionMarker,omitempty"`
	NewVersionMarker        string         `json:"newVersionMarker,omitempty"`
	RevertedToVersionMarker string         `json:"revertedToVersionMarker,omitempty"`
	ProcessingVersion       int            `json:"processingVersion"`
	Metadata                map[string]any `json:"metadata"`
	CreatedAt               time.Time      `json:"createdAt"`
}
