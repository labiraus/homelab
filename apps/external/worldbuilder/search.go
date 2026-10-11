package worldbuilder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"
)

type Discovery struct {
	WorldID    string   `json:"worldId"`
	Revision   int64    `json:"revision"`
	Visibility string   `json:"visibility"`
	IDs        []string `json:"ids"`
}
type Discover func(context.Context, string, string, int64, string) (Discovery, error)

// OpenSearch supplies bounded candidate IDs only. All answer content and final
// filtering come from the authorised PostgreSQL snapshot in searchSnapshot.
func OpenSearchDiscovery(endpoint, user, password string) Discover {
	if strings.TrimSpace(endpoint) == "" {
		return nil
	}
	client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return func(ctx context.Context, world, timeline string, revision int64, query string) (Discovery, error) {
		body := map[string]any{"size": 1, "_source": []string{"worldId", "revision", "visibility"}, "query": map[string]any{"bool": map[string]any{
			"filter": []any{map[string]any{"term": map[string]any{"worldId": world}}, map[string]any{"term": map[string]any{"visibility": "canon"}}, map[string]any{"range": map[string]any{"revision": map[string]int64{"lte": revision}}}},
			"must": []any{map[string]any{"nested": map[string]any{"path": "records", "query": map[string]any{"bool": map[string]any{
				"must": []any{map[string]any{"match": map[string]any{"records.text": query}}},
				"filter": []any{map[string]any{"bool": map[string]any{"minimum_should_match": 1, "should": []any{
					map[string]any{"term": map[string]any{"records.timeline": timeline}},
					map[string]any{"term": map[string]any{"records.kind": "world_settings"}},
					map[string]any{"bool": map[string]any{"must_not": map[string]any{"exists": map[string]any{"field": "records.timeline"}}}},
				}}}},
			}}, "inner_hits": map[string]any{"size": 100, "_source": []string{"records.id"}}}}},
		}}}
		raw, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/worldbuilder-canon-v1/_search", bytes.NewReader(raw))
		if err != nil {
			return Discovery{}, err
		}
		req.Header.Set("Content-Type", "application/json")
		if user != "" {
			req.SetBasicAuth(user, password)
		}
		response, err := client.Do(req)
		if err != nil {
			return Discovery{}, err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return Discovery{}, fmt.Errorf("canon discovery returned status %d", response.StatusCode)
		}
		raw, err = io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
		if err != nil || len(raw) > 1<<20 {
			return Discovery{}, fmt.Errorf("invalid discovery response size")
		}
		var result struct {
			Hits struct {
				Hits []struct {
					Source Discovery `json:"_source"`
					Inner  map[string]struct {
						Hits struct {
							Hits []struct {
								Source struct {
									ID string `json:"id"`
								} `json:"_source"`
							} `json:"hits"`
						} `json:"hits"`
					} `json:"inner_hits"`
				} `json:"hits"`
			} `json:"hits"`
		}
		if err = json.Unmarshal(raw, &result); err != nil {
			return Discovery{}, err
		}
		if len(result.Hits.Hits) == 0 {
			return Discovery{WorldID: world, Revision: -1, Visibility: "canon", IDs: []string{}}, nil
		}
		hit := result.Hits.Hits[0]
		discovery := hit.Source
		discovery.IDs = []string{}
		for _, record := range hit.Inner["records"].Hits.Hits {
			discovery.IDs = append(discovery.IDs, record.Source.ID)
		}
		return discovery, nil
	}
}

type SearchResult struct {
	ReviewPolicy         ReviewPolicy `json:"reviewPolicy"`
	Calendar             *Calendar    `json:"calendar"`
	World                World        `json:"world"`
	Revision             int64        `json:"revision"`
	Timeline             string       `json:"timeline"`
	Records              []Record     `json:"records"`
	Citations            []Evidence   `json:"citations"`
	Total                int          `json:"total"`
	DiscoveryStatus      string       `json:"discoveryStatus"`
	IndexRevision        *int64       `json:"indexRevision,omitempty"`
	ReconciledCandidates int          `json:"reconciledCandidates"`
}

func queryTerms(query string) []string {
	parts := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	seen := map[string]bool{}
	terms := []string{}
	for _, part := range parts {
		if !seen[part] {
			seen[part] = true
			terms = append(terms, part)
		}
	}
	return terms
}

func (sc *scope) searchSnapshot(ctx context.Context, s Snapshot, revision int64, timeline, query string, at *int64, limit, offset int64) (SearchResult, error) {
	result := SearchResult{World: sc.world, Revision: revision, Timeline: timeline, Calendar: snapshotCalendar(s), ReviewPolicy: snapshotReviewPolicy(s), Records: []Record{}, Citations: []Evidence{}, DiscoveryStatus: "postgres_only"}
	terms := queryTerms(query)
	if len(query) > 4000 || len(terms) == 0 || len(terms) > 64 {
		return result, bad("search requires 1–64 terms and at most 4000 characters")
	}
	if s[timeline].Kind != "timeline" {
		return result, bad("unknown timeline")
	}
	candidates := map[string]bool{}
	if revision != sc.world.Revision {
		result.DiscoveryStatus = "historical_snapshot"
	} else if sc.discover != nil {
		d, err := sc.discover(ctx, sc.world.ID, timeline, revision, query)
		switch {
		case err != nil:
			result.DiscoveryStatus = "index_unavailable"
		case d.WorldID != sc.world.ID || d.Visibility != "canon" || d.Revision > revision || d.Revision < -1 || len(d.IDs) > 100:
			result.DiscoveryStatus = "index_scope_rejected"
		case d.Revision == -1:
			result.DiscoveryStatus = "no_index_matches"
		default:
			result.IndexRevision = &d.Revision
			result.DiscoveryStatus = "current_index"
			if d.Revision < revision {
				result.DiscoveryStatus = "stale_index_reconciled"
			}
			for _, id := range d.IDs {
				candidates[id] = true
			}
		}
	}
	type ranked struct {
		record Record
		score  int
	}
	matches := []ranked{}
	// Bounded complete canon scan fills gaps in a lagging/missing projection.
	// Canonical scores and ID tie-breaks keep paging stable across index rebuilds.
	for _, record := range Records(s, timeline, "", at) {
		raw, _ := json.Marshal(record)
		haystack := strings.ToLower(string(raw))
		score := 0
		for _, term := range terms {
			if strings.Contains(haystack, term) {
				score++
			}
		}
		if score == 0 {
			continue
		}
		if candidates[record.ID] {
			result.ReconciledCandidates++
		}
		matches = append(matches, ranked{record, score})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			return matches[i].record.ID < matches[j].record.ID
		}
		return matches[i].score > matches[j].score
	})
	result.Total = len(matches)
	start := min(offset, int64(len(matches)))
	end := start + min(limit, int64(len(matches))-start)
	for _, match := range matches[start:end] {
		result.Records = append(result.Records, match.record)
	}
	result.Citations = Citations(sc.world.ID, revision, timeline, result.Records)
	return result, nil
}
