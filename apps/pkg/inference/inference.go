package inference

import (
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
	DefaultChatModel      = "Qwen/Qwen2.5-0.5B-Instruct"
	DefaultEmbeddingModel = "local-embeddings"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

type ChatRequest struct {
	Model       string            `json:"model"`
	Messages    []Message         `json:"messages"`
	Temperature float64           `json:"temperature"`
	MaxTokens   int               `json:"max_tokens"`
	Tools       []Tool            `json:"tools,omitempty"`
	ToolChoice  any               `json:"tool_choice,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type Function struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type EmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type EmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
}

func NewClientFromEnv() Client {
	return Client{
		BaseURL:    envFirst("AI_GATEWAY_BASE_URL", "LLM_BASE_URL"),
		HTTPClient: &http.Client{Timeout: 90 * time.Second},
	}
}

func (c Client) Configured() bool {
	return strings.TrimSpace(c.BaseURL) != ""
}

func (c Client) ChatCompletion(ctx context.Context, requestBody ChatRequest) (string, error) {
	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}
	raw, err := c.postJSON(ctx, "/chat/completions", body)
	if err != nil {
		return "", err
	}
	var response chatResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("chat completion returned no choices")
	}
	return strings.TrimSpace(response.Choices[0].Message.Content), nil
}

func (c Client) Embedding(ctx context.Context, requestBody EmbeddingRequest) (EmbeddingResponse, error) {
	body, err := json.Marshal(requestBody)
	if err != nil {
		return EmbeddingResponse{}, err
	}
	raw, err := c.postJSON(ctx, "/embeddings", body)
	if err != nil {
		return EmbeddingResponse{}, err
	}
	var response EmbeddingResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return EmbeddingResponse{}, err
	}
	if len(response.Data) == 0 || len(response.Data[0].Embedding) == 0 {
		return EmbeddingResponse{}, fmt.Errorf("embedding response did not include a vector")
	}
	if strings.TrimSpace(response.Model) == "" {
		response.Model = requestBody.Model
	}
	return response, nil
}

func (c Client) postJSON(ctx context.Context, path string, body []byte) ([]byte, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("AI_GATEWAY_BASE_URL is not configured")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("inference gateway returned status %d: %s", response.StatusCode, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

func ChatModelFromEnv() string {
	return envFirstDefault(DefaultChatModel, "AI_CHAT_MODEL", "LLM_MODEL")
}

func EmbeddingModelFromEnv() string {
	return envFirstDefault(DefaultEmbeddingModel, "AI_EMBEDDING_MODEL", "EMBEDDING_MODEL")
}

func MaxTokensFromEnv(defaultValue int) int {
	value, err := strconv.Atoi(strings.TrimSpace(envFirst("AI_CHAT_MAX_TOKENS", "LLM_MAX_TOKENS")))
	if err != nil || value <= 0 {
		return defaultValue
	}
	return value
}

func envFirst(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func envFirstDefault(defaultValue string, names ...string) string {
	if value := envFirst(names...); value != "" {
		return value
	}
	return defaultValue
}
