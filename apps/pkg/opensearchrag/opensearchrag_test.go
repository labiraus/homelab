package opensearchrag

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildSearchBodyIncludesFiltersAndPipelineInput(t *testing.T) {
	body := BuildSearchBody(SearchRequest{
		Query:      "ancient tower",
		DocumentID: "doc-1",
		Prefix:     "campaign/",
		Metadata:   map[string]any{"tag": "runbook"},
		Limit:      3,
	})
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("expected body to marshal: %v", err)
	}
	text := string(raw)
	for _, expected := range []string{
		`"query_text":"ancient tower"`,
		`"documentId.keyword":"doc-1"`,
		`"objectKey.keyword":"campaign/"`,
		`"metadata.tag.keyword":"runbook"`,
		`"passage_chunk.embedding"`,
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected search body to contain %s, got %s", expected, text)
		}
	}
}

func TestParseSearchResponseMapsInnerHitToChunk(t *testing.T) {
	raw := []byte(`{
		"hits": {
			"hits": [{
				"_id": "doc-1",
				"_score": 0.8,
				"_source": {
					"documentId": "doc-1",
					"sourceUri": "s3://documents/campaign/tower.html",
					"objectKey": "campaign/tower.html",
					"contentType": "text/html",
					"metadata": {"tag": "runbook"},
					"processingVersion": 4,
					"lastProcessedAt": "2026-04-14T12:00:00Z"
				},
				"inner_hits": {
					"_chunk_hits": {
						"hits": {
							"hits": [{
								"_score": 0.91,
								"_nested": {"field": "passage_chunk", "offset": 2},
								"_source": {"text": "Brazen Ward", "metadata": {"title": "Ancient Tower"}}
							}]
						}
					}
				}
			}]
		}
	}`)

	hits, err := ParseSearchResponse(raw)
	if err != nil {
		t.Fatalf("expected response to parse: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected one hit, got %#v", hits)
	}
	hit := hits[0]
	if hit.ChunkIndex != 2 || hit.ChunkText != "Brazen Ward" || hit.Similarity != 0.91 {
		t.Fatalf("expected inner hit chunk mapping, got %#v", hit)
	}
}
