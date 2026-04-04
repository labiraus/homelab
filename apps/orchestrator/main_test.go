package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestDocumentsHandlerValidatesPayload(t *testing.T) {
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents", `{"documentId":"","sourceUri":"s3://bucket/doc.txt","contentType":"text/plain","text":"hello"}`)

	documentsHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestDocumentsHandlerPublishesMessage(t *testing.T) {
	originalPublish := publishDocumentEvent
	t.Cleanup(func() {
		publishDocumentEvent = originalPublish
	})

	var published documentEvent
	publishDocumentEvent = func(ctx context.Context, event documentEvent) error {
		published = event
		return nil
	}

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents", `{"documentId":"doc-1","sourceUri":"s3://bucket/doc.txt","contentType":"text/plain","text":"hello"}`)
	documentsHandler(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, recorder.Code)
	}
	if published.DocumentID != "doc-1" {
		t.Fatalf("expected document to be published, got %+v", published)
	}
}

func TestDocumentsHandlerReturnsEnqueueError(t *testing.T) {
	originalPublish := publishDocumentEvent
	t.Cleanup(func() {
		publishDocumentEvent = originalPublish
	})

	publishDocumentEvent = func(ctx context.Context, event documentEvent) error {
		return errors.New("boom")
	}

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents", `{"documentId":"doc-1","sourceUri":"s3://bucket/doc.txt","contentType":"text/plain","text":"hello"}`)
	documentsHandler(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}

	var response errorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected JSON error response: %v", err)
	}
	if response.Error != "failed to enqueue document" {
		t.Fatalf("expected enqueue error message, got %q", response.Error)
	}
}
