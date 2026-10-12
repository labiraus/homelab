package canonsearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestVectorSearchRejectsPartialOrUntrustedResults(t *testing.T) {
	model, _ := fixtureModel(t)
	source := `"_source":{"worldId":"world","revision":1,"visibility":"canon","profile":"` + model.Profile.ID() + `"}`
	hits := `{"_source":{"id":"a"},"_score":null,"sort":[1.0,"a"]},{"_source":{"id":"b"},"_score":null,"sort":[0.5,"b"]}`
	base := `{"timed_out":false,"_shards":{"total":1,"successful":1,"failed":0},"hits":{"hits":[{` + source + `,"inner_hits":{"records":{"hits":{"hits":[` + hits + `]}}}}]}}`
	cases := map[string]string{
		"valid":               base,
		"timeout":             strings.Replace(base, `"timed_out":false`, `"timed_out":true`, 1),
		"partial shards":      strings.Replace(base, `"failed":0`, `"failed":1`, 1),
		"foreign world":       strings.Replace(base, `"worldId":"world"`, `"worldId":"other"`, 1),
		"stale":               strings.Replace(base, `"revision":1`, `"revision":0`, 1),
		"draft":               strings.Replace(base, `"visibility":"canon"`, `"visibility":"draft"`, 1),
		"profile":             strings.Replace(base, model.Profile.ID(), "other", 1),
		"missing score":       strings.Replace(base, `[1.0,"a"]`, `[null,"a"]`, 1),
		"missing sort":        strings.Replace(base, `"sort":[1.0,"a"]`, `"sort":[]`, 1),
		"bad score":           strings.Replace(base, `[1.0,"a"]`, `[-1,"a"]`, 1),
		"sort mismatch":       strings.Replace(base, `[1.0,"a"]`, `[1.0,"other"]`, 1),
		"score mismatch":      strings.Replace(base, `"_score":null`, `"_score":0.1`, 1),
		"duplicate":           strings.ReplaceAll(base, `"b"`, `"a"`),
		"hidden candidate":    strings.ReplaceAll(base, `"b"`, `"hidden"`),
		"order":               strings.Replace(base, `[0.5,"b"]`, `[1.5,"b"]`, 1),
		"incomplete coverage": strings.Replace(base, hits, strings.Split(hits, `},{`)[0]+`}`, 1),
	}
	for name, response := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(response))
			}))
			defer server.Close()
			i := &Index{Endpoint: server.URL, Model: model}
			d, err := i.Find(context.Background(), "world", 1, []string{"a", "b"}, "crossing water", "test-query")
			if name == "valid" {
				if err != nil || len(d.Candidates) != 2 || d.Candidates[0].Score != 1 {
					t.Fatalf("valid sorted scores: %+v %v", d, err)
				}
			} else if err == nil {
				t.Fatal("untrusted result accepted")
			}
		})
	}
}

// The HTTP model here is synthetic. This proves the actual OpenSearch vector
// contract and recovery, not the quality of the production CPU model.
func TestVectorAgainstDisposableOpenSearch(t *testing.T) {
	endpoint := os.Getenv("WORLDBUILDER_TEST_OPENSEARCH_URL")
	if endpoint == "" {
		t.Skip("owned disposable OpenSearch required")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() != "127.0.0.1" {
		t.Fatal("loopback fixture required")
	}
	ctx := context.Background()
	model, calls := fixtureModel(t)
	i := &Index{Endpoint: endpoint, Model: model}
	if err = i.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if status, _, e := i.request(ctx, "DELETE", "", nil, 65536); e != nil || status != 200 {
			t.Errorf("fixture index cleanup: %d %v", status, e)
		}
	})
	snapshot := []byte(`{"zboat":{"id":"zboat","kind":"entity","name":"ferry"},"asand":{"id":"asand","kind":"entity","name":"desert"},"secret":{"id":"secret","kind":"entity","name":"ferry on another timeline"}}`)
	usage, err := i.Publish(ctx, "world", 1, snapshot, "publication-1")
	if err != nil || usage.Requests != 1 {
		t.Fatalf("publish %+v %v", usage, err)
	}
	before := calls.Load()
	if _, err = i.Publish(ctx, "world", 1, snapshot, "duplicate"); err != nil || calls.Load() != before {
		t.Fatalf("idempotent cache %v", err)
	}
	d, err := i.Find(ctx, "world", 1, []string{"zboat", "asand"}, "crossing water", "query-1")
	if err != nil || len(d.Candidates) != 2 || d.Candidates[0].ID != "zboat" || d.Candidates[0].Score <= d.Candidates[1].Score || d.Usage.Requests != 1 {
		t.Fatalf("semantic-only recall %+v %v", d, err)
	}
	for _, c := range d.Candidates {
		if c.ID == "secret" {
			t.Fatal("canonical visibility filter bypassed")
		}
	}
	if _, err = i.Find(ctx, "other-world", 1, []string{"zboat"}, "crossing water", "wrong-world"); err == nil {
		t.Fatal("cross-world result accepted")
	}
	changed := []byte(strings.Replace(string(snapshot), `"desert"`, `"desert dunes"`, 1))
	before = calls.Load()
	usage, err = i.Publish(ctx, "world", 2, changed, "publication-2")
	if err != nil || calls.Load() != before+1 || usage.InputTokens != 4 {
		t.Fatalf("changed record cache %+v %v", usage, err)
	}
	if _, err = i.Publish(ctx, "world", 1, snapshot, "older"); err != nil || calls.Load() != before+1 {
		t.Fatalf("stale publication %v", err)
	}
	if ready, e := i.Ready(ctx, "world", 1); e != nil || ready {
		t.Fatalf("stale accepted %v", e)
	}
	if ready, e := i.Ready(ctx, "world", 2); e != nil || !ready {
		t.Fatalf("current refused %v", e)
	}
	if _, err = i.Find(ctx, "world", 1, []string{"zboat"}, "crossing water", "stale-query"); err == nil {
		t.Fatal("stale query accepted")
	}
	// A deleted derived index is restored solely from the immutable snapshot.
	if status, _, e := i.request(ctx, "DELETE", "", nil, 65536); e != nil || status != 200 {
		t.Fatalf("reset %d %v", status, e)
	}
	if err = i.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = i.Publish(ctx, "world", 2, changed, "rebuild"); err != nil {
		t.Fatal(err)
	}
	d, err = i.Find(ctx, "world", 2, []string{"zboat", "asand"}, "crossing water", "recovered-query")
	if err != nil || len(d.Candidates) != 2 || d.Candidates[0].ID != "zboat" {
		t.Fatalf("recovered recall %+v %v", d, err)
	}
	// An externally altered mapping identity must refuse startup.
	raw, _ := json.Marshal(map[string]any{"_meta": map[string]string{"contract": "canon-vector-mean-v1", "profile": "wrong-weights"}})
	if status, _, e := i.request(ctx, "PUT", "/_mapping", raw, 65536); e != nil || status != 200 {
		t.Fatalf("mapping fixture %d %v", status, e)
	}
	if err = i.Ensure(ctx); err == nil {
		t.Fatal("profile drift accepted")
	}
}
