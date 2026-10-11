package worldbuilder

import (
	"encoding/json"
	"testing"
)

func TestConnectionsScopedPinnedAndTemporal(t *testing.T) {
	h := setup(t)
	w := h.world()
	h.publish(w, h.draft(w, 0, fixture()))
	p := "/" + w + "/records/hero/connections?revision=1&timeline=main&at=30"
	var got struct {
		Records []Record `json:"records"`
		Total   int      `json:"total"`
	}
	if err := json.Unmarshal(h.call("reader@example.test", "GET", p, nil, 200), &got); err != nil {
		t.Fatal(err)
	}
	foundEdge := false
	for _, r := range got.Records {
		if r.ID == "age" {
			t.Fatal("expired fact leaked into traversal")
		}
		if r.ID == "edge" {
			foundEdge = true
		}
	}
	if !foundEdge {
		t.Fatal("incoming relationship missing")
	}
	h.call("stranger@example.test", "GET", p, nil, 404)
	h.call("reader@example.test", "GET", "/"+w+"/records/hero/connections?revision=0&timeline=main", nil, 404)
	h.call("reader@example.test", "GET", p+"&limit=1&offset=999", nil, 200)
	h.call("reader@example.test", "GET", "/"+w+"/records/hero/connections?revision=1&timeline=foreign", nil, 422)
}
