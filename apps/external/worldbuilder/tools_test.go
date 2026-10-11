package worldbuilder

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"pkg/api"
	"strings"
	"testing"
	"time"
)

func TestDelegatedToolsCannotAcquireHumanAuthority(t *testing.T) {
	h := setup(t)
	world := h.world()
	key := []byte(strings.Repeat("test-only-key-", 3))
	active := true
	handler := ToolsHandler(&Store{Pool: h.pool}, key, func(context.Context, string) (bool, string, error) { return active, "", nil })
	call := func(actor string, b toolRequest, signed bool, want int) []byte {
		t.Helper()
		raw, _ := json.Marshal(b)
		r := httptest.NewRequest("POST", ToolsPath, bytes.NewReader(raw))
		r.Header.Set("X-Forwarded-Email", "owner@example.test")
		if signed {
			token, err := api.SignDelegation(key, actor, ToolsAudience, "POST", ToolsPath, raw, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			r.Header.Set("X-Worldbuilder-Delegation", token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s as %s want %d got %d: %s", b.Operation, actor, want, w.Code, w.Body)
		}
		return w.Body.Bytes()
	}
	read := toolRequest{Operation: "canon", WorldID: world, Timeline: "main"}
	call("reader@example.test", read, false, 401)
	call("stranger@example.test", read, true, 404)
	call("reader@example.test", read, true, 200)
	active = false
	call("reader@example.test", read, true, 401)
	active = true
	proposal := toolRequest{Operation: "propose", WorldID: world, ID: uuid.NewString(), Reason: "Tool draft", Operations: fixture()}
	call("reader@example.test", proposal, true, 403)
	first := call("editor@example.test", proposal, true, 200)
	again := call("editor@example.test", proposal, true, 200)
	var c1, c2 Change
	json.Unmarshal(first, &c1)
	json.Unmarshal(again, &c2)
	if c1.ID != c2.ID || c1.Status != "draft" || c1.Author != "editor@example.test" {
		t.Fatalf("bad draft: %s", first)
	}
	for _, operation := range []string{"publish", "review", "submit", "administration", "members", "reason", "sessions", "restore"} {
		call("owner@example.test", toolRequest{Operation: operation, WorldID: world, ID: proposal.ID}, true, 403)
	}
	var revision int
	if err := h.pool.QueryRow(context.Background(), `SELECT revision FROM worldbuilder.world WHERE id=$1`, world).Scan(&revision); err != nil || revision != 0 {
		t.Fatalf("tool changed canon: %d %v", revision, err)
	}
	h.call("owner@example.test", "POST", "/"+world+"/administration", AdminRequest{RequestID: uuid.NewString(), ExpectedVersion: 3, Action: "remove_member", Principal: "reader@example.test", Reason: "Revoke"}, 200)
	call("reader@example.test", read, true, 404)
}

func TestDelegatedToolsRejectTamperingBeforeDatabase(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	raw := []byte(`{"operation":"worlds"}`)
	token, _ := api.SignDelegation(key, "owner@example.test", ToolsAudience, "POST", ToolsPath, raw, time.Now())
	handler := ToolsHandler(&Store{}, key, func(context.Context, string) (bool, string, error) {
		t.Fatal("tampered request reached identity lookup")
		return false, "", nil
	})
	r := httptest.NewRequest("POST", ToolsPath, strings.NewReader(`{"operation":"publish"}`))
	r.Header.Set("X-Worldbuilder-Delegation", token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("tampering: %d", w.Code)
	}
}
