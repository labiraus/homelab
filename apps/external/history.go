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
	documentHistoryLabel        = "documentHistoryHandler"
	defaultDocumentHistoryLimit = 50
	maxDocumentHistoryLimit     = 200
)

type documentHistoryRow struct {
	ID                int64
	DocumentID        string
	Subject           string
	ProcessingVersion int
	PayloadRaw        string
	OccurredAt        time.Time
	CreatedAt         time.Time
}

var listDocumentHistory = queryDocumentHistory

func documentHistoryHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	startTime := time.Now()
	prometheusutil.IncrementProcessed(documentHistoryLabel, "call")

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
			w.WriteHeader(http.StatusInternalServerError)
		}
		if err != nil {
			slog.ErrorContext(r.Context(), err.Error())
			prometheusutil.IncrementProcessed(documentHistoryLabel, "error")
		}
		prometheusutil.OpDuration(documentHistoryLabel, time.Since(startTime))
	}()

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !postgresConfigured() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "document history is unavailable"})
		return
	}

	var request DocumentHistoryRequest
	if err = json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "invalid request body"})
		return
	}

	request.DocumentID = strings.TrimSpace(request.DocumentID)
	switch {
	case request.DocumentID == "":
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "documentId is required"})
		return
	case request.Limit < 0:
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "limit must be positive"})
		return
	case request.ProcessingVersion < 0:
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "processingVersion must be positive"})
		return
	}

	limit := request.Limit
	if limit == 0 {
		limit = defaultDocumentHistoryLimit
	}
	if limit > maxDocumentHistoryLimit {
		limit = maxDocumentHistoryLimit
	}

	events, err := listDocumentHistory(r.Context(), request, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "could not list document history"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(DocumentHistoryResponse{
		DocumentID: request.DocumentID,
		Events:     events,
		Count:      len(events),
	}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func queryDocumentHistory(ctx context.Context, request DocumentHistoryRequest, limit int) ([]DocumentLifecycleHistoryEvent, error) {
	if postgresutil.Query == nil {
		return nil, fmt.Errorf("postgres is not initialized")
	}

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
	args := []any{request.DocumentID}
	nextArg := 2

	if request.ProcessingVersion > 0 {
		query += fmt.Sprintf("\n\tAND processing_version = $%d", nextArg)
		args = append(args, request.ProcessingVersion)
		nextArg++
	}

	query += fmt.Sprintf("\nORDER BY occurred_at DESC, id DESC\nLIMIT $%d", nextArg)
	args = append(args, limit)

	rows, err := postgresutil.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []DocumentLifecycleHistoryEvent{}
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
			return nil, err
		}

		payload, err := decodeDocumentEventPayload(row.PayloadRaw)
		if err != nil {
			return nil, err
		}

		events = append(events, DocumentLifecycleHistoryEvent{
			ID:                row.ID,
			DocumentID:        row.DocumentID,
			Subject:           row.Subject,
			ProcessingVersion: row.ProcessingVersion,
			OccurredAt:        row.OccurredAt.UTC().Format(time.RFC3339),
			CreatedAt:         row.CreatedAt.UTC().Format(time.RFC3339),
			Payload:           payload,
		})
	}

	return events, rows.Err()
}

func decodeDocumentEventPayload(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		return map[string]any{}, nil
	}
	return payload, nil
}
