package worldbuilder

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// The endpoint must be a freshly created disposable service: this test creates
// the fixed projection index and refuses to reuse an existing index.
func TestDiscoveryAgainstDisposableOpenSearch(t *testing.T) {
	endpoint := os.Getenv("WORLDBUILDER_TEST_OPENSEARCH_URL")
	if endpoint == "" {
		t.Skip("disposable OpenSearch endpoint not configured")
	}
	request := func(method, path, body string) int {
		t.Helper()
		req, err := http.NewRequest(method, strings.TrimRight(endpoint, "/")+path, bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	index := "/worldbuilder-canon-v1"
	mapping := `{"mappings":{"properties":{"worldId":{"type":"keyword"},"revision":{"type":"long"},"visibility":{"type":"keyword"},"records":{"type":"nested","properties":{"id":{"type":"keyword"},"kind":{"type":"keyword"},"timeline":{"type":"keyword"},"text":{"type":"text"}}}}}}`
	if code := request("PUT", index, mapping); code != 200 {
		t.Fatalf("cannot create disposable index (will not reuse it): %d", code)
	}
	t.Cleanup(func() { request("DELETE", index, "") })
	body := `{"worldId":"world","revision":2,"visibility":"canon","records":[{"id":"hero","kind":"entity","text":"Hero canon"},{"id":"age","kind":"fact","timeline":"main","text":"Hero is twenty"},{"id":"hidden","kind":"event","timeline":"other","text":"Hero secret"}]}`
	if code := request("PUT", index+"/_doc/world?refresh=true", body); code != 201 {
		t.Fatalf("index fixture: %d", code)
	}
	discover := OpenSearchDiscovery(endpoint, "", "")
	d, err := discover(context.Background(), "world", "main", 2, "Hero")
	if err != nil || d.Revision != 2 || len(d.IDs) != 2 {
		t.Fatalf("real discovery: %+v %v", d, err)
	}
	for _, id := range d.IDs {
		if id != "hero" && id != "age" {
			t.Fatalf("foreign timeline: %s", id)
		}
	}
	for _, query := range []struct {
		world    string
		revision int64
	}{{"other", 2}, {"world", 1}} {
		d, err = discover(context.Background(), query.world, "main", query.revision, "Hero")
		if err != nil || len(d.IDs) != 0 {
			t.Fatalf("scope bypass: %+v %v", d, err)
		}
	}
}

func TestDiscoveryQueryScopesAndReturnsOnlyIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/worldbuilder-canon-v1/_search" {
			t.Errorf("unexpected discovery route %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		filters := body["query"].(map[string]any)["bool"].(map[string]any)["filter"].([]any)
		if filters[0].(map[string]any)["term"].(map[string]any)["worldId"] != "world" {
			t.Error("world filter missing")
		}
		if filters[1].(map[string]any)["term"].(map[string]any)["visibility"] != "canon" {
			t.Error("visibility filter missing")
		}
		_, _ = w.Write([]byte(`{"hits":{"hits":[{"_source":{"worldId":"world","revision":2,"visibility":"canon","ids":["ignored"]},"inner_hits":{"records":{"hits":{"hits":[{"_source":{"id":"hero","text":"NEVER authoritative indexed prose"}}]}}}}]}}`))
	}))
	defer server.Close()
	d, err := OpenSearchDiscovery(server.URL, "", "")(context.Background(), "world", "main", 3, "hero")
	if err != nil || d.WorldID != "world" || d.Revision != 2 || len(d.IDs) != 1 || d.IDs[0] != "hero" {
		t.Fatalf("discovery: %+v %v", d, err)
	}
}

func TestSearchReconcilesStaleAndUntrustedIndexes(t *testing.T) {
	s, err := Apply(Snapshot{"main": {ID: "main", Kind: "timeline", Label: "Main"}}, fixture())
	if err != nil {
		t.Fatal(err)
	}
	s["hidden"] = Record{ID: "hidden", Kind: "event", Label: "Hero in another timeline", Timeline: "alternate", Source: "source", From: ptr(1)}
	s["alternate"] = Record{ID: "alternate", Kind: "timeline", Label: "Alternate"}
	sc := &scope{world: World{ID: "world", Revision: 3}}
	sc.discover = func(context.Context, string, string, int64, string) (Discovery, error) {
		return Discovery{WorldID: "world", Revision: 1, Visibility: "canon", IDs: []string{"hero", "hidden", "deleted"}}, nil
	}
	r, err := sc.searchSnapshot(context.Background(), s, 3, "main", "Hero", nil, 100, 0)
	if err != nil || r.DiscoveryStatus != "stale_index_reconciled" || r.ReconciledCandidates != 1 {
		t.Fatalf("stale reconciliation: %+v %v", r, err)
	}
	for _, record := range r.Records {
		if record.ID == "hidden" || record.ID == "deleted" {
			t.Fatal("index bypassed timeline or canonical existence")
		}
	}
	for _, citation := range r.Citations {
		if citation.WorldID != "world" || citation.Revision != 3 || citation.Timeline != "main" {
			t.Fatal("unscoped citation")
		}
	}
	sc.discover = func(context.Context, string, string, int64, string) (Discovery, error) {
		return Discovery{WorldID: "other", Revision: 3, Visibility: "canon", IDs: []string{"hero"}}, nil
	}
	r, err = sc.searchSnapshot(context.Background(), s, 3, "main", "Hero", nil, 100, 0)
	if err != nil || r.DiscoveryStatus != "index_scope_rejected" || r.Total == 0 || r.ReconciledCandidates != 0 {
		t.Fatalf("foreign projection: %+v %v", r, err)
	}
	sc.discover = func(context.Context, string, string, int64, string) (Discovery, error) {
		return Discovery{}, errors.New("unavailable")
	}
	r, err = sc.searchSnapshot(context.Background(), s, 3, "main", "Hero", nil, 100, 0)
	if err != nil || r.DiscoveryStatus != "index_unavailable" || r.Total == 0 {
		t.Fatalf("fallback failed: %+v %v", r, err)
	}
	sc.discover = func(context.Context, string, string, int64, string) (Discovery, error) {
		t.Error("historical query consulted current projection")
		return Discovery{}, nil
	}
	r, err = sc.searchSnapshot(context.Background(), s, 1, "main", "Hero", nil, 1, 1)
	if err != nil || r.DiscoveryStatus != "historical_snapshot" || len(r.Records) != 1 {
		t.Fatalf("historical paging: %+v %v", r, err)
	}
}

func TestSearchEndpointAuthTemporalAndCanonicalContent(t *testing.T) {
	h := setup(t)
	w := h.world()
	h.publish(w, h.draft(w, 0, fixture()))
	calls := 0
	discover := func(_ context.Context, world, timeline string, revision int64, query string) (Discovery, error) {
		calls++
		if world != w || timeline != "main" || revision != 1 {
			t.Error("unscoped discovery call")
		}
		return Discovery{WorldID: w, Revision: 0, Visibility: "canon", IDs: []string{"age", "hero"}}, nil
	}
	h.handler = Handler(&Store{Pool: h.pool, Discover: discover}, func(r *http.Request) (string, bool) { actor := r.Header.Get("Test-Actor"); return actor, actor != "" })
	path := "/" + w + "/search?timeline=main&q=age&at=30"
	h.call("stranger@example.test", "GET", path, nil, 404)
	if calls != 0 {
		t.Fatal("unauthorised request reached discovery")
	}
	raw := h.call("reader@example.test", "GET", path, nil, 200)
	var result SearchResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	for _, record := range result.Records {
		if record.ID == "age" {
			t.Fatal("expired index fact returned")
		}
	}
	h.call("reader@example.test", "GET", "/"+w+"/search?timeline=main&q=", nil, 422)
}
