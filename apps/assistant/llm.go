package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultLLMModel = "Qwen/Qwen2.5-0.5B-Instruct"
)

type assistantRunResult struct {
	Content   string         `json:"content"`
	Citations []any          `json:"citations"`
	Metadata  map[string]any `json:"metadata"`
	ToolCalls []toolCallRecord
}

type documentContextResult struct {
	Context   string
	Citations []any
	Raw       map[string]any
	IsError   bool
	Error     string
}

type chatCompletionRequest struct {
	Model       string            `json:"model"`
	Messages    []llmMessage      `json:"messages"`
	Temperature float64           `json:"temperature"`
	MaxTokens   int               `json:"max_tokens"`
	Tools       []llmTool         `json:"tools,omitempty"`
	ToolChoice  any               `json:"tool_choice,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type llmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmTool struct {
	Type     string      `json:"type"`
	Function llmFunction `json:"function"`
}

type llmFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type jsonRPCResponseEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

var assistantHTTPClient = &http.Client{Timeout: 90 * time.Second}

func runAssistantChat(ctx context.Context, userEmail string, conversation conversationRecord, message string, memories []memoryRecord) (assistantRunResult, error) {
	contextResult := callDocumentsContext(ctx, userEmail, message)
	toolCall := toolCallRecord{
		ToolName: "documents.context",
		Arguments: map[string]any{
			"query":    message,
			"limit":    4,
			"maxChars": 5000,
		},
		Result:  contextResult.Raw,
		IsError: contextResult.IsError,
	}
	if contextResult.IsError {
		toolCall.Result = map[string]any{"error": contextResult.Error}
	}

	metadata := map[string]any{
		"model":              llmModel(),
		"mcpContextToolUsed": true,
	}
	if contextResult.IsError {
		metadata["mcpContextError"] = contextResult.Error
	}

	reply, err := callLLM(ctx, userEmail, conversation, message, memories, contextResult)
	if err != nil || strings.TrimSpace(reply) == "" {
		reply = fallbackReply(contextResult)
		if err != nil {
			metadata["llmError"] = err.Error()
		}
	}

	return assistantRunResult{
		Content:   reply,
		Citations: contextResult.Citations,
		Metadata:  metadata,
		ToolCalls: []toolCallRecord{toolCall},
	}, nil
}

func callDocumentsContext(ctx context.Context, userEmail string, query string) documentContextResult {
	requestCtx, cancel := context.WithTimeout(ctx, mcpContextTimeout())
	defer cancel()

	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      "assistant-context",
		"method":  "tools/call",
		"params": map[string]any{
			"name": "documents.context",
			"arguments": map[string]any{
				"query":    query,
				"limit":    4,
				"maxChars": 5000,
			},
		},
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return documentContextResult{IsError: true, Error: err.Error()}
	}

	endpoint := strings.TrimRight(envOrDefault("MCP_BASE_URL", "http://homelab-mcp.homelab.svc.cluster.local/mcp"), "/")
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return documentContextResult{IsError: true, Error: err.Error()}
	}
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("MCP-Protocol-Version", "2025-11-25")
	request.Header.Set("X-Forwarded-Email", userEmail)
	request.Header.Set("UserID", userEmail)

	response, err := assistantHTTPClient.Do(request)
	if err != nil {
		return documentContextResult{IsError: true, Error: err.Error()}
	}
	defer response.Body.Close()

	raw, err := readSSEData(response.Body)
	if err != nil {
		return documentContextResult{IsError: true, Error: err.Error()}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return documentContextResult{IsError: true, Error: fmt.Sprintf("mcp returned status %d", response.StatusCode)}
	}

	var rpc jsonRPCResponseEnvelope
	if err := json.Unmarshal(raw, &rpc); err != nil {
		return documentContextResult{IsError: true, Error: err.Error()}
	}
	if rpc.Error != nil {
		return documentContextResult{IsError: true, Error: rpc.Error.Message}
	}

	var result map[string]any
	if err := json.Unmarshal(rpc.Result, &result); err != nil {
		return documentContextResult{IsError: true, Error: err.Error()}
	}
	contextResult := documentContextResult{Raw: result}
	if structured, ok := result["structuredContent"].(map[string]any); ok {
		if value, ok := structured["context"].(string); ok {
			contextResult.Context = value
		}
		if citations, ok := structured["citations"].([]any); ok {
			contextResult.Citations = citations
		}
	}
	if contextResult.Context == "" {
		contextResult.Context = textContentFromMCPResult(result)
	}
	return contextResult
}

func callLLM(ctx context.Context, userEmail string, conversation conversationRecord, message string, memories []memoryRecord, contextResult documentContextResult) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("LLM_BASE_URL")), "/")
	if baseURL == "" {
		return "", fmt.Errorf("LLM_BASE_URL is not configured")
	}
	requestBody := chatCompletionRequest{
		Model:       llmModel(),
		Temperature: 0.2,
		MaxTokens:   llmMaxTokens(),
		Messages: []llmMessage{
			{Role: "system", Content: assistantSystemPrompt(memories, contextResult)},
			{Role: "user", Content: message},
		},
		Tools: assistantToolDefinitions(),
		Metadata: map[string]string{
			"user_email":      userEmail,
			"conversation_id": conversation.ConversationID,
		},
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")

	response, err := assistantHTTPClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("llm returned status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var completion chatCompletionResponse
	if err := json.Unmarshal(body, &completion); err != nil {
		return "", err
	}
	if len(completion.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices")
	}
	return strings.TrimSpace(completion.Choices[0].Message.Content), nil
}

func assistantSystemPrompt(memories []memoryRecord, contextResult documentContextResult) string {
	var builder strings.Builder
	builder.WriteString("You are the Labiraus assistant. Answer using the provided Labiraus RAG context when it is relevant. ")
	builder.WriteString("Cite document references that appear in the context. Do not claim file changes have been made unless the user approved a persisted proposal.\n\n")
	if len(memories) > 0 {
		builder.WriteString("User-approved memories for this authenticated user:\n")
		for _, memory := range memories {
			builder.WriteString("- ")
			builder.WriteString(memory.Text)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	if contextResult.Context != "" {
		builder.WriteString("Labiraus RAG context:\n")
		builder.WriteString(contextResult.Context)
		builder.WriteString("\n")
	} else if contextResult.IsError {
		builder.WriteString("Labiraus RAG context could not be loaded: ")
		builder.WriteString(contextResult.Error)
		builder.WriteString("\n")
	} else {
		builder.WriteString("No Labiraus RAG context was found for this request.\n")
	}
	return builder.String()
}

func assistantToolDefinitions() []llmTool {
	return []llmTool{
		{
			Type: "function",
			Function: llmFunction{
				Name:        "documents_context",
				Description: "Read-only Labiraus RAG context lookup. File-writing tools are not exposed to the model.",
				Parameters: map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Natural-language RAG query.",
						},
					},
					"required": []string{"query"},
				},
			},
		},
	}
}

func readSSEData(reader io.Reader) ([]byte, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024), 2<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "data:") {
			return []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("mcp response did not include a data event")
}

func textContentFromMCPResult(result map[string]any) string {
	content, ok := result["content"].([]any)
	if !ok {
		return ""
	}
	var builder strings.Builder
	for _, item := range content {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if text, ok := itemMap["text"].(string); ok {
			if builder.Len() > 0 {
				builder.WriteString("\n")
			}
			builder.WriteString(text)
		}
	}
	return builder.String()
}

func fallbackReply(contextResult documentContextResult) string {
	if contextResult.IsError {
		return "I could not retrieve the Labiraus RAG context for that request. The conversation was saved, but the answer should be retried after the document context tool is healthy."
	}
	if strings.TrimSpace(contextResult.Context) == "" {
		return "I searched the Labiraus documents but did not find relevant context for that request."
	}
	return "I searched the Labiraus documents and found this relevant context:\n\n" + contextResult.Context
}

func llmModel() string {
	return envOrDefault("LLM_MODEL", defaultLLMModel)
}

func llmMaxTokens() int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv("LLM_MAX_TOKENS")))
	if err != nil || value <= 0 {
		return 768
	}
	return value
}

func mcpContextTimeout() time.Duration {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv("MCP_CONTEXT_TIMEOUT_SECONDS")))
	if err != nil || value <= 0 {
		return 5 * time.Second
	}
	return time.Duration(value) * time.Second
}

func envOrDefault(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
