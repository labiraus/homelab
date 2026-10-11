package worldbuilder

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func largeContext(t *testing.T) Snapshot {
	t.Helper()
	s, err := Apply(Snapshot{"main": {ID: "main", Kind: "timeline", Label: "Main"}}, fixture())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 150; i++ {
		id := fmt.Sprintf("background-%03d", i)
		s[id] = Record{ID: id, Kind: "entity", Type: "person", Label: "Background character"}
	}
	return s
}

func TestSelectedContextPreservesDependenciesScopeAndLimits(t *testing.T) {
	s := largeContext(t)
	s["settings"] = Record{ID: "settings", Kind: "world_settings", Label: "Novel", Timeline: "main"}
	s["rule"] = Record{ID: "rule", Kind: "rule", Label: "Policy", Type: "person", Timeline: "main"}
	s["hidden"] = Record{ID: "hidden", Kind: "event", Label: "Hero secret", Timeline: "other"}
	sc := &scope{world: World{ID: "world", Revision: 1}}
	records, coverage, err := sc.evidenceContext(context.Background(), s, 1, "main", "Hero", nil)
	if err != nil {
		t.Fatal(err)
	}
	if coverage.Coverage != "selected_records" || coverage.CanonRecordCount <= 100 || len(records) > 100 {
		t.Fatalf("invalid scope: %+v", coverage)
	}
	ids := map[string]bool{}
	for _, r := range records {
		ids[r.ID] = true
	}
	for _, id := range []string{"main", "settings", "rule", "hero", "person", "source", "age", "edge", "friend"} {
		if !ids[id] {
			t.Errorf("missing evidence dependency %s", id)
		}
	}
	if ids["hidden"] {
		t.Fatal("other timeline entered context")
	}
	// Broad searches remain bounded; source/type closures are never split.
	records, _, err = sc.evidenceContext(context.Background(), s, 1, "main", "Background", nil)
	if err != nil || len(records) > 100 {
		t.Fatalf("broad context: %d %v", len(records), err)
	}
	for i := 0; i < 101; i++ {
		id := fmt.Sprintf("rule-%03d", i)
		s[id] = Record{ID: id, Kind: "rule", Label: "Required policy"}
	}
	if _, _, err = sc.evidenceContext(context.Background(), s, 1, "main", "Hero", nil); err == nil {
		t.Fatal("silently dropped required policy")
	}
}

func TestLargeWorldReasoningUsesSelectedCanonicalEvidence(t *testing.T) {
	h := setup(t)
	w := h.world()
	h.publish(w, h.draft(w, 0, fixture()))
	for batch := 0; batch < 2; batch++ {
		ops := []Operation{}
		for i := 0; i < 60; i++ {
			id := fmt.Sprintf("background-%03d", batch*60+i)
			r := Record{ID: id, Kind: "entity", Type: "person", Label: "Background character"}
			ops = append(ops, Operation{Action: "put", ID: id, Record: &r})
		}
		h.publish(w, h.draft(w, int64(batch+1), ops))
	}
	h.handler = Handler(&Store{Pool: h.pool, Reasoner: func(_ context.Context, r ReasonRequest) (ReasonResult, error) {
		if r.ContextScope != "selected_records" || len(r.Evidence) > 100 || r.Revision != 3 {
			t.Fatalf("invalid selected context: %+v", r)
		}
		found := false
		for _, e := range r.Evidence {
			if e.Record.ID == "hero" {
				found = true
			}
			if e.WorldID != w || e.Revision != 3 || e.Timeline != "main" {
				t.Fatal("unscoped evidence")
			}
		}
		if !found {
			t.Fatal("relevant hero absent")
		}
		return ReasonResult{Answer: "Hero exists", Citations: []string{w + ":3:main:hero"}, EvidenceScope: EvidenceScope{Coverage: "forged_complete"}}, nil
	}}, func(r *http.Request) (string, bool) { a := r.Header.Get("Test-Actor"); return a, a != "" })
	raw := h.call("reader@example.test", "POST", "/"+w+"/reason", map[string]any{"workflow": "canon_query", "question": "Hero", "timeline": "main", "revision": 3}, 200)
	var result ReasonResult
	if err := json.Unmarshal(raw, &result); err != nil || result.EvidenceScope.Coverage != "selected_records" || result.BaseRevision != 3 {
		t.Fatalf("untrusted scope accepted: %s %v", raw, err)
	}
}
