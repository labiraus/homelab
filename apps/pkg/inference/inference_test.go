package inference

import "testing"

func TestChatModelFromEnvPrefersAIGatewayName(t *testing.T) {
	t.Setenv("AI_CHAT_MODEL", "bedrock-chat")
	t.Setenv("LLM_MODEL", "legacy-chat")

	if got := ChatModelFromEnv(); got != "bedrock-chat" {
		t.Fatalf("expected AI_CHAT_MODEL to win, got %q", got)
	}
}

func TestChatModelFromEnvFallsBackToLegacyName(t *testing.T) {
	t.Setenv("AI_CHAT_MODEL", "")
	t.Setenv("LLM_MODEL", "legacy-chat")

	if got := ChatModelFromEnv(); got != "legacy-chat" {
		t.Fatalf("expected LLM_MODEL fallback, got %q", got)
	}
}

func TestClientFromEnvKeepsLegacyBaseURL(t *testing.T) {
	t.Setenv("AI_GATEWAY_BASE_URL", "")
	t.Setenv("LLM_BASE_URL", "http://legacy-gateway/v1")

	if got := NewClientFromEnv().BaseURL; got != "http://legacy-gateway/v1" {
		t.Fatalf("expected legacy base URL fallback, got %q", got)
	}
}
