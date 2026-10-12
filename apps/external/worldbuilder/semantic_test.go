package worldbuilder

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"pkg/canonsearch"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

func TestHybridSearchActualIndexAuthorizationAllowanceAndEvidence(t *testing.T) {
	endpoint := os.Getenv("WORLDBUILDER_TEST_OPENSEARCH_URL")
	if endpoint == "" {
		t.Skip("owned disposable OpenSearch required")
	}
	h := setup(t)
	w := h.world()
	h.publish(w, h.draft(w, 0, fixture()))
	for batch := 0; batch < 2; batch++ {
		ops := []Operation{}
		for n := 0; n < 60; n++ {
			id := fmt.Sprintf("background-%03d", batch*60+n)
			r := Record{ID: id, Kind: "entity", Type: "person", Label: "Desert resident"}
			ops = append(ops, Operation{Action: "put", ID: id, Record: &r})
		}
		if batch == 1 {
			for _, r := range []Record{
				{ID: "zboat", Kind: "entity", Type: "person", Label: "Ferry captain"},
				{ID: "other", Kind: "timeline", Label: "Other"},
				{ID: "hidden", Kind: "event", Label: "Ferry secret", Timeline: "other", Source: "source", From: ptr(0)},
				{ID: "expired", Kind: "event", Label: "Ferry departure", Timeline: "main", Source: "source", From: ptr(10), Until: ptr(20)},
			} {
				copy := r
				ops = append(ops, Operation{Action: "put", ID: r.ID, Record: &copy})
			}
		}
		h.publish(w, h.draft(w, int64(batch+1), ops))
	}
	actor := uuid.NewString() + "@example.test"
	h.call("owner@example.test", "POST", "/"+w+"/members", map[string]string{"principal": actor, "role": "reader"}, 200)
	var calls atomic.Int32
	var fail atomic.Bool
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if fail.Load() {
			w.WriteHeader(503)
			return
		}
		var b struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if json.NewDecoder(r.Body).Decode(&b) != nil || len(b.Input) > 8 || r.Header.Get("X-Request-ID") == "" {
			t.Error("invalid model request")
		}
		data := []any{}
		for n, text := range b.Input {
			v := []float32{0, 1}
			if strings.Contains(strings.ToLower(text), "ferry") || strings.Contains(text, "crossing water") {
				v = []float32{1, 0}
			}
			data = append(data, map[string]any{"index": n, "embedding": v})
		}
		json.NewEncoder(w).Encode(map[string]any{"model": b.Model, "data": data, "usage": map[string]int{"prompt_tokens": 4 * len(b.Input)}})
	}))
	defer model.Close()
	index := &canonsearch.Index{Endpoint: endpoint, Model: &canonsearch.Embedder{Endpoint: model.URL + "/v1/embeddings", Profile: canonsearch.Profile{Model: "synthetic-external", Revision: uuid.NewString(), Dimensions: 2}}}
	ctx := context.Background()
	if err := index.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		req, _ := http.NewRequest("DELETE", endpoint+"/"+index.Name(), nil)
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Error(e)
			return
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Error("index cleanup failed")
		}
	})
	var snapshot []byte
	if e := h.pool.QueryRow(ctx, `SELECT snapshot FROM worldbuilder.revision WHERE world_id=$1 AND revision=3`, w).Scan(&snapshot); e != nil {
		t.Fatal(e)
	}
	if _, e := index.Publish(ctx, w, 3, snapshot, "test-publication"); e != nil {
		t.Fatal(e)
	}
	budget := &InferenceBudget{Pool: h.pool, DailyRequests: 100, Concurrent: 2, RequestMicros: 100, DailyMicros: 10000, Currency: "TST", GatewayRoute: "offline-private-model"}
	reasoned := false
	store := &Store{Pool: h.pool, Vector: index, Budget: budget, Reasoner: func(_ context.Context, r ReasonRequest) (ReasonResult, error) {
		reasoned = true
		found, hasType := false, false
		for _, e := range r.Evidence {
			if e.Record.ID == "zboat" {
				found = true
			}
			if e.Record.ID == "person" {
				hasType = true
			}
			if e.Record.ID == "hidden" {
				t.Fatal("hidden timeline entered AI context")
			}
		}
		if !found || !hasType || len(r.Evidence) > 100 || r.ContextScope != "selected_records" {
			t.Fatalf("semantic evidence or closure missing: %+v", r)
		}
		return ReasonResult{Answer: "The ferry captain.", Citations: []string{w + ":3:main:zboat"}, Model: "synthetic-reasoner", Usage: map[string]any{"retrieval": "forged"}}, nil
	}}
	h.handler = Handler(store, func(r *http.Request) (string, bool) { a := r.Header.Get("Test-Actor"); return a, a != "" })
	body := map[string]any{"revision": 3, "timeline": "main", "query": "crossing water", "at": "20", "limit": 1}
	before := calls.Load()
	h.call("stranger@example.test", "POST", "/"+w+"/search", body, 404)
	invalid := map[string]any{"revision": 2, "timeline": "main", "query": "crossing water"}
	h.call(actor, "POST", "/"+w+"/search", invalid, 409)
	invalid["revision"] = 3
	invalid["timeline"] = "missing"
	h.call(actor, "POST", "/"+w+"/search", invalid, 422)
	if calls.Load() != before {
		t.Fatal("unauthorized/invalid request attempted model")
	}
	var lexical SearchResult
	json.Unmarshal(h.call(actor, "GET", "/"+w+"/search?revision=3&timeline=main&q=crossing%20water", nil, 200), &lexical)
	if lexical.Total != 0 {
		t.Fatal("fixture query must have no lexical matches")
	}
	var result SearchResult
	json.Unmarshal(h.call(actor, "POST", "/"+w+"/search", body, 200), &result)
	if result.Mode != "hybrid" || len(result.Records) != 1 || result.Records[0].ID != "zboat" || result.Citations[0].Record.Label != "Ferry captain" || result.Provenance == nil || result.Provenance.Workflow != "canon_search" || len(result.SearchToken) != 64 {
		t.Fatalf("semantic recall/canonical citation/provenance: %+v", result)
	}
	body["offset"] = 1
	h.call(actor, "POST", "/"+w+"/search", body, 422)
	body["searchToken"] = result.SearchToken
	body["limit"] = 500
	var next SearchResult
	json.Unmarshal(h.call(actor, "POST", "/"+w+"/search", body, 200), &next)
	for _, r := range next.Records {
		if r.ID == "hidden" || r.ID == "expired" || r.ID == "zboat" {
			t.Fatalf("scope/page leak %s", r.ID)
		}
	}
	body["searchToken"] = strings.Repeat("0", 64)
	h.call(actor, "POST", "/"+w+"/search", body, 409)
	var answer ReasonResult
	json.Unmarshal(h.call(actor, "POST", "/"+w+"/reason", map[string]any{"revision": 3, "timeline": "main", "question": "crossing water", "workflow": "canon_query"}, 200), &answer)
	if !reasoned || answer.EvidenceScope.DiscoveryStatus != "current_vector_index" {
		t.Fatalf("semantic reasoning was not used: %+v", answer)
	}
	if _, ok := answer.Usage["retrieval"].(map[string]any); !ok {
		t.Fatal("untrusted model usage overwrote retrieval provenance")
	}
	fail.Store(true)
	delete(body, "searchToken")
	delete(body, "offset")
	h.call(actor, "POST", "/"+w+"/search", body, 502)
	var attempts, charged, failed int
	if e := h.pool.QueryRow(ctx, `SELECT count(*),sum(allocated_micros),count(*) FILTER(WHERE status='failed') FROM worldbuilder.inference_attempt WHERE actor=$1`, actor).Scan(&attempts, &charged, &failed); e != nil {
		t.Fatal(e)
	}
	if attempts != 5 || charged != 500 || failed != 2 {
		t.Fatalf("durable allowance attempts=%d charged=%d failed=%d", attempts, charged, failed)
	}
	// Missing current projection refuses before reserving another attempt.
	h.publish(w, h.draft(w, 3, []Operation{{Action: "delete", ID: "expired"}}))
	body["revision"] = 4
	before = calls.Load()
	h.call(actor, "POST", "/"+w+"/search", body, 409)
	if calls.Load() != before {
		t.Fatal("lagging index attempted inference")
	}
}
