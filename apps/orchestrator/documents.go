package main

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"

	"pkg/natsutil"
)

type documentRequest struct {
	DocumentID        string                 `json:"documentId"`
	Bucket            string                 `json:"bucket,omitempty"`
	ObjectKey         string                 `json:"objectKey,omitempty"`
	SourceURI         string                 `json:"sourceUri"`
	ContentType       string                 `json:"contentType"`
	VersionMarker     string                 `json:"versionMarker,omitempty"`
	ETag              string                 `json:"etag,omitempty"`
	SizeBytes         int64                  `json:"sizeBytes,omitempty"`
	LastModified      string                 `json:"lastModified,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	ProcessingVersion int                    `json:"processingVersion,omitempty"`
}

type documentEvent struct {
	DocumentID        string                 `json:"documentId"`
	Bucket            string                 `json:"bucket,omitempty"`
	ObjectKey         string                 `json:"objectKey,omitempty"`
	SourceURI         string                 `json:"sourceUri"`
	ContentType       string                 `json:"contentType"`
	VersionMarker     string                 `json:"versionMarker,omitempty"`
	ETag              string                 `json:"etag,omitempty"`
	SizeBytes         int64                  `json:"sizeBytes,omitempty"`
	LastModified      string                 `json:"lastModified,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	RequestedAt       string                 `json:"requestedAt"`
	ProcessingVersion int                    `json:"processingVersion,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

var publishDocumentEvent = enqueueDocument
var queueDocument = queuePendingDocument

func documentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var request documentRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	if err := validateDocumentRequest(request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	event := documentEvent{
		DocumentID:        request.DocumentID,
		Bucket:            request.Bucket,
		ObjectKey:         request.ObjectKey,
		SourceURI:         request.SourceURI,
		ContentType:       request.ContentType,
		VersionMarker:     request.VersionMarker,
		ETag:              request.ETag,
		SizeBytes:         request.SizeBytes,
		LastModified:      request.LastModified,
		Metadata:          request.Metadata,
		RequestedAt:       time.Now().UTC().Format(time.RFC3339),
		ProcessingVersion: defaultProcessingVersion(request.ProcessingVersion),
	}

	if err := queueDocument(r.Context(), event); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to enqueue document"})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":     "queued",
		"documentId": event.DocumentID,
	})
}

func validateDocumentRequest(request documentRequest) error {
	switch {
	case strings.TrimSpace(request.DocumentID) == "":
		return fmt.Errorf("documentId is required")
	case strings.TrimSpace(request.Bucket) == "":
		return fmt.Errorf("bucket is required")
	case strings.TrimSpace(request.ObjectKey) == "":
		return fmt.Errorf("objectKey is required")
	case strings.TrimSpace(request.SourceURI) == "":
		return fmt.Errorf("sourceUri is required")
	case strings.TrimSpace(request.ContentType) == "":
		return fmt.Errorf("contentType is required")
	case !supportedTextContentType(request.ContentType):
		return fmt.Errorf("contentType must be text/* for this endpoint")
	default:
		return nil
	}
}

func queuePendingDocument(ctx context.Context, event documentEvent) error {
	return runDocumentTx(ctx, func(txCtx context.Context) error {
		if err := upsertPendingRecord(txCtx, event); err != nil {
			return err
		}

		return publishDocumentEvent(txCtx, event)
	})
}

func enqueueDocument(ctx context.Context, event documentEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	err = natsutil.Publish(ctx, "documents", payload)
	if err != nil {
		return err
	}
	return nil
}

func streamName() string {
	stream := strings.TrimSpace(os.Getenv("NATS_STREAM"))
	if stream == "" {
		return "documents"
	}
	return stream
}

func subjectName() string {
	subject := strings.TrimSpace(os.Getenv("NATS_SUBJECT"))
	if subject == "" {
		return "documents.ingest"
	}
	return subject
}

func defaultProcessingVersion(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}

func supportedTextContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil {
		return false
	}

	return strings.HasPrefix(strings.ToLower(mediaType), "text/")
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
