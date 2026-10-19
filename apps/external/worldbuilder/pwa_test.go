package worldbuilder

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func setTestPolicy(t *testing.T, h *harness, w, mode string, n int) {
	t.Helper()
	actor := "policy-" + uuid.NewString() + "@example.test"
	if _, err := h.pool.Exec(context.Background(), `INSERT INTO worldbuilder.superuser(principal,granted_by) VALUES($1,'fixture')`, actor); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		h.pool.Exec(context.Background(), `DELETE FROM worldbuilder.superuser WHERE principal=$1`, actor)
	})
	var world World
	json.Unmarshal(h.call(actor, "GET", "/"+w, nil, 200), &world)
	h.call(actor, "POST", "/"+w+"/administration", AdminRequest{RequestID: uuid.NewString(), ExpectedVersion: world.AdminVersion, Action: "set_policy", Policy: &PublicationPolicy{Mode: mode, RequiredApprovals: n}, Reason: "Policy fixture"}, 200)
}
func TestCollectivePolicyMatrix(t *testing.T) {
	for _, mode := range []string{"independent", "direct", "unanimous"} {
		t.Run(mode, func(t *testing.T) {
			h := setup(t)
			w := h.world()
			p := "/" + w
			setTestPolicy(t, h, w, mode, 1)
			id := uuid.NewString()
			h.call("reader@example.test", "POST", p+"/changes", Change{ID: id, Reason: "Reader suggestion", Operations: fixture()}, 200)
			route := p + "/changes/" + id
			h.call("reader@example.test", "POST", route+"/submit", map[string]any{}, 200)
			h.call("reader@example.test", "POST", route+"/review", reviewBody("approved", "Reader cannot approve"), 403)
			h.call("reader@example.test", "POST", route+"/publish", map[string]any{}, 403)
			if mode != "direct" {
				h.call("owner@example.test", "POST", route+"/publish", map[string]any{}, 409)
				h.call("reviewer@example.test", "POST", route+"/review", reviewBody("approved", "Reviewed"), 200)
			}
			if mode == "unanimous" {
				h.call("owner@example.test", "POST", route+"/publish", map[string]any{}, 409)
				for _, actor := range []string{"owner@example.test", "editor@example.test"} {
					h.call(actor, "POST", route+"/review", reviewBody("approved", "Endorse"), 200)
				}
			}
			h.call("editor@example.test", "POST", route+"/publish", map[string]any{}, 200)
			id = h.draft(w, 1, put(Record{ID: "next", Kind: "source", Label: "Next"}))
			h.call("editor@example.test", "POST", p+"/changes/"+id+"/submit", map[string]any{}, 200)
			want := 403
			if mode == "unanimous" {
				want = 200
			}
			h.call("editor@example.test", "POST", p+"/changes/"+id+"/review", reviewBody("approved", "Own endorsement"), want)
		})
	}
}
func TestFinalAuthorAndPolicyAuthority(t *testing.T) {
	h := setup(t)
	w := h.world()
	p := "/" + w
	for _, principal := range []string{"editor@example.test", "reviewer@example.test"} {
		h.call("owner@example.test", "POST", p+"/members", map[string]string{"principal": principal, "role": "reader"}, 200)
	}
	h.call("owner@example.test", "POST", p+"/members", map[string]string{"principal": "owner@example.test", "role": "reader"}, 422)
	var current World
	json.Unmarshal(h.call("owner@example.test", "GET", p, nil, 200), &current)
	h.call("owner@example.test", "POST", p+"/administration", AdminRequest{RequestID: uuid.NewString(), ExpectedVersion: current.AdminVersion, Action: "remove_member", Principal: "owner@example.test", Reason: "Last"}, 422)
	h.call("owner@example.test", "POST", p+"/administration", AdminRequest{RequestID: uuid.NewString(), ExpectedVersion: current.AdminVersion, Action: "set_policy", Policy: &PublicationPolicy{Mode: "direct", RequiredApprovals: 1}, Reason: "Bypass"}, 403)
}
func TestEstablishedLedgerSupersessionAndHistory(t *testing.T) {
	h, _, admin := superuserFixture(t)
	w := h.world()
	p := "/" + w
	account := Record{ID: "witness", Kind: "established_account", Label: "Bridge collapse", Timeline: "main", Synopsis: "The party witnessed the western bridge collapse.", References: []string{"report"}}
	report := Record{ID: "report", Kind: "source", Label: "An uncertain history", EvidenceStatus: "reported", Attributes: map[string]string{"description": "The bridge never collapsed."}}
	h.publish(w, h.draft(w, 0, append(put(report), put(account)...)))
	for _, actor := range []string{"editor@example.test", admin} {
		changed := account
		changed.Synopsis = "Rewrite"
		for _, ops := range [][]Operation{put(changed), {{Action: "delete", ID: "witness"}}} {
			h.call(actor, "POST", p+"/changes", Change{ID: uuid.NewString(), BaseRevision: 1, Reason: "Overwrite", Operations: ops}, 422)
		}
	}
	for _, sql := range []string{`UPDATE worldbuilder.established_account SET record='{}' WHERE world_id=$1`, `DELETE FROM worldbuilder.established_account WHERE world_id=$1`, `INSERT INTO worldbuilder.revision(world_id,revision,snapshot,actor,reason) VALUES($1,99,'{}','test','restore bypass')`} {
		if _, err := h.pool.Exec(context.Background(), sql, w); err == nil {
			t.Fatal("ledger bypass", sql)
		}
	}
	clarification := Record{ID: "clarification", Kind: "established_account", Label: "Cause unknown", Timeline: "main", Synopsis: "Witnessing the collapse does not establish its cause.", Clarifies: account.ID}
	h.publish(w, h.draft(w, 1, put(clarification)))
	successor := account
	successor.ID = "corrected"
	successor.Supersedes = account.ID
	successor.Synopsis = "The party witnessed the western bridge's central span collapse."
	first := h.draft(w, 2, put(successor))
	successor.ID = "concurrent"
	second := h.draft(w, 2, put(successor))
	h.approve(w, first)
	h.approve(w, second)
	h.call("owner@example.test", "POST", p+"/changes/"+first+"/publish", map[string]any{}, 200)
	h.call(admin, "POST", p+"/changes/"+second+"/publish", map[string]string{"overrideReason": "Cannot bypass stale revision"}, 409)
	h.call("editor@example.test", "POST", p+"/changes", Change{ID: uuid.NewString(), BaseRevision: 3, Reason: "Second successor", Operations: put(successor)}, 422)
	historical := h.call("reader@example.test", "GET", p+"/established-accounts?timeline=main&revision=1", nil, 200)
	if bytes.Contains(historical, []byte("corrected")) || !bytes.Contains(historical, []byte("Bridge collapse")) {
		t.Fatal("historical authority changed", string(historical))
	}
	current := h.call("reader@example.test", "GET", p+"/established-accounts?timeline=main", nil, 200)
	if !bytes.Contains(current, []byte(`"witness":"corrected"`)) || !bytes.Contains(current, []byte(`"origins"`)) {
		t.Fatal("lineage missing", string(current))
	}
	// Linked reports remain editable while both conflicting accounts are inspectable.
	report.Label = "A disputed report"
	h.publish(w, h.draft(w, 3, put(report)))
	raw := h.call("reader@example.test", "GET", p+"/canon?timeline=main", nil, 200)
	if !bytes.Contains(raw, []byte("never collapsed")) || !bytes.Contains(raw, []byte("central span")) {
		t.Fatal("reports lost")
	}
}
func TestEstablishedGraphGuards(t *testing.T) {
	base := Snapshot{"main": {ID: "main", Kind: "timeline", Label: "Main"}, "alt": {ID: "alt", Kind: "timeline", Label: "Alternate"}, "a": {ID: "a", Kind: "established_account", Label: "A", Timeline: "main", Synopsis: "Observed"}}
	for _, record := range []Record{{ID: "b", Kind: "established_account", Label: "B", Timeline: "alt", Synopsis: "Elsewhere", Supersedes: "a"}, {ID: "b", Kind: "established_account", Label: "B", Timeline: "main", Synopsis: "Cycle", Supersedes: "b"}} {
		if _, err := Apply(base, put(record)); err == nil {
			t.Fatal("invalid graph")
		}
	}
	copy := base["a"]
	copy.ID = "branch_a"
	copy.Timeline = "alt"
	if _, err := Apply(base, put(copy)); err != nil {
		t.Fatal("separate branch", err)
	}
}
func TestConversationAttributionScopeAndResumption(t *testing.T) {
	h := setup(t)
	w := h.world()
	p := "/" + w
	calls := 0
	h.handler = Handler(&Store{Pool: h.pool, Reasoner: Reasoner(func(ctx context.Context, r ReasonRequest) (ReasonResult, error) {
		calls++
		return ReasonResult{Answer: "Unknown", Unknown: true}, nil
	})}, func(r *http.Request) (string, bool) { a := r.Header.Get("Test-Actor"); return a, a != "" })
	c := Conversation{ID: uuid.NewString(), Title: "Shared history", Timeline: "main"}
	h.call("reader@example.test", "POST", p+"/conversations", c, 200)
	h.call("reader@example.test", "POST", p+"/conversations", c, 200)
	route := p + "/conversations/" + c.ID + "/messages"
	id := uuid.NewString()
	b := map[string]any{"id": id, "request": ReasonInput{Workflow: "canon_query", Question: "What happened?", Timeline: "main"}}
	h.call("reader@example.test", "POST", route, b, 200)
	h.call("reader@example.test", "POST", route, b, 200)
	h.call("editor@example.test", "POST", route, b, 409)
	h.call("reader@example.test", "POST", route, map[string]any{"id": uuid.NewString(), "request": ReasonInput{Workflow: "story_turn", Question: "Private", Timeline: "main", SessionID: uuid.NewString()}}, 422)
	h.call("editor@example.test", "POST", route+"/"+id+"/generate", map[string]any{}, 403)
	first := h.call("reader@example.test", "POST", route+"/"+id+"/generate", map[string]any{}, 200)
	if !strings.Contains(string(first), "event: result") {
		t.Fatal(string(first))
	}
	h.call("reader@example.test", "POST", route+"/"+id+"/generate", map[string]any{}, 200)
	if calls != 1 {
		t.Fatal("duplicate inference", calls)
	}
	raw := h.call("editor@example.test", "GET", route, nil, 200)
	if !bytes.Contains(raw, []byte("reader@example.test")) {
		t.Fatal("attribution lost")
	}
	h.call("stranger@example.test", "GET", route, nil, 404)
}

func TestSandboxSceneAtomicRetryAndPrivateIsolation(t *testing.T) {
	h, _, admin := superuserFixture(t)
	w := h.world()
	p := "/" + w
	h.publish(w, h.draft(w, 0, fixture()))
	id := uuid.NewString()
	route := p + "/sessions/" + id
	h.call("reader@example.test", "POST", p+"/sessions", map[string]any{"id": id, "title": "Sandbox", "pinnedRevision": 1, "timeline": "main", "mode": "sandbox"}, 200)
	for _, who := range []string{"owner@example.test", admin} {
		h.call(who, "GET", route, nil, 404)
	}
	turn := Turn{RequestID: uuid.NewString(), ExpectedVersion: 0, Narration: "Carry the relic", Scene: &SceneState{Location: "hero", Inventory: []InventoryItem{{EntityID: "friend", Quantity: 1}}}}
	first := h.call("reader@example.test", "POST", route+"/turns", turn, 200)
	turn2 := Turn{RequestID: uuid.NewString(), ExpectedVersion: 1, Narration: "Move", Scene: &SceneState{Location: "friend"}}
	h.call("reader@example.test", "POST", route+"/turns", turn2, 200)
	again := h.call("reader@example.test", "POST", route+"/turns", turn, 200)
	var a, b map[string]any
	json.Unmarshal(first, &a)
	json.Unmarshal(again, &b)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("retry returned changed scene")
	}
	turn2.RequestID = uuid.NewString()
	turn2.ExpectedVersion = 2
	turn2.Scene = &SceneState{Location: "absent"}
	h.call("reader@example.test", "POST", route+"/turns", turn2, 422)
	var session Session
	json.Unmarshal(h.call("reader@example.test", "GET", route, nil, 200), &session)
	if session.Version != 2 || session.Scene.Location != "friend" {
		t.Fatal("failed scene changed version", session)
	}
	base, _ := Apply(Snapshot{"main": {ID: "main", Kind: "timeline", Label: "Main"}}, fixture())
	conflictFact := Record{ID: "conflict", Kind: "fact", Subject: "hero", Predicate: "age", ValueType: "number", Value: float64(99), Timeline: "main", Source: "source", From: ptr(15), Until: ptr(20)}
	if _, err := Apply(base, put(conflictFact)); err == nil {
		t.Fatal("canon overlap accepted")
	}
	if _, err := applyWithMode(base, put(conflictFact), true); err != nil {
		t.Fatal("sandbox fictional departure rejected", err)
	}
	conflictFact.Subject = "missing"
	if _, err := applyWithMode(base, put(conflictFact), true); err == nil {
		t.Fatal("sandbox reference bypass")
	}
}
func TestEstablishedRestoreAndScopedReasoning(t *testing.T) {
	h := setup(t)
	w := h.world()
	p := "/" + w
	account := Record{ID: "witness", Kind: "established_account", Label: "Observed", Timeline: "main", Synopsis: "The bridge collapsed.", From: ptr(-9223372036854775808)}
	report := Record{ID: "report", Kind: "source", Label: "Reported history", EvidenceStatus: "reported", Attributes: map[string]string{"description": "The bridge did not collapse."}}
	h.publish(w, h.draft(w, 0, append(put(account), put(report)...)))
	h.call("owner@example.test", "POST", p+"/restore", RestoreRequest{ID: uuid.NewString(), BaseRevision: 1, TargetRevision: 0, Reason: "Erase witness"}, 422)
	h.handler = Handler(&Store{Pool: h.pool, Reasoner: Reasoner(func(ctx context.Context, r ReasonRequest) (ReasonResult, error) {
		if r.At == nil || int64(*r.At) != -9223372036854775808 {
			t.Fatal("lost time scope")
		}
		seen := map[string]bool{}
		for _, e := range r.Evidence {
			seen[e.Record.ID] = true
		}
		if !seen["witness"] || !seen["report"] {
			t.Fatal("conflicting history not inspectable")
		}
		return ReasonResult{Answer: "The witnessed collapse is established; the chronicle disagrees.", Citations: []string{w + ":1:main:witness", w + ":1:main:report"}}, nil
	})}, func(r *http.Request) (string, bool) { a := r.Header.Get("Test-Actor"); return a, a != "" })
	raw := h.call("reader@example.test", "POST", p+"/reason", map[string]any{"workflow": "canon_query", "question": "What happened?", "revision": 1, "timeline": "main", "at": "-9223372036854775808"}, 200)
	var result ReasonResult
	if err := json.Unmarshal(raw, &result); err != nil || len(result.EstablishedAccounts) != 1 || result.EstablishedAccounts[0].Record.ID != "witness" {
		t.Fatal("effective account missing", string(raw), err)
	}
}
func TestAdministratorRegistryAndFinalProtection(t *testing.T) {
	h, _, admin := superuserFixture(t)
	target := "administrator-" + uuid.NewString() + "@example.test"
	h.call(admin, "POST", "/users", map[string]string{"email": target, "displayName": "Second Administrator"}, 200)
	h.call("reader@example.test", "POST", "/administrators", map[string]any{"principal": target, "enabled": true, "reason": "Unauthorized"}, 403)
	h.call(admin, "POST", "/administrators", map[string]any{"principal": target, "enabled": true, "reason": "Appoint"}, 200)
	h.call(target, "GET", "/users", nil, 200)
	tx, err := h.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), `DELETE FROM worldbuilder.superuser WHERE principal<>$1`, target); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), `SELECT worldbuilder.set_administrator($1,$1,false,'last')`, target); err == nil {
		t.Fatal("removed final Administrator")
	}
}
