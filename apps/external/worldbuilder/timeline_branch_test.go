package worldbuilder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func branchFixture(t *testing.T) Snapshot {
	t.Helper()
	s, err := Apply(Snapshot{"main": {ID: "main", Kind: "timeline", Label: "Main"}}, fixture())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestTimelineBranchCompleteCopyReferencesAndExactTime(t *testing.T) {
	parent := branchFixture(t)
	parent["local-type"] = Record{ID: "local-type", Kind: "type", Label: "Local", Timeline: "main", Fields: map[string]string{"ally": "entity"}, Relations: map[string]string{"knows": "local-type"}}
	parent["local-entity"] = Record{ID: "local-entity", Kind: "entity", Label: "Local entity", Type: "local-type", Timeline: "main"}
	parent["local-source"] = Record{ID: "local-source", Kind: "source", Label: "Local notes", Timeline: "main"}
	parent["local-fact"] = Record{ID: "local-fact", Kind: "fact", Subject: "local-entity", Predicate: "ally", ValueType: "entity", Value: "local-entity", Timeline: "main", Source: "local-source", From: ptr(-1 << 63), Until: ptr(1<<63 - 1)}
	parent["other"] = Record{ID: "other", Kind: "timeline", Label: "Other"}
	parent["other-event"] = Record{ID: "other-event", Kind: "event", Label: "Hidden", Timeline: "other", Source: "source", From: ptr(0)}
	before, _ := json.Marshal(parent)
	request := TimelineBranchRequest{Timeline: "branch", ParentTimeline: "main", Label: "Alternate"}
	ops, err := branchOperations(parent, parent, request)
	if err != nil || len(ops) != 7 {
		t.Fatalf("complete scoped copy: %d %v", len(ops), err)
	}
	after, err := Apply(parent, ops)
	if err != nil {
		t.Fatal(err)
	}
	id := func(source string) string { return branchRecordID("branch", source) }
	fact := after[id("local-fact")]
	if fact.Subject != id("local-entity") || fact.Value != id("local-entity") || fact.Source != id("local-source") || *fact.From != -1<<63 || *fact.Until != 1<<63-1 {
		t.Fatalf("references/time changed: %+v", fact)
	}
	if after[id("local-type")].Relations["knows"] != id("local-type") || after[id("local-entity")].Type != id("local-type") || after[id("age")].Subject != "hero" {
		t.Fatal("scoped references were not remapped or shared identity was copied")
	}
	if _, ok := after[id("other-event")]; ok {
		t.Fatal("other timeline copied")
	}
	if _, ok := after[id("hero")]; ok {
		t.Fatal("world-wide catalog copied")
	}
	encoded, _ := json.Marshal(parent)
	if !bytes.Equal(before, encoded) {
		t.Fatal("copy mutated parent maps")
	}
	for _, r := range Records(after, "branch", "", ptr(10)) {
		if r.ID == "age" || r.ID == "edge" || r.ID == "other-event" {
			t.Fatalf("parent/other assertion leaked: %s", r.ID)
		}
	}
}

func TestTimelineBranchRefusesPartialCopyAndUnavailableReferences(t *testing.T) {
	s := branchFixture(t)
	r := TimelineBranchRequest{Timeline: "branch", ParentTimeline: "main", Label: "Alternate"}
	for i := 0; i < 98; i++ {
		id := fmt.Sprintf("event-%d", i)
		s[id] = Record{ID: id, Kind: "event", Timeline: "main", Source: "source", Label: "Event", From: ptr(int64(i))}
	}
	if ops, err := branchOperations(s, s, r); err == nil || ops != nil {
		t.Fatal("oversized branch partially copied")
	}
	delete(s, "event-97")
	if ops, err := branchOperations(s, s, r); err != nil || len(ops) != 100 {
		t.Fatalf("exact boundary: %d %v", len(ops), err)
	}
	parent := branchFixture(t)
	current := branchFixture(t)
	delete(current, "source")
	if _, err := branchOperations(parent, current, r); err == nil {
		t.Fatal("historical missing source silently replaced")
	}
	current = branchFixture(t)
	current["hidden"] = Record{ID: "hidden", Kind: "timeline", Label: "Hidden"}
	source := current["source"]
	source.Timeline = "hidden"
	current["source"] = source
	if _, err := branchOperations(parent, current, r); err == nil {
		t.Fatal("hidden reference accepted")
	}
	current = branchFixture(t)
	current[branchRecordID("branch", "age")] = Record{ID: branchRecordID("branch", "age"), Kind: "source", Label: "Collision"}
	if _, err := branchOperations(parent, current, r); err == nil {
		t.Fatal("existing record overwritten")
	}
	parent["nested"] = Record{ID: "nested", Kind: "timeline", Label: "Scoped descriptor", Timeline: "main"}
	if _, err := branchOperations(parent, parent, r); err == nil {
		t.Fatal("scoped timeline descriptor silently omitted")
	}
	parent = branchFixture(t)
	parent["local-type"] = Record{ID: "local-type", Kind: "type", Label: "Local type", Timeline: "main", Fields: map[string]string{"birth": "number", "death": "number"}}
	parent["shared-rule"] = Record{ID: "shared-rule", Kind: "rule", Label: "Shared life bounds", Type: "local-type", Attributes: map[string]string{"check": "life_bounds", "birth": "birth", "death": "death"}}
	if err := Validate(parent); err != nil {
		t.Fatal(err)
	}
	if _, err := branchOperations(parent, parent, r); err == nil {
		t.Fatal("copy detached the shared rule from its original scoped type")
	}
	rule := parent["shared-rule"]
	rule.Timeline = "main"
	parent[rule.ID] = rule
	ops, err := branchOperations(parent, parent, r)
	if err != nil {
		t.Fatalf("explicitly scoped policy copy: %v", err)
	}
	after, err := Apply(parent, ops)
	if err != nil || after[branchRecordID("branch", rule.ID)].Type != branchRecordID("branch", "local-type") {
		t.Fatalf("copied rule lost its typed scope: %v", err)
	}
}

func TestTimelineBranchReviewedPublicationHistoryAndRetries(t *testing.T) {
	h := setup(t)
	w := h.world()
	p := "/" + w
	h.publish(w, h.draft(w, 0, fixture()))
	r := TimelineBranchRequest{ID: strings.ToUpper(uuid.NewString()), BaseRevision: ptr(1), ParentRevision: ptr(1), ParentTimeline: "main", Timeline: "alternate", Label: "Alternate history", Reason: "Explore a different history"}
	h.call("reader@example.test", "POST", p+"/timeline-branches", r, 403)
	h.call("stranger@example.test", "POST", p+"/timeline-branches", r, 404)
	r.BaseRevision = nil
	h.call("editor@example.test", "POST", p+"/timeline-branches", r, 422)
	r.BaseRevision = ptr(1)
	r.ParentRevision = nil
	h.call("editor@example.test", "POST", p+"/timeline-branches", r, 422)
	r.ParentRevision = ptr(1)
	raw := h.call("editor@example.test", "POST", p+"/timeline-branches", r, 200)
	var c Change
	if err := json.Unmarshal(raw, &c); err != nil || c.Status != "draft" || len(c.Operations) != 3 || c.ID != strings.ToLower(r.ID) {
		t.Fatalf("not a complete draft: %s %v", raw, err)
	}
	h.call("editor@example.test", "POST", p+"/timeline-branches", r, 200)
	h.call("reader@example.test", "GET", p+"/timelines/alternate/origin", nil, 404)
	h.call("reviewer@example.test", "POST", p+"/changes/"+r.ID+"/publish", map[string]any{}, 409)
	var count int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM worldbuilder.audit WHERE world_id=$1 AND action='timeline.branch'`, w).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate audit: %d %v", count, err)
	}
	h.publish(w, r.ID)
	h.call("editor@example.test", "POST", p+"/timeline-branches", r, 200)
	h.call("owner@example.test", "POST", p+"/timeline-branches", r, 409)
	changed := r
	changed.Label = "Changed retry"
	h.call("editor@example.test", "POST", p+"/timeline-branches", changed, 409)
	var origin TimelineBranchOrigin
	raw = h.call("reader@example.test", "GET", p+"/timelines/alternate/origin?revision=2", nil, 200)
	if err := json.Unmarshal(raw, &origin); err != nil || origin.PublishedRevision != 2 || len(origin.Records) != 2 || origin.Records["age"] != branchRecordID("alternate", "age") {
		t.Fatalf("origin: %s %v", raw, err)
	}
	h.call("reader@example.test", "GET", p+"/timelines/alternate/origin?revision=1", nil, 404)
	h.call("stranger@example.test", "GET", p+"/timelines/alternate/origin", nil, 404)
	if raw := h.call("reader@example.test", "GET", p+"/timelines/main/origin", nil, 200); string(bytes.TrimSpace(raw)) != "null" {
		t.Fatalf("invented parent: %s", raw)
	}
	age := *fixture()[4].Record
	age.Value = float64(30)
	h.publish(w, h.draft(w, 2, put(age)))
	for _, check := range []struct{ timeline, value string }{{"main", "30"}, {"alternate", "20"}} {
		raw = h.call("reader@example.test", "GET", p+"/search?timeline="+check.timeline+"&q=age&revision=3&at=10", nil, 200)
		var result SearchResult
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, citation := range result.Citations {
			if citation.Record.Kind != "fact" {
				continue
			}
			found = true
			value, _ := json.Marshal(citation.Record.Value)
			if string(value) != check.value || citation.Timeline != check.timeline || citation.Revision != 3 {
				t.Fatalf("branch changed or citation escaped scope: %+v", citation)
			}
		}
		if !found {
			t.Fatalf("missing branch evidence: %s", raw)
		}
	}
	// Historical parent creation still uses its pinned assertions against the
	// current shared catalog, and stale destination revisions cannot be promoted.
	r.ID, r.Timeline, r.BaseRevision = uuid.NewString(), "past", ptr(2)
	h.call("editor@example.test", "POST", p+"/timeline-branches", r, 409)
	r.BaseRevision = ptr(3)
	h.call("editor@example.test", "POST", p+"/timeline-branches", r, 200)
	h.publish(w, r.ID)
	raw = h.call("reader@example.test", "GET", p+"/records/"+branchRecordID("past", "age")+"?timeline=past&revision=4&at=10", nil, 200)
	if !bytes.Contains(raw, []byte(`"value":20`)) {
		t.Fatalf("historical parent was replaced: %s", raw)
	}
	// Branch copies diverge independently, while the explicit world-wide catalog
	// remains shared and older snapshots retain the original presentation.
	branchAge := age
	branchAge.ID, branchAge.Timeline, branchAge.Value = branchRecordID("alternate", "age"), "alternate", float64(22)
	sharedHero := Record{ID: "hero", Kind: "entity", Type: "person", Label: "Shared renamed hero"}
	h.publish(w, h.draft(w, 4, append(put(branchAge), put(sharedHero)...)))
	for _, check := range []struct{ timeline, id, value string }{{"main", "age", "30"}, {"alternate", branchAge.ID, "22"}, {"past", branchRecordID("past", "age"), "20"}} {
		raw = h.call("reader@example.test", "GET", p+"/records/"+check.id+"?timeline="+check.timeline+"&revision=5&at=10", nil, 200)
		if !bytes.Contains(raw, []byte(`"value":`+check.value)) {
			t.Fatalf("branch edit leaked: %s", raw)
		}
		raw = h.call("reader@example.test", "GET", p+"/records/hero?timeline="+check.timeline+"&revision=5", nil, 200)
		if !bytes.Contains(raw, []byte("Shared renamed hero")) {
			t.Fatalf("catalog was silently frozen: %s", raw)
		}
	}
	raw = h.call("reader@example.test", "GET", p+"/records/hero?timeline=main&revision=1", nil, 200)
	if bytes.Contains(raw, []byte("Shared renamed hero")) {
		t.Fatalf("shared edit rewrote history: %s", raw)
	}
}

func TestTimelineBranchStaleDraftRetiredIdentityAndAuditRollback(t *testing.T) {
	h := setup(t)
	w := h.world()
	p := "/" + w
	h.publish(w, h.draft(w, 0, fixture()))
	r := TimelineBranchRequest{ID: uuid.NewString(), BaseRevision: ptr(1), ParentRevision: ptr(1), ParentTimeline: "main", Timeline: "branch", Label: "Branch", Reason: "Branch history"}
	h.call("editor@example.test", "POST", p+"/timeline-branches", r, 200)
	h.approve(w, r.ID)
	h.publish(w, h.draft(w, 1, put(Record{ID: "notes", Kind: "source", Label: "More notes"})))
	h.call("reviewer@example.test", "POST", p+"/changes/"+r.ID+"/publish", map[string]any{}, 409)
	h.call("editor@example.test", "GET", p+"/changes/"+strings.ToUpper(r.ID)+"/reconciliation", nil, 422)
	h.call("editor@example.test", "POST", p+"/changes/"+strings.ToUpper(r.ID)+"/rebase", ReconcileRequest{ID: uuid.NewString(), BaseRevision: 2, Reason: "Omit copied facts", Resolutions: map[string]string{branchRecordID("branch", "age"): "current"}}, 422)
	r.ID, r.BaseRevision = uuid.NewString(), ptr(2)
	h.call("editor@example.test", "POST", p+"/timeline-branches", r, 200)
	h.publish(w, r.ID)
	// A removed timeline cannot acquire a second, contradictory creation origin.
	h.publish(w, h.draft(w, 3, []Operation{{Action: "delete", ID: branchRecordID("branch", "age")}, {Action: "delete", ID: branchRecordID("branch", "edge")}, {Action: "delete", ID: "branch"}}))
	r.ID, r.BaseRevision = uuid.NewString(), ptr(4)
	h.call("editor@example.test", "POST", p+"/timeline-branches", r, 409)
	r.Timeline = "audit-failure"
	ctx := context.Background()
	if _, err := h.pool.Exec(ctx, `CREATE FUNCTION worldbuilder.test_branch_audit_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='timeline.branch' AND NEW.world_id::text=TG_ARGV[0] THEN RAISE EXCEPTION 'injected branch audit failure'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, `CREATE TRIGGER test_branch_audit_fail BEFORE INSERT ON worldbuilder.audit FOR EACH ROW EXECUTE FUNCTION worldbuilder.test_branch_audit_fail('`+w+`')`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		h.pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS test_branch_audit_fail ON worldbuilder.audit; DROP FUNCTION IF EXISTS worldbuilder.test_branch_audit_fail()`)
	})
	h.call("editor@example.test", "POST", p+"/timeline-branches", r, 500)
	var count int
	if err := h.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM worldbuilder.change_set WHERE world_id=$1 AND id=$2)+(SELECT count(*) FROM worldbuilder.audit WHERE world_id=$1 AND resource=$2::text)`, w, r.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("audit failure left partial draft: %d %v", count, err)
	}
}
