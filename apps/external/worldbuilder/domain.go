// Package worldbuilder owns canon validation and publication inside external.
package worldbuilder

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type Record struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	Label      string            `json:"label,omitempty"`
	Type       string            `json:"type,omitempty"`
	Subject    string            `json:"subject,omitempty"`
	Target     string            `json:"target,omitempty"`
	Timeline   string            `json:"timeline,omitempty"`
	Source     string            `json:"source,omitempty"`
	Predicate  string            `json:"predicate,omitempty"`
	ValueType  string            `json:"valueType,omitempty"`
	Value      any               `json:"value,omitempty"`
	From       *int64            `json:"from,omitempty"`
	Until      *int64            `json:"until,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Fields     map[string]string `json:"fields,omitempty"`
	Relations  map[string]string `json:"relations,omitempty"`
}
type Snapshot map[string]Record
type Operation struct {
	Action string  `json:"action"`
	ID     string  `json:"id"`
	Record *Record `json:"record,omitempty"`
}
type World struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Revision int64  `json:"revision"`
	Role     string `json:"role,omitempty"`
}
type Change struct {
	ID                string      `json:"id"`
	Author            string      `json:"author"`
	BaseRevision      int64       `json:"baseRevision"`
	Reason            string      `json:"reason"`
	Operations        []Operation `json:"operations"`
	Status            string      `json:"status"`
	Reviewer          string      `json:"reviewer"`
	ReviewReason      string      `json:"reviewReason"`
	PublishedRevision *int64      `json:"publishedRevision,omitempty"`
}
type Problem struct {
	Status  int
	Code    string
	Message string
}

func (p *Problem) Error() string    { return p.Message }
func bad(message string) error      { return &Problem{422, "validation", message} }
func conflict(message string) error { return &Problem{409, "conflict", message} }

var forbidden = &Problem{403, "forbidden", "operation is not permitted"}
var missing = &Problem{404, "not_found", "resource not found"}
var identifier = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func Apply(before Snapshot, ops []Operation) (Snapshot, error) {
	if len(ops) == 0 || len(ops) > 100 {
		return nil, bad("provide between 1 and 100 operations")
	}
	after := make(Snapshot, len(before))
	for k, v := range before {
		after[k] = v
	}
	seen := map[string]bool{}
	for _, op := range ops {
		if !identifier.MatchString(op.ID) || seen[op.ID] {
			return nil, bad("invalid or duplicate operation id")
		}
		seen[op.ID] = true
		switch op.Action {
		case "put":
			if op.Record == nil || op.Record.ID != op.ID {
				return nil, bad("put requires matching record id")
			}
			if prev, ok := before[op.ID]; ok && prev.Kind != op.Record.Kind {
				return nil, bad("record kind is immutable")
			}
			after[op.ID] = *op.Record
		case "delete":
			if op.Record != nil {
				return nil, bad("delete cannot contain a record")
			}
			if _, ok := after[op.ID]; !ok {
				return nil, bad("cannot delete missing record")
			}
			delete(after, op.ID)
		default:
			return nil, bad("unsupported operation")
		}
	}
	if len(after) > 5000 {
		return nil, bad("world exceeds the initial 5000-record limit")
	}
	return after, Validate(after)
}

func Validate(s Snapshot) error {
	ref := func(id, kind string) bool { r, ok := s[id]; return ok && r.Kind == kind }
	for id, r := range s {
		if id != r.ID || !identifier.MatchString(id) {
			return bad("invalid record identity")
		}
		if len(r.Label) > 500 {
			return bad("label exceeds 500 characters")
		}
		if r.From != nil && r.Until != nil && *r.From >= *r.Until {
			return bad("fictional intervals are half-open and must increase")
		}
		switch r.Kind {
		case "type":
			for p, t := range r.Fields {
				if !identifier.MatchString(p) || !valueType(t) {
					return bad("invalid typed predicate")
				}
			}
			for p, t := range r.Relations {
				if !identifier.MatchString(p) || !ref(t, "type") {
					return bad("relationship schema references an unknown type")
				}
			}
		case "timeline", "source", "rule":
			if strings.TrimSpace(r.Label) == "" {
				return bad("timeline, source and rule require a label")
			}
		case "entity":
			if !ref(r.Type, "type") || strings.TrimSpace(r.Label) == "" {
				return bad("entity requires a type and label")
			}
		case "fact":
			if !ref(r.Subject, "entity") || !ref(r.Timeline, "timeline") || !ref(r.Source, "source") {
				return bad("fact references must resolve within this world")
			}
			t := s[s[r.Subject].Type]
			if t.Fields[r.Predicate] != r.ValueType || !valueType(r.ValueType) || !typedValue(r.ValueType, r.Value, s) {
				return bad("fact does not match its entity type schema")
			}
		case "relationship":
			if !ref(r.Subject, "entity") || !ref(r.Target, "entity") || !ref(r.Timeline, "timeline") || !ref(r.Source, "source") {
				return bad("relationship references must resolve within this world")
			}
			t := s[s[r.Subject].Type]
			if t.Relations[r.Predicate] != s[r.Target].Type {
				return bad("relationship target type is not permitted")
			}
		case "event":
			if !ref(r.Timeline, "timeline") || !ref(r.Source, "source") || r.From == nil || strings.TrimSpace(r.Label) == "" {
				return bad("event requires timeline, source, fictional date and label")
			}
			if r.Subject != "" && !ref(r.Subject, "entity") {
				return bad("event participant does not exist")
			}
		default:
			return bad("unsupported record kind")
		}
		// Reject reference fields outside their valid kinds; all SQL references remain valid.
		for _, x := range []struct{ id, kind string }{{r.Type, "type"}, {r.Subject, "entity"}, {r.Target, "entity"}, {r.Timeline, "timeline"}, {r.Source, "source"}} {
			if x.id != "" && !ref(x.id, x.kind) {
				return bad("invalid scoped reference")
			}
		}
	}

	// Explicit deterministic life-bound rule. Fictional dates are integer ticks, not timestamps.
	for _, rule := range s {
		if rule.Kind != "rule" || rule.Attributes["check"] == "" {
			continue
		}
		if rule.Attributes["check"] != "life_bounds" {
			return bad("unsupported deterministic world rule")
		}
		birth, death := rule.Attributes["birth"], rule.Attributes["death"]
		if birth == "" || death == "" {
			return bad("life_bounds requires birth and death predicates")
		}
		for _, event := range s {
			if event.Kind != "event" || event.Subject == "" || event.From == nil {
				continue
			}
			for _, fact := range s {
				if fact.Kind != "fact" || fact.Subject != event.Subject || fact.Timeline != event.Timeline || !Visible(fact, event.Timeline, event.From) {
					continue
				}
				if fact.Predicate != birth && fact.Predicate != death {
					continue
				}
				tick, ok := fact.Value.(float64)
				if !ok {
					return bad("life-bound dates must be numeric fictional ticks")
				}
				if (fact.Predicate == birth && float64(*event.From) < tick) || (fact.Predicate == death && float64(*event.From) >= tick) {
					return bad("event falls outside its participant's canonical life bounds")
				}
			}
		}
	}
	facts := []Record{}
	for _, r := range s {
		if r.Kind == "fact" {
			facts = append(facts, r)
		}
	}
	for i, a := range facts {
		for _, b := range facts[i+1:] {
			if a.Subject == b.Subject && a.Predicate == b.Predicate && a.Timeline == b.Timeline && overlaps(a, b) {
				return bad("single-valued facts overlap in fictional time")
			}
		}
	}
	return nil
}
func valueType(t string) bool {
	return t == "string" || t == "number" || t == "boolean" || t == "entity"
}
func typedValue(t string, v any, s Snapshot) bool {
	switch t {
	case "string":
		_, ok := v.(string)
		return ok
	case "number":
		_, ok := v.(float64)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "entity":
		id, ok := v.(string)
		return ok && s[id].Kind == "entity"
	}
	return false
}
func overlaps(a, b Record) bool {
	return (a.Until == nil || b.From == nil || *b.From < *a.Until) && (b.Until == nil || a.From == nil || *a.From < *b.Until)
}
func Visible(r Record, timeline string, at *int64) bool {
	if r.Timeline != "" && r.Timeline != timeline {
		return false
	}
	return at == nil || ((r.From == nil || *r.From <= *at) && (r.Until == nil || *at < *r.Until))
}
func Records(s Snapshot, timeline, query string, at *int64) []Record {
	result := []Record{}
	for _, r := range s {
		raw, _ := json.Marshal(r)
		if Visible(r, timeline, at) && (query == "" || strings.Contains(strings.ToLower(string(raw)), strings.ToLower(query))) {
			result = append(result, r)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

type Evidence struct {
	ID       string `json:"id"`
	WorldID  string `json:"worldId"`
	Revision int64  `json:"revision"`
	Timeline string `json:"timeline"`
	Record   Record `json:"record"`
}

func Citations(world string, revision int64, timeline string, records []Record) []Evidence {
	result := []Evidence{}
	for _, r := range records {
		result = append(result, Evidence{fmt.Sprintf("%s:%d:%s:%s", world, revision, timeline, r.ID), world, revision, timeline, r})
	}
	return result
}
