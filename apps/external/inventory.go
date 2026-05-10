package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"pkg/postgresutil"
	"pkg/prometheusutil"
)

const (
	documentInventoryLabel        = "documentInventoryHandler"
	defaultDocumentInventoryLimit = 50
	maxDocumentInventoryLimit     = 200
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

var listDocumentInventory = queryDocumentInventory

func documentInventoryHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	startTime := time.Now()
	prometheusutil.IncrementProcessed(documentInventoryLabel, "call")

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
			w.WriteHeader(http.StatusInternalServerError)
		}
		if err != nil {
			slog.ErrorContext(r.Context(), err.Error())
			prometheusutil.IncrementProcessed(documentInventoryLabel, "error")
		}
		prometheusutil.OpDuration(documentInventoryLabel, time.Since(startTime))
	}()

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !postgresConfigured() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "document inventory is unavailable"})
		return
	}

	var request DocumentInventoryRequest
	if err = json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid request body"})
		return
	}

	request.Status = strings.TrimSpace(request.Status)
	request.DocumentID = strings.TrimSpace(request.DocumentID)
	request.Prefix = normalizePrefix(request.Prefix)
	request.Metadata = normalizeMetadataFilter(request.Metadata)
	if request.Limit < 0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "limit must be positive"})
		return
	}

	limit := request.Limit
	if limit == 0 {
		limit = defaultDocumentInventoryLimit
	}
	if limit > maxDocumentInventoryLimit {
		limit = maxDocumentInventoryLimit
	}

	documents, err := listDocumentInventory(r.Context(), request, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "could not list document inventory"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(DocumentInventoryResponse{
		Documents: documents,
		Count:     len(documents),
	}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func queryDocumentInventory(ctx context.Context, request DocumentInventoryRequest, limit int) ([]DocumentInventoryRecord, error) {
	if postgresutil.Query == nil {
		return nil, fmt.Errorf("postgres is not initialized")
	}

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

	if request.Status != "" {
		query += fmt.Sprintf("\n\tAND status = $%d", nextArg)
		args = append(args, request.Status)
		nextArg++
	}
	if request.DocumentID != "" {
		query += fmt.Sprintf("\n\tAND document_id = $%d", nextArg)
		args = append(args, request.DocumentID)
		nextArg++
	}
	if request.Prefix != "" {
		query += fmt.Sprintf("\n\tAND object_key LIKE $%d", nextArg)
		args = append(args, request.Prefix+"%")
		nextArg++
	}
	if len(request.Metadata) > 0 {
		metadataFilter, err := json.Marshal(request.Metadata)
		if err != nil {
			return nil, err
		}
		query += fmt.Sprintf("\n\tAND metadata @> $%d::jsonb", nextArg)
		args = append(args, string(metadataFilter))
		nextArg++
	}

	query += fmt.Sprintf("\nORDER BY updated_at DESC, document_id\nLIMIT $%d", nextArg)
	args = append(args, limit)

	rows, err := postgresutil.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	documents := []DocumentInventoryRecord{}
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
			return nil, err
		}

		metadata, err := decodeDocumentMetadata(row.MetadataRaw)
		if err != nil {
			return nil, err
		}

		documents = append(documents, DocumentInventoryRecord{
			DocumentID:               row.DocumentID,
			Bucket:                   row.Bucket,
			ObjectKey:                row.ObjectKey,
			SourceURI:                row.SourceURI,
			ContentType:              row.ContentType,
			Status:                   row.Status,
			Metadata:                 metadata,
			DesiredProcessingVersion: row.DesiredProcessingVersion,
			CurrentProcessingVersion: row.CurrentProcessingVersion,
			LastReconciledAt:         formatInventoryTime(row.LastReconciledAt),
			LastProcessedAt:          formatInventoryTime(row.LastProcessedAt),
			LastEventSubject:         row.LastEventSubject,
			LastEventAt:              formatInventoryTime(row.LastEventAt),
			LastError:                row.LastError,
		})
	}

	return documents, rows.Err()
}

func formatInventoryTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
