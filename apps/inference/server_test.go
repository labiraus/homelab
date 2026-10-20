package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type memoryLedger struct {
	mu           sync.Mutex
	seen         map[string]bool
	spent        int64
	fail         bool
	settled      int
	reservations int
}

func (l *memoryLedger) Reserve(_ context.Context, id, _, _ string, _ Model, n, limit int64, _ int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.fail {
		return errors.New("offline")
	}
	if l.seen[id] {
		return errDuplicate
	}
	if l.spent+n > limit {
		return errBudget
	}
	l.seen[id] = true
	l.spent += n
	l.reservations++
	return nil
}
func (l *memoryLedger) Settle(_ context.Context, _ string, _, _, _ int64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.settled++
	return nil
}
func (l *memoryLedger) Totals(_ context.Context, n int64) (Totals, error) {
	return Totals{Limit: n}, nil
}
func fixture(t *testing.T, handler http.HandlerFunc) (*Server, *memoryLedger) {
	t.Helper()
	up := httptest.NewServer(handler)
	t.Cleanup(up.Close)
	models := []Model{{Alias: "local-default", Revision: "local-r1", Provider: "local", Model: "local", Enabled: true, MaxInput: 4096, MaxOutput: 768, URL: up.URL},
		{Alias: "bedrock-nova", Revision: "nova-r1", Provider: "bedrock", Model: "eu.amazon.nova-2-lite-v1:0", Enabled: true, MaxInput: 1000000, MaxOutput: 4096, InputRate: 1000000, OutputRate: 2000000, PriceVersion: "test", PriceValidUntil: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), URL: up.URL}}
	c := Config{Models: models, CloudEnabled: true, MonthlyMicros: 100000000, LocalConcurrent: 2, CloudConcurrent: 2}
	if e := c.validate(); e != nil {
		t.Fatal(e)
	}
	l := &memoryLedger{seen: map[string]bool{}}
	return NewServer(c, []Client{{ID: "worldbuilder", Token: strings.Repeat("w", 32), Cloud: true}, {ID: "operator", Token: strings.Repeat("o", 32), Cloud: true}, {ID: "local", Token: strings.Repeat("l", 32)}}, l), l
}
func call(s *Server, body, token, id string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	if id != "" {
		r.Header.Set("X-Request-ID", id)
	}
	r.Header.Set("x-ai-eg-model", "global.forged")
	r.Header.Set("X-Amz-Security-Token", "forged")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

const normalReply = `{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2}}`

func TestRoutingBudgetAndLocalIndependence(t *testing.T) {
	count := 0
	s, l := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		count++
		if r.Header.Get("Authorization") != "" || r.Header.Get("x-ai-eg-model") != "" || r.Header.Get("X-Amz-Security-Token") != "" {
			t.Error("caller headers leaked")
		}
		var q Request
		_ = json.NewDecoder(r.Body).Decode(&q)
		if q.Model == "bedrock-nova" || q.Revision != "" {
			t.Error("alias was not translated")
		}
		w.Write([]byte(normalReply))
	})
	local := `{"messages":[{"role":"user","content":"hi"}],"max_tokens":10}`
	cloud := `{"model":"bedrock-nova","profile_revision":"nova-r1","messages":[{"role":"user","content":"hi"}],"max_tokens":10}`
	if w := call(s, local, "bad", ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := call(s, cloud, strings.Repeat("l", 32), ""); w.Code != 403 {
		t.Fatal(w.Code)
	}
	id := uuid.NewString()
	if w := call(s, cloud, strings.Repeat("w", 32), id); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call(s, cloud, strings.Repeat("w", 32), id); w.Code != 409 {
		t.Fatal(w.Code)
	}
	l.spent = s.Config.MonthlyMicros
	if w := call(s, cloud, strings.Repeat("w", 32), ""); w.Code != 429 {
		t.Fatal(w.Code)
	}
	if w := call(s, local, strings.Repeat("w", 32), ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	l.fail = true
	if w := call(s, local, strings.Repeat("w", 32), ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := call(s, cloud, strings.Repeat("w", 32), ""); w.Code != 503 {
		t.Fatal(w.Code)
	}
	if count != 3 || l.settled != 1 {
		t.Fatalf("unexpected dispatch/settlement: %d/%d", count, l.settled)
	}
}
func TestRejectUnsupportedAndChangedProfiles(t *testing.T) {
	s, _ := fixture(t, func(http.ResponseWriter, *http.Request) { t.Fatal("must not dispatch") })
	for _, suffix := range []string{`,"stream":true`, `,"thinking":{"type":"enabled"}`, `,"n":2`, `,"profile_revision":"changed"`, `,"response_format":{"type":"json_schema","json_schema":{"name":"x","schema":{"$ref":"https://example.com/schema"}}}`} {
		q := `{"model":"local-default","messages":[{"role":"user","content":"hi"}],"max_tokens":10` + suffix + `}`
		if w := call(s, q, strings.Repeat("w", 32), ""); w.Code != 400 && w.Code != 409 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if w := call(s, `{"model":"global.unapproved","messages":[],"max_tokens":10}`, strings.Repeat("w", 32), ""); w.Code != 403 {
		t.Fatal(w.Code)
	}
}
func TestStructuredCloudTranslationAndAccounting(t *testing.T) {
	s, l := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		var q Request
		_ = json.NewDecoder(r.Body).Decode(&q)
		if q.Format != nil || len(q.Tools) != 1 || string(q.ToolChoice) != `"required"` {
			t.Fatal("missing tool adaptation")
		}
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"1","type":"function","function":{"name":"structured_response","arguments":"{\"answer\":\"yes\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`))
	})
	body := `{"model":"bedrock-nova","messages":[{"role":"user","content":"hi"}],"max_tokens":100,"response_format":{"type":"json_schema","json_schema":{"name":"response","schema":{"type":"object","required":["answer"],"properties":{"answer":{"type":"string"}},"additionalProperties":false}}}}`
	w := call(s, body, strings.Repeat("w", 32), "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"accountingStatus":"estimated"`) || strings.Contains(w.Body.String(), `"tool_calls"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if l.settled != 1 {
		t.Fatal("usage not settled")
	}
}
func TestUncertainOutcomesNeverRefundOrRetry(t *testing.T) {
	for _, response := range []string{`{}`, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`, `{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":1000001,"completion_tokens":5}}`} {
		calls := 0
		s, l := fixture(t, func(w http.ResponseWriter, r *http.Request) { calls++; w.Write([]byte(response)) })
		body := `{"model":"bedrock-nova","messages":[{"role":"user","content":"hi"}],"max_tokens":100}`
		id := uuid.NewString()
		_ = call(s, body, strings.Repeat("w", 32), id)
		if w := call(s, body, strings.Repeat("w", 32), id); w.Code != 409 {
			t.Fatal(w.Code)
		}
		if calls != 1 || l.settled != 0 || l.reservations != 1 {
			t.Fatal("uncertain call refunded or replayed")
		}
	}
}
func TestConcurrencyAndMaintenance(t *testing.T) {
	entered, release := make(chan struct{}, 2), make(chan struct{})
	s, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release
		w.Write([]byte(normalReply))
	})
	q := `{"messages":[{"role":"user","content":"hi"}],"max_tokens":10}`
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); call(s, q, strings.Repeat("w", 32), "") }()
	}
	<-entered
	<-entered
	if w := call(s, q, strings.Repeat("w", 32), ""); w.Code != 429 {
		t.Fatal(w.Code)
	}
	s.drained.Store(true)
	close(release)
	wg.Wait()
	if w := call(s, q, strings.Repeat("w", 32), ""); w.Code != 503 {
		t.Fatal(w.Code)
	}
}
func TestCloudPricingExpiryAndOverflowBounds(t *testing.T) {
	s, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	})
	s.Config.Models[1].PriceValidUntil = time.Now().Add(-time.Second).Format(time.RFC3339)
	if s.Config.validate() != nil {
		t.Fatal("expired pricing blocked local startup")
	}
	if w := call(s, `{"model":"local-default","messages":[{"role":"user","content":"hi"}],"max_tokens":10}`, strings.Repeat("w", 32), ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := call(s, `{"model":"bedrock-nova","messages":[{"role":"user","content":"hi"}],"max_tokens":10}`, strings.Repeat("w", 32), ""); w.Code != 503 {
		t.Fatal(w.Code)
	}
	s.Config.Models[1].InputRate = 1 << 62
	if s.Config.Models[1].priced(time.Now()) {
		t.Fatal("overflowing rate accepted")
	}
}

func TestCancellationRetainsReservationAndNeverReplays(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	s, ledger := fixture(t, func(w http.ResponseWriter, r *http.Request) { close(started); <-release })
	id := uuid.NewString()
	body := `{"model":"bedrock-nova","messages":[{"role":"user","content":"hello"}],"max_tokens":10}`
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("w", 32))
	req.Header.Set("X-Request-ID", id)
	reply := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { defer close(done); s.ServeHTTP(reply, req) }()
	<-started
	cancel()
	<-done
	close(release)
	if reply.Code != 502 || ledger.reservations != 1 || ledger.settled != 0 {
		t.Fatal(reply.Code, ledger)
	}
	if replay := call(s, body, strings.Repeat("w", 32), id); replay.Code != 409 {
		t.Fatal("cancelled request replayed", replay.Code)
	}
}

func TestToolChoiceAndTruncatedStructuredResponseRefused(t *testing.T) {
	s, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"x","type":"function","function":{"name":"structured_response","arguments":"{}"}}]},"finish_reason":"length"}]}`))
	})
	token := strings.Repeat("w", 32)
	for _, choice := range []string{`"arbitrary"`, `{"type":"function","function":{"name":"missing"}}`, `{"type":"function","function":{"name":"f"},"region":"us-east-1"}`} {
		body := `{"model":"local-default","messages":[{"role":"user","content":"hi"}],"max_tokens":10,"tools":[{"type":"function","function":{"name":"f","parameters":{"type":"object"}}}],"tool_choice":` + choice + `}`
		if r := call(s, body, token, ""); r.Code != 400 {
			t.Fatal("unsupported tool selection accepted", r.Code)
		}
	}
	body := `{"model":"bedrock-nova","messages":[{"role":"user","content":"hi"}],"max_tokens":10,"response_format":{"type":"json_schema","json_schema":{"name":"r","schema":{"type":"object"}}}}`
	if r := call(s, body, token, ""); r.Code != 502 {
		t.Fatal("truncated tool output accepted", r.Code)
	}
}
