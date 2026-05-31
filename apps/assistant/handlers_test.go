package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"pkg/api"
	"pkg/base"
	"pkg/prometheusutil"
)

func TestMain(m *testing.M) {
	base.ServiceName = "assistant_test"
	prometheusutil.Start(http.NewServeMux())
	os.Exit(m.Run())
}

func TestChatHandlerPersistsConversationMessagesAndToolCalls(t *testing.T) {
	originalRequire := requirePostgresFn
	originalEnsure := ensureConversationFn
	originalInsertMessage := insertMessageFn
	originalListMemories := listMemoriesFn
	originalRun := runAssistantChatFn
	originalInsertToolCall := insertToolCallFn
	t.Cleanup(func() {
		requirePostgresFn = originalRequire
		ensureConversationFn = originalEnsure
		insertMessageFn = originalInsertMessage
		listMemoriesFn = originalListMemories
		runAssistantChatFn = originalRun
		insertToolCallFn = originalInsertToolCall
	})

	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	conversation := conversationRecord{
		ID:             42,
		ConversationID: "conv-1",
		UserEmail:      "user@example.com",
		Title:          "Campaign",
		Status:         "active",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	requirePostgresFn = func() error { return nil }
	ensureConversationFn = func(ctx context.Context, userEmail string, conversationID string, title string) (conversationRecord, error) {
		if userEmail != "user@example.com" || conversationID != "conv-1" {
			t.Fatalf("unexpected conversation identity: %s/%s", userEmail, conversationID)
		}
		return conversation, nil
	}
	insertedRoles := []string{}
	insertMessageFn = func(ctx context.Context, conversation conversationRecord, role string, content string, citations []any, metadata map[string]any) (messageRecord, error) {
		insertedRoles = append(insertedRoles, role)
		return messageRecord{
			MessageID:      "msg-" + role,
			ConversationID: conversation.ConversationID,
			Role:           role,
			Content:        content,
			Citations:      citations,
			Metadata:       metadata,
			CreatedAt:      now,
		}, nil
	}
	listMemoriesFn = func(ctx context.Context, userEmail string, includeArchived bool) ([]memoryRecord, error) {
		if userEmail != "user@example.com" || includeArchived {
			t.Fatalf("unexpected memory scope: %s archived=%v", userEmail, includeArchived)
		}
		return []memoryRecord{{MemoryID: "mem-1", UserEmail: userEmail, Text: "Use concise answers", Status: "active", CreatedAt: now}}, nil
	}
	runAssistantChatFn = func(ctx context.Context, userEmail string, conversation conversationRecord, message string, memories []memoryRecord) (assistantRunResult, error) {
		if userEmail != "user@example.com" || message != "what changed?" || len(memories) != 1 {
			t.Fatalf("unexpected assistant run inputs: %s %q %+v", userEmail, message, memories)
		}
		return assistantRunResult{
			Content:   "Found context",
			Citations: []any{map[string]any{"label": "notes.md chunk 0"}},
			Metadata:  map[string]any{"model": "test"},
			ToolCalls: []toolCallRecord{{ToolName: "documents.context", Arguments: map[string]any{"query": message}, Result: map[string]any{"context": "notes"}}},
		}, nil
	}
	insertToolCallFn = func(ctx context.Context, record toolCallRecord) (toolCallRecord, error) {
		if record.UserEmail != "user@example.com" || record.ConversationID != "conv-1" || record.MessageID != "msg-assistant" {
			t.Fatalf("unexpected tool call record: %+v", record)
		}
		record.ToolCallID = "tool-1"
		record.CreatedAt = now
		return record, nil
	}

	request := authenticatedRequest(http.MethodPost, "/assistant/chat", `{"conversationId":"conv-1","message":"what changed?"}`)
	recorder := httptest.NewRecorder()
	chatHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(insertedRoles) != 2 || insertedRoles[0] != "user" || insertedRoles[1] != "assistant" {
		t.Fatalf("expected user and assistant messages, got %+v", insertedRoles)
	}
	var response chatResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected chat response: %v", err)
	}
	if response.Reply.Content != "Found context" || len(response.ToolCalls) != 1 {
		t.Fatalf("unexpected chat response: %+v", response)
	}
}

func TestMemoriesHandlerScopesByAuthenticatedEmail(t *testing.T) {
	originalRequire := requirePostgresFn
	originalList := listMemoriesFn
	t.Cleanup(func() {
		requirePostgresFn = originalRequire
		listMemoriesFn = originalList
	})

	requirePostgresFn = func() error { return nil }
	listMemoriesFn = func(ctx context.Context, userEmail string, includeArchived bool) ([]memoryRecord, error) {
		if userEmail != "alice@example.com" {
			t.Fatalf("expected authenticated email scope, got %q", userEmail)
		}
		return []memoryRecord{{MemoryID: "mem-1", UserEmail: userEmail, Text: "private memory", Status: "active"}}, nil
	}

	request := authenticatedRequestWithEmail(http.MethodGet, "/assistant/memories", "", "alice@example.com")
	recorder := httptest.NewRecorder()
	memoriesHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestProposalApprovalTransitionsPendingProposal(t *testing.T) {
	originalRequire := requirePostgresFn
	originalGet := getProposalFn
	originalApprove := approveProposalFn
	originalUpdate := updateProposalDecisionFn
	t.Cleanup(func() {
		requirePostgresFn = originalRequire
		getProposalFn = originalGet
		approveProposalFn = originalApprove
		updateProposalDecisionFn = originalUpdate
	})

	proposal := fileProposalRecord{
		ProposalID:     "prop-1",
		ConversationID: "conv-1",
		UserEmail:      "user@example.com",
		Action:         "edit",
		DocumentID:     "doc-1",
		ObjectKey:      "notes/doc-1.md",
		ContentType:    "text/markdown",
		ProposedText:   "edited",
		Status:         "pending",
	}
	requirePostgresFn = func() error { return nil }
	getProposalFn = func(ctx context.Context, userEmail string, proposalID string) (fileProposalRecord, bool, error) {
		if userEmail != "user@example.com" || proposalID != "prop-1" {
			t.Fatalf("unexpected proposal lookup: %s/%s", userEmail, proposalID)
		}
		return proposal, true, nil
	}
	approveProposalFn = func(ctx context.Context, received fileProposalRecord) (map[string]any, error) {
		if received.ProposalID != "prop-1" || received.Action != "edit" {
			t.Fatalf("unexpected approval proposal: %+v", received)
		}
		return map[string]any{"status": "queued", "processingVersion": float64(4)}, nil
	}
	updateProposalDecisionFn = func(ctx context.Context, userEmail string, proposalID string, status string, response map[string]any) (fileProposalRecord, error) {
		if status != "approved" || response["status"] != "queued" {
			t.Fatalf("unexpected proposal decision: %s %+v", status, response)
		}
		proposal.Status = status
		proposal.OrchestratorResponse = response
		return proposal, nil
	}

	request := authenticatedRequest(http.MethodPost, "/assistant/proposals/prop-1/approve", `{}`)
	recorder := httptest.NewRecorder()
	proposalActionHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Proposal fileProposalRecord `json:"proposal"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected proposal response: %v", err)
	}
	if response.Proposal.Status != "approved" {
		t.Fatalf("expected approved proposal, got %+v", response.Proposal)
	}
}

func TestProposalRejectRecordsReasonWithoutCallingOrchestrator(t *testing.T) {
	originalRequire := requirePostgresFn
	originalGet := getProposalFn
	originalApprove := approveProposalFn
	originalUpdate := updateProposalDecisionFn
	t.Cleanup(func() {
		requirePostgresFn = originalRequire
		getProposalFn = originalGet
		approveProposalFn = originalApprove
		updateProposalDecisionFn = originalUpdate
	})

	proposal := fileProposalRecord{
		ProposalID:     "prop-1",
		ConversationID: "conv-1",
		UserEmail:      "user@example.com",
		Action:         "edit",
		DocumentID:     "doc-1",
		ObjectKey:      "notes/doc-1.md",
		ContentType:    "text/markdown",
		ProposedText:   "edited",
		Status:         "pending",
	}
	requirePostgresFn = func() error { return nil }
	getProposalFn = func(ctx context.Context, userEmail string, proposalID string) (fileProposalRecord, bool, error) {
		if userEmail != "user@example.com" || proposalID != "prop-1" {
			t.Fatalf("unexpected proposal lookup: %s/%s", userEmail, proposalID)
		}
		return proposal, true, nil
	}
	approveProposalFn = func(ctx context.Context, received fileProposalRecord) (map[string]any, error) {
		t.Fatalf("reject must not call orchestrator approval path: %+v", received)
		return nil, nil
	}
	updateProposalDecisionFn = func(ctx context.Context, userEmail string, proposalID string, status string, response map[string]any) (fileProposalRecord, error) {
		if userEmail != "user@example.com" || proposalID != "prop-1" || status != "rejected" {
			t.Fatalf("unexpected proposal decision: %s/%s/%s", userEmail, proposalID, status)
		}
		if response["reason"] != "needs a clearer diff" {
			t.Fatalf("expected rejection reason, got %+v", response)
		}
		proposal.Status = status
		proposal.OrchestratorResponse = response
		return proposal, nil
	}

	request := authenticatedRequest(http.MethodPost, "/assistant/proposals/prop-1/reject", `{"reason":" needs a clearer diff "}`)
	recorder := httptest.NewRecorder()
	proposalActionHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Proposal fileProposalRecord `json:"proposal"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected proposal response: %v", err)
	}
	if response.Proposal.Status != "rejected" || response.Proposal.OrchestratorResponse["reason"] != "needs a clearer diff" {
		t.Fatalf("expected rejected proposal with reason, got %+v", response.Proposal)
	}
}

func TestProposalApprovalRejectsAlreadyDecidedProposalWithoutOrchestratorCall(t *testing.T) {
	originalRequire := requirePostgresFn
	originalGet := getProposalFn
	originalApprove := approveProposalFn
	originalUpdate := updateProposalDecisionFn
	t.Cleanup(func() {
		requirePostgresFn = originalRequire
		getProposalFn = originalGet
		approveProposalFn = originalApprove
		updateProposalDecisionFn = originalUpdate
	})

	requirePostgresFn = func() error { return nil }
	getProposalFn = func(ctx context.Context, userEmail string, proposalID string) (fileProposalRecord, bool, error) {
		return fileProposalRecord{
			ProposalID: "prop-1",
			UserEmail:  userEmail,
			Action:     "edit",
			Status:     "approved",
		}, true, nil
	}
	approveProposalFn = func(ctx context.Context, proposal fileProposalRecord) (map[string]any, error) {
		t.Fatalf("already-decided proposal must not call orchestrator: %+v", proposal)
		return nil, nil
	}
	updateProposalDecisionFn = func(ctx context.Context, userEmail string, proposalID string, status string, response map[string]any) (fileProposalRecord, error) {
		t.Fatalf("already-decided proposal must not update decision")
		return fileProposalRecord{}, nil
	}

	request := authenticatedRequest(http.MethodPost, "/assistant/proposals/prop-1/approve", `{}`)
	recorder := httptest.NewRecorder()
	proposalActionHandler(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAssistantToolDefinitionsExposeOnlyReadOnlyRAGTool(t *testing.T) {
	tools := assistantToolDefinitions()
	if len(tools) != 1 {
		t.Fatalf("expected one assistant tool, got %+v", tools)
	}
	if tools[0].Function.Name != "documents_context" {
		t.Fatalf("expected read-only document context tool, got %+v", tools[0])
	}
	for _, forbidden := range []string{"edit", "create", "delete", "move", "revert"} {
		if strings.Contains(tools[0].Function.Name, forbidden) {
			t.Fatalf("write-like tool leaked into assistant allowlist: %+v", tools[0])
		}
	}
}

func authenticatedRequest(method string, path string, body string) *http.Request {
	return authenticatedRequestWithEmail(method, path, body, "user@example.com")
}

func authenticatedRequestWithEmail(method string, path string, body string, email string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	ctx := api.WithAuthStatus(request.Context(), api.AuthStatus{
		Mode:  api.AuthModeOIDC,
		Email: email,
		Valid: true,
	})
	return request.WithContext(ctx)
}
