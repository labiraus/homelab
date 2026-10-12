package worldbuilder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"pkg/canonsearch"
	"sort"
	"time"
)

type HybridInput struct {
	Revision    int64           `json:"revision"`
	Timeline    string          `json:"timeline"`
	Query       string          `json:"query"`
	At          json.RawMessage `json:"at,omitempty"`
	Limit       int64           `json:"limit"`
	Offset      int64           `json:"offset"`
	SearchToken string          `json:"searchToken,omitempty"`
}

func visibleIDs(s Snapshot, timeline string, at *int64) []string {
	ids := []string{}
	for _, r := range Records(s, timeline, "", at) {
		ids = append(ids, r.ID)
	}
	return ids
}

// reciprocalRank combines canonical lexical ranks with bounded vector ranks.
// Every returned record, including semantic-only matches, is re-read from the
// authorized snapshot. The search index never supplies citation content.
func (sc *scope) reciprocalRank(s Snapshot, lexical SearchResult, d canonsearch.Discovery, query string, at *int64, limit, offset int64) (SearchResult, error) {
	if sc.vector == nil || d.WorldID != sc.world.ID || d.Revision != lexical.Revision || d.Visibility != "canon" || d.Profile != sc.vector.Model.Profile.ID() || len(d.Candidates) > 100 {
		return SearchResult{}, &Problem{502, "semantic_scope", "semantic candidate scope was rejected"}
	}
	scores := map[string]float64{}
	for rank, r := range lexical.Records {
		scores[r.ID] += 1 / float64(61+rank)
	}
	seen := map[string]bool{}
	for rank, c := range d.Candidates {
		r, ok := s[c.ID]
		if !ok || seen[c.ID] || !Visible(r, lexical.Timeline, at) {
			return SearchResult{}, &Problem{502, "semantic_scope", "semantic candidate visibility was rejected"}
		}
		seen[c.ID] = true
		scores[c.ID] += 1 / float64(61+rank)
	}
	ids := make([]string, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if scores[ids[i]] == scores[ids[j]] {
			return ids[i] < ids[j]
		}
		return scores[ids[i]] > scores[ids[j]]
	})
	raw, _ := json.Marshal(struct {
		World, Timeline, Query, Profile string
		Revision                        int64
		At                              *int64
		IDs                             []string
	}{sc.world.ID, lexical.Timeline, query, d.Profile, d.Revision, at, ids})
	digest := sha256.Sum256(raw)
	result := lexical
	result.Mode, result.DiscoveryStatus, result.SemanticProfile = "hybrid", "current_vector_index", d.Profile
	result.SearchToken = hex.EncodeToString(digest[:])
	result.IndexRevision = &d.Revision
	result.SemanticCandidates = len(d.Candidates)
	result.ReconciledCandidates = len(d.Candidates)
	result.Total, result.Records = len(ids), []Record{}
	start := min(offset, int64(len(ids)))
	end := start + min(limit, int64(len(ids))-start)
	for _, id := range ids[start:end] {
		result.Records = append(result.Records, s[id])
	}
	result.Citations = Citations(sc.world.ID, result.Revision, result.Timeline, result.Records)
	return result, nil
}

func (sc *scope) hybridSearch(w http.ResponseWriter, r *http.Request) (response any, returnErr error) {
	var b HybridInput
	if err := decode(w, r, &b); err != nil {
		return nil, err
	}
	if b.Limit == 0 {
		b.Limit = 100
	}
	if b.Limit < 1 || b.Limit > 500 || b.Offset < 0 || b.Offset > 1000000 {
		return nil, bad("invalid search page")
	}
	if (b.SearchToken != "" && len(b.SearchToken) != 64) || (b.Offset > 0 && len(b.SearchToken) != 64) {
		return nil, bad("continued hybrid search requires its prior searchToken")
	}
	if _, err := hex.DecodeString(b.SearchToken); err != nil {
		return nil, bad("invalid searchToken")
	}
	if sc.vector == nil || sc.budget == nil {
		return nil, &Problem{503, "semantic_unavailable", "semantic search is not configured"}
	}
	if b.Revision != sc.world.Revision {
		return nil, conflict("semantic search requires current canon; use lexical search for history")
	}
	ctx := r.Context()
	snapshot, err := sc.snapshot(ctx, b.Revision)
	if err != nil {
		return nil, err
	}
	at, err := parseTick(b.At)
	if err != nil {
		return nil, bad(err.Error())
	}
	// Normalize equivalent numeric/string ticks before hashing the intent.
	if at != nil {
		b.At, _ = json.Marshal(*at)
	} else {
		b.At = nil
	}
	lexical, err := sc.lexicalCandidates(ctx, snapshot, b.Revision, b.Timeline, b.Query, at)
	if err != nil {
		return nil, err
	}
	if err = sc.vectorReady(ctx, b.Revision); err != nil {
		return nil, err
	}
	// Bind the immutable request hash to all inputs before attempting inference.
	intent, _ := json.Marshal(struct {
		Input   HybridInput
		Profile string
	}{b, sc.vector.Model.Profile.ID()})
	provenance, err := sc.budget.reserve(ctx, sc.actor, "", ReasonRequest{WorldID: sc.world.ID, Revision: b.Revision, Timeline: b.Timeline, Question: b.Query, Workflow: "canon_search", ContextScope: string(intent)})
	if err != nil {
		return nil, err
	}
	started := time.Now()
	var d canonsearch.Discovery
	defer func() {
		provenance.LatencyMS = time.Since(started).Milliseconds()
		if e := sc.budget.finish(provenance, ReasonResult{Model: d.Usage.Model, Usage: map[string]any{"retrieval": d.Usage}}, returnErr == nil); e != nil && returnErr == nil {
			response = nil
			returnErr = &Problem{503, "inference_ledger", "inference completion could not be recorded"}
		}
	}()
	d, err = sc.vector.Find(ctx, sc.world.ID, b.Revision, visibleIDs(snapshot, b.Timeline, at), b.Query, provenance.RequestID)
	if err != nil {
		return nil, &Problem{502, "semantic_unavailable", "semantic retrieval failed; the attempt remains charged to your allowance"}
	}
	result, err := sc.reciprocalRank(snapshot, lexical, d, b.Query, at, b.Limit, b.Offset)
	if err != nil {
		return nil, err
	}
	if b.SearchToken != "" && result.SearchToken != b.SearchToken {
		return nil, conflict("semantic ranking changed; restart search from the first page")
	}
	result.Provenance = &provenance
	return result, nil
}

func (sc *scope) lexicalCandidates(ctx context.Context, s Snapshot, revision int64, timeline, query string, at *int64) (SearchResult, error) {
	copy := *sc
	copy.discover = nil
	return copy.searchSnapshot(ctx, s, revision, timeline, query, at, 5000, 0)
}
func (sc *scope) vectorReady(ctx context.Context, revision int64) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	ready, err := sc.vector.Ready(ctx, sc.world.ID, revision)
	if err != nil {
		return &Problem{503, "semantic_unavailable", "semantic index is unavailable"}
	}
	if !ready {
		return conflict("semantic index is catching up; use lexical search or retry later")
	}
	return nil
}
