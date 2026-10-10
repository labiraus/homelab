package worldbuilder

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestPrivateDefinitionPinMemoryAndTurnRetry(t *testing.T) {
	h := setup(t)
	w := h.world()
	h.publish(w, h.draft(w, 0, fixture()))
	path := "/" + w
	d := StoryDefinition{ID: uuid.NewString(), Title: "At the tower", PinnedRevision: 1, Timeline: "main", Premise: "Explore the tower", Objective: "Find a map", Style: "Restrained", Participants: []string{"hero"}}
	h.call("reader@example.test", "POST", path+"/story-definitions", d, 200)
	h.call("reader@example.test", "POST", path+"/story-definitions", d, 200)
	changed := d
	changed.Premise = "Replacement"
	h.call("reader@example.test", "POST", path+"/story-definitions", changed, 409)
	h.call("owner@example.test", "GET", path+"/story-definitions/"+d.ID, nil, 404)
	raw := h.call("owner@example.test", "GET", path+"/story-definitions", nil, 200)
	if string(raw) != "[]\n" {
		t.Fatalf("private definition leaked: %s", raw)
	}
	session := map[string]any{"id": uuid.NewString(), "title": d.Title, "definitionId": d.ID, "pinnedRevision": 0, "timeline": "main"}
	h.call("reader@example.test", "POST", path+"/sessions", session, 422)
	session["pinnedRevision"] = 1
	h.call("reader@example.test", "POST", path+"/sessions", session, 200)
	id := session["id"].(string)
	turn := Turn{RequestID: uuid.NewString(), Narration: "Hero found a map", Direction: "Explore", MemorySummary: "Hero has a map", Choices: []StoryChoice{{ID: "north", Label: "Go north", Direction: "Follow the northern trail"}}}
	first := h.call("reader@example.test", "POST", path+"/sessions/"+id+"/turns", turn, 200)
	again := h.call("reader@example.test", "POST", path+"/sessions/"+id+"/turns", turn, 200)
	var original, replayed any
	json.Unmarshal(first, &original)
	json.Unmarshal(again, &replayed)
	if !reflect.DeepEqual(original, replayed) {
		t.Fatal("turn retry lost original memory")
	}
	turn.MemorySummary = "Changed retry"
	h.call("reader@example.test", "POST", path+"/sessions/"+id+"/turns", turn, 409)
	var saved Session
	json.Unmarshal(h.call("reader@example.test", "GET", path+"/sessions/"+id, nil, 200), &saved)
	if saved.Memory.ThroughVersion != 1 || saved.Memory.Summary != "Hero has a map" || len(saved.Memory.Choices) != 1 || saved.Definition.Premise != d.Premise {
		t.Fatalf("memory/definition lost: %+v", saved)
	}
	h.call("reader@example.test", "POST", path+"/sessions/"+id+"/turns", Turn{RequestID: uuid.NewString(), ExpectedVersion: 1, Narration: "A later manual scene"}, 200)
	turn.MemorySummary = "Hero has a map"
	again = h.call("reader@example.test", "POST", path+"/sessions/"+id+"/turns", turn, 200)
	json.Unmarshal(again, &replayed)
	if !reflect.DeepEqual(original, replayed) {
		t.Fatal("later turn changed retry memory")
	}
	h.call("owner@example.test", "GET", path+"/sessions/"+id, nil, 404)
	var revision int
	h.pool.QueryRow(context.Background(), `SELECT revision FROM worldbuilder.world WHERE id=$1`, w).Scan(&revision)
	if revision != 1 {
		t.Fatal("story changed canon")
	}
	if _, err := h.pool.Exec(context.Background(), `UPDATE worldbuilder.story_definition SET definition='{}' WHERE world_id=$1`, w); err == nil {
		t.Fatal("definition history rewritable")
	}
}

func TestStoryReasoningCarriesBoundedMemoryAndRejectsStaleVersion(t *testing.T) {
	h := setup(t)
	w := h.world()
	h.publish(w, h.draft(w, 0, fixture()))
	id := uuid.NewString()
	p := "/" + w
	h.call("reader@example.test", "POST", p+"/sessions", map[string]any{"id": id, "title": "Journey", "pinnedRevision": 1, "timeline": "main"}, 200)
	for version := int64(0); version < 4; version++ {
		h.call("reader@example.test", "POST", p+"/sessions/"+id+"/turns", Turn{RequestID: uuid.NewString(), ExpectedVersion: version, Narration: strings.Repeat("x", 2500), MemorySummary: "A private summary"}, 200)
	}
	calls := 0
	h.handler = Handler(&Store{Pool: h.pool, Reasoner: func(_ context.Context, r ReasonRequest) (ReasonResult, error) {
		calls++
		if r.Story == nil || r.Story.Version != 4 || r.Story.Memory.ThroughVersion != 4 || len(r.Story.RecentTurns) != 3 || r.Story.RecentTurns[0]["truncated"] != true {
			t.Fatalf("unbounded/stale story context: %+v", r.Story)
		}
		return ReasonResult{Answer: "Next scene", Citations: []string{w + ":1:main:hero"}, MemorySummary: "A new provisional summary", Choices: []StoryChoice{{ID: "wait", Label: "Wait", Direction: "Wait for dawn"}}, SessionVersion: ptr(99)}, nil
	}}, func(r *http.Request) (string, bool) { a := r.Header.Get("Test-Actor"); return a, a != "" })
	body := map[string]any{"workflow": "story_turn", "question": "Continue", "timeline": "main", "revision": 1, "sessionId": id, "expectedVersion": 3}
	h.call("reader@example.test", "POST", p+"/reason", body, 409)
	if calls != 0 {
		t.Fatal("stale session spent inference")
	}
	body["expectedVersion"] = 4
	var result ReasonResult
	json.Unmarshal(h.call("reader@example.test", "POST", p+"/reason", body, 200), &result)
	if result.SessionVersion == nil || *result.SessionVersion != 4 || result.MemorySummary == "" {
		t.Fatal("model overrode session version")
	}
	var saved Session
	json.Unmarshal(h.call("reader@example.test", "GET", p+"/sessions/"+id, nil, 200), &saved)
	if saved.Version != 4 || saved.Memory.Summary != "A private summary" {
		t.Fatal("generated candidate persisted itself")
	}
}
