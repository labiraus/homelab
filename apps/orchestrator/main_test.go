package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
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

func TestDocumentCurationHandlerUpdatesMetadata(t *testing.T) {
	originalUpdate := updateCurationRecord
	t.Cleanup(func() {
		updateCurationRecord = originalUpdate
	})

	updateCurationRecord = func(ctx context.Context, documentID string, metadata map[string]interface{}, replace bool) (map[string]interface{}, bool, error) {
		if documentID != "doc-1" {
			t.Fatalf("expected doc-1 update, got %q", documentID)
		}
		if replace {
			t.Fatal("expected merge update")
		}
		if metadata["summary"] != "Curated summary" {
			t.Fatalf("expected metadata to be forwarded, got %+v", metadata)
		}
		return map[string]interface{}{
			"summary": "Curated summary",
			"tags":    []interface{}{"campaign", "npc"},
		}, true, nil
	}

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents/curation", `{"documentId":"doc-1","metadata":{"summary":"Curated summary","tags":["campaign","npc"]}}`)
	documentCurationHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var response documentCurationResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected curation response: %v", err)
	}
	if response.Status != "updated" || response.Metadata["summary"] != "Curated summary" {
		t.Fatalf("unexpected curation response: %+v", response)
	}
}

func TestDocumentCurationHandlerValidatesMetadata(t *testing.T) {
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents/curation", `{"documentId":"doc-1"}`)
	documentCurationHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestDocumentCurationHandlerReturnsNotFound(t *testing.T) {
	originalUpdate := updateCurationRecord
	t.Cleanup(func() {
		updateCurationRecord = originalUpdate
	})

	updateCurationRecord = func(ctx context.Context, documentID string, metadata map[string]interface{}, replace bool) (map[string]interface{}, bool, error) {
		return nil, false, nil
	}

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents/curation", `{"documentId":"missing-doc","metadata":{"summary":"Curated summary"}}`)
	documentCurationHandler(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, recorder.Code)
	}
}

func TestEditTextDocumentHandlerWritesObjectAndQueuesNextVersion(t *testing.T) {
	originalLookup := lookupReprocessRecord
	originalWrite := writeTextObjectForEdit
	originalQueue := queueDocument
	originalAudit := recordChangeAudit
	t.Cleanup(func() {
		lookupReprocessRecord = originalLookup
		writeTextObjectForEdit = originalWrite
		queueDocument = originalQueue
		recordChangeAudit = originalAudit
	})

	lookupReprocessRecord = func(ctx context.Context, documentID string) (reprocessDocumentRecord, bool, error) {
		if documentID != "doc-1" {
			t.Fatalf("expected doc-1 lookup, got %q", documentID)
		}
		return reprocessDocumentRecord{
			DocumentID:               "doc-1",
			Bucket:                   "documents",
			ObjectKey:                "campaign/doc-1.md",
			SourceURI:                "s3://documents/campaign/doc-1.md",
			ContentType:              "text/markdown; charset=utf-8",
			Metadata:                 map[string]interface{}{"source": "session-notes"},
			DesiredProcessingVersion: 2,
			CurrentProcessingVersion: 2,
		}, true, nil
	}

	modified := time.Date(2026, 5, 8, 20, 30, 0, 0, time.UTC)
	writeTextObjectForEdit = func(ctx context.Context, bucket string, objectKey string, text string, contentType string) (minio.ObjectInfo, error) {
		if bucket != "documents" || objectKey != "campaign/doc-1.md" {
			t.Fatalf("unexpected edit target: %s/%s", bucket, objectKey)
		}
		if text != "edited notes" {
			t.Fatalf("unexpected text payload: %q", text)
		}
		if contentType != "text/markdown; charset=utf-8" {
			t.Fatalf("unexpected content type: %q", contentType)
		}
		return minio.ObjectInfo{
			Key:          objectKey,
			ETag:         "etag-edited",
			Size:         int64(len(text)),
			LastModified: modified,
			VersionID:    "version-edited",
		}, nil
	}

	var queued documentEvent
	queueDocument = func(ctx context.Context, event documentEvent) error {
		queued = event
		return nil
	}
	var audit documentChangeAudit
	recordChangeAudit = func(ctx context.Context, record documentChangeAudit) error {
		audit = record
		return nil
	}

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents/edit-text", `{"documentId":"doc-1","text":"edited notes","metadata":{"summary":"edited"}}`)
	editTextDocumentHandler(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, recorder.Code)
	}
	if queued.DocumentID != "doc-1" || queued.ProcessingVersion != 3 {
		t.Fatalf("expected document to be queued at version 3, got %+v", queued)
	}
	if queued.ETag != "etag-edited" || queued.VersionMarker != "version-edited" || queued.SizeBytes != int64(len("edited notes")) {
		t.Fatalf("expected edited object metadata in queued event, got %+v", queued)
	}
	if queued.Metadata["source"] != "session-notes" || queued.Metadata["summary"] != "edited" || queued.Metadata["editedBy"] != "orchestrator.editText" {
		t.Fatalf("expected merged edit metadata, got %+v", queued.Metadata)
	}
	if audit.Action != "edit" || audit.DocumentID != "doc-1" || audit.NewVersionMarker != "version-edited" {
		t.Fatalf("expected edit audit, got %+v", audit)
	}

	var response editTextDocumentResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected edit response: %v", err)
	}
	if response.Status != "queued" || response.ProcessingVersion != 3 || response.ETag != "etag-edited" {
		t.Fatalf("unexpected edit response: %+v", response)
	}
}

func TestEditTextDocumentHandlerAllowsEmptyText(t *testing.T) {
	originalLookup := lookupReprocessRecord
	originalWrite := writeTextObjectForEdit
	originalQueue := queueDocument
	originalAudit := recordChangeAudit
	t.Cleanup(func() {
		lookupReprocessRecord = originalLookup
		writeTextObjectForEdit = originalWrite
		queueDocument = originalQueue
		recordChangeAudit = originalAudit
	})

	lookupReprocessRecord = func(ctx context.Context, documentID string) (reprocessDocumentRecord, bool, error) {
		return reprocessDocumentRecord{
			DocumentID:               documentID,
			Bucket:                   "documents",
			ObjectKey:                "campaign/doc-1.txt",
			SourceURI:                "s3://documents/campaign/doc-1.txt",
			ContentType:              "text/plain",
			DesiredProcessingVersion: 1,
			CurrentProcessingVersion: 1,
		}, true, nil
	}
	writeTextObjectForEdit = func(ctx context.Context, bucket string, objectKey string, text string, contentType string) (minio.ObjectInfo, error) {
		if text != "" {
			t.Fatalf("expected empty text edit, got %q", text)
		}
		return minio.ObjectInfo{Key: objectKey, Size: 0}, nil
	}
	queueDocument = func(ctx context.Context, event documentEvent) error {
		return nil
	}
	recordChangeAudit = func(ctx context.Context, record documentChangeAudit) error {
		return nil
	}

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents/edit-text", `{"documentId":"doc-1","text":""}`)
	editTextDocumentHandler(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, recorder.Code)
	}
}

func TestCreateTextDocumentHandlerWritesObjectQueuesAndAudits(t *testing.T) {
	originalStat := statObjectForCreate
	originalWrite := writeTextObjectForEdit
	originalQueue := queueDocument
	originalAudit := recordChangeAudit
	t.Cleanup(func() {
		statObjectForCreate = originalStat
		writeTextObjectForEdit = originalWrite
		queueDocument = originalQueue
		recordChangeAudit = originalAudit
	})

	statObjectForCreate = func(ctx context.Context, bucket string, objectKey string) (minio.ObjectInfo, error) {
		return minio.ObjectInfo{}, errors.New("not found")
	}
	writeTextObjectForEdit = func(ctx context.Context, bucket string, objectKey string, text string, contentType string) (minio.ObjectInfo, error) {
		if bucket != "documents" || objectKey != "campaign/new.md" {
			t.Fatalf("unexpected create target: %s/%s", bucket, objectKey)
		}
		if text != "new notes" {
			t.Fatalf("unexpected text payload: %q", text)
		}
		return minio.ObjectInfo{Key: objectKey, ETag: "etag-new", VersionID: "version-new", Size: int64(len(text))}, nil
	}

	var queued documentEvent
	queueDocument = func(ctx context.Context, event documentEvent) error {
		queued = event
		return nil
	}
	var audit documentChangeAudit
	recordChangeAudit = func(ctx context.Context, record documentChangeAudit) error {
		audit = record
		return nil
	}

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents/create-text", `{"objectKey":"campaign/new.md","text":"new notes","contentType":"text/markdown","actorEmail":"user@example.com","conversationId":"conv-1","proposalId":"prop-1"}`)
	createTextDocumentHandler(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, recorder.Code)
	}
	if queued.DocumentID != "s3://documents/campaign/new.md" || queued.VersionMarker != "version-new" || queued.Metadata["createdBy"] != "orchestrator.createText" {
		t.Fatalf("unexpected queued create event: %+v", queued)
	}
	if audit.Action != "create" || audit.ActorEmail != "user@example.com" || audit.ProposalID != "prop-1" || audit.NewVersionMarker != "version-new" {
		t.Fatalf("unexpected create audit: %+v", audit)
	}
}

func TestRevertDocumentHandlerRestoresVersionQueuesAndAudits(t *testing.T) {
	originalLookup := lookupReprocessRecord
	originalRead := readObjectVersionForRevert
	originalWrite := writeTextObjectForEdit
	originalQueue := queueDocument
	originalAudit := recordChangeAudit
	t.Cleanup(func() {
		lookupReprocessRecord = originalLookup
		readObjectVersionForRevert = originalRead
		writeTextObjectForEdit = originalWrite
		queueDocument = originalQueue
		recordChangeAudit = originalAudit
	})

	lookupReprocessRecord = func(ctx context.Context, documentID string) (reprocessDocumentRecord, bool, error) {
		return reprocessDocumentRecord{
			DocumentID:               documentID,
			Bucket:                   "documents",
			ObjectKey:                "campaign/doc-1.md",
			SourceURI:                "s3://documents/campaign/doc-1.md",
			ContentType:              "text/markdown",
			VersionMarker:            "version-current",
			DesiredProcessingVersion: 3,
			CurrentProcessingVersion: 3,
		}, true, nil
	}
	readObjectVersionForRevert = func(ctx context.Context, bucket string, objectKey string, versionMarker string) ([]byte, error) {
		if versionMarker != "version-old" {
			t.Fatalf("unexpected revert target version: %q", versionMarker)
		}
		return []byte("old notes"), nil
	}
	writeTextObjectForEdit = func(ctx context.Context, bucket string, objectKey string, text string, contentType string) (minio.ObjectInfo, error) {
		if text != "old notes" {
			t.Fatalf("unexpected reverted body: %q", text)
		}
		return minio.ObjectInfo{Key: objectKey, VersionID: "version-reverted", Size: int64(len(text))}, nil
	}
	var queued documentEvent
	queueDocument = func(ctx context.Context, event documentEvent) error {
		queued = event
		return nil
	}
	var audit documentChangeAudit
	recordChangeAudit = func(ctx context.Context, record documentChangeAudit) error {
		audit = record
		return nil
	}

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents/revert", `{"documentId":"doc-1","versionMarker":"version-old","actorEmail":"user@example.com","conversationId":"conv-1"}`)
	revertDocumentHandler(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, recorder.Code)
	}
	if queued.ProcessingVersion != 4 || queued.VersionMarker != "version-reverted" || queued.Metadata["revertedBy"] != "orchestrator.revert" {
		t.Fatalf("unexpected queued revert event: %+v", queued)
	}
	if audit.Action != "revert" || audit.OldVersionMarker != "version-current" || audit.NewVersionMarker != "version-reverted" || audit.RevertedToVersionMarker != "version-old" {
		t.Fatalf("unexpected revert audit: %+v", audit)
	}
}

func TestEditTextDocumentHandlerValidatesText(t *testing.T) {
	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents/edit-text", `{"documentId":"doc-1"}`)
	editTextDocumentHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestEditTextDocumentHandlerRejectsUnsupportedInventory(t *testing.T) {
	originalLookup := lookupReprocessRecord
	t.Cleanup(func() {
		lookupReprocessRecord = originalLookup
	})

	lookupReprocessRecord = func(ctx context.Context, documentID string) (reprocessDocumentRecord, bool, error) {
		return reprocessDocumentRecord{
			DocumentID:               documentID,
			Bucket:                   "documents",
			ObjectKey:                "campaign/map.pdf",
			SourceURI:                "s3://documents/campaign/map.pdf",
			ContentType:              "application/pdf",
			DesiredProcessingVersion: 1,
			CurrentProcessingVersion: 0,
		}, true, nil
	}

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents/edit-text", `{"documentId":"doc-1","text":"edited"}`)
	editTextDocumentHandler(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, recorder.Code)
	}
}

func TestReprocessDocumentHandlerQueuesNextVersion(t *testing.T) {
	originalLookup := lookupReprocessRecord
	originalQueue := queueDocument
	t.Cleanup(func() {
		lookupReprocessRecord = originalLookup
		queueDocument = originalQueue
	})

	lookupReprocessRecord = func(ctx context.Context, documentID string) (reprocessDocumentRecord, bool, error) {
		if documentID != "doc-1" {
			t.Fatalf("expected doc-1 lookup, got %q", documentID)
		}
		return reprocessDocumentRecord{
			DocumentID:               "doc-1",
			Bucket:                   "documents",
			ObjectKey:                "incoming/doc-1.txt",
			SourceURI:                "s3://documents/incoming/doc-1.txt",
			ContentType:              "text/plain",
			ETag:                     "etag-1",
			SizeBytes:                12,
			DesiredProcessingVersion: 1,
			CurrentProcessingVersion: 1,
		}, true, nil
	}

	var queued documentEvent
	queueDocument = func(ctx context.Context, event documentEvent) error {
		queued = event
		return nil
	}

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents/reprocess", `{"documentId":"doc-1"}`)
	reprocessDocumentHandler(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, recorder.Code)
	}
	if queued.DocumentID != "doc-1" || queued.ProcessingVersion != 2 {
		t.Fatalf("expected document to be requeued at version 2, got %+v", queued)
	}
	if queued.Metadata["reprocessedBy"] != "orchestrator.reprocess" {
		t.Fatalf("expected reprocess metadata, got %+v", queued.Metadata)
	}

	var response reprocessDocumentResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected reprocess response: %v", err)
	}
	if response.ProcessingVersion != 2 {
		t.Fatalf("expected response processing version 2, got %+v", response)
	}
}

func TestReprocessDocumentHandlerRejectsStaleRequestedVersion(t *testing.T) {
	originalLookup := lookupReprocessRecord
	originalQueue := queueDocument
	t.Cleanup(func() {
		lookupReprocessRecord = originalLookup
		queueDocument = originalQueue
	})

	lookupReprocessRecord = func(ctx context.Context, documentID string) (reprocessDocumentRecord, bool, error) {
		return reprocessDocumentRecord{
			DocumentID:               documentID,
			Bucket:                   "documents",
			ObjectKey:                "incoming/doc-1.txt",
			SourceURI:                "s3://documents/incoming/doc-1.txt",
			ContentType:              "text/plain",
			DesiredProcessingVersion: 3,
			CurrentProcessingVersion: 2,
		}, true, nil
	}
	queueDocument = func(ctx context.Context, event documentEvent) error {
		t.Fatalf("did not expect stale version to be queued: %+v", event)
		return nil
	}

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents/reprocess", `{"documentId":"doc-1","processingVersion":3}`)
	reprocessDocumentHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestReprocessDocumentHandlerReturnsNotFound(t *testing.T) {
	originalLookup := lookupReprocessRecord
	t.Cleanup(func() {
		lookupReprocessRecord = originalLookup
	})

	lookupReprocessRecord = func(ctx context.Context, documentID string) (reprocessDocumentRecord, bool, error) {
		return reprocessDocumentRecord{}, false, nil
	}

	request, recorder := httptestJSONRequest(t, http.MethodPost, "/documents/reprocess", `{"documentId":"missing-doc"}`)
	reprocessDocumentHandler(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, recorder.Code)
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

func TestUpdateDocumentLifecycleEventStatementCastsProcessingVersion(t *testing.T) {
	if strings.Count(updateDocumentLifecycleEventStatement, "$4::integer") != 2 {
		t.Fatalf("expected lifecycle insert statement to cast processing version in both insert branches")
	}
	if !strings.Contains(updateDocumentLifecycleEventStatement, "NULL::bigint") {
		t.Fatalf("expected lifecycle insert fallback branch to cast document_pk null")
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
