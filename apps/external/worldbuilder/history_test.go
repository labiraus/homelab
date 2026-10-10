package worldbuilder

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func put(r Record) []Operation { return []Operation{{Action: "put", ID: r.ID, Record: &r}} }

func TestDiffAndThreeWayConflictClassification(t *testing.T) {
	base := Snapshot{"a": {ID: "a", Kind: "source", Label: "Original"}}
	current := Snapshot{"a": {ID: "a", Kind: "source", Label: "Current"}}
	proposed := put(Record{ID: "a", Kind: "source", Label: "Proposed"})
	r, err := reconciliation(base, current, proposed)
	if err != nil || len(r.Records) != 1 || !r.Records[0].Conflict {
		t.Fatalf("expected conflict: %+v %v", r, err)
	}
	r, err = reconciliation(base, Snapshot{}, proposed)
	if err != nil || !r.Records[0].Conflict || r.Records[0].Current != nil {
		t.Fatalf("deletion conflict missing: %+v %v", r, err)
	}
	r, err = reconciliation(base, Snapshot{"a": *proposed[0].Record}, proposed)
	if err != nil || r.Records[0].Conflict {
		t.Fatalf("identical edit must not conflict: %+v %v", r, err)
	}
	d := Diff(current, base)
	if len(d) != 1 || d[0].Before.Label != "Current" || d[0].After.Label != "Original" {
		t.Fatalf("bad diff: %+v", d)
	}
}

func TestRebaseConflictsFreshReviewAndIdempotency(t *testing.T) {
	h := setup(t)
	w := h.world()
	p := "/" + w
	h.publish(w, h.draft(w, 0, fixture()))
	old := h.draft(w, 1, put(Record{ID: "hero", Kind: "entity", Type: "person", Label: "Proposed hero"}))
	h.approve(w, old)
	h.publish(w, h.draft(w, 1, put(Record{ID: "hero", Kind: "entity", Type: "person", Label: "Current hero"})))
	url := p + "/changes/" + old
	raw := h.call("editor@example.test", "GET", url+"/reconciliation", nil, 200)
	var preview Reconciliation
	if err := json.Unmarshal(raw, &preview); err != nil {
		t.Fatal(err)
	}
	if preview.BaseRevision != 1 || preview.CurrentRevision != 2 || len(preview.Records) != 1 || !preview.Records[0].Conflict {
		t.Fatalf("unexpected preview: %s", raw)
	}
	h.call("stranger@example.test", "GET", url+"/reconciliation", nil, 404)
	request := ReconcileRequest{ID: uuid.NewString(), BaseRevision: 2, Reason: "Reconcile hero"}
	h.call("reader@example.test", "POST", url+"/rebase", request, 403)
	h.call("editor@example.test", "POST", url+"/rebase", request, 409)
	request.Resolutions = map[string]string{"hero": "proposed"}
	request.BaseRevision = 1
	h.call("editor@example.test", "POST", url+"/rebase", request, 409)
	request.BaseRevision = 2
	raw = h.call("editor@example.test", "POST", url+"/rebase", request, 200)
	var rebased Change
	if err := json.Unmarshal(raw, &rebased); err != nil {
		t.Fatal(err)
	}
	if rebased.Status != "draft" || rebased.Reviewer != "" || rebased.BaseRevision != 2 {
		t.Fatalf("approval copied: %s", raw)
	}
	h.call("reviewer@example.test", "POST", p+"/changes/"+request.ID+"/publish", map[string]any{}, 409)
	h.publish(w, request.ID)
	h.call("editor@example.test", "POST", url+"/rebase", request, 200) // exact retry after publication
	request.Resolutions["hero"] = "current"
	h.call("editor@example.test", "POST", url+"/rebase", request, 409)
	h.call("reader@example.test", "GET", url+"/diff", nil, 200)
	raw = h.call("reader@example.test", "GET", p+"/canon?timeline=main&revision=2", nil, 200)
	if !bytes.Contains(raw, []byte("Current hero")) || bytes.Contains(raw, []byte("Proposed hero")) {
		t.Fatal("historical revision changed")
	}
}

func TestReconciliationRevalidatesCrossRecordChanges(t *testing.T) {
	h := setup(t)
	w := h.world()
	h.publish(w, h.draft(w, 0, fixture()))
	old := h.draft(w, 1, put(Record{ID: "new-age", Kind: "fact", Subject: "friend", Predicate: "age", ValueType: "number", Value: float64(25), Timeline: "main", Source: "source"}))
	h.publish(w, h.draft(w, 1, []Operation{{Action: "delete", ID: "edge"}, {Action: "delete", ID: "friend"}}))
	url := "/" + w + "/changes/" + old
	raw := h.call("editor@example.test", "GET", url+"/reconciliation", nil, 200)
	var r Reconciliation
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if r.ValidationError == "" {
		t.Fatal("missing deleted-reference finding")
	}
	h.call("editor@example.test", "POST", url+"/rebase", ReconcileRequest{ID: uuid.NewString(), BaseRevision: 2, Reason: "Invalid reference"}, 422)
}

func TestStoryReconciliationPrivateStateAndVersionChecks(t *testing.T) {
	h := setup(t)
	w := h.world()
	p := "/" + w
	h.publish(w, h.draft(w, 0, fixture()))
	sid := uuid.NewString()
	u := p + "/sessions/" + sid
	h.call("editor@example.test", "POST", p+"/sessions", map[string]any{"id": sid, "title": "Adventure", "pinnedRevision": 1, "timeline": "main"}, 200)
	ops := put(Record{ID: "arrival", Kind: "event", Label: "Arrival", Timeline: "main", Source: "source", Subject: "hero", From: ptr(20)})
	h.call("editor@example.test", "POST", u+"/turns", Turn{RequestID: uuid.NewString(), Narration: "Hero arrives", Operations: ops}, 200)
	h.publish(w, h.draft(w, 1, put(Record{ID: "notes", Kind: "source", Label: "New notes"})))
	h.call("owner@example.test", "GET", u+"/reconciliation", nil, 404)
	raw := h.call("editor@example.test", "GET", u+"/reconciliation", nil, 200)
	var preview Reconciliation
	if err := json.Unmarshal(raw, &preview); err != nil {
		t.Fatal(err)
	}
	if preview.ExpectedVersion != 1 || preview.CurrentRevision != 2 || preview.BaseRevision != 1 {
		t.Fatalf("bad story preview: %s", raw)
	}
	r := ReconcileRequest{ID: uuid.NewString(), BaseRevision: 2, Reason: "Arrival outcome"}
	h.call("editor@example.test", "POST", u+"/reconcile", r, 409)
	r.ExpectedVersion = 1
	h.call("editor@example.test", "POST", u+"/reconcile", r, 200)
	h.publish(w, r.ID)
	h.call("editor@example.test", "POST", u+"/turns", Turn{RequestID: uuid.NewString(), ExpectedVersion: 1, Narration: "Next scene"}, 200)
	h.call("editor@example.test", "POST", u+"/reconcile", r, 200)
	raw = h.call("editor@example.test", "GET", u, nil, 200)
	var session Session
	if err := json.Unmarshal(raw, &session); err != nil {
		t.Fatal(err)
	}
	if session.PinnedRevision != 1 || session.Version != 2 {
		t.Fatal("reconcile repinned session")
	}
}

func TestHistoryPaginationAndCompensatingRestore(t *testing.T) {
	h := setup(t)
	w := h.world()
	p := "/" + w
	h.publish(w, h.draft(w, 0, fixture()))
	h.publish(w, h.draft(w, 1, put(Record{ID: "hero", Kind: "entity", Type: "person", Label: "Changed hero"})))
	raw := h.call("reader@example.test", "GET", p+"/revisions?limit=1&offset=1", nil, 200)
	var revisions []Revision
	if err := json.Unmarshal(raw, &revisions); err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 1 || revisions[0].Revision != 1 || revisions[0].Created.IsZero() {
		t.Fatalf("bad history: %s", raw)
	}
	raw = h.call("reader@example.test", "GET", p+"/diff?from=0&to=1&limit=1&offset=1", nil, 200)
	var diff struct {
		Records []RecordDiff
		Total   int
	}
	if err := json.Unmarshal(raw, &diff); err != nil {
		t.Fatal(err)
	}
	if len(diff.Records) != 1 || diff.Total != len(fixture()) {
		t.Fatalf("bad paginated diff: %s", raw)
	}
	h.call("reader@example.test", "GET", p+"/diff?from=99", nil, 404)
	h.call("stranger@example.test", "GET", p+"/revisions", nil, 404)
	r := RestoreRequest{ID: uuid.NewString(), BaseRevision: 2, TargetRevision: 1, Reason: "Restore original label"}
	h.call("reader@example.test", "POST", p+"/restore", r, 403)
	h.call("editor@example.test", "POST", p+"/restore", r, 200)
	h.publish(w, r.ID)
	h.call("editor@example.test", "POST", p+"/restore", r, 200)
	raw = h.call("reader@example.test", "GET", p+"/diff?from=1&to=3", nil, 200)
	if err := json.Unmarshal(raw, &diff); err != nil {
		t.Fatal(err)
	}
	if len(diff.Records) != 0 || diff.Total != 0 {
		t.Fatalf("compensation mismatch: %s", raw)
	}
	raw = h.call("reader@example.test", "GET", p+"/canon?timeline=main&revision=2", nil, 200)
	if !bytes.Contains(raw, []byte("Changed hero")) {
		t.Fatal("restore rewrote history")
	}
}
