package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type reprocessDocumentRequest struct {
	DocumentID        string `json:"documentId"`
	ProcessingVersion int    `json:"processingVersion,omitempty"`
}

type reprocessDocumentResponse struct {
	Status            string `json:"status"`
	DocumentID        string `json:"documentId"`
	ProcessingVersion int    `json:"processingVersion"`
	SourceURI         string `json:"sourceUri"`
}

func reprocessDocumentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var request reprocessDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	request.DocumentID = strings.TrimSpace(request.DocumentID)
	if request.DocumentID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "documentId is required"})
		return
	}

	record, found, err := lookupReprocessRecord(r.Context(), request.DocumentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to read document inventory"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "document not found"})
		return
	}

	if err := validateReprocessRecord(record); err != nil {
		writeJSON(w, http.StatusConflict, errorResponse{Error: err.Error()})
		return
	}

	processingVersion := request.ProcessingVersion
	if processingVersion <= 0 {
		processingVersion = nextProcessingVersion(record)
	}
	if processingVersion <= maxInt(record.DesiredProcessingVersion, record.CurrentProcessingVersion) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "processingVersion must be newer than the current desired processing version"})
		return
	}

	event := eventFromReprocessRecord(record, processingVersion)
	if err := queueDocument(r.Context(), event); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to enqueue document"})
		return
	}

	writeJSON(w, http.StatusAccepted, reprocessDocumentResponse{
		Status:            "queued",
		DocumentID:        event.DocumentID,
		ProcessingVersion: event.ProcessingVersion,
		SourceURI:         event.SourceURI,
	})
}

func validateReprocessRecord(record reprocessDocumentRecord) error {
	switch {
	case strings.TrimSpace(record.Bucket) == "":
		return fmt.Errorf("document bucket is missing")
	case strings.TrimSpace(record.ObjectKey) == "":
		return fmt.Errorf("document objectKey is missing")
	case strings.TrimSpace(record.SourceURI) == "":
		return fmt.Errorf("document sourceUri is missing")
	case strings.TrimSpace(record.ContentType) == "":
		return fmt.Errorf("document contentType is missing")
	case !supportedTextContentType(record.ContentType):
		return fmt.Errorf("document contentType must be text/* for reprocessing")
	default:
		return nil
	}
}

func eventFromReprocessRecord(record reprocessDocumentRecord, processingVersion int) documentEvent {
	metadata := map[string]interface{}{}
	for key, value := range record.Metadata {
		metadata[key] = value
	}
	metadata["reprocessedBy"] = "orchestrator.reprocess"

	event := documentEvent{
		DocumentID:        record.DocumentID,
		Bucket:            record.Bucket,
		ObjectKey:         record.ObjectKey,
		SourceURI:         record.SourceURI,
		ContentType:       record.ContentType,
		VersionMarker:     record.VersionMarker,
		ETag:              record.ETag,
		SizeBytes:         record.SizeBytes,
		Metadata:          metadata,
		RequestedAt:       time.Now().UTC().Format(time.RFC3339),
		ProcessingVersion: processingVersion,
	}
	if record.HasLastModified {
		event.LastModified = record.LastModified.UTC().Format(time.RFC3339Nano)
	}
	return event
}

func nextProcessingVersion(record reprocessDocumentRecord) int {
	return maxInt(record.DesiredProcessingVersion, record.CurrentProcessingVersion) + 1
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
