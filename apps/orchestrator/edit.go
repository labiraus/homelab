package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"pkg/minioutil"

	"github.com/minio/minio-go/v7"
)

type editTextDocumentRequest struct {
	DocumentID        string                 `json:"documentId"`
	Text              *string                `json:"text"`
	ContentType       string                 `json:"contentType,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	ProcessingVersion int                    `json:"processingVersion,omitempty"`
}

type editTextDocumentResponse struct {
	Status            string `json:"status"`
	DocumentID        string `json:"documentId"`
	ProcessingVersion int    `json:"processingVersion"`
	SourceURI         string `json:"sourceUri"`
	ObjectKey         string `json:"objectKey"`
	ETag              string `json:"etag,omitempty"`
	VersionMarker     string `json:"versionMarker,omitempty"`
	SizeBytes         int64  `json:"sizeBytes,omitempty"`
	LastModified      string `json:"lastModified,omitempty"`
}

var writeTextObjectForEdit = writeEditedTextObject

func editTextDocumentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var request editTextDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	request.DocumentID = strings.TrimSpace(request.DocumentID)
	if request.DocumentID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "documentId is required"})
		return
	}
	if request.Text == nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "text is required"})
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

	contentType := strings.TrimSpace(request.ContentType)
	if contentType == "" {
		contentType = record.ContentType
	}
	if !supportedTextContentType(contentType) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "contentType must be text/* for this endpoint"})
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

	object, err := writeTextObjectForEdit(r.Context(), record.Bucket, record.ObjectKey, *request.Text, contentType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to write document object"})
		return
	}

	event := eventFromEditedRecord(record, object, contentType, request.Metadata, processingVersion)
	if err := queueDocument(r.Context(), event); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to enqueue document"})
		return
	}

	writeJSON(w, http.StatusAccepted, editTextDocumentResponse{
		Status:            "queued",
		DocumentID:        event.DocumentID,
		ProcessingVersion: event.ProcessingVersion,
		SourceURI:         event.SourceURI,
		ObjectKey:         event.ObjectKey,
		ETag:              event.ETag,
		VersionMarker:     event.VersionMarker,
		SizeBytes:         event.SizeBytes,
		LastModified:      event.LastModified,
	})
}

func writeEditedTextObject(ctx context.Context, bucket string, objectKey string, text string, contentType string) (minio.ObjectInfo, error) {
	return minioutil.PutTextObjectToBucket(ctx, bucket, objectKey, text, minio.PutObjectOptions{
		ContentType: contentType,
	})
}

func eventFromEditedRecord(record reprocessDocumentRecord, object minio.ObjectInfo, contentType string, requestMetadata map[string]interface{}, processingVersion int) documentEvent {
	metadata := map[string]interface{}{}
	for key, value := range record.Metadata {
		metadata[key] = value
	}
	for key, value := range requestMetadata {
		metadata[key] = value
	}
	metadata["editedBy"] = "orchestrator.editText"

	event := documentEvent{
		DocumentID:        record.DocumentID,
		Bucket:            record.Bucket,
		ObjectKey:         record.ObjectKey,
		SourceURI:         record.SourceURI,
		ContentType:       contentType,
		VersionMarker:     strings.TrimSpace(object.VersionID),
		ETag:              strings.TrimSpace(object.ETag),
		SizeBytes:         object.Size,
		Metadata:          metadata,
		RequestedAt:       time.Now().UTC().Format(time.RFC3339),
		ProcessingVersion: processingVersion,
	}
	if !object.LastModified.IsZero() {
		event.LastModified = object.LastModified.UTC().Format(time.RFC3339Nano)
	}
	return event
}
