package canonsearch

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

func profile() Profile {
	return Profile{Model: "fixture-model", Revision: "fixture-weights-v1", Dimensions: 2, QueryPrefix: "query: "}
}
func fixtureModel(t *testing.T) (*Embedder, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "POST" || r.URL.Path != "/v1/embeddings" || r.Header.Get("X-Request-ID") == "" {
			t.Error("invalid model request")
		}
		var b struct {
			Model    string   `json:"model"`
			Input    []string `json:"input"`
			Encoding string   `json:"encoding_format"`
		}
		if json.NewDecoder(r.Body).Decode(&b) != nil || b.Model != profile().Model || b.Encoding != "float" || len(b.Input) > 8 {
			t.Error("invalid model contract")
		}
		data := []any{}
		for i := len(b.Input) - 1; i >= 0; i-- {
			text := strings.ToLower(b.Input[i])
			v := []float32{0, 1}
			if strings.Contains(text, "ferry") || strings.Contains(text, "crossing water") || strings.Contains(text, "boat") {
				v = []float32{1, 0}
			}
			data = append(data, map[string]any{"index": i, "embedding": v})
		}
		json.NewEncoder(w).Encode(map[string]any{"model": profile().Model, "data": data, "usage": map[string]int{"prompt_tokens": len(b.Input) * 4}})
	}))
	t.Cleanup(s.Close)
	return &Embedder{Profile: profile(), Endpoint: s.URL + "/v1/embeddings"}, calls
}
func TestModelBatchesPreserveIndexOrderAndFullUTF8(t *testing.T) {
	e, calls := fixtureModel(t)
	text := strings.Repeat("雨🌍\"\n", 1000) + " boat"
	chunks, err := Chunks(text)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(chunks, "") != text {
		t.Fatal("source bytes lost")
	}
	for _, chunk := range chunks {
		if !utf8.ValidString(chunk) || len(chunk) > ChunkBytes {
			t.Fatal("invalid chunk boundary")
		}
	}
	values, u, err := e.Texts(context.Background(), []string{"ferry", "desert"}, "test-id", false)
	if err != nil || values[0][0] != 1 || values[1][1] != 1 || u.Requests != 1 || u.InputTokens != 8 {
		t.Fatalf("batch/index/provenance: %v %+v %v", values, u, err)
	}
	v, u, err := e.Mean(context.Background(), text, "long-id", false)
	if err != nil || v[0] <= 0 || u.Requests < 2 || calls.Load() < 3 {
		t.Fatalf("final chunk dropped or batching failed: %v %+v %v", v, u, err)
	}
	if math.Abs(float64(v[0]*v[0]+v[1]*v[1])-1) > 1e-5 {
		t.Fatal("mean not normalized")
	}
	texts := make([]string, 32)
	for n := range texts {
		texts[n] = "ferry"
	}
	values, u, err = e.Texts(context.Background(), texts, "record-batch", false)
	if err != nil || len(values) != 32 || u.Requests != 4 {
		t.Fatalf("32 records must fit the service's eight-input ceiling: %d %+v %v", len(values), u, err)
	}
}
func TestModelRefusesMalformedResponsesAndRedirects(t *testing.T) {
	responses := []string{
		`{"model":"other","data":[{"index":0,"embedding":[1,0]}],"usage":{"prompt_tokens":1}}`,
		`{"model":"fixture-model","data":[{"index":0,"embedding":[0,0]}],"usage":{"prompt_tokens":1}}`,
		`{"model":"fixture-model","data":[{"index":0,"embedding":[1]}],"usage":{"prompt_tokens":1}}`,
		`{"model":"fixture-model","data":[{"index":1,"embedding":[1,0]}],"usage":{"prompt_tokens":1}}`,
		`{"model":"fixture-model","data":[{"embedding":[1,0]}],"usage":{"prompt_tokens":1}}`,
		`{"model":"fixture-model","data":[{"index":0,"embedding":[1,0]},{"index":0,"embedding":[1,0]}],"usage":{"prompt_tokens":1}}`,
		`{"model":"fixture-model","data":[{"index":0,"embedding":[1,0]}]}`,
		`{"model":"fixture-model","data":[{"index":0,"embedding":[1e999,0]}],"usage":{"prompt_tokens":1}}`,
		`{"model":"fixture-model","data":[{"index":0,"embedding":[1,0]}],"usage":{"prompt_tokens":-1}}`,
		strings.Repeat("x", (4<<20)+1),
	}
	for _, body := range responses {
		t.Run(body[:min(48, len(body))], func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
			defer s.Close()
			e := &Embedder{Profile: profile(), Endpoint: s.URL}
			_, u, err := e.Batch(context.Background(), []string{"input"}, "refusal")
			if err == nil || u.Requests != 1 {
				t.Fatalf("invalid output passed: %+v %v", u, err)
			}
		})
	}
	var followed atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { followed.Store(true) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer source.Close()
	e := &Embedder{Profile: profile(), Endpoint: source.URL, Token: "fixture-token"}
	if _, _, err := e.Batch(context.Background(), []string{"input"}, "redirect"); err == nil || followed.Load() {
		t.Fatal("redirect followed or accepted")
	}
}
func TestModelBoundariesCancelAndProfile(t *testing.T) {
	e, _ := fixtureModel(t)
	for _, text := range []string{"", string([]byte{0xff}), strings.Repeat("x", ChunkBytes*MaxChunks+1)} {
		if _, err := Chunks(text); err == nil {
			t.Fatal("invalid source accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := e.Mean(ctx, "boat", "cancel", true); err == nil {
		t.Fatal("cancellation ignored")
	}
	p := profile()
	id := p.ID()
	p.Revision = "new-weights"
	if id == p.ID() {
		t.Fatal("profile failed to pin model revision")
	}
	p = profile()
	p.Dimensions = -1
	e.Profile = p
	if _, _, err := e.Mean(context.Background(), "x", "invalid", false); err == nil {
		t.Fatal("invalid dimensions accepted")
	}
}
func TestModelConcurrentQueueCancellation(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		w.Write([]byte(`{"model":"fixture-model","data":[{"index":0,"embedding":[1,0]}],"usage":{"prompt_tokens":1}}`))
	}))
	defer s.Close()
	e := &Embedder{Profile: profile(), Endpoint: s.URL}
	done := make(chan struct{})
	go func() { defer close(done); e.Batch(context.Background(), []string{"one"}, "first") }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, u, err := e.Batch(ctx, []string{"two"}, "second")
	close(release)
	<-done
	if err == nil || u.Requests != 0 {
		t.Fatalf("queued cancellation performed inference: %+v %v", u, err)
	}
}
