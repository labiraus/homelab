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
	originalQueue := queueDocument
	t.Cleanup(func() {
		queueDocument = originalQueue
	})

	var published documentEvent
	queueDocument = func(ctx context.Context, event documentEvent) error {
		published = event
		return nil
	}

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents", `{"documentId":"doc-1","bucket":"documents","objectKey":"incoming/doc-1.txt","sourceUri":"s3://documents/incoming/doc-1.txt","contentType":"text/plain"}`)
	documentsHandler(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, recorder.Code)
	}
	if published.DocumentID != "doc-1" {
		t.Fatalf("expected document to be published, got %+v", published)
	}
	if published.ObjectKey != "incoming/doc-1.txt" {
		t.Fatalf("expected object reference to be published, got %+v", published)
	}
}

func TestDocumentsHandlerReturnsEnqueueError(t *testing.T) {
	originalQueue := queueDocument
	t.Cleanup(func() {
		queueDocument = originalQueue
	})

	queueDocument = func(ctx context.Context, event documentEvent) error {
		return errors.New("boom")
	}

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents", `{"documentId":"doc-1","bucket":"documents","objectKey":"incoming/doc-1.txt","sourceUri":"s3://documents/incoming/doc-1.txt","contentType":"text/plain"}`)
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

func TestDocumentsHandlerRejectsNonTextContentType(t *testing.T) {
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents", `{"documentId":"doc-1","bucket":"documents","objectKey":"incoming/doc-1.pdf","sourceUri":"s3://documents/incoming/doc-1.pdf","contentType":"application/pdf"}`)

	documentsHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestQueuePendingDocumentDoesNotCommitOnPublishFailure(t *testing.T) {
	originalRunTx := runDocumentTx
	originalUpsert := upsertPendingRecord
	originalPublish := publishDocumentEvent
	t.Cleanup(func() {
		runDocumentTx = originalRunTx
		upsertPendingRecord = originalUpsert
		publishDocumentEvent = originalPublish
	})

	committed := false
	runDocumentTx = func(ctx context.Context, fn func(context.Context) error) error {
		if err := fn(ctx); err != nil {
			return err
		}
		committed = true
		return nil
	}

	upsertPendingRecord = func(ctx context.Context, event documentEvent) error {
		return nil
	}
	publishDocumentEvent = func(ctx context.Context, event documentEvent) error {
		return errors.New("boom")
	}

	err := queuePendingDocument(context.Background(), documentEvent{
		DocumentID:  "doc-1",
		Bucket:      "documents",
		ObjectKey:   "incoming/doc-1.txt",
		SourceURI:   "s3://documents/incoming/doc-1.txt",
		ContentType: "text/plain",
	})
	if err == nil {
		t.Fatalf("expected queue failure")
	}
	if committed {
		t.Fatalf("expected transaction to stay uncommitted on publish failure")
	}
}
