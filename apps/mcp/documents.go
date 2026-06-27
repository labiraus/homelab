package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"pkg/minioutil"
	"pkg/opensearchrag"
	"pkg/postgresutil"

	"github.com/jackc/pgx/v5"
)

const (
	defaultDocumentSearchLimit   = 8
	maxDocumentSearchLimit       = 20
	defaultDocumentContextLimit  = 6
	defaultDocumentContextChars  = 6000
	maxDocumentContextCharacters = 20000
	defaultDocumentHistoryLimit  = 50
	maxDocumentHistoryLimit      = 200
)

type documentInventoryRow struct {
	DocumentID               string
	Bucket                   string
	ObjectKey                string
	SourceURI                string
	ContentType              string
	Status                   string
	MetadataRaw              string
	DesiredProcessingVersion int
	CurrentProcessingVersion int
	LastReconciledAt         *time.Time
	LastProcessedAt          *time.Time
	LastEventSubject         string
	LastEventAt              *time.Time
	LastError                string
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
	ChunkMetadataRaw  string
	ProcessingVersion int
	Distance          float64
	LastProcessedAt   *time.Time
}

type documentHistoryRow struct {
	ID                int64
	DocumentID        string
	Subject           string
	ProcessingVersion int
	PayloadRaw        string
	OccurredAt        time.Time
	CreatedAt         time.Time
}

var queryDocumentSearchHits = queryOpenSearchDocumentChunks

func postgresUserByEmail(ctx context.Context, email string) (operationResponse, *jsonRPCError) {
	if postgresutil.QueryRow == nil {
		return backendUnavailable("Postgres backend is unavailable")
	}

	var payload string
	err := postgresutil.QueryRow(
		ctx,
		`SELECT json_build_object(
			'email', email,
			'displayName', display_name,
			'createdAt', created_at,
			'updatedAt', updated_at
		)::text
		FROM auth.users
		WHERE email = $1`,
		email,
	).Scan(&payload)
	if err != nil {
		if err == pgx.ErrNoRows {
			return operationResponse{
					ContentType: "application/json",
					Body:        fmt.Sprintf(`{"error":"auth user not found","email":%q}`, email),
				}, &jsonRPCError{
					Code:    -32004,
					Message: "Auth user not found",
					Data:    email,
				}
		}
		return operationResponse{
				ContentType: "application/json",
				Body:        fmt.Sprintf(`{"error":%q}`, err.Error()),
			}, &jsonRPCError{
				Code:    -32000,
				Message: "Auth user lookup failed",
				Data:    email,
			}
	}

	return operationResponse{
		ContentType: "application/json",
		Body:        payload,
	}, nil
}

func postgresDocumentInventory(ctx context.Context, arguments map[string]any) (operationResponse, *jsonRPCError) {
	if postgresutil.Query == nil {
		return backendUnavailable("Postgres backend is unavailable")
	}

	limit := optionalIntArgument(arguments, "limit", 50)
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	status := strings.TrimSpace(optionalStringArgument(arguments, "status", ""))
	prefix := normalizeDocumentPrefix(optionalStringArgument(arguments, "prefix", ""))
	documentID := strings.TrimSpace(optionalStringArgument(arguments, "documentId", ""))

	query := `
SELECT
	document_id,
	COALESCE(bucket_name, ''),
	COALESCE(object_key, ''),
	source_uri,
	COALESCE(content_type, ''),
	status,
	COALESCE(metadata::text, '{}'),
	desired_processing_version,
	current_processing_version,
	last_reconciled_at,
	last_processed_at,
	COALESCE(last_event_subject, ''),
	last_event_at,
	COALESCE(last_error, '')
FROM rag.documents
WHERE true`
	args := []any{}
	nextArg := 1

	if status != "" {
		query += fmt.Sprintf("\n\tAND status = $%d", nextArg)
		args = append(args, status)
		nextArg++
	}
	if prefix != "" {
		query += fmt.Sprintf("\n\tAND object_key LIKE $%d", nextArg)
		args = append(args, prefix+"%")
		nextArg++
	}
	if documentID != "" {
		query += fmt.Sprintf("\n\tAND document_id = $%d", nextArg)
		args = append(args, documentID)
		nextArg++
	}
	metadata := optionalJSONMapArgument(arguments, "metadata")
	if len(metadata) > 0 {
		metadataFilter, err := json.Marshal(metadata)
		if err != nil {
			return operationResponse{}, &jsonRPCError{
				Code:    -32602,
				Message: "Invalid metadata filter",
				Data:    err.Error(),
			}
		}
		query += fmt.Sprintf("\n\tAND metadata @> $%d::jsonb", nextArg)
		args = append(args, string(metadataFilter))
		nextArg++
	}

	query += fmt.Sprintf("\nORDER BY updated_at DESC, document_id\nLIMIT $%d", nextArg)
	args = append(args, limit)

	rows, err := postgresutil.Query(ctx, query, args...)
	if err != nil {
		return operationResponse{
				ContentType: "application/json",
				Body:        fmt.Sprintf(`{"error":%q}`, err.Error()),
			}, &jsonRPCError{
				Code:    -32000,
				Message: "Document inventory query failed",
			}
	}
	defer rows.Close()

	documents := []map[string]any{}
	for rows.Next() {
		var row documentInventoryRow
		if err := rows.Scan(
			&row.DocumentID,
			&row.Bucket,
			&row.ObjectKey,
			&row.SourceURI,
			&row.ContentType,
			&row.Status,
			&row.MetadataRaw,
			&row.DesiredProcessingVersion,
			&row.CurrentProcessingVersion,
			&row.LastReconciledAt,
			&row.LastProcessedAt,
			&row.LastEventSubject,
			&row.LastEventAt,
			&row.LastError,
		); err != nil {
			return operationResponse{}, &jsonRPCError{
				Code:    -32000,
				Message: "Document inventory row scan failed",
				Data:    err.Error(),
			}
		}

		metadata, rpcErr := decodeDocumentMetadata(row.MetadataRaw)
		if rpcErr != nil {
			return operationResponse{}, rpcErr
		}

		documents = append(documents, map[string]any{
			"documentId":               row.DocumentID,
			"bucket":                   row.Bucket,
			"objectKey":                row.ObjectKey,
			"sourceUri":                row.SourceURI,
			"contentType":              row.ContentType,
			"status":                   row.Status,
			"metadata":                 metadata,
			"desiredProcessingVersion": row.DesiredProcessingVersion,
			"currentProcessingVersion": row.CurrentProcessingVersion,
			"lastReconciledAt":         formatOptionalTime(row.LastReconciledAt),
			"lastProcessedAt":          formatOptionalTime(row.LastProcessedAt),
			"lastEventSubject":         row.LastEventSubject,
			"lastEventAt":              formatOptionalTime(row.LastEventAt),
			"lastError":                row.LastError,
		})
	}
	if err := rows.Err(); err != nil {
		return operationResponse{}, &jsonRPCError{
			Code:    -32000,
			Message: "Document inventory query failed",
			Data:    err.Error(),
		}
	}

	body, err := json.Marshal(map[string]any{
		"documents": documents,
		"count":     len(documents),
	})
	if err != nil {
		return operationResponse{}, &jsonRPCError{
			Code:    -32000,
			Message: "Failed to encode document inventory response",
			Data:    err.Error(),
		}
	}
	return operationResponse{ContentType: "application/json", Body: string(body)}, nil
}

func postgresDocumentHistory(ctx context.Context, arguments map[string]any) (operationResponse, *jsonRPCError) {
	if postgresutil.Query == nil {
		return backendUnavailable("Postgres backend is unavailable")
	}

	documentID := strings.TrimSpace(optionalStringArgument(arguments, "documentId", ""))
	if documentID == "" {
		return operationResponse{}, &jsonRPCError{
			Code:    -32602,
			Message: "documentId is required",
		}
	}

	limit := optionalIntArgument(arguments, "limit", defaultDocumentHistoryLimit)
	if limit <= 0 {
		limit = defaultDocumentHistoryLimit
	}
	if limit > maxDocumentHistoryLimit {
		limit = maxDocumentHistoryLimit
	}

	processingVersion := optionalIntArgument(arguments, "processingVersion", 0)
	query := `
SELECT
	id,
	document_id,
	subject,
	processing_version,
	event_payload::text,
	occurred_at,
	created_at
FROM rag.document_lifecycle_events
WHERE document_id = $1`
	args := []any{documentID}
	nextArg := 2

	if processingVersion > 0 {
		query += fmt.Sprintf("\n\tAND processing_version = $%d", nextArg)
		args = append(args, processingVersion)
		nextArg++
	}

	query += fmt.Sprintf("\nORDER BY occurred_at DESC, id DESC\nLIMIT $%d", nextArg)
	args = append(args, limit)

	rows, err := postgresutil.Query(ctx, query, args...)
	if err != nil {
		return operationResponse{
				ContentType: "application/json",
				Body:        fmt.Sprintf(`{"error":%q}`, err.Error()),
			}, &jsonRPCError{
				Code:    -32000,
				Message: "Document history query failed",
			}
	}
	defer rows.Close()

	events := []map[string]any{}
	for rows.Next() {
		var row documentHistoryRow
		if err := rows.Scan(
			&row.ID,
			&row.DocumentID,
			&row.Subject,
			&row.ProcessingVersion,
			&row.PayloadRaw,
			&row.OccurredAt,
			&row.CreatedAt,
		); err != nil {
			return operationResponse{}, &jsonRPCError{
				Code:    -32000,
				Message: "Document history row scan failed",
				Data:    err.Error(),
			}
		}

		payload, rpcErr := decodeDocumentPayload(row.PayloadRaw)
		if rpcErr != nil {
			return operationResponse{}, rpcErr
		}

		events = append(events, map[string]any{
			"id":                row.ID,
			"documentId":        row.DocumentID,
			"subject":           row.Subject,
			"processingVersion": row.ProcessingVersion,
			"occurredAt":        row.OccurredAt.UTC().Format(time.RFC3339),
			"createdAt":         row.CreatedAt.UTC().Format(time.RFC3339),
			"payload":           payload,
		})
	}
	if err := rows.Err(); err != nil {
		return operationResponse{}, &jsonRPCError{
			Code:    -32000,
			Message: "Document history query failed",
			Data:    err.Error(),
		}
	}

	body, err := json.Marshal(map[string]any{
		"documentId": documentID,
		"events":     events,
		"count":      len(events),
	})
	if err != nil {
		return operationResponse{}, &jsonRPCError{
			Code:    -32000,
			Message: "Failed to encode document history response",
			Data:    err.Error(),
		}
	}
	return operationResponse{ContentType: "application/json", Body: string(body)}, nil
}

func decodeDocumentMetadata(raw string) (map[string]any, *jsonRPCError) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}

	var metadata map[string]any
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return nil, &jsonRPCError{
			Code:    -32000,
			Message: "Document metadata decode failed",
			Data:    err.Error(),
		}
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	return metadata, nil
}

func decodeDocumentPayload(raw string) (map[string]any, *jsonRPCError) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, &jsonRPCError{
			Code:    -32000,
			Message: "Document lifecycle payload decode failed",
			Data:    err.Error(),
		}
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return payload, nil
}

func postgresDocumentSearch(ctx context.Context, arguments map[string]any) (operationResponse, *jsonRPCError) {
	if !opensearchrag.ConfigFromEnv().Configured() {
		return backendUnavailable("OpenSearch backend is unavailable")
	}

	queryText := strings.TrimSpace(optionalStringArgument(arguments, "query", ""))
	if queryText == "" {
		return operationResponse{}, &jsonRPCError{
			Code:    -32602,
			Message: "query is required",
		}
	}

	limit := optionalIntArgument(arguments, "limit", defaultDocumentSearchLimit)
	if limit <= 0 {
		limit = defaultDocumentSearchLimit
	}
	if limit > maxDocumentSearchLimit {
		limit = maxDocumentSearchLimit
	}

	hits, rpcErr := queryDocumentSearchHits(ctx, queryText, arguments, limit)
	if rpcErr != nil {
		return operationResponse{}, rpcErr
	}

	body, err := json.Marshal(map[string]any{
		"query": queryText,
		"hits":  hits,
	})
	if err != nil {
		return operationResponse{}, &jsonRPCError{
			Code:    -32000,
			Message: "Failed to encode document search response",
			Data:    err.Error(),
		}
	}
	return operationResponse{ContentType: "application/json", Body: string(body)}, nil
}

func postgresDocumentContext(ctx context.Context, arguments map[string]any) (operationResponse, *jsonRPCError) {
	if !opensearchrag.ConfigFromEnv().Configured() {
		return backendUnavailable("OpenSearch backend is unavailable")
	}

	queryText := strings.TrimSpace(optionalStringArgument(arguments, "query", ""))
	if queryText == "" {
		return operationResponse{}, &jsonRPCError{
			Code:    -32602,
			Message: "query is required",
		}
	}

	limit := optionalIntArgument(arguments, "limit", defaultDocumentContextLimit)
	if limit <= 0 {
		limit = defaultDocumentContextLimit
	}
	if limit > maxDocumentSearchLimit {
		limit = maxDocumentSearchLimit
	}

	maxChars := optionalIntArgument(arguments, "maxChars", defaultDocumentContextChars)
	if maxChars <= 0 {
		maxChars = defaultDocumentContextChars
	}
	if maxChars > maxDocumentContextCharacters {
		maxChars = maxDocumentContextCharacters
	}

	hits, rpcErr := queryDocumentSearchHits(ctx, queryText, arguments, limit)
	if rpcErr != nil {
		return operationResponse{}, rpcErr
	}

	body, err := json.Marshal(buildDocumentContextPayload(queryText, hits, maxChars))
	if err != nil {
		return operationResponse{}, &jsonRPCError{
			Code:    -32000,
			Message: "Failed to encode document context response",
			Data:    err.Error(),
		}
	}
	return operationResponse{ContentType: "application/json", Body: string(body)}, nil
}

func buildDocumentContextPayload(queryText string, hits []map[string]any, maxChars int) map[string]any {
	if maxChars <= 0 {
		maxChars = defaultDocumentContextChars
	}

	var context strings.Builder
	citations := []map[string]any{}
	truncated := false

	for index, hit := range hits {
		reference := fmt.Sprintf("[%d]", index+1)
		citation := documentContextMap(hit["citation"])
		label := reference
		if value := strings.TrimSpace(documentContextString(citation["label"])); value != "" {
			label += " " + value
		}

		separator := ""
		if context.Len() > 0 {
			separator = "\n\n"
		}
		block := separator + label + "\n" + strings.TrimSpace(documentContextString(hit["chunkText"]))
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

		citations = append(citations, map[string]any{
			"reference": reference,
			"citation":  citation,
		})
		if truncated {
			break
		}
	}

	return map[string]any{
		"query":     queryText,
		"context":   context.String(),
		"citations": citations,
		"hits":      hits,
		"maxChars":  maxChars,
		"truncated": truncated,
	}
}

func documentContextMap(value any) map[string]any {
	typed, ok := value.(map[string]any)
	if !ok || typed == nil {
		return map[string]any{}
	}
	return typed
}

func documentContextString(value any) string {
	if value == nil {
		return ""
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return fmt.Sprint(value)
}

func queryOpenSearchDocumentChunks(ctx context.Context, queryText string, arguments map[string]any, limit int) ([]map[string]any, *jsonRPCError) {
	documentID := strings.TrimSpace(optionalStringArgument(arguments, "documentId", ""))
	prefix := normalizeDocumentPrefix(optionalStringArgument(arguments, "prefix", ""))
	metadata := optionalJSONMapArgument(arguments, "metadata")
	openSearchHits, err := opensearchrag.Search(ctx, opensearchrag.ConfigFromEnv(), opensearchrag.SearchRequest{
		Query:      queryText,
		DocumentID: documentID,
		Prefix:     prefix,
		Metadata:   metadata,
		Limit:      limit,
	})
	if err != nil {
		return nil, &jsonRPCError{
			Code:    -32000,
			Message: "Document search query failed",
			Data:    err.Error(),
		}
	}

	hits := []map[string]any{}
	for _, hit := range openSearchHits {
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
		hits = append(hits, map[string]any{
			"documentId":        hit.DocumentID,
			"sourceUri":         hit.SourceURI,
			"objectKey":         hit.ObjectKey,
			"contentType":       hit.ContentType,
			"metadata":          hit.Metadata,
			"chunkId":           hit.ChunkID,
			"chunkIndex":        hit.ChunkIndex,
			"chunkText":         hit.ChunkText,
			"chunkMetadata":     hit.ChunkMetadata,
			"processingVersion": hit.ProcessingVersion,
			"distance":          hit.Distance,
			"similarity":        hit.Similarity,
			"lastProcessedAt":   hit.LastProcessedAt,
			"citation":          buildDocumentCitation(row, hit.ChunkMetadata),
		})
	}

	return hits, nil
}

func buildDocumentCitation(row documentSearchRow, chunkMetadata map[string]any) map[string]any {
	source := defaultSearchString(row.SourceURI, row.DocumentID)
	labelSource := defaultSearchString(row.ObjectKey, row.DocumentID)
	return map[string]any{
		"id":                fmt.Sprintf("%s#chunk-%d", source, row.ChunkIndex),
		"label":             formatSearchCitationLabel(labelSource, row.ChunkIndex, chunkMetadata),
		"sourceUri":         row.SourceURI,
		"objectKey":         row.ObjectKey,
		"chunkId":           row.ChunkID,
		"chunkIndex":        row.ChunkIndex,
		"processingVersion": row.ProcessingVersion,
		"chunkMetadata":     chunkMetadata,
	}
}

func formatSearchCitationLabel(labelSource string, chunkIndex int, chunkMetadata map[string]any) string {
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

	title := strings.TrimSpace(documentMetadataString(chunkMetadata["title"]))
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

func minioMoveBucketObject(ctx context.Context, bucket string, sourceObjectKey string, destinationObjectKey string) (operationResponse, *jsonRPCError) {
	object, err := minioutil.MoveObjectInBucket(ctx, bucket, sourceObjectKey, destinationObjectKey)
	if err != nil {
		return operationResponse{
				ContentType: "application/json",
				Body:        fmt.Sprintf(`{"error":%q}`, err.Error()),
			}, &jsonRPCError{
				Code:    -32000,
				Message: "MinIO move operation failed",
				Data: map[string]any{
					"bucket":               bucket,
					"sourceObjectKey":      sourceObjectKey,
					"destinationObjectKey": destinationObjectKey,
				},
			}
	}

	body, err := json.Marshal(map[string]any{
		"bucket":               bucket,
		"sourceObjectKey":      sourceObjectKey,
		"destinationObjectKey": destinationObjectKey,
		"etag":                 object.ETag,
		"sizeBytes":            object.Size,
		"contentType":          object.ContentType,
		"lastModified":         object.LastModified.UTC().Format(time.RFC3339),
		"moved":                true,
	})
	if err != nil {
		return operationResponse{}, &jsonRPCError{
			Code:    -32000,
			Message: "Failed to encode MinIO move response",
			Data:    err.Error(),
		}
	}

	return operationResponse{ContentType: "application/json", Body: string(body)}, nil
}

func defaultSearchString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func normalizeDocumentPrefix(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = strings.TrimPrefix(value, "/")
	if value != "" && !strings.HasSuffix(value, "/") {
		value += "/"
	}
	return value
}

func formatOptionalTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
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

func backendUnavailable(message string) (operationResponse, *jsonRPCError) {
	return operationResponse{
			ContentType: "application/json",
			Body:        fmt.Sprintf(`{"error":%q}`, message),
		}, &jsonRPCError{
			Code:    -32000,
			Message: message,
		}
}
