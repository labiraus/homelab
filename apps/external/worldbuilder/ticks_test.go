package worldbuilder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/google/uuid"
)

func TestExactTickWireCompatibility(t *testing.T) {
	for _, tick := range []int64{math.MinInt64, -9007199254740993, -1, 0, 1, 9007199254740993, math.MaxInt64} {
		for _, token := range []string{fmt.Sprint(tick), fmt.Sprintf("%q", fmt.Sprint(tick))} {
			var record Record
			raw := fmt.Sprintf(`{"id":"event","kind":"event","from":%s}`, token)
			if err := json.Unmarshal([]byte(raw), &record); err != nil || record.From == nil || *record.From != tick {
				t.Fatalf("%s: %+v %v", raw, record, err)
			}
			encoded, err := json.Marshal(record)
			want := fmt.Sprintf(`{"id":"event","kind":"event","from":%d}`, tick)
			if err != nil || string(encoded) != want {
				t.Fatalf("changed canonical numeric encoding: %s %v", encoded, err)
			}
			var tool toolRequest
			if err := json.Unmarshal([]byte(`{"operation":"canon","at":`+token+`}`), &tool); err != nil || tool.At == nil || *tool.At != tick {
				t.Fatalf("tool tick %s: %+v %v", token, tool, err)
			}
		}
	}
	for _, raw := range []string{
		`{"from":9223372036854775808}`, `{"until":"-9223372036854775809"}`,
		`{"from":1.0}`, `{"from":1e3}`, `{"from":"1.2"}`, `{"from":"+1"}`,
		`{"from":" 1"}`, `{"from":"01"}`, `{"from":true}`, `{"until":{}}`,
		`{"from":"1","unknown":true}`,
	} {
		var record Record
		if err := json.Unmarshal([]byte(raw), &record); err == nil {
			t.Fatalf("invalid record accepted: %s", raw)
		}
	}
	var record Record
	if err := json.Unmarshal([]byte(`{"from":null,"until":null,"attributes":{"from":"not a date"}}`), &record); err != nil || record.From != nil || record.Until != nil {
		t.Fatalf("unbounded/attribute compatibility: %+v %v", record, err)
	}
}

func TestExactTicksPublicationHistoryAndIdempotency(t *testing.T) {
	h := setup(t)
	w := h.world()
	h.publish(w, h.draft(w, 0, fixture()))
	id := uuid.NewString()
	body := fmt.Sprintf(`{"id":%q,"baseRevision":1,"reason":"exact ticks","operations":[{"action":"put","id":"epoch","record":{"id":"epoch","kind":"event","label":"Epoch","timeline":"main","source":"source","from":"-9223372036854775808","until":"9223372036854775807"}}]}`, id)
	first := h.call("editor@example.test", "POST", "/"+w+"/changes", json.RawMessage(body), 200)
	// A numeric retry is the same request after canonical decoding.
	numeric := bytes.ReplaceAll([]byte(body), []byte(`"-9223372036854775808"`), []byte(`-9223372036854775808`))
	numeric = bytes.ReplaceAll(numeric, []byte(`"9223372036854775807"`), []byte(`9223372036854775807`))
	again := h.call("editor@example.test", "POST", "/"+w+"/changes", json.RawMessage(numeric), 200)
	if !bytes.Equal(first, again) {
		t.Fatal("representation changed idempotency")
	}
	h.publish(w, id)
	for _, at := range []string{"-9223372036854775808", "-9007199254740993", "9223372036854775806"} {
		raw := h.call("reader@example.test", "GET", "/"+w+"/records/epoch?revision=2&timeline=main&at="+at, nil, 200)
		if !bytes.Contains(raw, []byte(`"from":-9223372036854775808`)) || !bytes.Contains(raw, []byte(`"until":9223372036854775807`)) {
			t.Fatalf("rounded historical response: %s", raw)
		}
	}
	h.call("reader@example.test", "GET", "/"+w+"/records/epoch?revision=2&timeline=main&at=9223372036854775807", nil, 404)
	h.call("reader@example.test", "GET", "/"+w+"/records/epoch?revision=1&timeline=main", nil, 404)
	h.publish(w, h.draft(w, 2, []Operation{{Action: "delete", ID: "epoch"}}))
	h.call("reader@example.test", "GET", "/"+w+"/records/epoch?revision=2&timeline=main&at=-9223372036854775808", nil, 200)
	h.call("reader@example.test", "GET", "/"+w+"/records/epoch?revision=3&timeline=main", nil, 404)
}
