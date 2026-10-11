package worldbuilder

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func testCalendar() *Calendar {
	return &Calendar{Name: "Harbour", EpochTick: "-9223372036854775808", EpochYear: "-10", TicksPerDay: 24,
		Months: []CalendarMonth{{Name: "Tide", Days: 30}, {Name: "Rest", LeapDays: 1}}, CycleYears: 4, LeapYears: []int{4},
		Weekdays: []string{"Sun", "Moon"}, WeekdayAtEpoch: 1}
}

func TestCalendarValidation(t *testing.T) {
	if err := validateCalendar(testCalendar()); err != nil {
		t.Fatal(err)
	}
	for label, mutate := range map[string]func(*Calendar){
		"epoch precision":        func(c *Calendar) { c.EpochTick = "9223372036854775808" },
		"canonical year":         func(c *Calendar) { c.EpochYear = "-0" },
		"fractional tick":        func(c *Calendar) { c.EpochTick = "1.5" },
		"whitespace":             func(c *Calendar) { c.Name = " bad" },
		"ticks":                  func(c *Calendar) { c.TicksPerDay = 0 },
		"cycle bound":            func(c *Calendar) { c.CycleYears = 1001 },
		"duplicate month":        func(c *Calendar) { c.Months[1].Name = "Tide" },
		"empty month":            func(c *Calendar) { c.Months[1].LeapDays = 0 },
		"empty year":             func(c *Calendar) { c.Months[0].Days = 0; c.Months[0].LeapDays = 1 },
		"missing leap positions": func(c *Calendar) { c.LeapYears = nil },
		"duplicate leap":         func(c *Calendar) { c.LeapYears = []int{4, 4} },
		"unordered leap":         func(c *Calendar) { c.LeapYears = []int{4, 1} },
		"out of cycle":           func(c *Calendar) { c.LeapYears = []int{5} },
		"duplicate weekday":      func(c *Calendar) { c.Weekdays = []string{"Sun", "Sun"} },
		"epoch weekday":          func(c *Calendar) { c.WeekdayAtEpoch = 2 },
	} {
		t.Run(label, func(t *testing.T) {
			c := testCalendar()
			mutate(c)
			if validateCalendar(c) == nil {
				t.Fatal("invalid calendar accepted")
			}
		})
	}
	s := Snapshot{"main": {ID: "main", Kind: "timeline", Label: "Main", Calendar: testCalendar()}}
	if Validate(s) == nil {
		t.Fatal("calendar accepted outside settings")
	}
	var r Record
	if json.Unmarshal([]byte(`{"kind":"world_settings","calendar":{"unknown":true}}`), &r) == nil {
		t.Fatal("unknown nested calendar field accepted")
	}
}

func TestFullRangeLifeDatesAndLegacyNumbers(t *testing.T) {
	s, err := Apply(Snapshot{"main": {ID: "main", Kind: "timeline", Label: "Main"}}, fixture())
	if err != nil {
		t.Fatal(err)
	}
	typ := s["person"]
	typ.Fields = map[string]string{"age": "number", "birth": "tick", "death": "tick"}
	s[typ.ID] = typ
	s["birth"] = Record{ID: "birth", Kind: "fact", Subject: "hero", Predicate: "birth", ValueType: "tick", Value: "-9223372036854775808", Timeline: "main", Source: "source"}
	s["death"] = Record{ID: "death", Kind: "fact", Subject: "hero", Predicate: "death", ValueType: "tick", Value: "9223372036854775807", Timeline: "main", Source: "source"}
	s["life"] = Record{ID: "life", Kind: "rule", Label: "Life", Attributes: map[string]string{"check": "life_bounds", "birth": "birth", "death": "death"}}
	for _, n := range []int64{math.MinInt64, -9007199254740993, math.MaxInt64 - 1} {
		s["appearance"] = Record{ID: "appearance", Kind: "event", Label: "Appears", Subject: "hero", Timeline: "main", Source: "source", From: ptr(n)}
		if err := Validate(s); err != nil {
			t.Fatal(n, err)
		}
	}
	r := s["appearance"]
	r.From = ptr(math.MaxInt64)
	s[r.ID] = r
	if Validate(s) == nil {
		t.Fatal("exclusive death accepted")
	}
	for _, value := range []any{float64(1), "01", "+1", "-0", "1e3", "9223372036854775808", ""} {
		if typedValue("tick", value, s) {
			t.Fatalf("inexact date fact accepted: %v", value)
		}
	}
	for _, n := range []float64{-9007199254740991, 0, 9007199254740991} {
		if tick, ok := lifeTick(Record{ValueType: "number", Value: n}); !ok || tick != int64(n) {
			t.Fatal("legacy integer rejected")
		}
	}
	for _, n := range []float64{math.NaN(), math.Inf(1), 1.5, 9007199254740992} {
		if _, ok := lifeTick(Record{ValueType: "number", Value: n}); ok {
			t.Fatal("inexact legacy integer accepted")
		}
	}
}

func TestReviewedCalendarAndDateFactPublication(t *testing.T) {
	h := setup(t)
	w := h.world()
	p := "/" + w
	h.publish(w, h.draft(w, 0, fixture()))
	before := h.call("reader@example.test", "GET", p+"/canon?timeline=main&revision=1", nil, 200)
	settings := Record{ID: "world_settings", Kind: "world_settings", Label: "Test world", Timeline: "main", Calendar: testCalendar()}
	ops := put(settings)
	ops = append(ops, put(Record{ID: "dated", Kind: "type", Fields: map[string]string{"birth": "tick"}})...)
	ops = append(ops, put(Record{ID: "ancient", Kind: "entity", Label: "Ancient", Type: "dated"})...)
	ops = append(ops, put(Record{ID: "born", Kind: "fact", Subject: "ancient", Predicate: "birth", ValueType: "tick", Value: "-9223372036854775808", Timeline: "main", Source: "source"})...)
	draft := h.draft(w, 1, ops)
	read := func(route string) SearchResult {
		var result SearchResult
		if err := json.Unmarshal(h.call("reader@example.test", "GET", p+route, nil, 200), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	if read("/canon?timeline=main").Calendar != nil {
		t.Fatal("draft calendar leaked")
	}
	h.publish(w, draft)
	for _, route := range []string{"/canon?timeline=main&kind=fact&limit=1", "/search?timeline=main&q=ancient&limit=1", "/search?timeline=main&q=nomatch"} {
		result := read(route)
		if !reflect.DeepEqual(result.Calendar, settings.Calendar) {
			t.Fatalf("calendar missing from filtered scope: %+v", result)
		}
	}
	var projected Calendar
	var raw []byte
	if err := h.pool.QueryRow(context.Background(), `SELECT calendar FROM worldbuilder.record WHERE world_id=$1 AND id='world_settings'`, w).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(raw, &projected) != nil || !reflect.DeepEqual(&projected, settings.Calendar) {
		t.Fatal("calendar projection lost")
	}
	var date string
	if err := h.pool.QueryRow(context.Background(), `SELECT value #>> '{}' FROM worldbuilder.record WHERE world_id=$1 AND id='born'`, w).Scan(&date); err != nil || date != "-9223372036854775808" {
		t.Fatal(date, err)
	}
	settings.Calendar = nil
	h.publish(w, h.draft(w, 2, put(settings)))
	if read("/canon?timeline=main").Calendar != nil || read("/search?timeline=main&q=ancient&revision=2").Calendar == nil || read("/canon?timeline=main&revision=1").Calendar != nil {
		t.Fatal("historical calendar pin lost")
	}
	// Current world metadata changes revision, but the published records remain identical.
	var old, again SearchResult
	json.Unmarshal(before, &old)
	json.Unmarshal(h.call("reader@example.test", "GET", p+"/canon?timeline=main&revision=1", nil, 200), &again)
	a, _ := json.Marshal(old.Records)
	b, _ := json.Marshal(again.Records)
	if !bytes.Equal(a, b) {
		t.Fatal("calendar upgrade rewrote existing records")
	}
}
