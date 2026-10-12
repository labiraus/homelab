package worldbuilder

import "context"

type EvidenceScope struct {
	Coverage         string `json:"coverage"`
	EvidenceCount    int    `json:"evidenceCount"`
	CanonRecordCount int    `json:"canonRecordCount"`
	SearchMatches    int    `json:"searchMatches"`
	DiscoveryStatus  string `json:"discoveryStatus"`
}

func recordReferences(record Record) []string {
	refs := []string{record.Type, record.Subject, record.Target, record.Timeline, record.Source}
	if record.ValueType == "entity" {
		if value, ok := record.Value.(string); ok {
			refs = append(refs, value)
		}
	}
	for _, targetType := range record.Relations {
		refs = append(refs, targetType)
	}
	return refs
}

func (sc *scope) evidenceContext(ctx context.Context, s Snapshot, revision int64, timeline, question string, provisional Snapshot, requiredRefs ...string) ([]Record, EvidenceScope, error) {
	return sc.evidenceContextWithSearch(ctx, s, revision, timeline, question, provisional, nil, requiredRefs...)
}
func (sc *scope) evidenceContextWithSearch(ctx context.Context, s Snapshot, revision int64, timeline, question string, provisional Snapshot, ranked *SearchResult, requiredRefs ...string) ([]Record, EvidenceScope, error) {
	all := Records(s, timeline, "", nil)
	scope := EvidenceScope{Coverage: "complete_timeline", CanonRecordCount: len(all), EvidenceCount: len(all), DiscoveryStatus: "not_needed"}
	if len(all) <= 100 {
		return all, scope, nil
	}
	scope.Coverage = "selected_records"
	var search SearchResult
	if ranked != nil {
		search = *ranked
	} else {
		var err error
		search, err = sc.searchSnapshot(ctx, s, revision, timeline, question, nil, 20, 0)
		if err != nil {
			return nil, scope, err
		}
	}
	scope.SearchMatches, scope.DiscoveryStatus = search.Total, search.DiscoveryStatus
	selected := Snapshot{}
	// Add each record with its visible reference closure, or skip that complete
	// group if it does not fit. This never leaves a partially loaded type/source.
	include := func(seed string) bool {
		pending := []string{seed}
		group := Snapshot{}
		for len(pending) > 0 {
			id := pending[0]
			pending = pending[1:]
			if id == "" {
				continue
			}
			if _, ok := selected[id]; ok {
				continue
			}
			if _, ok := group[id]; ok {
				continue
			}
			record, ok := s[id]
			if !ok || !Visible(record, timeline, nil) {
				continue
			}
			group[id] = record
			if len(selected)+len(group) > 100 {
				return false
			}
			pending = append(pending, recordReferences(record)...)
		}
		for id, record := range group {
			selected[id] = record
		}
		return true
	}
	for _, record := range all {
		if record.Kind == "rule" || record.Kind == "world_settings" || record.ID == timeline {
			if !include(record.ID) {
				return nil, scope, bad("required rules/settings exceed the evidence budget")
			}
		}
	}
	for _, id := range requiredRefs {
		if !include(id) {
			return nil, scope, bad("required referenced records exceed evidence budget")
		}
	}
	for _, record := range Records(provisional, timeline, "", nil) {
		for _, id := range recordReferences(record) {
			if !include(id) {
				return nil, scope, bad("story references exceed the evidence budget")
			}
		}
	}
	subjects := map[string]bool{}
	for _, record := range search.Records {
		if include(record.ID) {
			if record.Kind == "entity" {
				subjects[record.ID] = true
			}
			if record.Subject != "" {
				subjects[record.Subject] = true
			}
			if record.Target != "" {
				subjects[record.Target] = true
			}
		}
	}
	for _, record := range all {
		if subjects[record.Subject] || subjects[record.Target] {
			include(record.ID)
		}
	}
	result := Records(selected, timeline, "", nil)
	scope.EvidenceCount = len(result)
	return result, scope, nil
}
