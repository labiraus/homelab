package worldbuilder

import (
	"sort"
	"strconv"
	"strings"
)

func rulePredicates(value string, minimum int) (map[string]bool, error) {
	result := map[string]bool{}
	if value != "" {
		for _, part := range strings.Split(value, ",") {
			predicate := strings.TrimSpace(part)
			if !identifier.MatchString(predicate) || result[predicate] {
				return nil, bad("rule predicates must be unique valid identifiers")
			}
			result[predicate] = true
		}
	}
	if len(result) < minimum || len(result) > 20 {
		return nil, bad("rule predicate count is outside supported bounds")
	}
	return result, nil
}

// Count intervals after applying all deltas at each boundary, starting at the
// smallest representable tick. No boundary +/-1 arithmetic can overflow.
func countWithin(items []Record, from, until *int64, minimum, maximum int) bool {
	if from == nil {
		first := int64(-1 << 63)
		from = &first
	}
	if from != nil && until != nil && *from >= *until {
		return true
	}
	deltas := map[int64]int{}
	count := 0
	for _, item := range items {
		if item.From == nil {
			count++
		} else {
			deltas[*item.From]++
		}
		if item.Until != nil {
			deltas[*item.Until]--
		}
	}
	if from != nil {
		deltas[*from] += 0
	}
	if until != nil {
		deltas[*until] += 0
	}
	points := make([]int64, 0, len(deltas))
	for tick := range deltas {
		points = append(points, tick)
	}
	sort.Slice(points, func(i, j int) bool { return points[i] < points[j] })
	for _, tick := range points {
		count += deltas[tick]
		if (from == nil || tick >= *from) && (until == nil || tick < *until) && (count < minimum || count > maximum) {
			return false
		}
	}
	return true
}

func intersectValidity(a, b Record) (*int64, *int64) {
	from, until := a.From, a.Until
	if b.From != nil && (from == nil || *b.From > *from) {
		from = b.From
	}
	if b.Until != nil && (until == nil || *b.Until < *until) {
		until = b.Until
	}
	return from, until
}

func validateAdditionalRule(rule Record, s Snapshot) error {
	if s[rule.Type].Kind != "type" || s[rule.Timeline].Kind != "timeline" {
		return bad("cardinality/exclusivity rules require a subject type and timeline")
	}
	minimum, maximum := 0, 1
	check := rule.Attributes["check"]
	var predicates map[string]bool
	if check == "relationship_cardinality" {
		if s[rule.Type].Relations[rule.Predicate] == "" {
			return bad("cardinality predicate must be declared by the subject type")
		}
		maximum = 5000
		if rule.Attributes["min"] == "" && rule.Attributes["max"] == "" {
			return bad("cardinality requires min or max")
		}
		for key, target := range map[string]*int{"min": &minimum, "max": &maximum} {
			if value := rule.Attributes[key]; value != "" {
				n, err := strconv.Atoi(value)
				if err != nil || n < 0 || n > 5000 {
					return bad("cardinality bounds must be integers from 0 to 5000")
				}
				*target = n
			}
		}
		if minimum > maximum {
			return bad("cardinality min cannot exceed max")
		}
	} else if check == "exclusive_facts" {
		var err error
		predicates, err = rulePredicates(rule.Attributes["predicates"], 2)
		if err != nil {
			return err
		}
		for predicate := range predicates {
			if s[rule.Type].Fields[predicate] != "boolean" {
				return bad("exclusive predicates must be declared boolean facts")
			}
		}
	} else {
		return bad("unsupported deterministic world rule")
	}
	bySubject := map[string][]Record{}
	for _, record := range s {
		if record.Timeline != rule.Timeline {
			continue
		}
		if (check == "relationship_cardinality" && record.Kind == "relationship" && record.Predicate == rule.Predicate) ||
			(check == "exclusive_facts" && record.Kind == "fact" && predicates[record.Predicate] && record.Value == true) {
			bySubject[record.Subject] = append(bySubject[record.Subject], record)
		}
	}
	for _, entity := range s {
		if entity.Kind != "entity" || entity.Type != rule.Type {
			continue
		}
		from, until := intersectValidity(rule, entity)
		if !countWithin(bySubject[entity.ID], from, until, minimum, maximum) {
			return bad("rule " + rule.ID + " violated by " + entity.ID + " in timeline " + rule.Timeline)
		}
	}
	return nil
}

func hasChronologyException(s Snapshot, event Record, predicates map[string]bool) bool {
	if len(predicates) == 0 {
		return false
	}
	for _, fact := range s {
		if fact.Kind == "fact" && fact.Subject == event.Subject && fact.Timeline == event.Timeline && predicates[fact.Predicate] && fact.Value == true && Visible(fact, event.Timeline, event.From) {
			return true
		}
	}
	return false
}
