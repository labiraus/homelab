package canonsearch

import (
	"context"
	"math"
	"net/url"
	"os"
	"strings"
	"testing"
)

// Opt-in real CPU smoke, separate from the synthetic integration fixture.
// Call only through an owned loopback bridge to the pinned model container.
func TestPinnedCPUModelRepresentation(t *testing.T) {
	endpoint := os.Getenv("WORLDBUILDER_TEST_EMBEDDING_URL")
	if endpoint == "" {
		t.Skip("owned pinned CPU model endpoint required")
	}
	target, err := url.Parse(endpoint)
	if err != nil || target.Hostname() != "127.0.0.1" {
		t.Fatal("owned loopback model endpoint required")
	}
	e := &Embedder{Endpoint: endpoint, Profile: Profile{Model: "BAAI/bge-large-en-v1.5", Revision: "d4aa6901d3a41ba39fb536a557fa166f842b0e09", Dimensions: 1024, QueryPrefix: "Represent this sentence for searching relevant passages: "}}
	texts := []string{
		`{"id":"mara","kind":"entity","type":"person","label":"Mara","attributes":{"description":"Mara pilots a ferry carrying villagers between the two banks of the river."}}`,
		`{"id":"sera","kind":"entity","type":"person","label":"Sera","attributes":{"description":"Sera maintains an isolated desert outpost surrounded by sand dunes."}}`,
		`{"id":"ilan","kind":"entity","type":"person","label":"Ilan","attributes":{"description":"Ilan is a blacksmith who forges iron swords and restores old weapons in his workshop."}}`,
		`{"id":"nera","kind":"entity","type":"person","label":"Nera","attributes":{"description":"Nera is a physician who treats injuries and tends to wounded villagers at the infirmary."}}`,
		`{"id":"rin","kind":"entity","type":"person","label":"Rin","attributes":{"description":"Rin harvests apples and pears from trees in the village orchard."}}`,
		`{"id":"oren","kind":"entity","type":"person","label":"Oren","attributes":{"description":"Oren is a historian who deciphers ancient inscriptions and studies forgotten languages."}}`,
		`{"id":"person","kind":"type","label":"Person"}`,
		`{"id":"main","kind":"timeline","label":"Main"}`,
		`{"id":"settings","kind":"world_settings","label":"River village","timeline":"main"}`,
	}
	vectors, usage, err := e.Texts(context.Background(), texts, "cpu-quality-documents", false)
	if err != nil || usage.Requests != 2 || usage.InputTokens <= 0 {
		t.Fatalf("real CPU document contract: %+v %v", usage, err)
	}
	cases := []struct {
		query string
		want  int
	}{
		{"Who helps people cross the water?", 0},
		{"Where can I get a damaged blade repaired?", 2},
		{"Who can help heal a broken arm?", 3},
		{"Who would know about fruit picking?", 4},
	}
	for _, c := range cases {
		v, u, err := e.Mean(context.Background(), c.query, "cpu-quality-query", true)
		if err != nil || u.Requests != 1 || u.InputTokens <= 0 {
			t.Fatalf("real CPU query contract: %+v %v", u, err)
		}
		best, score, runner := -1, math.Inf(-1), math.Inf(-1)
		for n, d := range vectors {
			var sim float64
			for j, x := range v {
				sim += float64(x) * float64(d[j])
			}
			if sim > score {
				runner = score
				best = n
				score = sim
			} else if sim > runner {
				runner = sim
			}
		}
		t.Logf("query=%q best=%d expected=%d cosine=%.4f margin=%.4f", c.query, best, c.want, score, score-runner)
		// This is a top-match smoke, not a calibrated confidence threshold.
		// Retain the observed margins; cosine gaps do not establish factual certainty.
		if best != c.want || score <= runner {
			t.Fatalf("real model relevance smoke failed: best=%d margin=%.4f", best, score-runner)
		}
	}
	batch := make([]string, 32)
	for n := range batch {
		batch[n] = texts[n%len(texts)]
	}
	values, u, err := e.Texts(context.Background(), batch, "cpu-quality-record-batch", false)
	if err != nil || len(values) != 32 || u.Requests != 4 {
		t.Fatalf("real 32-record/eight-input batch: %d %+v %v", len(values), u, err)
	}
	if _, u, err = e.Mean(context.Background(), strings.Repeat("Rain 雨🌍. ", 100), "cpu-quality-unicode", false); err != nil || u.InputTokens <= 0 {
		t.Fatalf("real UTF-8 chunk coverage: %+v %v", u, err)
	}
}
