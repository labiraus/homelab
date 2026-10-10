package worldbuilder

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestGovernedSettingsAndStableDeletedIdentity(t *testing.T) {
	h := setup(t)
	w := h.world()
	p := "/" + w
	settings := Record{ID: "world_settings", Kind: "world_settings", Label: "Renamed world", Timeline: "alternate", Attributes: map[string]string{"tone": "hopeful"}}
	ops := append(put(Record{ID: "alternate", Kind: "timeline", Label: "Alternate continuity"}), put(settings)...)
	draft := h.draft(w, 0, ops)
	var before World
	if err := json.Unmarshal(h.call("reader@example.test", "GET", p, nil, 200), &before); err != nil {
		t.Fatal(err)
	}
	if before.Name != "Test world" || before.DefaultTimeline != "main" {
		t.Fatal("draft changed settings")
	}
	h.publish(w, draft)
	var after World
	if err := json.Unmarshal(h.call("reader@example.test", "GET", p, nil, 200), &after); err != nil {
		t.Fatal(err)
	}
	if after.Name != "Renamed world" || after.DefaultTimeline != "alternate" || after.Revision != 1 {
		t.Fatalf("settings not published: %+v", after)
	}
	h.call("owner@example.test", "POST", "", map[string]string{"id": w, "name": "Test world"}, 200) // original create retry after rename
	h.call("editor@example.test", "POST", p+"/changes", map[string]any{"id": uuid.NewString(), "baseRevision": 1, "reason": "Remove settings", "operations": []Operation{{Action: "delete", ID: "world_settings"}}}, 422)
	// Settings are world-wide even when their default points to another timeline.
	raw := h.call("reader@example.test", "GET", p+"/canon?timeline=main&kind=world_settings", nil, 200)
	var catalog struct{ Records []Record }
	if err := json.Unmarshal(raw, &catalog); err != nil || len(catalog.Records) != 1 {
		t.Fatalf("missing global settings: %s %v", raw, err)
	}
	h.publish(w, h.draft(w, 1, put(Record{ID: "deleted", Kind: "source", Label: "Former source"})))
	h.publish(w, h.draft(w, 2, []Operation{{Action: "delete", ID: "deleted"}}))
	h.call("editor@example.test", "POST", p+"/changes", map[string]any{"id": uuid.NewString(), "baseRevision": 3, "reason": "Reuse identity", "operations": put(Record{ID: "deleted", Kind: "timeline", Label: "Wrong kind"})}, 422)
	h.publish(w, h.draft(w, 3, put(Record{ID: "deleted", Kind: "source", Label: "Restored source"})))
	if _, err := h.pool.Exec(context.Background(), `UPDATE worldbuilder.record_identity SET kind='timeline' WHERE world_id=$1 AND id='deleted'`, w); err == nil {
		t.Fatal("identity history writable")
	}
}

func TestArchiveMembershipTransferConcurrencyAndRetries(t *testing.T) {
	h := setup(t)
	w := h.world()
	p := "/" + w
	var current World
	if err := json.Unmarshal(h.call("owner@example.test", "GET", p, nil, 200), &current); err != nil {
		t.Fatal(err)
	}
	version := current.AdminVersion
	h.call("owner@example.test", "GET", p+"/members?limit=2", nil, 200)
	h.call("reader@example.test", "GET", p+"/members", nil, 403)
	r := AdminRequest{RequestID: uuid.NewString(), ExpectedVersion: version, Action: "archive", Reason: "Pause world"}
	h.call("editor@example.test", "POST", p+"/administration", r, 403)
	raw := h.call("owner@example.test", "POST", p+"/administration", r, 200)
	if err := json.Unmarshal(raw, &current); err != nil || !current.Archived || current.AdminVersion != version+1 {
		t.Fatalf("archive: %s %v", raw, err)
	}
	h.call("owner@example.test", "POST", p+"/administration", r, 200)
	h.call("editor@example.test", "POST", p+"/changes", map[string]any{"id": uuid.NewString(), "baseRevision": 0, "reason": "Archived write", "operations": fixture()}, 409)
	h.call("reader@example.test", "GET", p+"/canon?timeline=main", nil, 200)
	r.Action = "unarchive"
	h.call("owner@example.test", "POST", p+"/administration", r, 409)
	r.RequestID = uuid.NewString()
	h.call("owner@example.test", "POST", p+"/administration", r, 409) // old administration version
	r.ExpectedVersion = version + 1
	h.call("owner@example.test", "POST", p+"/administration", r, 200)
	r = AdminRequest{RequestID: uuid.NewString(), ExpectedVersion: version + 2, Action: "remove_member", Principal: "reader@example.test", Reason: "Revoke access"}
	h.call("owner@example.test", "POST", p+"/administration", r, 200)
	h.call("reader@example.test", "GET", p, nil, 404)
	r = AdminRequest{RequestID: uuid.NewString(), ExpectedVersion: version + 3, Action: "transfer_owner", Principal: "editor@example.test", Reason: "Transfer stewardship"}
	h.call("owner@example.test", "POST", p+"/administration", r, 200)
	raw = h.call("owner@example.test", "POST", p+"/administration", r, 200) // exact retry remains available to former owner/admin
	if err := json.Unmarshal(raw, &current); err != nil || current.Role != "admin" || current.AdminVersion != version+4 {
		t.Fatalf("transfer retry: %s %v", raw, err)
	}
	h.call("owner@example.test", "POST", p+"/administration", AdminRequest{RequestID: uuid.NewString(), ExpectedVersion: version + 4, Action: "archive", Reason: "Former owner"}, 403)
	h.call("editor@example.test", "POST", p+"/administration", AdminRequest{RequestID: uuid.NewString(), ExpectedVersion: version + 4, Action: "remove_member", Principal: "editor@example.test", Reason: "Remove self"}, 422)
	var count int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM worldbuilder.member WHERE world_id=$1 AND role='owner'`, w).Scan(&count); err != nil || count != 1 {
		t.Fatalf("owners %d %v", count, err)
	}
	results := make(chan int, 2)
	for _, action := range []string{"archive", "unarchive"} {
		go func(action string) {
			code, _ := h.request("editor@example.test", "POST", p+"/administration", AdminRequest{RequestID: uuid.NewString(), ExpectedVersion: version + 4, Action: action, Reason: "Concurrent administration"})
			results <- code
		}(action)
	}
	got := map[int]int{}
	got[<-results]++
	got[<-results]++
	if got[200] != 1 || got[409] != 1 {
		t.Fatalf("concurrent administration: %+v", got)
	}
}
