package worldbuilder

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestAdvisoryFindingsEvidenceAndBounds(t *testing.T) {
	valid := AdvisoryFinding{Severity: "warning", Uncertainty: "high", Message: "Possible contradiction", Citations: []string{"world:1:main:hero"}}
	allowed := map[string]bool{"world:1:main:hero": true}
	if err := validateFindings("consistency_check", []AdvisoryFinding{valid}, allowed); err != nil {
		t.Fatal(err)
	}
	cases := []AdvisoryFinding{valid, valid, valid, valid}
	cases[0].Citations = []string{"other:1:main:hero"}
	cases[1].Uncertainty = "certain"
	cases[2].Severity = "approved"
	cases[3].Citations = nil
	for _, c := range cases {
		if validateFindings("consistency_check", []AdvisoryFinding{c}, allowed) == nil {
			t.Fatalf("accepted invalid finding: %+v", c)
		}
	}
	if validateFindings("canon_query", []AdvisoryFinding{valid}, allowed) == nil {
		t.Fatal("findings escaped workflow")
	}
}
func TestAdvisoryProposalCheckValidatesBeforeInference(t *testing.T) {
	h := setup(t)
	w := h.world()
	h.publish(w, h.draft(w, 0, fixture()))
	ops := fixture()
	proposal := h.draft(w, 1, ops)
	calls := 0
	h.handler = Handler(&Store{Pool: h.pool, Reasoner: func(_ context.Context, r ReasonRequest) (ReasonResult, error) {
		calls++
		if r.Review == nil || r.Review.ChangeID != proposal || r.Review.BaseRevision != 1 {
			t.Fatal("lost proposal review scope")
		}
		return ReasonResult{Answer: "Advisory review", Unknown: true, Findings: []AdvisoryFinding{{Severity: "info", Uncertainty: "high", Message: "Check the hero's motive", Citations: []string{w + ":1:main:hero"}}}}, nil
	}}, func(r *http.Request) (string, bool) { a := r.Header.Get("Test-Actor"); return a, a != "" })
	b := ReasonInput{Workflow: "consistency_check", ChangeID: proposal, Revision: 1, Timeline: "main", Question: "Review"}
	var result ReasonResult
	if err := json.Unmarshal(h.call("reviewer@example.test", "POST", "/"+w+"/reason", b, 200), &result); err != nil {
		t.Fatal(err)
	}
	if result.Deterministic == nil || !result.Deterministic.Passed || result.Deterministic.ChangeID != proposal || len(result.Findings) != 1 {
		t.Fatal("lost validated review metadata")
	}
	var status string
	if err := h.pool.QueryRow(context.Background(), `SELECT status FROM worldbuilder.change_set WHERE world_id=$1 AND id=$2`, w, proposal).Scan(&status); err != nil || status != "draft" {
		t.Fatal("advisory altered human review state")
	}
	h.publish(w, h.draft(w, 1, fixture()))
	h.call("reviewer@example.test", "POST", "/"+w+"/reason", b, 409)
	if calls != 1 {
		t.Fatal("stale proposal spent inference")
	}
	b.ChangeID = "00000000-0000-4000-8000-000000000099"
	h.call("reviewer@example.test", "POST", "/"+w+"/reason", b, 404)
	if calls != 1 {
		t.Fatal("unknown proposal spent inference")
	}
}
