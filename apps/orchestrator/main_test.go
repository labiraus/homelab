package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
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
	originalLifecycle := publishLifecycleNotification
	t.Cleanup(func() {
		runDocumentTx = originalRunTx
		upsertPendingRecord = originalUpsert
		publishDocumentEvent = originalPublish
		publishLifecycleNotification = originalLifecycle
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
	publishLifecycleNotification = func(ctx context.Context, event documentEvent) error {
		return nil
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

func TestQueuePendingDocumentPublishesLifecycleEventAfterCommit(t *testing.T) {
	originalRunTx := runDocumentTx
	originalUpsert := upsertPendingRecord
	originalPublish := publishDocumentEvent
	originalLifecycle := publishLifecycleNotification
	t.Cleanup(func() {
		runDocumentTx = originalRunTx
		upsertPendingRecord = originalUpsert
		publishDocumentEvent = originalPublish
		publishLifecycleNotification = originalLifecycle
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
		return nil
	}

	lifecyclePublished := false
	publishLifecycleNotification = func(ctx context.Context, event documentEvent) error {
		if !committed {
			t.Fatal("expected lifecycle notification to publish after commit")
		}
		lifecyclePublished = true
		return nil
	}

	err := queuePendingDocument(context.Background(), documentEvent{
		DocumentID:  "doc-1",
		Bucket:      "documents",
		ObjectKey:   "incoming/doc-1.txt",
		SourceURI:   "s3://documents/incoming/doc-1.txt",
		ContentType: "text/plain",
	})
	if err != nil {
		t.Fatalf("expected queue to succeed: %v", err)
	}
	if !lifecyclePublished {
		t.Fatal("expected lifecycle notification to be published")
	}
}

func TestQueuePendingDocumentIgnoresLifecyclePublishFailureAfterCommit(t *testing.T) {
	originalRunTx := runDocumentTx
	originalUpsert := upsertPendingRecord
	originalPublish := publishDocumentEvent
	originalLifecycle := publishLifecycleNotification
	t.Cleanup(func() {
		runDocumentTx = originalRunTx
		upsertPendingRecord = originalUpsert
		publishDocumentEvent = originalPublish
		publishLifecycleNotification = originalLifecycle
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
		return nil
	}
	publishLifecycleNotification = func(ctx context.Context, event documentEvent) error {
		if !committed {
			t.Fatal("expected lifecycle notification after commit")
		}
		return errors.New("boom")
	}

	err := queuePendingDocument(context.Background(), documentEvent{
		DocumentID:  "doc-1",
		Bucket:      "documents",
		ObjectKey:   "incoming/doc-1.txt",
		SourceURI:   "s3://documents/incoming/doc-1.txt",
		ContentType: "text/plain",
	})
	if err != nil {
		t.Fatalf("expected queue to succeed when lifecycle publish fails: %v", err)
	}
}

func TestScanBucketQueuesNewTextObjectsAndMarksUnsupported(t *testing.T) {
	originalList := listBucketObjectsForScan
	originalFind := findInventoryRecord
	originalUpsert := upsertInventoryRecord
	originalQueue := queueDocument
	t.Cleanup(func() {
		listBucketObjectsForScan = originalList
		findInventoryRecord = originalFind
		upsertInventoryRecord = originalUpsert
		queueDocument = originalQueue
	})

	listBucketObjectsForScan = func(ctx context.Context, bucket string, prefix string, maxKeys int) ([]minio.ObjectInfo, error) {
		if bucket != "documents" {
			t.Fatalf("expected documents bucket, got %q", bucket)
		}
		if prefix != "campaign/" {
			t.Fatalf("expected campaign prefix, got %q", prefix)
		}
		return []minio.ObjectInfo{
			{
				Key:          "campaign/notes.md",
				ETag:         "etag-1",
				Size:         128,
				LastModified: time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC),
			},
			{
				Key:         "campaign/map.pdf",
				ContentType: "application/pdf",
				ETag:        "etag-2",
				Size:        256,
			},
		}, nil
	}
	findInventoryRecord = func(ctx context.Context, bucket string, objectKey string) (documentInventoryRecord, bool, error) {
		return documentInventoryRecord{}, false, nil
	}

	var unsupported []documentEvent
	upsertInventoryRecord = func(ctx context.Context, event documentEvent, status string, preserveStatus bool) error {
		if status != "unsupported" {
			t.Fatalf("expected unsupported status, got %q", status)
		}
		if preserveStatus {
			t.Fatal("expected unsupported upsert to overwrite status")
		}
		unsupported = append(unsupported, event)
		return nil
	}

	var queued []documentEvent
	queueDocument = func(ctx context.Context, event documentEvent) error {
		queued = append(queued, event)
		return nil
	}

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents/scan-bucket", `{"prefix":"/campaign","maxKeys":20}`)
	scanBucketHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if len(queued) != 1 {
		t.Fatalf("expected one queued document, got %+v", queued)
	}
	if queued[0].DocumentID != "s3://documents/campaign/notes.md" {
		t.Fatalf("expected source URI document id, got %q", queued[0].DocumentID)
	}
	if queued[0].ContentType != "text/markdown; charset=utf-8" {
		t.Fatalf("expected markdown content type, got %q", queued[0].ContentType)
	}
	if len(unsupported) != 1 || unsupported[0].ObjectKey != "campaign/map.pdf" {
		t.Fatalf("expected pdf to be marked unsupported, got %+v", unsupported)
	}

	var response scanBucketResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected scan response: %v", err)
	}
	if response.Scanned != 2 || response.Created != 2 || response.Queued != 1 || response.Unsupported != 1 {
		t.Fatalf("unexpected scan response: %+v", response)
	}
}

func TestScanBucketSkipsUnchangedInventory(t *testing.T) {
	originalList := listBucketObjectsForScan
	originalFind := findInventoryRecord
	originalUpsert := upsertInventoryRecord
	originalQueue := queueDocument
	t.Cleanup(func() {
		listBucketObjectsForScan = originalList
		findInventoryRecord = originalFind
		upsertInventoryRecord = originalUpsert
		queueDocument = originalQueue
	})

	modified := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	listBucketObjectsForScan = func(ctx context.Context, bucket string, prefix string, maxKeys int) ([]minio.ObjectInfo, error) {
		return []minio.ObjectInfo{
			{
				Key:          "campaign/notes.txt",
				ContentType:  "text/plain",
				ETag:         "etag-1",
				Size:         128,
				LastModified: modified,
			},
		}, nil
	}
	findInventoryRecord = func(ctx context.Context, bucket string, objectKey string) (documentInventoryRecord, bool, error) {
		return documentInventoryRecord{
			DocumentID:               "existing-doc",
			ETag:                     "etag-1",
			SizeBytes:                128,
			LastModified:             modified,
			HasLastModified:          true,
			Status:                   "processed",
			CurrentProcessingVersion: 1,
		}, true, nil
	}

	upserted := false
	upsertInventoryRecord = func(ctx context.Context, event documentEvent, status string, preserveStatus bool) error {
		upserted = true
		if event.DocumentID != "existing-doc" {
			t.Fatalf("expected existing document id, got %q", event.DocumentID)
		}
		if !preserveStatus {
			t.Fatal("expected unchanged inventory upsert to preserve status")
		}
		return nil
	}
	queueDocument = func(ctx context.Context, event documentEvent) error {
		t.Fatalf("did not expect unchanged document to be queued: %+v", event)
		return nil
	}

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents/scan-bucket", `{}`)
	scanBucketHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if !upserted {
		t.Fatal("expected unchanged inventory to refresh reconciliation timestamp")
	}

	var response scanBucketResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected scan response: %v", err)
	}
	if response.Skipped != 1 || response.Queued != 0 {
		t.Fatalf("unexpected scan response: %+v", response)
	}
}

func TestDocumentNeedsQueueDoesNotDuplicatePendingWork(t *testing.T) {
	modified := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	record := documentInventoryRecord{
		ETag:                     "etag-1",
		SizeBytes:                128,
		LastModified:             modified,
		HasLastModified:          true,
		Status:                   "pending",
		DesiredProcessingVersion: 1,
		CurrentProcessingVersion: 0,
	}
	event := documentEvent{
		ETag:              "etag-1",
		SizeBytes:         128,
		LastModified:      modified.Format(time.RFC3339Nano),
		ProcessingVersion: 1,
	}

	if documentNeedsQueue(record, event) {
		t.Fatal("expected unchanged pending work not to be queued again")
	}

	event.ETag = "etag-2"
	if !documentNeedsQueue(record, event) {
		t.Fatal("expected changed pending source to be queued")
	}
}
