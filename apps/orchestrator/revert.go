package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"pkg/minioutil"

	"github.com/minio/minio-go/v7"
)

type revertDocumentRequest struct {
	DocumentID        string                 `json:"documentId"`
	VersionMarker     string                 `json:"versionMarker"`
	ContentType       string                 `json:"contentType,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	ProcessingVersion int                    `json:"processingVersion,omitempty"`
	ActorEmail        string                 `json:"actorEmail,omitempty"`
	ConversationID    string                 `json:"conversationId,omitempty"`
	ProposalID        string                 `json:"proposalId,omitempty"`
}

var readObjectVersionForRevert = readDocumentObjectVersion

func revertDocumentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var request revertDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}
	request.DocumentID = strings.TrimSpace(request.DocumentID)
	request.VersionMarker = strings.TrimSpace(request.VersionMarker)
	if request.DocumentID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "documentId is required"})
		return
	}
	if request.VersionMarker == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "versionMarker is required"})
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

	body, err := readObjectVersionForRevert(r.Context(), record.Bucket, record.ObjectKey, request.VersionMarker)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "failed to read requested object version"})
		return
	}
	object, err := writeTextObjectForEdit(r.Context(), record.Bucket, record.ObjectKey, string(body), contentType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to write document object"})
		return
	}

	metadata := map[string]interface{}{}
	for key, value := range request.Metadata {
		metadata[key] = value
	}
	metadata["revertedBy"] = "orchestrator.revert"
	metadata["revertedToVersionMarker"] = request.VersionMarker

	event := eventFromEditedRecord(record, object, contentType, metadata, processingVersion)
	if err := queueDocument(r.Context(), event); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to enqueue document"})
		return
	}
	if err := recordChangeAudit(r.Context(), documentChangeAudit{
		DocumentID:              event.DocumentID,
		Bucket:                  event.Bucket,
		ObjectKey:               event.ObjectKey,
		Action:                  "revert",
		ActorEmail:              request.ActorEmail,
		ConversationID:          request.ConversationID,
		ProposalID:              request.ProposalID,
		OldVersionMarker:        record.VersionMarker,
		NewVersionMarker:        event.VersionMarker,
		RevertedToVersionMarker: request.VersionMarker,
		ProcessingVersion:       event.ProcessingVersion,
		Metadata:                metadata,
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

func readDocumentObjectVersion(ctx context.Context, bucket string, objectKey string, versionMarker string) ([]byte, error) {
	object, err := minioutil.GetObjectFromBucket(ctx, bucket, objectKey, minio.GetObjectOptions{
		VersionID: versionMarker,
	})
	if err != nil {
		return nil, err
	}
	defer object.Close()

	return io.ReadAll(object)
}
