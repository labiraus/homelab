package opensearchrag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	DefaultIndex          = "rag-documents"
	DefaultSearchPipeline = "rag-neural-search"
)

type Config struct {
	BaseURL        string
	Index          string
	SearchPipeline string
	Username       string
	Password       string
	HTTPClient     *http.Client
}

type SearchRequest struct {
	Query      string
	DocumentID string
	Prefix     string
	Metadata   map[string]any
	Limit      int
}

type Hit struct {
	DocumentID        string
	SourceURI         string
	ObjectKey         string
	ContentType       string
	Metadata          map[string]any
	ChunkID           int64
	ChunkIndex        int
	ChunkText         string
	ChunkMetadata     map[string]any
	ProcessingVersion int
	Distance          float64
	Similarity        float64
	LastProcessedAt   string
}

type searchResponse struct {
	Hits struct {
		Hits []searchHit `json:"hits"`
	} `json:"hits"`
}

type searchHit struct {
	ID        string          `json:"_id"`
	Score     float64         `json:"_score"`
	Source    json.RawMessage `json:"_source"`
	InnerHits map[string]struct {
		Hits struct {
			Hits []innerHit `json:"hits"`
		} `json:"hits"`
	} `json:"inner_hits"`
}

type innerHit struct {
	ID     string          `json:"_id"`
	Score  float64         `json:"_score"`
	Source json.RawMessage `json:"_source"`
	Nested *struct {
		Field  string `json:"field"`
		Offset int    `json:"offset"`
	} `json:"_nested"`
}

type sourceDocument struct {
	DocumentID        string         `json:"documentId"`
	SourceURI         string         `json:"sourceUri"`
	ObjectKey         string         `json:"objectKey"`
	ContentType       string         `json:"contentType"`
	Status            string         `json:"status"`
	Metadata          map[string]any `json:"metadata"`
	ProcessingVersion int            `json:"processingVersion"`
	LastProcessedAt   string         `json:"lastProcessedAt"`
	PassageChunks     []passageChunk `json:"passage_chunk"`
}

type passageChunk struct {
	Text     string         `json:"text"`
	Metadata map[string]any `json:"metadata"`
}

func ConfigFromEnv() Config {
	return Config{
		BaseURL:        strings.TrimSpace(os.Getenv("OPENSEARCH_BASE_URL")),
		Index:          envOrDefault("OPENSEARCH_RAG_INDEX", DefaultIndex),
		SearchPipeline: envOrDefault("OPENSEARCH_RAG_SEARCH_PIPELINE", DefaultSearchPipeline),
		Username:       strings.TrimSpace(os.Getenv("OPENSEARCH_USERNAME")),
		Password:       strings.TrimSpace(os.Getenv("OPENSEARCH_PASSWORD")),
		HTTPClient:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (c Config) Configured() bool {
	return strings.TrimSpace(c.BaseURL) != ""
}

func Search(ctx context.Context, config Config, request SearchRequest) ([]Hit, error) {
	if !config.Configured() {
		return nil, fmt.Errorf("OPENSEARCH_BASE_URL is not configured")
	}
	body, err := json.Marshal(BuildSearchBody(request))
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(config.BaseURL, "/") + "/" + url.PathEscape(config.Index) + "/_search"
	if strings.TrimSpace(config.SearchPipeline) != "" {
		endpoint += "?search_pipeline=" + url.QueryEscape(config.SearchPipeline)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Content-Type", "application/json")
	if config.Username != "" || config.Password != "" {
		httpRequest.SetBasicAuth(config.Username, config.Password)
	}

	client := config.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(response.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("opensearch search failed: %s: %s", response.Status, strings.TrimSpace(string(raw)))
	}

	return ParseSearchResponse(raw)
}

func BuildSearchBody(request SearchRequest) map[string]any {
	limit := request.Limit
	if limit <= 0 {
		limit = 8
	}
	filters := []any{
		map[string]any{"term": map[string]any{"status": "processed"}},
	}
	if strings.TrimSpace(request.DocumentID) != "" {
		filters = append(filters, map[string]any{"term": map[string]any{"documentId.keyword": request.DocumentID}})
	}
	if strings.TrimSpace(request.Prefix) != "" {
		filters = append(filters, map[string]any{"prefix": map[string]any{"objectKey.keyword": request.Prefix}})
	}
	for key, value := range request.Metadata {
		key = strings.TrimSpace(key)
		if key == "" || value == nil {
			continue
		}
		field := "metadata." + key
		if _, ok := value.(string); ok {
			field += ".keyword"
		}
		filters = append(filters, map[string]any{"term": map[string]any{field: value}})
	}

	nestedQuery := map[string]any{
		"nested": map[string]any{
			"path": "passage_chunk",
			"query": map[string]any{
				"knn": map[string]any{
					"passage_chunk.embedding": map[string]any{
						"vector": "${ext.ml_inference.params.query_embedding}",
						"k":      limit,
					},
				},
			},
			"inner_hits": map[string]any{
				"name": "_chunk_hits",
				"size": 1,
				"_source": map[string]any{
					"excludes": []string{"passage_chunk.embedding"},
				},
			},
		},
	}

	return map[string]any{
		"size": limit,
		"_source": map[string]any{
			"excludes": []string{"passage_chunk.embedding"},
		},
		"query": map[string]any{
			"bool": map[string]any{
				"filter": filters,
				"must":   []any{nestedQuery},
			},
		},
		"ext": map[string]any{
			"ml_inference": map[string]any{
				"params": map[string]any{
					"query_text": request.Query,
				},
			},
		},
	}
}

func ParseSearchResponse(raw []byte) ([]Hit, error) {
	var response searchResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	hits := []Hit{}
	for _, hit := range response.Hits.Hits {
		source, err := decodeSource(hit.Source)
		if err != nil {
			return nil, err
		}
		chunkIndex, chunkText, chunkMetadata, chunkScore := selectChunk(hit, source)
		score := chunkScore
		if score == 0 {
			score = hit.Score
		}
		hits = append(hits, Hit{
			DocumentID:        source.DocumentID,
			SourceURI:         source.SourceURI,
			ObjectKey:         source.ObjectKey,
			ContentType:       source.ContentType,
			Metadata:          source.Metadata,
			ChunkID:           stableChunkID(hit.ID, chunkIndex),
			ChunkIndex:        chunkIndex,
			ChunkText:         chunkText,
			ChunkMetadata:     chunkMetadata,
			ProcessingVersion: source.ProcessingVersion,
			Distance:          distanceFromScore(score),
			Similarity:        score,
			LastProcessedAt:   source.LastProcessedAt,
		})
	}
	return hits, nil
}

func decodeSource(raw json.RawMessage) (sourceDocument, error) {
	var source sourceDocument
	if len(raw) == 0 {
		return source, nil
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		return sourceDocument{}, err
	}
	if source.Metadata == nil {
		source.Metadata = map[string]any{}
	}
	return source, nil
}

func selectChunk(hit searchHit, source sourceDocument) (int, string, map[string]any, float64) {
	for _, group := range hit.InnerHits {
		if len(group.Hits.Hits) == 0 {
			continue
		}
		inner := group.Hits.Hits[0]
		var chunk passageChunk
		_ = json.Unmarshal(inner.Source, &chunk)
		index := 0
		if inner.Nested != nil {
			index = inner.Nested.Offset
		}
		return index, chunk.Text, normalizeMetadata(chunk.Metadata), inner.Score
	}
	if len(source.PassageChunks) == 0 {
		return 0, "", map[string]any{}, hit.Score
	}
	return 0, source.PassageChunks[0].Text, normalizeMetadata(source.PassageChunks[0].Metadata), hit.Score
}

func normalizeMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return map[string]any{}
	}
	return metadata
}

func stableChunkID(id string, chunkIndex int) int64 {
	var value int64 = 17
	for _, char := range id {
		value = value*31 + int64(char)
	}
	return value*31 + int64(chunkIndex)
}

func distanceFromScore(score float64) float64 {
	if score <= 0 {
		return 1
	}
	if score >= 1 {
		return 0
	}
	return 1 - score
}

func envOrDefault(name string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
