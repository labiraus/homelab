package worldbuilder

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func policySettings(n int) Record {
	return Record{ID: "world_settings", Kind: "world_settings", Label: "Test world", Timeline: "main", ReviewPolicy: &ReviewPolicy{RequiredApprovals: n}}
}
func decodeChange(t *testing.T, raw []byte) Change {
	t.Helper()
	var c Change
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	return c
}
func reviewBody(decision, reason string) map[string]string {
	return map[string]string{"decision": decision, "reason": reason}
}
func twoReviewerWorld(t *testing.T, h *harness) string {
	t.Helper()
	w := h.world()
	c := h.draft(w, 0, put(policySettings(2)))
	h.approve(w, c)
	h.call("owner@example.test", "POST", "/"+w+"/changes/"+c+"/review", reviewBody("approved", "Policy independently checked"), 200)
	h.call("owner@example.test", "POST", "/"+w+"/changes/"+c+"/publish", map[string]any{}, 200)
	return w
}

func TestReviewPolicyValidation(t *testing.T) {
	for _, n := range []int{0, -1, 6} {
		if _, err := Apply(Snapshot{"main": {ID: "main", Kind: "timeline", Label: "Main"}}, put(policySettings(n))); err == nil {
			t.Fatal("invalid quorum accepted", n)
		}
	}
	r := policySettings(2)
	r.Kind = "timeline"
	if Validate(Snapshot{r.ID: r}) == nil {
		t.Fatal("review policy outside settings")
	}
	for _, pair := range [][2]int{{1, 5}, {5, 1}, {2, 3}} {
		before, after := Snapshot{"world_settings": policySettings(pair[0])}, Snapshot{"world_settings": policySettings(pair[1])}
		want := pair[0]
		if pair[1] > want {
			want = pair[1]
		}
		if requiredReviews(before, after) != want {
			t.Fatal("weakened policy transition")
		}
	}
	if snapshotReviewPolicy(Snapshot{}).RequiredApprovals != 1 {
		t.Fatal("legacy policy changed")
	}
}

func TestReviewQuorumPublicationAndGovernedPolicyChanges(t *testing.T) {
	h := setup(t)
	w := h.world()
	p := "/" + w
	id := h.draft(w, 0, put(policySettings(2)))
	route := p + "/changes/" + id
	h.call("editor@example.test", "POST", route+"/submit", map[string]any{}, 200)
	h.call("editor@example.test", "POST", route+"/review", reviewBody("approved", "self"), 403)
	first := h.call("reviewer@example.test", "POST", route+"/review", reviewBody("approved", "checked"), 200)
	c := decodeChange(t, first)
	if c.Status != "ready_for_review" || c.RequiredApprovals != 2 || len(c.Approvals) != 1 {
		t.Fatalf("first approval bypassed quorum: %+v", c)
	}
	h.call("owner@example.test", "POST", route+"/publish", map[string]any{}, 409)
	again := decodeChange(t, h.call("reviewer@example.test", "POST", route+"/review", reviewBody("approved", "checked"), 200))
	if len(again.Reviews) != 1 || len(again.Approvals) != 1 {
		t.Fatal("retry counted twice")
	}
	h.call("reviewer@example.test", "POST", route+"/review", reviewBody("approved", "rewrite reason"), 409)
	h.call("owner@example.test", "POST", route+"/review", reviewBody("approved", "checked independently"), 200)
	c = decodeChange(t, h.call("owner@example.test", "POST", route+"/publish", map[string]any{}, 200))
	if c.Status != "published" || len(c.Approvals) != 2 || len(c.Reviews) != 2 {
		t.Fatal("publication lost distinct decisions")
	}
	h.call("reviewer@example.test", "POST", route+"/review", reviewBody("approved", "checked"), 200) // exact retry after publication
	// Both old and new policies constrain the transition, including removal.
	lower := h.draft(w, 1, put(policySettings(1)))
	lowerRoute := p + "/changes/" + lower
	h.approve(w, lower)
	h.call("owner@example.test", "POST", lowerRoute+"/publish", map[string]any{}, 409)
	h.call("owner@example.test", "POST", lowerRoute+"/review", reviewBody("approved", "Lower quorum after review"), 200)
	h.call("owner@example.test", "POST", lowerRoute+"/publish", map[string]any{}, 200)
	for query, want := range map[string]int{"/canon?timeline=main&revision=0": 1, "/canon?timeline=main&revision=1&kind=entity": 2, "/search?timeline=main&q=nomatch&revision=1": 2, "/canon?timeline=main": 1} {
		var result SearchResult
		if err := json.Unmarshal(h.call("reader@example.test", "GET", p+query, nil, 200), &result); err != nil || result.ReviewPolicy.RequiredApprovals != want {
			t.Fatalf("policy scope %s: %+v %v", query, result, err)
		}
	}
	for _, sql := range []string{
		`UPDATE worldbuilder.change_review SET reason='rewritten' WHERE world_id=$1`,
		`DELETE FROM worldbuilder.change_review WHERE world_id=$1`,
		`UPDATE worldbuilder.change_set SET required_approvals=1 WHERE world_id=$1 AND required_approvals=2`,
		`UPDATE worldbuilder.change_set SET published_approvals='{}' WHERE world_id=$1 AND status='published'`,
		`INSERT INTO worldbuilder.change_review(world_id,change_id,reviewer,authority_id,decision,reason) SELECT world_id,id,author,gen_random_uuid(),'approved','self approval' FROM worldbuilder.change_set WHERE world_id=$1 LIMIT 1`,
	} {
		if _, err := h.pool.Exec(context.Background(), sql, w); err == nil {
			t.Fatal("immutable/independent database guard bypassed", sql)
		}
	}
}

func TestReviewRevocationRegrantAndHistoricalPublication(t *testing.T) {
	h := setup(t)
	w := twoReviewerWorld(t, h)
	p := "/" + w
	id := h.draft(w, 1, fixture())
	route := p + "/changes/" + id
	h.approve(w, id)
	h.call("owner@example.test", "POST", route+"/review", reviewBody("approved", "checked"), 200)
	setRole := func(role string) {
		h.call("owner@example.test", "POST", p+"/members", map[string]string{"principal": "reviewer@example.test", "role": role}, 200)
	}
	setRole("reader")
	setRole("reviewer")
	c := decodeChange(t, h.call("reader@example.test", "GET", route, nil, 200))
	if len(c.Approvals) != 1 {
		t.Fatal("restored role resurrected approval")
	}
	h.call("owner@example.test", "POST", route+"/publish", map[string]any{}, 409)
	h.call("reviewer@example.test", "POST", route+"/review", reviewBody("approved", "fresh review"), 200)
	setRole("reviewer") // no actual role change must not invalidate the fresh review
	c = decodeChange(t, h.call("owner@example.test", "POST", route+"/publish", map[string]any{}, 200))
	if len(c.Reviews) != 3 || len(c.Approvals) != 2 {
		t.Fatal("fresh authority/history missing")
	}
	setRole("reader")
	c = decodeChange(t, h.call("reader@example.test", "GET", route, nil, 200))
	if len(c.Approvals) != 2 {
		t.Fatal("published decision changed after membership change")
	}
	// Deletion and recreation also acquire a new authority, even with the same role.
	setRole("reviewer")
	id = h.draft(w, 2, put(Record{ID: "extra", Kind: "source", Label: "Extra"}))
	route = p + "/changes/" + id
	h.approve(w, id)
	var world World
	json.Unmarshal(h.call("owner@example.test", "GET", p, nil, 200), &world)
	h.call("owner@example.test", "POST", p+"/administration", AdminRequest{RequestID: uuid.NewString(), ExpectedVersion: world.AdminVersion, Action: "remove_member", Principal: "reviewer@example.test", Reason: "Revoke"}, 200)
	setRole("reviewer")
	c = decodeChange(t, h.call("owner@example.test", "GET", route, nil, 200))
	if len(c.Approvals) != 0 {
		t.Fatal("membership recreation revived old approval")
	}
}

func TestConcurrentReviewStaleCanonAndRejection(t *testing.T) {
	h := setup(t)
	w := twoReviewerWorld(t, h)
	p := "/" + w
	id := h.draft(w, 1, fixture())
	route := p + "/changes/" + id
	h.call("editor@example.test", "POST", route+"/submit", map[string]any{}, 200)
	var wg sync.WaitGroup
	results := make(chan int, 3)
	for _, actor := range []string{"owner@example.test", "reviewer@example.test", "reviewer@example.test"} {
		wg.Add(1)
		go func(actor string) {
			defer wg.Done()
			code, _ := h.request(actor, "POST", route+"/review", reviewBody("approved", "concurrent"))
			results <- code
		}(actor)
	}
	wg.Wait()
	close(results)
	for code := range results {
		if code != 200 {
			t.Fatal("concurrent review", code)
		}
	}
	c := decodeChange(t, h.call("owner@example.test", "GET", route, nil, 200))
	if c.Status != "approved" || len(c.Reviews) != 2 {
		t.Fatal("duplicate or lost concurrent decision")
	}
	stale := h.draft(w, 1, put(Record{ID: "stale", Kind: "source", Label: "Stale"}))
	h.approve(w, stale)
	h.call("owner@example.test", "POST", route+"/publish", map[string]any{}, 200)
	h.call("owner@example.test", "POST", p+"/changes/"+stale+"/review", reviewBody("approved", "stale"), 409)
	denied := h.draft(w, 2, put(Record{ID: "rejected", Kind: "source", Label: "Rejected"}))
	h.approve(w, denied)
	h.call("owner@example.test", "POST", p+"/changes/"+denied+"/review", reviewBody("rejected", "requires changes"), 200)
	h.call("reviewer@example.test", "POST", p+"/changes/"+denied+"/publish", map[string]any{}, 409)
	h.call("owner@example.test", "POST", p+"/changes/"+denied+"/review", reviewBody("approved", "changed my mind"), 409)
}

func TestReviewRejectionAndPublicationSerialize(t *testing.T) {
	h := setup(t)
	w := twoReviewerWorld(t, h)
	id := h.draft(w, 1, fixture())
	route := "/" + w + "/changes/" + id
	h.approve(w, id)
	h.call("owner@example.test", "POST", route+"/review", reviewBody("approved", "checked"), 200)
	h.call("owner@example.test", "POST", "/"+w+"/members", map[string]string{"principal": "third@example.test", "role": "reviewer"}, 200)
	foreign := h.world()
	h.call("reviewer@example.test", "POST", "/"+foreign+"/changes/"+id+"/review", reviewBody("approved", "foreign change"), 404)
	type result struct {
		action string
		code   int
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	go func() {
		<-start
		code, _ := h.request("third@example.test", "POST", route+"/review", reviewBody("rejected", "New concern"))
		results <- result{"reject", code}
	}()
	go func() {
		<-start
		code, _ := h.request("owner@example.test", "POST", route+"/publish", map[string]any{})
		results <- result{"publish", code}
	}()
	close(start)
	a, b := <-results, <-results
	if !((a.code == 200 && b.code == 409) || (a.code == 409 && b.code == 200)) {
		t.Fatalf("rejection/publication both committed or lost: %+v %+v", a, b)
	}
	c := decodeChange(t, h.call("owner@example.test", "GET", route, nil, 200))
	if c.Status != "published" && c.Status != "rejected" {
		t.Fatal("nonterminal race outcome", c.Status)
	}
	// An earlier approval retry cannot undo whichever terminal action won.
	c = decodeChange(t, h.call("owner@example.test", "POST", route+"/review", reviewBody("approved", "checked"), 200))
	if c.Status != "published" && c.Status != "rejected" {
		t.Fatal("approval retry changed terminal outcome")
	}
}
