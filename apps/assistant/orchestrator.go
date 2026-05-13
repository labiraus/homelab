package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const maxOrchestratorApprovalResponse = 2 << 20

var orchestratorApprovalHTTPClient = &http.Client{Timeout: 30 * time.Second}

func approveProposal(ctx context.Context, proposal fileProposalRecord) (map[string]any, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("ORCHESTRATOR_BASE_URL")), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("ORCHESTRATOR_BASE_URL is not configured")
	}

	path := "/documents/create-text"
	body := map[string]any{
		"documentId":     proposal.DocumentID,
		"objectKey":      proposal.ObjectKey,
		"contentType":    proposal.ContentType,
		"text":           proposal.ProposedText,
		"actorEmail":     proposal.UserEmail,
		"conversationId": proposal.ConversationID,
		"proposalId":     proposal.ProposalID,
		"metadata": map[string]any{
			"assistantProposalId": proposal.ProposalID,
			"assistantRationale":  proposal.Rationale,
		},
	}
	if proposal.Action == "edit" {
		path = "/documents/edit-text"
	} else if proposal.Action != "create" {
		return nil, fmt.Errorf("unsupported proposal action %q", proposal.Action)
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Forwarded-Email", proposal.UserEmail)
	request.Header.Set("UserID", proposal.UserEmail)

	response, err := orchestratorApprovalHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxOrchestratorApprovalResponse+1))
	if err != nil {
		return nil, err
	}
	if len(responseBody) > maxOrchestratorApprovalResponse {
		return nil, fmt.Errorf("orchestrator response exceeded %d bytes", maxOrchestratorApprovalResponse)
	}

	var decoded map[string]any
	if len(strings.TrimSpace(string(responseBody))) > 0 {
		if err := json.Unmarshal(responseBody, &decoded); err != nil {
			return nil, err
		}
	} else {
		decoded = map[string]any{}
	}
	decoded["statusCode"] = response.StatusCode
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return decoded, fmt.Errorf("orchestrator returned status %d", response.StatusCode)
	}
	return decoded, nil
}
