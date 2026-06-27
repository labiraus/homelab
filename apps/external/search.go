package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"pkg/opensearchrag"
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
)

type documentSearchRow struct {
	DocumentID        string
	SourceURI         string
	ObjectKey         string
	ContentType       string
	MetadataRaw       string
	ChunkID           int64
	ChunkIndex        int
	ChunkText         string
	ChunkMetadataRaw  string
	ProcessingVersion int
	Distance          float64
	LastProcessedAt   *time.Time
}

var (
	searchDocuments = queryOpenSearchDocuments
	assembleContext = assembleDocumentContext
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

	if !opensearchConfigured() {
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

	hits, err := searchDocuments(r.Context(), request, limit)
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

	if !opensearchConfigured() {
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

	hits, err := searchDocuments(r.Context(), searchRequest, limit)
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

func queryOpenSearchDocuments(ctx context.Context, request DocumentSearchRequest, limit int) ([]DocumentSearchHit, error) {
	hits, err := opensearchrag.Search(ctx, opensearchrag.ConfigFromEnv(), opensearchrag.SearchRequest{
		Query:      request.Query,
		DocumentID: request.DocumentID,
		Prefix:     request.Prefix,
		Metadata:   request.Metadata,
		Limit:      limit,
	})
	if err != nil {
		return nil, err
	}

	results := make([]DocumentSearchHit, 0, len(hits))
	for _, hit := range hits {
		row := documentSearchRow{
			DocumentID:        hit.DocumentID,
			SourceURI:         hit.SourceURI,
			ObjectKey:         hit.ObjectKey,
			ContentType:       hit.ContentType,
			ChunkID:           hit.ChunkID,
			ChunkIndex:        hit.ChunkIndex,
			ChunkText:         hit.ChunkText,
			ProcessingVersion: hit.ProcessingVersion,
			Distance:          hit.Distance,
		}
		results = append(results, DocumentSearchHit{
			DocumentID:        hit.DocumentID,
			SourceURI:         hit.SourceURI,
			ObjectKey:         hit.ObjectKey,
			ContentType:       hit.ContentType,
			Metadata:          hit.Metadata,
			ChunkID:           hit.ChunkID,
			ChunkIndex:        hit.ChunkIndex,
			ChunkText:         hit.ChunkText,
			ProcessingVersion: hit.ProcessingVersion,
			Distance:          hit.Distance,
			Similarity:        hit.Similarity,
			ChunkMetadata:     hit.ChunkMetadata,
			LastProcessedAt:   hit.LastProcessedAt,
			Citation:          buildDocumentCitation(row, hit.ChunkMetadata),
		})
	}
	return results, nil
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

func buildDocumentCitation(row documentSearchRow, chunkMetadata map[string]any) *DocumentCitation {
	source := defaultString(row.SourceURI, row.DocumentID)
	labelSource := defaultString(row.ObjectKey, row.DocumentID)
	return &DocumentCitation{
		ID:                fmt.Sprintf("%s#chunk-%d", source, row.ChunkIndex),
		Label:             formatDocumentCitationLabel(labelSource, row.ChunkIndex, chunkMetadata),
		SourceURI:         row.SourceURI,
		ObjectKey:         row.ObjectKey,
		ChunkID:           row.ChunkID,
		ChunkIndex:        row.ChunkIndex,
		ProcessingVersion: row.ProcessingVersion,
		ChunkMetadata:     chunkMetadata,
	}
}

func formatDocumentCitationLabel(labelSource string, chunkIndex int, chunkMetadata map[string]any) string {
	location := citationLocation(chunkMetadata)
	if location == "" {
		return fmt.Sprintf("%s chunk %d", labelSource, chunkIndex)
	}
	return fmt.Sprintf("%s %s chunk %d", labelSource, location, chunkIndex)
}

func citationLocation(chunkMetadata map[string]any) string {
	if len(chunkMetadata) == 0 {
		return ""
	}

	title := strings.TrimSpace(defaultString(documentMetadataString(chunkMetadata["title"]), ""))
	headingPath := documentMetadataStringSlice(chunkMetadata["headingPath"])
	switch {
	case title != "" && len(headingPath) > 0:
		if strings.EqualFold(title, headingPath[0]) {
			return strings.Join(headingPath, " > ")
		}
		return title + " > " + strings.Join(headingPath, " > ")
	case len(headingPath) > 0:
		return strings.Join(headingPath, " > ")
	default:
		return title
	}
}

func documentMetadataString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return typed
}

func documentMetadataStringSlice(value any) []string {
	rawValues, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]string); ok {
			return typed
		}
		return nil
	}

	values := []string{}
	for _, entry := range rawValues {
		text := strings.TrimSpace(documentMetadataString(entry))
		if text != "" {
			values = append(values, text)
		}
	}
	return values
}

func opensearchConfigured() bool {
	return opensearchrag.ConfigFromEnv().Configured()
}
