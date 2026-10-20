package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

type Message struct {
	Role       string     `json:"role"`
	Content    *string    `json:"content"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}
type Tool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description,omitempty"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}
type Format struct {
	Type   string `json:"type"`
	Schema struct {
		Name   string         `json:"name"`
		Strict bool           `json:"strict,omitempty"`
		Schema map[string]any `json:"schema"`
	} `json:"json_schema"`
}
type Request struct {
	Model       string          `json:"model"`
	Revision    string          `json:"profile_revision,omitempty"`
	Messages    []Message       `json:"messages"`
	MaxTokens   int64           `json:"max_tokens"`
	Temperature *float64        `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Tools       []Tool          `json:"tools,omitempty"`
	ToolChoice  json.RawMessage `json:"tool_choice,omitempty"`
	Format      *Format         `json:"response_format,omitempty"`
}
type Usage struct {
	Input  *int64 `json:"prompt_tokens"`
	Output *int64 `json:"completion_tokens"`
	Total  *int64 `json:"total_tokens,omitempty"`
}
type Reply struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int     `json:"index"`
		Message Message `json:"message"`
		Finish  string  `json:"finish_reason"`
	} `json:"choices"`
	Usage     Usage       `json:"usage"`
	Inference *Provenance `json:"inference,omitempty"`
}
type Provenance struct {
	RequestID  string `json:"requestId"`
	Profile    string `json:"profile"`
	Revision   string `json:"profileRevision"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	Accounting string `json:"accountingStatus"`
	Cost       int64  `json:"estimatedMicros"`
	Reserved   int64  `json:"reservedMicros"`
	Currency   string `json:"currency"`
}
type Server struct {
	Config   Config
	Clients  []Client
	Ledger   Ledger
	HTTP     *http.Client
	local    chan struct{}
	cloud    chan struct{}
	drained  atomic.Bool
	calls    atomic.Int64
	failures atomic.Int64
}

func NewServer(c Config, clients []Client, ledger Ledger) *Server {
	return &Server{Config: c, Clients: clients, Ledger: ledger, HTTP: &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, local: make(chan struct{}, c.LocalConcurrent), cloud: make(chan struct{}, c.CloudConcurrent)}
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, code string) {
	if status == 429 {
		w.Header().Set("Retry-After", "5")
	}
	writeJSON(w, status, map[string]any{"error": map[string]string{"type": "inference_error", "code": code, "message": code}})
}
func (s *Server) client(r *http.Request) (Client, bool) {
	supplied := sha256.Sum256([]byte(r.Header.Get("Authorization")))
	for _, c := range s.Clients {
		expected := sha256.Sum256([]byte("Bearer " + c.Token))
		if subtle.ConstantTimeCompare(supplied[:], expected[:]) == 1 {
			return c, true
		}
	}
	return Client{}, false
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if (r.URL.Path == "/healthz" || r.URL.Path == "/readyz") && r.Method == "GET" {
		writeJSON(w, 200, map[string]bool{"ready": true})
		return
	}
	if r.URL.Path == "/metrics" && r.Method == "GET" {
		s.metrics(w, r)
		return
	}
	c, ok := s.client(r)
	if !ok {
		problem(w, 401, "unauthenticated")
		return
	}
	if r.URL.RawQuery != "" {
		problem(w, 400, "query_parameters_not_supported")
		return
	}
	switch {
	case r.URL.Path == "/v1/models" && r.Method == "GET":
		models := []any{}
		for _, m := range s.Config.Models {
			if m.Enabled && (m.Provider == "local" || (s.Config.CloudEnabled && c.Cloud && m.priced(time.Now()))) {
				models = append(models, map[string]any{"id": m.Alias, "object": "model", "owned_by": m.Provider, "profile_revision": m.Revision, "max_input_tokens": m.MaxInput, "max_output_tokens": m.MaxOutput, "capabilities": []string{"text", "tools", "json_schema"}, "streaming": false})
			}
		}
		writeJSON(w, 200, map[string]any{"object": "list", "data": models})
	case r.URL.Path == "/v1/usage" && r.Method == "GET" && c.ID == "operator":
		if s.Ledger == nil {
			problem(w, 503, "accounting_unavailable")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		t, e := s.Ledger.Totals(ctx, s.Config.MonthlyMicros)
		if e != nil {
			problem(w, 503, "accounting_unavailable")
			return
		}
		writeJSON(w, 200, t)
	case r.URL.Path == "/admin/local" && c.ID == "operator":
		if r.Method == "POST" {
			var b struct {
				Drain *bool `json:"drain"`
			}
			if decodeBody(w, r, &b) != nil || b.Drain == nil {
				problem(w, 400, "drain_boolean_required")
				return
			}
			s.drained.Store(*b.Drain)
		} else if r.Method != "GET" {
			problem(w, 405, "method_not_allowed")
			return
		}
		writeJSON(w, 200, map[string]any{"drained": s.drained.Load() || s.Config.LocalPaused, "active": len(s.local)})
	case r.URL.Path == "/v1/chat/completions" && r.Method == "POST":
		s.chat(w, r, c)
	default:
		problem(w, 404, "not_found")
	}
}
func decodeBody(w http.ResponseWriter, r *http.Request, v any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
func compileSchema(schema map[string]any) (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	c.LoadURL = func(string) (io.ReadCloser, error) { return nil, errors.New("external schema references prohibited") }
	b, e := json.Marshal(schema)
	if e != nil {
		return nil, e
	}
	if e = c.AddResource("urn:inference:schema", bytes.NewReader(b)); e != nil {
		return nil, e
	}
	return c.Compile("urn:inference:schema")
}
func validateRequest(q Request, m Model) (*jsonschema.Schema, error) {
	bad := errors.New("unsupported_or_invalid_request")
	if q.Stream || len(q.Messages) == 0 || len(q.Messages) > 128 || q.MaxTokens < 1 || q.MaxTokens > m.MaxOutput || (q.Temperature != nil && (*q.Temperature < 0 || *q.Temperature > 2)) || len(q.Tools) > 32 {
		return nil, bad
	}
	for _, msg := range q.Messages {
		if msg.Role != "system" && msg.Role != "user" && msg.Role != "assistant" && msg.Role != "tool" {
			return nil, bad
		}
		if msg.Content == nil && (msg.Role != "assistant" || len(msg.ToolCalls) == 0) {
			return nil, bad
		}
		if (msg.ToolCallID != "" && msg.Role != "tool") || (msg.Role == "tool" && msg.ToolCallID == "") || (len(msg.ToolCalls) > 0 && msg.Role != "assistant") {
			return nil, bad
		}
		for _, tc := range msg.ToolCalls {
			if tc.Type != "function" || tc.ID == "" || tc.Function.Name == "" || !json.Valid([]byte(tc.Function.Arguments)) {
				return nil, bad
			}
		}
	}
	names := map[string]bool{}
	for _, tool := range q.Tools {
		if tool.Type != "function" || tool.Function.Name == "" || tool.Function.Parameters == nil {
			return nil, bad
		}
		if names[tool.Function.Name] {
			return nil, bad
		}
		names[tool.Function.Name] = true
		if _, e := compileSchema(tool.Function.Parameters); e != nil {
			return nil, bad
		}
	}
	if len(q.ToolChoice) > 0 {
		if len(q.Tools) == 0 {
			return nil, bad
		}
		var choice string
		if json.Unmarshal(q.ToolChoice, &choice) == nil {
			if choice != "auto" && choice != "none" && choice != "required" {
				return nil, bad
			}
		} else {
			var named struct {
				Type     string `json:"type"`
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			}
			d := json.NewDecoder(bytes.NewReader(q.ToolChoice))
			d.DisallowUnknownFields()
			if d.Decode(&named) != nil || named.Type != "function" || !names[named.Function.Name] {
				return nil, bad
			}
		}
	}
	if q.Format == nil {
		return nil, nil
	}
	if q.Format.Type != "json_schema" || q.Format.Schema.Name == "" || q.Format.Schema.Schema == nil || len(q.Tools) > 0 || len(q.ToolChoice) > 0 {
		return nil, bad
	}
	return compileSchema(q.Format.Schema.Schema)
}
func (s *Server) chat(w http.ResponseWriter, r *http.Request, c Client) {
	var q Request
	if e := decodeBody(w, r, &q); e != nil {
		problem(w, 400, "invalid_request")
		return
	}
	if q.Model == "" {
		q.Model = "local-default"
	}
	var m Model
	for _, candidate := range s.Config.Models {
		if candidate.Alias == q.Model {
			m = candidate
		}
	}
	if !m.Enabled || (m.Provider == "bedrock" && (!s.Config.CloudEnabled || !c.Cloud)) {
		problem(w, 403, "model_not_allowed")
		return
	}
	if q.Revision != "" && q.Revision != m.Revision {
		problem(w, 409, "profile_revision_changed")
		return
	}
	schema, e := validateRequest(q, m)
	if e != nil {
		problem(w, 400, "unsupported_or_invalid_request")
		return
	}
	id := r.Header.Get("X-Request-ID")
	if id == "" {
		id = uuid.NewString()
	} else {
		u, e := uuid.Parse(id)
		if e != nil {
			problem(w, 400, "invalid_request_id")
			return
		}
		id = u.String()
	}
	p := &Provenance{RequestID: id, Profile: m.Alias, Revision: m.Revision, Provider: m.Provider, Model: m.Model, Accounting: "local", Currency: "USD"}
	w.Header().Set("X-Request-ID", id)
	sem := s.local
	if m.Provider == "bedrock" {
		sem = s.cloud
	} else if (s.drained.Load() || s.Config.LocalPaused) && c.ID != "operator" {
		problem(w, 503, "local_maintenance")
		return
	}
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	default:
		problem(w, 429, "capacity_exhausted")
		return
	}
	if m.Provider == "local" && (s.drained.Load() || s.Config.LocalPaused) && c.ID != "operator" {
		problem(w, 503, "local_maintenance")
		return
	}
	if m.Provider == "bedrock" {
		if s.Ledger == nil || !m.priced(time.Now()) {
			problem(w, 503, "cloud_accounting_unavailable")
			return
		}
		raw, _ := json.Marshal(q)
		digest := sha256.Sum256(raw)
		p.Reserved = m.cost(m.MaxInput, q.MaxTokens)
		p.Accounting = "reserved"
		accounting, stop := context.WithTimeout(r.Context(), 5*time.Second)
		e = s.Ledger.Reserve(accounting, id, c.ID, hex.EncodeToString(digest[:]), m, p.Reserved, s.Config.MonthlyMicros, s.Config.CloudConcurrent)
		stop()
		if e != nil {
			status := 503
			code := "accounting_unavailable"
			switch {
			case errors.Is(e, errDuplicate):
				status = 409
				code = "duplicate_request"
			case errors.Is(e, errCapacity):
				status = 429
				code = "capacity_exhausted"
			case errors.Is(e, errBudget):
				status = 429
				code = "monthly_budget_exhausted"
			}
			problem(w, status, code)
			return
		}
	}
	q.Model = m.Model
	q.Revision = ""
	if schema != nil && m.Provider == "bedrock" {
		var tool Tool
		tool.Type = "function"
		tool.Function.Name = "structured_response"
		tool.Function.Description = "Return the requested structured response"
		tool.Function.Parameters = q.Format.Schema.Schema
		q.Tools = []Tool{tool}
		q.ToolChoice = json.RawMessage(`"required"`)
		q.Format = nil
	}
	body, _ := json.Marshal(q)
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	up, e := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(m.URL, "/")+"/chat/completions", bytes.NewReader(body))
	if e != nil {
		problem(w, 502, "upstream_unavailable")
		return
	}
	// Construct headers from scratch: caller identity/routing/AWS headers never propagate.
	up.Header.Set("Content-Type", "application/json")
	up.Header.Set("X-Request-ID", id)
	s.calls.Add(1)
	res, e := s.HTTP.Do(up)
	if e != nil {
		s.failures.Add(1)
		problem(w, 502, "upstream_outcome_uncertain")
		return
	}
	defer res.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
	if e != nil || len(raw) > 2<<20 || res.StatusCode != 200 {
		s.failures.Add(1)
		problem(w, 502, "upstream_outcome_uncertain")
		return
	}
	var reply Reply
	if json.Unmarshal(raw, &reply) != nil {
		s.failures.Add(1)
		problem(w, 502, "invalid_upstream_response")
		return
	}
	// Account even if generated content subsequently fails schema validation.
	if m.Provider == "bedrock" && reply.Usage.Input != nil && reply.Usage.Output != nil && *reply.Usage.Input >= 0 && *reply.Usage.Input <= m.MaxInput && *reply.Usage.Output >= 0 && *reply.Usage.Output <= q.MaxTokens {
		cost := m.cost(*reply.Usage.Input, *reply.Usage.Output)
		ctx, stop := settlementContext()
		e = s.Ledger.Settle(ctx, id, cost, *reply.Usage.Input, *reply.Usage.Output)
		stop()
		if e == nil {
			p.Accounting = "estimated"
			p.Cost = cost
		} else {
			p.Accounting = "uncertain"
		}
	} else if m.Provider == "bedrock" {
		p.Accounting = "uncertain"
	}
	if len(reply.Choices) != 1 {
		problem(w, 502, "invalid_upstream_response")
		return
	}
	if schema != nil {
		msg := &reply.Choices[0].Message
		if reply.Choices[0].Finish == "length" {
			problem(w, 502, "invalid_structured_response")
			return
		}
		if m.Provider == "bedrock" {
			if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Function.Name != "structured_response" {
				problem(w, 502, "invalid_structured_response")
				return
			}
			v := msg.ToolCalls[0].Function.Arguments
			msg.Content = &v
			msg.ToolCalls = nil
			reply.Choices[0].Finish = "stop"
		}
		if msg.Content == nil || reply.Choices[0].Finish == "length" {
			problem(w, 502, "invalid_structured_response")
			return
		}
		var v any
		d := json.NewDecoder(strings.NewReader(*msg.Content))
		d.UseNumber()
		if d.Decode(&v) != nil || schema.Validate(v) != nil {
			problem(w, 502, "invalid_structured_response")
			return
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			problem(w, 502, "invalid_structured_response")
			return
		}
	}
	if reply.Model == "" {
		reply.Model = m.Model
		if m.Provider == "bedrock" {
			reply.Model = strings.TrimPrefix(m.Model, "eu.")
		}
	}
	p.Model = reply.Model
	reply.Inference = p
	w.Header().Set("X-Request-ID", id)
	writeJSON(w, 200, reply)
}
func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "inference_requests_total %d\ninference_upstream_errors_total %d\ninference_local_active %d\ninference_cloud_active %d\ninference_monthly_limit_micros %d\n", s.calls.Load(), s.failures.Load(), len(s.local), len(s.cloud), s.Config.MonthlyMicros)
	if s.Ledger != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		t, e := s.Ledger.Totals(ctx, s.Config.MonthlyMicros)
		if e == nil {
			fmt.Fprintf(w, "inference_accounting_available 1\ninference_monthly_settled_micros %d\ninference_monthly_reserved_micros %d\ninference_monthly_remaining_micros %d\n", t.Settled, t.Reserved, t.Remaining)
			return
		}
	}
	fmt.Fprintln(w, "inference_accounting_available 0")
}
