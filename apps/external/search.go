package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"pkg/embeddingutil"
	"pkg/postgresutil"
	"pkg/prometheusutil"
)

const (
	documentSearchLabel         = "documentSearchHandler"
	documentContextLabel        = "documentContextHandler"
	defaultSearchLimit          = 8
	maxSearchLimit              = 20
	defaultContextLimit         = 6
	defaultContextMaxCharacters = 6000
	maxContextMaxCharacters     = 20000
	defaultEmbeddingModel       = embeddingutil.DefaultModel
)

type queryEmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
}

type documentSearchRow struct {
	DocumentID        string
	SourceURI         string
	ObjectKey         string
	ContentType       string
	MetadataRaw       string
	ChunkID           int64
	ChunkIndex        int
	ChunkText         string
	ProcessingVersion int
	Distance          float64
	LastProcessedAt   *time.Time
}

var (
	fetchQueryEmbedding = getQueryEmbedding
	searchDocuments     = queryDocumentSearch
	assembleContext     = assembleDocumentContext
	httpClient          = &http.Client{Timeout: 30 * time.Second}
)

func documentSearchHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	startTime := time.Now()
	prometheusutil.IncrementProcessed(documentSearchLabel, "call")

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
			w.WriteHeader(http.StatusInternalServerError)
		}
		if err != nil {
			slog.ErrorContext(r.Context(), err.Error())
			prometheusutil.IncrementProcessed(documentSearchLabel, "error")
		}
		prometheusutil.OpDuration(documentSearchLabel, time.Since(startTime))
	}()

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !postgresConfigured() || !embeddingsConfigured() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "document search is unavailable"})
		return
	}

	var request DocumentSearchRequest
	if err = json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid request body"})
		return
	}

	request.Query = strings.TrimSpace(request.Query)
	request.DocumentID = strings.TrimSpace(request.DocumentID)
	request.Prefix = normalizePrefix(request.Prefix)
	request.Metadata = normalizeMetadataFilter(request.Metadata)
	switch {
	case request.Query == "":
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "query is required"})
		return
	case request.Limit < 0:
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "limit must be positive"})
		return
	}

	limit := request.Limit
	if limit == 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	embedding, model, err := fetchQueryEmbedding(r.Context(), request.Query)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "could not embed search query"})
		return
	}

	hits, err := searchDocuments(r.Context(), embedding, model, request, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "could not search documents"})
		return
	}

	response := DocumentSearchResponse{
		Query: request.Query,
		Hits:  hits,
	}

	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(response); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func documentContextHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	startTime := time.Now()
	prometheusutil.IncrementProcessed(documentContextLabel, "call")

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
			w.WriteHeader(http.StatusInternalServerError)
		}
		if err != nil {
			slog.ErrorContext(r.Context(), err.Error())
			prometheusutil.IncrementProcessed(documentContextLabel, "error")
		}
		prometheusutil.OpDuration(documentContextLabel, time.Since(startTime))
	}()

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !postgresConfigured() || !embeddingsConfigured() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "document context is unavailable"})
		return
	}

	var request DocumentContextRequest
	if err = json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid request body"})
		return
	}

	searchRequest, limit, maxChars, validationErr := normalizeDocumentContextRequest(request)
	if validationErr != "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: validationErr})
		return
	}

	embedding, model, err := fetchQueryEmbedding(r.Context(), searchRequest.Query)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "could not embed context query"})
		return
	}

	hits, err := searchDocuments(r.Context(), embedding, model, searchRequest, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "could not assemble document context"})
		return
	}

	response := assembleContext(searchRequest.Query, hits, maxChars)

	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(response); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func normalizeDocumentContextRequest(request DocumentContextRequest) (DocumentSearchRequest, int, int, string) {
	searchRequest := DocumentSearchRequest{
		Query:      strings.TrimSpace(request.Query),
		DocumentID: strings.TrimSpace(request.DocumentID),
		Prefix:     normalizePrefix(request.Prefix),
		Metadata:   normalizeMetadataFilter(request.Metadata),
		Limit:      request.Limit,
	}

	switch {
	case searchRequest.Query == "":
		return DocumentSearchRequest{}, 0, 0, "query is required"
	case request.Limit < 0:
		return DocumentSearchRequest{}, 0, 0, "limit must be positive"
	case request.MaxChars < 0:
		return DocumentSearchRequest{}, 0, 0, "maxChars must be positive"
	}

	limit := request.Limit
	if limit == 0 {
		limit = defaultContextLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	maxChars := request.MaxChars
	if maxChars == 0 {
		maxChars = defaultContextMaxCharacters
	}
	if maxChars > maxContextMaxCharacters {
		maxChars = maxContextMaxCharacters
	}

	return searchRequest, limit, maxChars, ""
}

func getQueryEmbedding(ctx context.Context, input string) ([]float64, string, error) {
	if useLocalEmbeddings() {
		return embeddingutil.EmbedText(input), embeddingModel(), nil
	}

	payload, err := json.Marshal(map[string]any{
		"model": embeddingModel(),
		"input": input,
	})
	if err != nil {
		return nil, "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, embeddingEndpoint(), bytes.NewReader(payload))
	if err != nil {
		return nil, "", err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, "", fmt.Errorf("embedding request failed: %s", response.Status)
	}

	var body queryEmbeddingResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return nil, "", err
	}
	if len(body.Data) == 0 || len(body.Data[0].Embedding) == 0 {
		return nil, "", fmt.Errorf("embedding response did not include a vector")
	}

	return body.Data[0].Embedding, defaultString(strings.TrimSpace(body.Model), embeddingModel()), nil
}

func queryDocumentSearch(
	ctx context.Context,
	embedding []float64,
	model string,
	request DocumentSearchRequest,
	limit int,
) ([]DocumentSearchHit, error) {
	if postgresutil.Query == nil {
		return nil, fmt.Errorf("postgres is not initialized")
	}

	query := documentSearchBaseQuery()

	args := []any{toVectorLiteral(embedding), model}
	nextArg := 3

	if request.DocumentID != "" {
		query += fmt.Sprintf("\n\tAND d.document_id = $%d", nextArg)
		args = append(args, request.DocumentID)
		nextArg++
	}

	if request.Prefix != "" {
		query += fmt.Sprintf("\n\tAND d.object_key LIKE $%d", nextArg)
		args = append(args, request.Prefix+"%")
		nextArg++
	}

	if len(request.Metadata) > 0 {
		metadataFilter, err := json.Marshal(request.Metadata)
		if err != nil {
			return nil, err
		}
		query += fmt.Sprintf("\n\tAND d.metadata @> $%d::jsonb", nextArg)
		args = append(args, string(metadataFilter))
		nextArg++
	}

	query += fmt.Sprintf("\nORDER BY e.vector <=> $1::vector\nLIMIT $%d", nextArg)
	args = append(args, limit)

	rows, err := postgresutil.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hits := []DocumentSearchHit{}
	for rows.Next() {
		var row documentSearchRow
		if err := rows.Scan(
			&row.DocumentID,
			&row.SourceURI,
			&row.ObjectKey,
			&row.ContentType,
			&row.MetadataRaw,
			&row.ChunkID,
			&row.ChunkIndex,
			&row.ChunkText,
			&row.ProcessingVersion,
			&row.Distance,
			&row.LastProcessedAt,
		); err != nil {
			return nil, err
		}

		metadata, err := decodeDocumentMetadata(row.MetadataRaw)
		if err != nil {
			return nil, err
		}

		hit := DocumentSearchHit{
			DocumentID:        row.DocumentID,
			SourceURI:         row.SourceURI,
			ObjectKey:         row.ObjectKey,
			ContentType:       row.ContentType,
			Metadata:          metadata,
			ChunkID:           row.ChunkID,
			ChunkIndex:        row.ChunkIndex,
			ChunkText:         row.ChunkText,
			ProcessingVersion: row.ProcessingVersion,
			Distance:          row.Distance,
			Similarity:        maxSimilarity(row.Distance),
			Citation:          buildDocumentCitation(row),
		}
		if row.LastProcessedAt != nil && !row.LastProcessedAt.IsZero() {
			hit.LastProcessedAt = row.LastProcessedAt.UTC().Format(time.RFC3339)
		}
		hits = append(hits, hit)
	}

	return hits, rows.Err()
}

func documentSearchBaseQuery() string {
	return `
SELECT
	d.document_id,
	d.source_uri,
	COALESCE(d.object_key, ''),
	COALESCE(d.content_type, ''),
	COALESCE(d.metadata::text, '{}'),
	c.id,
	c.chunk_index,
	c.chunk_text,
	c.processing_version,
	e.vector <=> $1::vector AS distance,
	d.last_processed_at
FROM rag.embeddings e
JOIN rag.chunks c ON c.id = e.chunk_id
JOIN rag.documents d ON d.id = c.document_pk
WHERE d.status = 'processed'
	AND e.model = $2
	AND c.processing_version = d.current_processing_version
	AND e.vector IS NOT NULL`
}

func normalizeMetadataFilter(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}

	normalized := map[string]any{}
	for key, value := range metadata {
		key = strings.TrimSpace(key)
		if key == "" || value == nil {
			continue
		}
		if stringValue, ok := value.(string); ok {
			value = strings.TrimSpace(stringValue)
		}
		if value == "" {
			continue
		}
		normalized[key] = value
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func decodeDocumentMetadata(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}

	var metadata map[string]any
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return nil, err
	}
	if metadata == nil {
		return map[string]any{}, nil
	}
	return metadata, nil
}

func assembleDocumentContext(query string, hits []DocumentSearchHit, maxChars int) DocumentContextResponse {
	if maxChars <= 0 {
		maxChars = defaultContextMaxCharacters
	}

	var context strings.Builder
	citations := []DocumentContextCitation{}
	truncated := false

	for index, hit := range hits {
		reference := fmt.Sprintf("[%d]", index+1)
		label := reference
		if hit.Citation != nil && strings.TrimSpace(hit.Citation.Label) != "" {
			label += " " + hit.Citation.Label
		}

		separator := ""
		if context.Len() > 0 {
			separator = "\n\n"
		}
		blockPrefix := separator + label + "\n"
		block := blockPrefix + strings.TrimSpace(hit.ChunkText)
		remaining := maxChars - context.Len()
		if remaining <= 0 {
			truncated = true
			break
		}
		if len(block) > remaining {
			block = block[:remaining]
			truncated = true
		}
		context.WriteString(block)

		citations = append(citations, DocumentContextCitation{
			Reference: reference,
			Citation:  hit.Citation,
		})
		if truncated {
			break
		}
	}

	return DocumentContextResponse{
		Query:     query,
		Context:   context.String(),
		Citations: citations,
		Hits:      hits,
		MaxChars:  maxChars,
		Truncated: truncated,
	}
}

func buildDocumentCitation(row documentSearchRow) *DocumentCitation {
	source := defaultString(row.SourceURI, row.DocumentID)
	labelSource := defaultString(row.ObjectKey, row.DocumentID)
	return &DocumentCitation{
		ID:                fmt.Sprintf("%s#chunk-%d", source, row.ChunkIndex),
		Label:             fmt.Sprintf("%s chunk %d", labelSource, row.ChunkIndex),
		SourceURI:         row.SourceURI,
		ObjectKey:         row.ObjectKey,
		ChunkID:           row.ChunkID,
		ChunkIndex:        row.ChunkIndex,
		ProcessingVersion: row.ProcessingVersion,
	}
}

func embeddingsConfigured() bool {
	return useLocalEmbeddings() || strings.TrimSpace(os.Getenv("EMBEDDING_ENDPOINT")) != ""
}

func embeddingEndpoint() string {
	return strings.TrimSpace(os.Getenv("EMBEDDING_ENDPOINT"))
}

func embeddingModel() string {
	model := strings.TrimSpace(os.Getenv("EMBEDDING_MODEL"))
	if model == "" {
		return defaultEmbeddingModel
	}
	return model
}

func useLocalEmbeddings() bool {
	return strings.TrimSpace(os.Getenv("EMBEDDING_ENDPOINT")) == "" && embeddingModel() == embeddingutil.DefaultModel
}

func maxSimilarity(distance float64) float64 {
	similarity := 1 - distance
	if similarity < 0 {
		return 0
	}
	if similarity > 1 {
		return 1
	}
	return similarity
}

func toVectorLiteral(values []float64) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprintf("%g", value))
	}
	return "[" + strings.Join(parts, ",") + "]"
}
