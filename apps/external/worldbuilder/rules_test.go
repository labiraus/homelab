package worldbuilder

import "testing"

func TestCardinalityHalfOpenIntervalsAndRuleScope(t *testing.T) {
	s, err := Apply(Snapshot{"main": {ID: "main", Kind: "timeline", Label: "Main"}}, fixture())
	if err != nil {
		t.Fatal(err)
	}
	s["limit"] = Record{ID: "limit", Kind: "rule", Label: "One friend at a time", Type: "person", Timeline: "main", Predicate: "friend", Attributes: map[string]string{"check": "relationship_cardinality", "max": "1"}}
	s["edge"] = Record{ID: "edge", Kind: "relationship", Subject: "hero", Target: "friend", Predicate: "friend", Source: "source", Timeline: "main", Until: ptr(10)}
	s["second"] = Record{ID: "second", Kind: "relationship", Subject: "hero", Target: "friend", Predicate: "friend", Source: "source", Timeline: "main", From: ptr(10)}
	if err := Validate(s); err != nil {
		t.Fatalf("adjacent edges overlap: %v", err)
	}
	r := s["second"]
	r.From = ptr(9)
	s[r.ID] = r
	if err := Validate(s); err == nil {
		t.Fatal("overlapping cardinality accepted")
	}
	r.Timeline = "alternate"
	s[r.ID] = r
	s["alternate"] = Record{ID: "alternate", Kind: "timeline", Label: "Alternate"}
	if err := Validate(s); err != nil {
		t.Fatalf("other timeline counted: %v", err)
	}
	rule := s["limit"]
	rule.Attributes = map[string]string{"check": "relationship_cardinality", "min": "1"}
	rule.From = ptr(0)
	rule.Until = ptr(10)
	s[rule.ID] = rule
	// Both people must have one declared relationship throughout the rule interval.
	s["friend-edge"] = Record{ID: "friend-edge", Kind: "relationship", Subject: "friend", Target: "hero", Predicate: "friend", Source: "source", Timeline: "main", From: ptr(0), Until: ptr(10)}
	if err := Validate(s); err != nil {
		t.Fatalf("valid minimum rejected: %v", err)
	}
	r = s["friend-edge"]
	r.From = ptr(1)
	s[r.ID] = r
	if err := Validate(s); err == nil {
		t.Fatal("gap below minimum accepted")
	}
}

func TestExclusiveFactsAndExplicitChronologyExceptions(t *testing.T) {
	s, err := Apply(Snapshot{"main": {ID: "main", Kind: "timeline", Label: "Main"}}, fixture())
	if err != nil {
		t.Fatal(err)
	}
	typeRecord := s["person"]
	typeRecord.Fields = map[string]string{"age": "number", "birth": "number", "death": "number", "alive": "boolean", "undead": "boolean"}
	s[typeRecord.ID] = typeRecord
	s["exclusive"] = Record{ID: "exclusive", Kind: "rule", Label: "Alive or undead", Type: "person", Timeline: "main", Attributes: map[string]string{"check": "exclusive_facts", "predicates": "alive,undead"}}
	s["alive"] = Record{ID: "alive", Kind: "fact", Subject: "hero", Predicate: "alive", ValueType: "boolean", Value: true, Timeline: "main", Source: "source", Until: ptr(10)}
	s["undead"] = Record{ID: "undead", Kind: "fact", Subject: "hero", Predicate: "undead", ValueType: "boolean", Value: true, Timeline: "main", Source: "source", From: ptr(10)}
	if err := Validate(s); err != nil {
		t.Fatalf("exclusive adjacent periods rejected: %v", err)
	}
	r := s["alive"]
	r.Until = ptr(11)
	s[r.ID] = r
	if err := Validate(s); err == nil {
		t.Fatal("exclusive overlap accepted")
	}
	r.Until = ptr(10)
	s[r.ID] = r
	s["death"] = Record{ID: "death", Kind: "fact", Subject: "hero", Predicate: "death", ValueType: "number", Value: float64(10), Timeline: "main", Source: "source"}
	s["appearance"] = Record{ID: "appearance", Kind: "event", Label: "Returns", Subject: "hero", Timeline: "main", Source: "source", From: ptr(12)}
	s["life"] = Record{ID: "life", Kind: "rule", Label: "Life bounds", Type: "person", Timeline: "main", Attributes: map[string]string{"check": "life_bounds", "birth": "birth", "death": "death"}}
	if err := Validate(s); err == nil {
		t.Fatal("post-death event accepted without exception")
	}
	r = s["life"]
	r.Attributes["exceptions"] = "undead"
	s[r.ID] = r
	if err := Validate(s); err != nil {
		t.Fatalf("canonical active exception rejected: %v", err)
	}
	r = s["undead"]
	r.From = ptr(13)
	s[r.ID] = r
	if err := Validate(s); err == nil {
		t.Fatal("future exception excused past event")
	}
}

func TestRuleConfigurationAndIntegerExtremes(t *testing.T) {
	min, max := int64(-1<<63), int64(1<<63-1)
	if !countWithin([]Record{{From: &min, Until: &max}}, &min, &max, 1, 1) {
		t.Fatal("integer boundary overflow")
	}
	if !countWithin([]Record{{From: &min}}, nil, nil, 1, 1) {
		t.Fatal("smallest representable start treated as a gap")
	}
	if countWithin([]Record{{Until: &min}}, &min, nil, 1, 1) {
		t.Fatal("expired interval counted at minimum integer")
	}
	if _, err := rulePredicates("alive,alive", 2); err == nil {
		t.Fatal("duplicate predicate accepted")
	}
}

func TestRulePublicationAndGroupedPolicyChange(t *testing.T) {
	h := setup(t)
	w := h.world()
	p := "/" + w
	h.publish(w, h.draft(w, 0, fixture()))
	rule := Record{ID: "friend-limit", Kind: "rule", Label: "One friend", Type: "person", Timeline: "main", Predicate: "friend", Attributes: map[string]string{"check": "relationship_cardinality", "max": "1"}}
	h.publish(w, h.draft(w, 1, put(rule)))
	edge := Record{ID: "another-edge", Kind: "relationship", Subject: "hero", Target: "friend", Timeline: "main", Predicate: "friend", Source: "source"}
	h.call("editor@example.test", "POST", p+"/changes", map[string]any{"id": "00000000-0000-4000-8000-000000000099", "baseRevision": 2, "reason": "Invalid multiplicity", "operations": put(edge)}, 422)
	rule.Attributes["max"] = "2"
	h.publish(w, h.draft(w, 2, append(put(rule), put(edge)...)))
}
