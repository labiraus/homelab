package main

import (
	"testing"
	"time"
)

func TestMCPContextTimeoutDefaultsToFiveSeconds(t *testing.T) {
	t.Setenv("MCP_CONTEXT_TIMEOUT_SECONDS", "")

	if got := mcpContextTimeout(); got != 5*time.Second {
		t.Fatalf("expected default MCP context timeout to be 5s, got %s", got)
	}
}

func TestMCPContextTimeoutUsesConfiguredSeconds(t *testing.T) {
	t.Setenv("MCP_CONTEXT_TIMEOUT_SECONDS", "12")

	if got := mcpContextTimeout(); got != 12*time.Second {
		t.Fatalf("expected configured MCP context timeout to be 12s, got %s", got)
	}
}
