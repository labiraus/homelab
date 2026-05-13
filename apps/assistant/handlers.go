package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"pkg/api"
	"pkg/prometheusutil"
)

const (
	assistantBodyLimit = 2 << 20
)

type conversationDetailResponse struct {
	Conversation conversationRecord   `json:"conversation"`
	Messages     []messageRecord      `json:"messages"`
	Proposals    []fileProposalRecord `json:"proposals"`
	Audits       []auditRecord        `json:"audits"`
}

type proposalDecisionRequest struct {
	Reason string `json:"reason,omitempty"`
}

var (
	requirePostgresFn        = requirePostgres
	listConversationsFn      = listConversations
	ensureConversationFn     = ensureConversation
	getConversationFn        = getConversation
	listMessagesFn           = listMessages
	insertMessageFn          = insertMessage
	insertToolCallFn         = insertToolCall
	listMemoriesFn           = listMemories
	insertMemoryFn           = insertMemory
	archiveMemoryFn          = archiveMemory
	insertProposalFn         = insertProposal
	getProposalFn            = getProposal
	listProposalsFn          = listProposals
	updateProposalDecisionFn = updateProposalDecision
	listAuditsFn             = listAudits
	runAssistantChatFn       = runAssistantChat
	approveProposalFn        = approveProposal
)

func conversationsHandler(w http.ResponseWriter, r *http.Request) {
	handleAssistantAPI("assistantConversationsHandler", w, r, func(userEmail string) {
		switch r.Method {
		case http.MethodGet:
			if err := requirePostgresFn(); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: err.Error()})
				return
			}
			conversations, err := listConversationsFn(r.Context(), userEmail)
			if err != nil {
				slog.ErrorContext(r.Context(), "failed to list assistant conversations", "error", err)
				writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to list conversations"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"conversations": conversations})
		case http.MethodPost:
			if err := requirePostgresFn(); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: err.Error()})
				return
			}
			var request createConversationRequest
			if err := decodeJSONBody(r.Body, &request); err != nil {
				writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
				return
			}
			conversation, err := ensureConversationFn(r.Context(), userEmail, "", request.Title)
			if err != nil {
				slog.ErrorContext(r.Context(), "failed to create assistant conversation", "error", err)
				writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to create conversation"})
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"conversation": conversation})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

func conversationDetailHandler(w http.ResponseWriter, r *http.Request) {
	handleAssistantAPI("assistantConversationDetailHandler", w, r, func(userEmail string) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := requirePostgresFn(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: err.Error()})
			return
		}
		conversationID := pathValue(r.URL.Path, "/assistant/conversations/")
		if conversationID == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "conversationId is required"})
			return
		}
		conversation, found, err := getConversationFn(r.Context(), userEmail, conversationID)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to read assistant conversation", "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to read conversation"})
			return
		}
		if !found {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "conversation not found"})
			return
		}
		messages, err := listMessagesFn(r.Context(), userEmail, conversationID)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to read assistant messages", "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to read messages"})
			return
		}
		proposals, err := listProposalsFn(r.Context(), userEmail, conversationID)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to read assistant proposals", "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to read proposals"})
			return
		}
		audits, err := listAuditsFn(r.Context(), userEmail, conversationID)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to read assistant audits", "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to read audits"})
			return
		}
		writeJSON(w, http.StatusOK, conversationDetailResponse{
			Conversation: conversation,
			Messages:     messages,
			Proposals:    proposals,
			Audits:       audits,
		})
	})
}

func chatHandler(w http.ResponseWriter, r *http.Request) {
	handleAssistantAPI("assistantChatHandler", w, r, func(userEmail string) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := requirePostgresFn(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: err.Error()})
			return
		}
		var request chatRequest
		if err := decodeJSONBody(r.Body, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
			return
		}
		request.Message = strings.TrimSpace(request.Message)
		if request.Message == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "message is required"})
			return
		}

		conversation, err := ensureConversationFn(r.Context(), userEmail, request.ConversationID, request.Title)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to ensure assistant conversation", "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to create conversation"})
			return
		}
		userMessage, err := insertMessageFn(r.Context(), conversation, "user", request.Message, nil, nil)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to persist user message", "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to persist message"})
			return
		}
		memories, err := listMemoriesFn(r.Context(), userEmail, false)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to load assistant memories", "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to load memories"})
			return
		}
		result, err := runAssistantChatFn(r.Context(), userEmail, conversation, request.Message, memories)
		if err != nil {
			slog.ErrorContext(r.Context(), "assistant model call failed", "error", err)
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "assistant model request failed"})
			return
		}
		reply, err := insertMessageFn(r.Context(), conversation, "assistant", result.Content, result.Citations, result.Metadata)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to persist assistant reply", "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to persist reply"})
			return
		}
		persistedToolCalls := make([]toolCallRecord, 0, len(result.ToolCalls))
		for _, toolCall := range result.ToolCalls {
			toolCall.ConversationID = conversation.ConversationID
			toolCall.MessageID = reply.MessageID
			toolCall.UserEmail = userEmail
			persisted, err := insertToolCallFn(r.Context(), toolCall)
			if err != nil {
				slog.ErrorContext(r.Context(), "failed to persist assistant tool call", "error", err)
				writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to persist tool call"})
				return
			}
			persistedToolCalls = append(persistedToolCalls, persisted)
		}
		writeJSON(w, http.StatusOK, chatResponse{
			Conversation: conversation,
			UserMessage:  userMessage,
			Reply:        reply,
			ToolCalls:    persistedToolCalls,
		})
	})
}

func memoriesHandler(w http.ResponseWriter, r *http.Request) {
	handleAssistantAPI("assistantMemoriesHandler", w, r, func(userEmail string) {
		if err := requirePostgresFn(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: err.Error()})
			return
		}
		switch r.Method {
		case http.MethodGet:
			memories, err := listMemoriesFn(r.Context(), userEmail, r.URL.Query().Get("includeArchived") == "true")
			if err != nil {
				slog.ErrorContext(r.Context(), "failed to list assistant memories", "error", err)
				writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to list memories"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"memories": memories})
		case http.MethodPost:
			var request createMemoryRequest
			if err := decodeJSONBody(r.Body, &request); err != nil {
				writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
				return
			}
			if strings.TrimSpace(request.Text) == "" {
				writeJSON(w, http.StatusBadRequest, errorResponse{Error: "text is required"})
				return
			}
			memory, err := insertMemoryFn(r.Context(), userEmail, strings.TrimSpace(request.Text), request.SourceConversationID)
			if err != nil {
				slog.ErrorContext(r.Context(), "failed to create assistant memory", "error", err)
				writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to create memory"})
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"memory": memory})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

func memoryActionHandler(w http.ResponseWriter, r *http.Request) {
	handleAssistantAPI("assistantMemoryActionHandler", w, r, func(userEmail string) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := requirePostgresFn(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: err.Error()})
			return
		}
		memoryID, action := splitObjectAction(r.URL.Path, "/assistant/memories/")
		if memoryID == "" || action != "archive" {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "memory action not found"})
			return
		}
		updated, err := archiveMemoryFn(r.Context(), userEmail, memoryID)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to archive assistant memory", "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to archive memory"})
			return
		}
		if !updated {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "memory not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "archived"})
	})
}

func proposalsHandler(w http.ResponseWriter, r *http.Request) {
	handleAssistantAPI("assistantProposalsHandler", w, r, func(userEmail string) {
		if err := requirePostgresFn(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: err.Error()})
			return
		}
		switch r.Method {
		case http.MethodGet:
			conversationID := strings.TrimSpace(r.URL.Query().Get("conversationId"))
			if conversationID == "" {
				writeJSON(w, http.StatusBadRequest, errorResponse{Error: "conversationId is required"})
				return
			}
			proposals, err := listProposalsFn(r.Context(), userEmail, conversationID)
			if err != nil {
				slog.ErrorContext(r.Context(), "failed to list assistant proposals", "error", err)
				writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to list proposals"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"proposals": proposals})
		case http.MethodPost:
			var request createProposalRequest
			if err := decodeJSONBody(r.Body, &request); err != nil {
				writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
				return
			}
			if err := validateProposalRequest(request); err != nil {
				writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
				return
			}
			proposal, err := insertProposalFn(r.Context(), userEmail, request)
			if err != nil {
				slog.ErrorContext(r.Context(), "failed to create assistant proposal", "error", err)
				writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to create proposal"})
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"proposal": proposal})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

func proposalActionHandler(w http.ResponseWriter, r *http.Request) {
	handleAssistantAPI("assistantProposalActionHandler", w, r, func(userEmail string) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := requirePostgresFn(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: err.Error()})
			return
		}
		proposalID, action := splitObjectAction(r.URL.Path, "/assistant/proposals/")
		if proposalID == "" || (action != "approve" && action != "reject") {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "proposal action not found"})
			return
		}
		proposal, found, err := getProposalFn(r.Context(), userEmail, proposalID)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to read assistant proposal", "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to read proposal"})
			return
		}
		if !found {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "proposal not found"})
			return
		}
		if proposal.Status != "pending" {
			writeJSON(w, http.StatusConflict, errorResponse{Error: "proposal has already been decided"})
			return
		}

		if action == "reject" {
			var request proposalDecisionRequest
			_ = decodeJSONBody(r.Body, &request)
			updated, err := updateProposalDecisionFn(r.Context(), userEmail, proposalID, "rejected", map[string]any{
				"reason": strings.TrimSpace(request.Reason),
			})
			if err != nil {
				slog.ErrorContext(r.Context(), "failed to reject assistant proposal", "error", err)
				writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to reject proposal"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"proposal": updated})
			return
		}

		response, err := approveProposalFn(r.Context(), proposal)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to approve assistant proposal", "error", err)
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "document change request failed"})
			return
		}
		updated, err := updateProposalDecisionFn(r.Context(), userEmail, proposalID, "approved", response)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to mark assistant proposal approved", "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to approve proposal"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"proposal": updated})
	})
}

func auditsHandler(w http.ResponseWriter, r *http.Request) {
	handleAssistantAPI("assistantAuditsHandler", w, r, func(userEmail string) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := requirePostgresFn(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: err.Error()})
			return
		}
		audits, err := listAuditsFn(r.Context(), userEmail, r.URL.Query().Get("conversationId"))
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to list assistant audits", "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to list audits"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"audits": audits})
	})
}

func handleAssistantAPI(label string, w http.ResponseWriter, r *http.Request, fn func(userEmail string)) {
	var err error
	startTime := time.Now()
	prometheusutil.IncrementProcessed(label, "call")
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
			w.WriteHeader(http.StatusInternalServerError)
		}
		if err != nil {
			slog.ErrorContext(r.Context(), err.Error())
			prometheusutil.IncrementProcessed(label, "error")
		}
		prometheusutil.OpDuration(label, time.Since(startTime))
	}()

	userEmail, ok := authenticatedEmail(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authenticated email is required"})
		return
	}
	fn(userEmail)
}

func authenticatedEmail(r *http.Request) (string, bool) {
	if status, ok := api.AuthStatusFromContext(r.Context()); ok && status.Valid && strings.TrimSpace(status.Email) != "" {
		return strings.ToLower(strings.TrimSpace(status.Email)), true
	}
	if userID, ok := api.UserIDFromContext(r.Context()); ok && strings.TrimSpace(userID) != "" {
		return strings.ToLower(strings.TrimSpace(userID)), true
	}
	for _, header := range []string{"X-Forwarded-Email", "X-Auth-Request-Email", "UserID", "X-User"} {
		if value := strings.TrimSpace(r.Header.Get(header)); value != "" {
			return strings.ToLower(value), true
		}
	}
	return "", false
}

func decodeJSONBody(body io.Reader, target any) error {
	raw, err := io.ReadAll(io.LimitReader(body, assistantBodyLimit+1))
	if err != nil {
		return err
	}
	if len(raw) > assistantBodyLimit {
		return fmt.Errorf("request body is too large")
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		raw = []byte("{}")
	}
	return json.Unmarshal(raw, target)
}

func validateProposalRequest(request createProposalRequest) error {
	action := strings.ToLower(strings.TrimSpace(request.Action))
	if action != "create" && action != "edit" {
		return fmt.Errorf("action must be create or edit")
	}
	if strings.TrimSpace(request.ConversationID) == "" {
		return fmt.Errorf("conversationId is required")
	}
	if strings.TrimSpace(request.ObjectKey) == "" {
		return fmt.Errorf("objectKey is required")
	}
	if strings.TrimSpace(request.ProposedText) == "" {
		return fmt.Errorf("proposedText is required")
	}
	if action == "edit" && strings.TrimSpace(request.DocumentID) == "" {
		return fmt.Errorf("documentId is required for edits")
	}
	return nil
}

func pathValue(path string, prefix string) string {
	return strings.Trim(strings.TrimPrefix(path, prefix), "/")
}

func splitObjectAction(path string, prefix string) (string, string) {
	value := pathValue(path, prefix)
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return "", ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
