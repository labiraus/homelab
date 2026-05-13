package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"pkg/minioutil"

	"github.com/minio/minio-go/v7"
)

type createTextDocumentRequest struct {
	DocumentID        string                 `json:"documentId,omitempty"`
	ObjectKey         string                 `json:"objectKey"`
	Text              *string                `json:"text"`
	ContentType       string                 `json:"contentType,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	ProcessingVersion int                    `json:"processingVersion,omitempty"`
	ActorEmail        string                 `json:"actorEmail,omitempty"`
	ConversationID    string                 `json:"conversationId,omitempty"`
	ProposalID        string                 `json:"proposalId,omitempty"`
}

var statObjectForCreate = statDocumentObject

func createTextDocumentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var request createTextDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}
	request.ObjectKey = strings.Trim(strings.TrimSpace(request.ObjectKey), "/")
	if request.ObjectKey == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "objectKey is required"})
		return
	}
	if request.Text == nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "text is required"})
		return
	}
	contentType := strings.TrimSpace(request.ContentType)
	if contentType == "" {
		contentType = "text/plain; charset=utf-8"
	}
	if !supportedTextContentType(contentType) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "contentType must be text/* for this endpoint"})
		return
	}

	bucket := documentsBucket()
	if _, err := statObjectForCreate(r.Context(), bucket, request.ObjectKey); err == nil {
		writeJSON(w, http.StatusConflict, errorResponse{Error: "object already exists"})
		return
	}

	object, err := writeTextObjectForEdit(r.Context(), bucket, request.ObjectKey, *request.Text, contentType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to write document object"})
		return
	}

	processingVersion := defaultProcessingVersion(request.ProcessingVersion)
	event := eventFromCreatedText(bucket, request, object, contentType, processingVersion)
	if err := queueDocument(r.Context(), event); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to enqueue document"})
		return
	}
	if err := recordChangeAudit(r.Context(), documentChangeAudit{
		DocumentID:        event.DocumentID,
		Bucket:            event.Bucket,
		ObjectKey:         event.ObjectKey,
		Action:            "create",
		ActorEmail:        request.ActorEmail,
		ConversationID:    request.ConversationID,
		ProposalID:        request.ProposalID,
		NewVersionMarker:  event.VersionMarker,
		ProcessingVersion: event.ProcessingVersion,
		Metadata:          request.Metadata,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to record document audit"})
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

func statDocumentObject(ctx context.Context, bucket string, objectKey string) (minio.ObjectInfo, error) {
	return minioutil.StatObjectInBucket(ctx, bucket, objectKey, minio.StatObjectOptions{})
}

func eventFromCreatedText(bucket string, request createTextDocumentRequest, object minio.ObjectInfo, contentType string, processingVersion int) documentEvent {
	sourceURI := fmt.Sprintf("s3://%s/%s", bucket, request.ObjectKey)
	documentID := strings.TrimSpace(request.DocumentID)
	if documentID == "" {
		documentID = sourceURI
	}
	metadata := map[string]interface{}{}
	for key, value := range request.Metadata {
		metadata[key] = value
	}
	metadata["createdBy"] = "orchestrator.createText"

	event := documentEvent{
		DocumentID:        documentID,
		Bucket:            bucket,
		ObjectKey:         request.ObjectKey,
		SourceURI:         sourceURI,
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
