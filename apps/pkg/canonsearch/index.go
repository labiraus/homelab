package canonsearch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxIndexBytes = 96 << 20

type Index struct {
	Endpoint, Username, Password string
	Model                        *Embedder
	Client                       *http.Client
}
type IndexedRecord struct {
	ID     string    `json:"id"`
	Digest string    `json:"digest"`
	Vector []float32 `json:"vector"`
}
type Document struct {
	WorldID    string          `json:"worldId"`
	Revision   int64           `json:"revision"`
	Visibility string          `json:"visibility"`
	Profile    string          `json:"profile"`
	Records    []IndexedRecord `json:"records"`
}
type Candidate struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
}
type Discovery struct {
	WorldID    string      `json:"worldId"`
	Revision   int64       `json:"revision"`
	Visibility string      `json:"visibility"`
	Profile    string      `json:"profile"`
	Candidates []Candidate `json:"candidates"`
	Usage      Usage       `json:"usage"`
}

func (i *Index) Name() string { return "worldbuilder-canon-vector-" + i.Model.Profile.ID() }
func (i *Index) Validate() error {
	if i.Model == nil {
		return fmt.Errorf("embedding profile required")
	}
	if err := i.Model.Validate(); err != nil {
		return err
	}
	return endpoint(i.Endpoint)
}
func (i *Index) request(ctx context.Context, method, path string, body []byte, maximum int64) (int, []byte, error) {
	if len(body) > maxIndexBytes {
		return 0, nil, fmt.Errorf("canon vector projection exceeds byte limit")
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(i.Endpoint, "/")+"/"+i.Name()+path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if i.Username != "" {
		req.SetBasicAuth(i.Username, i.Password)
	}
	res, err := httpClient(i.Client, 30*time.Second).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, maximum+1))
	if err != nil || int64(len(raw)) > maximum {
		return res.StatusCode, nil, fmt.Errorf("invalid vector index response size")
	}
	return res.StatusCode, raw, nil
}
func (i *Index) Ensure(ctx context.Context) error {
	if err := i.Validate(); err != nil {
		return err
	}
	metadata := map[string]any{"contract": "canon-vector-mean-v1", "profile": i.Model.Profile.ID()}
	mapping := map[string]any{"settings": map[string]any{"number_of_shards": 1, "index.knn": false}, "mappings": map[string]any{"dynamic": "strict", "_meta": metadata, "properties": map[string]any{
		"worldId": map[string]string{"type": "keyword"}, "revision": map[string]string{"type": "long"}, "visibility": map[string]string{"type": "keyword"}, "profile": map[string]string{"type": "keyword"},
		"records": map[string]any{"type": "nested", "properties": map[string]any{"id": map[string]string{"type": "keyword"}, "digest": map[string]string{"type": "keyword"}, "vector": map[string]any{"type": "knn_vector", "dimension": i.Model.Profile.Dimensions}}}}}}
	status, _, err := i.request(ctx, "HEAD", "", nil, 4096)
	if err != nil {
		return err
	}
	if status == 404 {
		raw, _ := json.Marshal(mapping)
		status, body, err := i.request(ctx, "PUT", "", raw, 65536)
		if err != nil {
			return err
		}
		if status != 200 && status != 201 && !(status == 400 && bytes.Contains(body, []byte("resource_already_exists_exception"))) {
			return fmt.Errorf("vector index create returned %d", status)
		}
	} else if status != 200 {
		return fmt.Errorf("vector index check returned %d", status)
	}
	status, raw, err := i.request(ctx, "GET", "/_mapping", nil, 65536)
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("vector mapping returned %d", status)
	}
	var maps map[string]struct {
		Mappings struct {
			Dynamic string `json:"dynamic"`
			Meta    struct {
				Contract string `json:"contract"`
				Profile  string `json:"profile"`
			} `json:"_meta"`
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"mappings"`
	}
	if err = json.Unmarshal(raw, &maps); err != nil {
		return err
	}
	m, ok := maps[i.Name()]
	if !ok || m.Mappings.Dynamic != "strict" || m.Mappings.Meta.Contract != "canon-vector-mean-v1" || m.Mappings.Meta.Profile != i.Model.Profile.ID() {
		return fmt.Errorf("incompatible vector mapping profile")
	}
	var records struct {
		Type       string `json:"type"`
		Properties map[string]struct {
			Type      string `json:"type"`
			Dimension int    `json:"dimension"`
		} `json:"properties"`
	}
	if err = json.Unmarshal(m.Mappings.Properties["records"], &records); err != nil {
		return err
	}
	if records.Type != "nested" || records.Properties["id"].Type != "keyword" || records.Properties["digest"].Type != "keyword" || records.Properties["vector"].Type != "knn_vector" || records.Properties["vector"].Dimension != i.Model.Profile.Dimensions {
		return fmt.Errorf("incompatible vector record mapping")
	}
	for key, kind := range map[string]string{"worldId": "keyword", "revision": "long", "visibility": "keyword", "profile": "keyword"} {
		var field struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(m.Mappings.Properties[key], &field) != nil || field.Type != kind {
			return fmt.Errorf("incompatible vector scope mapping")
		}
	}
	return nil
}
func (i *Index) current(ctx context.Context, world string, includeVectors bool) (Document, bool, error) {
	path := "/_doc/" + url.PathEscape(world)
	if !includeVectors {
		path += "?_source_excludes=records"
	}
	status, raw, err := i.request(ctx, "GET", path, nil, maxIndexBytes)
	if err != nil {
		return Document{}, false, err
	}
	if status == 404 {
		return Document{}, false, nil
	}
	if status != 200 {
		return Document{}, false, fmt.Errorf("vector snapshot returned %d", status)
	}
	var hit struct {
		Source Document `json:"_source"`
	}
	if err = json.Unmarshal(raw, &hit); err != nil {
		return Document{}, false, err
	}
	d := hit.Source
	if d.WorldID != world || d.Visibility != "canon" || d.Revision < 0 || d.Profile != i.Model.Profile.ID() || len(d.Records) > 5000 {
		return Document{}, false, fmt.Errorf("vector snapshot scope/profile mismatch")
	}
	return d, true, nil
}

// Ready performs no inference; callers can refuse stale/missing projections
// before reserving an interactive attempt.
func (i *Index) Ready(ctx context.Context, world string, revision int64) (bool, error) {
	if err := i.Validate(); err != nil {
		return false, err
	}
	if world == "" || revision < 0 {
		return false, fmt.Errorf("invalid vector readiness scope")
	}
	d, found, err := i.current(ctx, world, false)
	return found && d.Revision == revision, err
}

// Publish receives only an immutable, authorized published snapshot from the
// relay. Existing record hashes reuse model vectors within this exact profile.
func (i *Index) Publish(ctx context.Context, world string, revision int64, snapshot []byte, requestID string) (Usage, error) {
	usage := Usage{}
	if err := i.Validate(); err != nil {
		return usage, err
	}
	if world == "" || revision < 0 || revision == math.MaxInt64 {
		return usage, fmt.Errorf("invalid vector publication scope")
	}
	previous, found, err := i.current(ctx, world, true)
	if err != nil {
		return usage, err
	}
	if found && previous.Revision > revision {
		return usage, nil
	}
	var source map[string]map[string]json.RawMessage
	if err = json.Unmarshal(snapshot, &source); err != nil {
		return usage, err
	}
	if len(source) > 5000 {
		return usage, fmt.Errorf("canon record limit exceeded")
	}
	cached := map[string]IndexedRecord{}
	for _, r := range previous.Records {
		if _, exists := cached[r.ID]; exists {
			return usage, fmt.Errorf("duplicate cached record")
		}
		if !vectorValid(r.Vector, i.Model.Profile.Dimensions) {
			return usage, fmt.Errorf("invalid cached vector")
		}
		cached[r.ID] = r
	}
	ids := make([]string, 0, len(source))
	for id := range source {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	doc := Document{world, revision, "canon", i.Model.Profile.ID(), []IndexedRecord{}}
	pending := []int{}
	texts := []string{}
	flush := func() error {
		if len(texts) == 0 {
			return nil
		}
		vectors, u, e := i.Model.Texts(ctx, texts, requestID, false)
		usage.Requests += u.Requests
		usage.LatencyMS += u.LatencyMS
		usage.InputTokens += u.InputTokens
		if u.Model != "" {
			usage.Model = u.Model
		}
		if e != nil {
			return e
		}
		for n, pos := range pending {
			doc.Records[pos].Vector = vectors[n]
		}
		pending = nil
		texts = nil
		return nil
	}
	for _, id := range ids {
		raw, _ := json.Marshal(source[id])
		var recordID string
		if json.Unmarshal(source[id]["id"], &recordID) != nil || recordID != id || id == "" || len(id) > 64 {
			return usage, fmt.Errorf("invalid canonical record identity")
		}
		hash := sha256.Sum256(raw)
		digest := hex.EncodeToString(hash[:])
		record := IndexedRecord{ID: id, Digest: digest}
		if old, ok := cached[id]; ok && old.Digest == digest {
			record.Vector = old.Vector
		}
		doc.Records = append(doc.Records, record)
		if record.Vector == nil {
			pending = append(pending, len(doc.Records)-1)
			texts = append(texts, string(raw))
			if len(texts) == MaxBatch {
				if err = flush(); err != nil {
					return usage, err
				}
			}
		}
	}
	if err = flush(); err != nil {
		return usage, err
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return usage, err
	}
	status, _, err := i.request(ctx, "PUT", "/_doc/"+url.PathEscape(world)+"?version="+strconv.FormatInt(revision+1, 10)+"&version_type=external_gte&refresh=wait_for", raw, 65536)
	if err != nil {
		return usage, err
	}
	if status != 200 && status != 201 && status != 409 {
		return usage, fmt.Errorf("vector publication returned %d", status)
	}
	return usage, nil
}

// Find uses exact filtered vector scoring, suitable for the bounded 5000-record
// canon contract. IDs come from a canonical visibility/time-filtered snapshot.
func (i *Index) Find(ctx context.Context, world string, revision int64, ids []string, query, requestID string) (Discovery, error) {
	if err := i.Validate(); err != nil {
		return Discovery{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	if world == "" || revision < 0 || len(query) == 0 || len(query) > 4000 {
		return Discovery{}, fmt.Errorf("invalid interactive vector scope/query")
	}
	d := Discovery{WorldID: world, Revision: revision, Visibility: "canon", Profile: i.Model.Profile.ID(), Candidates: []Candidate{}}
	if len(ids) > 5000 {
		return d, fmt.Errorf("candidate scope exceeds canon bounds")
	}
	if len(ids) == 0 {
		return d, nil
	}
	vector, usage, err := i.Model.Mean(ctx, query, requestID, true)
	d.Usage = usage
	if err != nil {
		return d, err
	}
	body := map[string]any{"size": 1, "_source": []string{"worldId", "revision", "visibility", "profile"}, "query": map[string]any{"bool": map[string]any{
		"filter": []any{map[string]any{"term": map[string]any{"worldId": world}}, map[string]any{"term": map[string]any{"revision": revision}}, map[string]any{"term": map[string]any{"visibility": "canon"}}, map[string]any{"term": map[string]any{"profile": i.Model.Profile.ID()}}},
		"must":   []any{map[string]any{"nested": map[string]any{"path": "records", "query": map[string]any{"script_score": map[string]any{"query": map[string]any{"terms": map[string]any{"records.id": ids}}, "script": map[string]any{"lang": "knn", "source": "knn_score", "params": map[string]any{"field": "records.vector", "query_value": vector, "space_type": "cosinesimil"}}}}, "inner_hits": map[string]any{"size": 100, "track_scores": true, "_source": []string{"records.id"}, "sort": []any{map[string]string{"_score": "desc"}, map[string]string{"records.id": "asc"}}}}}}}}}
	raw, _ := json.Marshal(body)
	status, raw, err := i.request(ctx, "POST", "/_search", raw, 1<<20)
	if err != nil {
		return d, err
	}
	if status != 200 {
		return d, fmt.Errorf("vector search returned %d", status)
	}
	var result struct {
		TimedOut *bool                                    `json:"timed_out"`
		Shards   struct{ Total, Successful, Failed *int } `json:"_shards"`
		Hits     struct {
			Hits []struct {
				Source Document `json:"_source"`
				Inner  map[string]struct {
					Hits struct {
						Hits []struct {
							Source struct {
								ID string `json:"id"`
							} `json:"_source"`
							Score *float64          `json:"_score"`
							Sort  []json.RawMessage `json:"sort"`
						} `json:"hits"`
					} `json:"hits"`
				} `json:"inner_hits"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return d, err
	}
	if result.TimedOut == nil || *result.TimedOut || result.Shards.Total == nil || result.Shards.Successful == nil || result.Shards.Failed == nil || *result.Shards.Total < 1 || *result.Shards.Successful != *result.Shards.Total || *result.Shards.Failed != 0 {
		return d, fmt.Errorf("incomplete vector search execution")
	}
	if len(result.Hits.Hits) != 1 {
		return d, fmt.Errorf("vector index changed or returned ambiguous scope")
	}
	hit := result.Hits.Hits[0]
	if hit.Source.WorldID != world || hit.Source.Revision != revision || hit.Source.Visibility != "canon" || hit.Source.Profile != i.Model.Profile.ID() {
		return d, fmt.Errorf("vector candidate scope/profile mismatch")
	}
	allowed := map[string]bool{}
	for _, id := range ids {
		allowed[id] = true
	}
	seen := map[string]bool{}
	for _, h := range hit.Inner["records"].Hits.Hits {
		// OpenSearch 3.9 can emit null _score for sorted inner hits even with
		// track_scores. The explicit [_score, records.id] sort values preserve
		// the real vector ranking; missing scores must never become zero.
		var score *float64
		var id string
		if len(h.Sort) != 2 || json.Unmarshal(h.Sort[0], &score) != nil || json.Unmarshal(h.Sort[1], &id) != nil || score == nil || id != h.Source.ID {
			return d, fmt.Errorf("missing vector sort evidence")
		}
		if !allowed[h.Source.ID] || seen[h.Source.ID] || math.IsNaN(*score) || math.IsInf(*score, 0) || *score < 0 || *score > 2.000001 || (h.Score != nil && *h.Score != *score) {
			return d, fmt.Errorf("invalid vector candidate")
		}
		if len(d.Candidates) > 0 {
			previous := d.Candidates[len(d.Candidates)-1]
			if previous.Score < *score || (previous.Score == *score && previous.ID >= id) {
				return d, fmt.Errorf("invalid vector candidate order")
			}
		}
		seen[h.Source.ID] = true
		d.Candidates = append(d.Candidates, Candidate{h.Source.ID, *score})
	}
	if len(d.Candidates) != min(100, len(allowed)) {
		return d, fmt.Errorf("vector candidate coverage mismatch")
	}
	return d, nil
}
