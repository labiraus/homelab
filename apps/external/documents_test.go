package main

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pkg/base"
	"pkg/documentevents"
	"pkg/minioutil"
	"pkg/prometheusutil"

	"github.com/minio/minio-go/v7"
)

func TestDocumentsTreeHandlerReturnsFolderStructure(t *testing.T) {
	setMinIOEnv(t)
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	previous := listDocumentFolder
	t.Cleanup(func() {
		listDocumentFolder = previous
	})

	listDocumentFolder = func(ctx context.Context, prefix string, maxKeys int) ([]minioutil.FolderEntry, error) {
		if prefix != "reports/" {
			t.Fatalf("expected reports/ prefix, got %q", prefix)
		}
		return []minioutil.FolderEntry{
			{Name: "archive", Type: "folder", Prefix: "reports/archive/"},
			{Name: "summary.pdf", Type: "file", ObjectKey: "reports/summary.pdf", SizeBytes: 512, ContentType: "application/pdf", LastModified: time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)},
		}, nil
	}

	request := httptest.NewRequest(http.MethodGet, "/api/documents/tree?prefix=reports", nil)
	recorder := httptest.NewRecorder()

	documentsTreeHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var response DocumentTreeResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json response: %v", err)
	}

	if response.Prefix != "reports/" {
		t.Fatalf("expected normalized prefix, got %q", response.Prefix)
	}
	if len(response.Breadcrumbs) != 2 {
		t.Fatalf("expected breadcrumbs for root and reports, got %#v", response.Breadcrumbs)
	}
	if len(response.Entries) != 2 {
		t.Fatalf("expected two entries, got %#v", response.Entries)
	}
	if response.Entries[0].Type != "folder" {
		t.Fatalf("expected folders first, got %#v", response.Entries[0])
	}
}

func TestDocumentObjectHandlerStreamsContent(t *testing.T) {
	setMinIOEnv(t)
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	previous := readDocumentObject
	t.Cleanup(func() {
		readDocumentObject = previous
	})

	readDocumentObject = func(ctx context.Context, objectKey string) (minioutil.Object, error) {
		if objectKey != "reports/summary.txt" {
			t.Fatalf("expected reports/summary.txt, got %q", objectKey)
		}
		return minioutil.Object{
			Info: minioObjectInfo("reports/summary.txt", "text/plain; charset=utf-8", time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)),
			Body: []byte("hello"),
		}, nil
	}

	request := httptest.NewRequest(http.MethodGet, "/api/documents/object?objectKey=reports/summary.txt", nil)
	recorder := httptest.NewRecorder()

	documentObjectHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if recorder.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("unexpected content type %q", recorder.Header().Get("Content-Type"))
	}
	if body := recorder.Body.String(); body != "hello" {
		t.Fatalf("expected hello body, got %q", body)
	}
}

func TestDocumentUploadHandlerStoresFile(t *testing.T) {
	setMinIOEnv(t)
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	previous := putDocumentObject
	previousPublish := publishStoredDocumentEvent
	t.Cleanup(func() {
		putDocumentObject = previous
		publishStoredDocumentEvent = previousPublish
	})

	putDocumentObject = func(ctx context.Context, objectKey string, body []byte, contentType string) (uploadedDocument, error) {
		if objectKey != "reports/demo.txt" {
			t.Fatalf("expected reports/demo.txt, got %q", objectKey)
		}
		if string(body) != "hello" {
			t.Fatalf("expected hello body, got %q", string(body))
		}
		return uploadedDocument{
			ObjectKey:   objectKey,
			SizeBytes:   int64(len(body)),
			ContentType: contentType,
		}, nil
	}
	var published uploadedDocument
	publishStoredDocumentEvent = func(ctx context.Context, uploaded uploadedDocument) error {
		published = uploaded
		return nil
	}

	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	fileWriter, err := writer.CreateFormFile("file", "demo.txt")
	if err != nil {
		t.Fatalf("failed to create multipart file: %v", err)
	}
	if _, err := fileWriter.Write([]byte("hello")); err != nil {
		t.Fatalf("failed to write file body: %v", err)
	}
	if err := writer.WriteField("prefix", "reports"); err != nil {
		t.Fatalf("failed to write prefix: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/documents/upload", &payload)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()

	documentUploadHandler(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", recorder.Code)
	}

	var response DocumentUploadResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json response: %v", err)
	}
	if response.ObjectKey != "reports/demo.txt" {
		t.Fatalf("expected uploaded object key, got %q", response.ObjectKey)
	}
	if !strings.Contains(response.ContentType, "text/plain") {
		t.Fatalf("expected text content type, got %q", response.ContentType)
	}
	if published.ObjectKey != "reports/demo.txt" || published.SizeBytes != int64(len("hello")) {
		t.Fatalf("expected stored lifecycle event for uploaded object, got %+v", published)
	}
}

func TestDocumentUploadHandlerIgnoresStoredNotificationFailure(t *testing.T) {
	setMinIOEnv(t)
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	previousPut := putDocumentObject
	previousPublish := publishStoredDocumentEvent
	t.Cleanup(func() {
		putDocumentObject = previousPut
		publishStoredDocumentEvent = previousPublish
	})

	putDocumentObject = func(ctx context.Context, objectKey string, body []byte, contentType string) (uploadedDocument, error) {
		return uploadedDocument{
			ObjectKey:   objectKey,
			SizeBytes:   int64(len(body)),
			ContentType: contentType,
		}, nil
	}
	publishStoredDocumentEvent = func(ctx context.Context, uploaded uploadedDocument) error {
		return context.Canceled
	}

	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	fileWriter, err := writer.CreateFormFile("file", "demo.txt")
	if err != nil {
		t.Fatalf("failed to create multipart file: %v", err)
	}
	if _, err := fileWriter.Write([]byte("hello")); err != nil {
		t.Fatalf("failed to write file body: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/documents/upload", &payload)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()

	documentUploadHandler(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status 201 despite notification failure, got %d", recorder.Code)
	}
}

func TestStoredDocumentLifecycleEventUsesS3DocumentID(t *testing.T) {
	setMinIOEnv(t)

	event, ok := storedDocumentLifecycleEvent(uploadedDocument{
		ObjectKey:   "reports/demo.txt",
		ContentType: "text/plain; charset=utf-8",
	})

	if !ok {
		t.Fatal("expected stored lifecycle event")
	}
	if event.Subject != documentevents.SubjectMinIOStored {
		t.Fatalf("expected minio stored subject, got %q", event.Subject)
	}
	if event.DocumentID != "s3://documents/reports/demo.txt" || event.Bucket != "documents" || event.ObjectKey != "reports/demo.txt" {
		t.Fatalf("unexpected object identity: %+v", event)
	}
	if event.ContentType != "text/plain; charset=utf-8" {
		t.Fatalf("expected content type in event, got %q", event.ContentType)
	}
	if event.ProcessingVersion != 0 {
		t.Fatalf("stored event should not claim a processing version, got %d", event.ProcessingVersion)
	}
	if event.OccurredAt == "" {
		t.Fatal("expected occurredAt timestamp")
	}
}

func TestDocumentInventoryHandlerReturnsFilteredInventory(t *testing.T) {
	setPostgresEnv(t)
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	previous := listDocumentInventory
	t.Cleanup(func() {
		listDocumentInventory = previous
	})

	listDocumentInventory = func(ctx context.Context, request DocumentInventoryRequest, limit int) ([]DocumentInventoryRecord, error) {
		if request.Status != "processed" {
			t.Fatalf("expected processed status, got %q", request.Status)
		}
		if request.Prefix != "runbooks/" {
			t.Fatalf("expected normalized runbooks/ prefix, got %q", request.Prefix)
		}
		if request.Metadata["tag"] != "runbook" {
			t.Fatalf("expected metadata filter, got %#v", request.Metadata)
		}
		if limit != 4 {
			t.Fatalf("expected limit 4, got %d", limit)
		}
		return []DocumentInventoryRecord{
			{
				DocumentID:               "doc-1",
				Bucket:                   "documents",
				ObjectKey:                "runbooks/process.md",
				SourceURI:                "s3://documents/runbooks/process.md",
				ContentType:              "text/markdown",
				Status:                   "processed",
				Metadata:                 map[string]any{"tag": "runbook"},
				DesiredProcessingVersion: 3,
				CurrentProcessingVersion: 3,
				LastEventSubject:         documentevents.SubjectProcessorCompleted,
				LastEventAt:              "2026-05-10T18:00:00Z",
			},
		}, nil
	}

	request := httptest.NewRequest(http.MethodPost, "/api/documents/inventory", strings.NewReader(`{"status":"processed","prefix":"runbooks","metadata":{"tag":"runbook"},"limit":4}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	documentInventoryHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var response DocumentInventoryResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected inventory response: %v", err)
	}
	if response.Count != 1 || response.Documents[0].ObjectKey != "runbooks/process.md" {
		t.Fatalf("unexpected inventory response: %#v", response)
	}
	if response.Documents[0].Metadata["tag"] != "runbook" {
		t.Fatalf("expected response metadata, got %#v", response.Documents[0].Metadata)
	}
}

func TestDocumentInventoryHandlerValidatesLimit(t *testing.T) {
	setPostgresEnv(t)
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	request := httptest.NewRequest(http.MethodPost, "/api/documents/inventory", strings.NewReader(`{"limit":-1}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	documentInventoryHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestDocumentSearchHandlerReturnsRankedHits(t *testing.T) {
	setEmbeddingEnv(t)
	setPostgresEnv(t)
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	previousEmbedding := fetchQueryEmbedding
	previousSearch := searchDocuments
	t.Cleanup(func() {
		fetchQueryEmbedding = previousEmbedding
		searchDocuments = previousSearch
	})

	fetchQueryEmbedding = func(ctx context.Context, input string) ([]float64, string, error) {
		if input != "refresh kubeconfig" {
			t.Fatalf("expected search query to be forwarded, got %q", input)
		}
		return []float64{0.1, 0.2, 0.3}, "local-embeddings", nil
	}
	searchDocuments = func(ctx context.Context, embedding []float64, model string, request DocumentSearchRequest, limit int) ([]DocumentSearchHit, error) {
		if model != "local-embeddings" {
			t.Fatalf("expected embedding model, got %q", model)
		}
		if request.Prefix != "scripts/" {
			t.Fatalf("expected normalized prefix, got %q", request.Prefix)
		}
		if limit != 5 {
			t.Fatalf("expected explicit limit, got %d", limit)
		}
		return []DocumentSearchHit{
			{
				DocumentID:        "doc-1",
				SourceURI:         "s3://documents/scripts/refresh-kubeconfig.sh",
				ObjectKey:         "scripts/refresh-kubeconfig.sh",
				ContentType:       "text/x-shellscript",
				ChunkID:           42,
				ChunkIndex:        0,
				ChunkText:         "aws eks update-kubeconfig --name homelab",
				ProcessingVersion: 1,
				Distance:          0.08,
				Similarity:        0.92,
				LastProcessedAt:   "2026-04-14T12:00:00Z",
				Citation: &DocumentCitation{
					ID:                "s3://documents/scripts/refresh-kubeconfig.sh#chunk-0",
					Label:             "scripts/refresh-kubeconfig.sh chunk 0",
					SourceURI:         "s3://documents/scripts/refresh-kubeconfig.sh",
					ObjectKey:         "scripts/refresh-kubeconfig.sh",
					ChunkID:           42,
					ChunkIndex:        0,
					ProcessingVersion: 1,
				},
			},
		}, nil
	}

	request := httptest.NewRequest(http.MethodPost, "/api/documents/search", strings.NewReader(`{"query":"refresh kubeconfig","prefix":"scripts","limit":5}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	documentSearchHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var response DocumentSearchResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json response: %v", err)
	}

	if response.Query != "refresh kubeconfig" {
		t.Fatalf("expected query echo, got %q", response.Query)
	}
	if len(response.Hits) != 1 {
		t.Fatalf("expected one hit, got %#v", response.Hits)
	}
	if response.Hits[0].DocumentID != "doc-1" {
		t.Fatalf("expected doc-1 hit, got %#v", response.Hits[0])
	}
	if response.Hits[0].Citation == nil || response.Hits[0].Citation.Label != "scripts/refresh-kubeconfig.sh chunk 0" {
		t.Fatalf("expected citation payload, got %#v", response.Hits[0].Citation)
	}
}

func TestDocumentContextHandlerAssemblesCitedContext(t *testing.T) {
	setEmbeddingEnv(t)
	setPostgresEnv(t)
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	previousEmbedding := fetchQueryEmbedding
	previousSearch := searchDocuments
	t.Cleanup(func() {
		fetchQueryEmbedding = previousEmbedding
		searchDocuments = previousSearch
	})

	fetchQueryEmbedding = func(ctx context.Context, input string) ([]float64, string, error) {
		if input != "ancient tower" {
			t.Fatalf("expected context query to be forwarded, got %q", input)
		}
		return []float64{0.3, 0.2, 0.1}, "local-embeddings", nil
	}
	searchDocuments = func(ctx context.Context, embedding []float64, model string, request DocumentSearchRequest, limit int) ([]DocumentSearchHit, error) {
		if request.Prefix != "campaign/" {
			t.Fatalf("expected normalized prefix, got %q", request.Prefix)
		}
		if limit != 2 {
			t.Fatalf("expected explicit limit, got %d", limit)
		}
		return []DocumentSearchHit{
			{
				DocumentID:        "doc-1",
				ObjectKey:         "campaign/tower.md",
				ChunkID:           7,
				ChunkIndex:        0,
				ChunkText:         "The ancient tower has a brass door.",
				ProcessingVersion: 2,
				Citation: &DocumentCitation{
					ID:                "s3://documents/campaign/tower.md#chunk-0",
					Label:             "campaign/tower.md chunk 0",
					ObjectKey:         "campaign/tower.md",
					ChunkID:           7,
					ChunkIndex:        0,
					ProcessingVersion: 2,
				},
			},
		}, nil
	}

	request := httptest.NewRequest(http.MethodPost, "/api/documents/context", strings.NewReader(`{"query":"ancient tower","prefix":"campaign","limit":2,"maxChars":120}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	documentContextHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var response DocumentContextResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json response: %v", err)
	}
	if !strings.Contains(response.Context, "[1] campaign/tower.md chunk 0") {
		t.Fatalf("expected citation marker in context, got %q", response.Context)
	}
	if len(response.Citations) != 1 || response.Citations[0].Reference != "[1]" {
		t.Fatalf("expected citation list with reference, got %#v", response.Citations)
	}
}

func TestAssembleDocumentContextTruncatesToBudget(t *testing.T) {
	response := assembleDocumentContext("query", []DocumentSearchHit{
		{
			ChunkText: "abcdef",
			Citation:  &DocumentCitation{Label: "doc chunk 0"},
		},
	}, 12)

	if len(response.Context) != 12 {
		t.Fatalf("expected context to respect max chars, got %d: %q", len(response.Context), response.Context)
	}
	if !response.Truncated {
		t.Fatal("expected context to report truncation")
	}
}

func TestDocumentSearchBaseQueryUsesCurrentProcessingVersion(t *testing.T) {
	if !strings.Contains(documentSearchBaseQuery(), "c.processing_version = d.current_processing_version") {
		t.Fatalf("expected search query to filter chunks to the document current processing version")
	}
	if !strings.Contains(documentSearchBaseQuery(), "COALESCE(d.metadata::text, '{}')") {
		t.Fatalf("expected search query to return document metadata")
	}
}

func TestNormalizeMetadataFilterDropsEmptyEntries(t *testing.T) {
	filter := normalizeMetadataFilter(map[string]any{
		" tag ": " runbook ",
		"":      "ignored",
		"empty": " ",
		"tags":  []any{"ops", "runbook"},
	})

	if len(filter) != 2 || filter["tag"] != "runbook" {
		t.Fatalf("expected normalized metadata filter, got %#v", filter)
	}
}

func TestDocumentSearchHandlerValidatesRequest(t *testing.T) {
	setEmbeddingEnv(t)
	setPostgresEnv(t)
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	request := httptest.NewRequest(http.MethodPost, "/api/documents/search", strings.NewReader(`{"query":" "}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	documentSearchHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json response: %v", err)
	}
	if response.Error != "query is required" {
		t.Fatalf("expected validation error, got %q", response.Error)
	}
}

func TestDocumentHistoryHandlerReturnsLifecycleEvents(t *testing.T) {
	setPostgresEnv(t)
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	previousHistory := listDocumentHistory
	t.Cleanup(func() {
		listDocumentHistory = previousHistory
	})

	listDocumentHistory = func(ctx context.Context, request DocumentHistoryRequest, limit int) ([]DocumentLifecycleHistoryEvent, error) {
		if request.DocumentID != "doc-1" {
			t.Fatalf("expected document id doc-1, got %q", request.DocumentID)
		}
		if request.ProcessingVersion != 2 {
			t.Fatalf("expected processing version 2, got %d", request.ProcessingVersion)
		}
		if limit != 3 {
			t.Fatalf("expected explicit limit 3, got %d", limit)
		}
		return []DocumentLifecycleHistoryEvent{
			{
				ID:                9,
				DocumentID:        "doc-1",
				Subject:           "documents.events.processor.completed",
				ProcessingVersion: 2,
				OccurredAt:        "2026-05-09T20:00:00Z",
				CreatedAt:         "2026-05-09T20:00:01Z",
				Payload: map[string]any{
					"documentId": "doc-1",
				},
			},
		}, nil
	}

	request := httptest.NewRequest(http.MethodPost, "/api/documents/history", strings.NewReader(`{"documentId":"doc-1","processingVersion":2,"limit":3}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	documentHistoryHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var response DocumentHistoryResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json response: %v", err)
	}
	if response.DocumentID != "doc-1" || response.Count != 1 {
		t.Fatalf("unexpected history response: %#v", response)
	}
	if response.Events[0].Subject != "documents.events.processor.completed" {
		t.Fatalf("expected completed event, got %#v", response.Events[0])
	}
}

func TestDocumentHistoryHandlerValidatesDocumentID(t *testing.T) {
	setPostgresEnv(t)
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	request := httptest.NewRequest(http.MethodPost, "/api/documents/history", strings.NewReader(`{"documentId":" "}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	documentHistoryHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected valid json response: %v", err)
	}
	if response.Error != "documentId is required" {
		t.Fatalf("expected validation error, got %q", response.Error)
	}
}

func TestDocumentCurationHandlerProxiesToOrchestrator(t *testing.T) {
	setOrchestratorEnv(t)
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	previousProxy := proxyOrchestratorDocumentAction
	t.Cleanup(func() {
		proxyOrchestratorDocumentAction = previousProxy
	})

	proxyOrchestratorDocumentAction = func(ctx context.Context, path string, body []byte) (orchestratorDocumentActionResponse, error) {
		if path != "/documents/curation" {
			t.Fatalf("expected curation path, got %q", path)
		}

		var request map[string]any
		if err := json.Unmarshal(body, &request); err != nil {
			t.Fatalf("expected valid proxied json: %v", err)
		}
		if request["documentId"] != "doc-1" {
			t.Fatalf("expected document id doc-1, got %#v", request["documentId"])
		}
		metadata, ok := request["metadata"].(map[string]any)
		if !ok || metadata["tag"] != "runbook" {
			t.Fatalf("expected metadata to be proxied, got %#v", request["metadata"])
		}

		return orchestratorDocumentActionResponse{
			statusCode:  http.StatusOK,
			contentType: "application/json",
			body:        []byte(`{"status":"updated","documentId":"doc-1","metadata":{"tag":"runbook"}}`),
		}, nil
	}

	request := httptest.NewRequest(http.MethodPost, "/api/documents/curation", strings.NewReader(`{"documentId":"doc-1","metadata":{"tag":"runbook"}}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	documentCurationHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"status":"updated"`) {
		t.Fatalf("expected orchestrator body to be returned, got %q", recorder.Body.String())
	}
}

func TestDocumentEditTextHandlerProxiesToOrchestrator(t *testing.T) {
	setOrchestratorEnv(t)
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	previousProxy := proxyOrchestratorDocumentAction
	t.Cleanup(func() {
		proxyOrchestratorDocumentAction = previousProxy
	})

	proxyOrchestratorDocumentAction = func(ctx context.Context, path string, body []byte) (orchestratorDocumentActionResponse, error) {
		if path != "/documents/edit-text" {
			t.Fatalf("expected edit-text path, got %q", path)
		}

		var request map[string]any
		if err := json.Unmarshal(body, &request); err != nil {
			t.Fatalf("expected valid proxied json: %v", err)
		}
		if request["documentId"] != "doc-1" {
			t.Fatalf("expected document id doc-1, got %#v", request["documentId"])
		}
		if request["text"] != "updated notes" {
			t.Fatalf("expected replacement text to be proxied, got %#v", request["text"])
		}
		if request["contentType"] != "text/markdown" {
			t.Fatalf("expected content type to be proxied, got %#v", request["contentType"])
		}

		return orchestratorDocumentActionResponse{
			statusCode:  http.StatusAccepted,
			contentType: "application/json",
			body:        []byte(`{"status":"queued","documentId":"doc-1","processingVersion":6,"sourceUri":"s3://documents/runbooks/process.md","objectKey":"runbooks/process.md"}`),
		}, nil
	}

	request := httptest.NewRequest(http.MethodPost, "/api/documents/edit-text", strings.NewReader(`{"documentId":"doc-1","text":"updated notes","contentType":"text/markdown"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	documentEditTextHandler(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"processingVersion":6`) {
		t.Fatalf("expected orchestrator body to be returned, got %q", recorder.Body.String())
	}
}

func TestDocumentReprocessHandlerProxiesToOrchestrator(t *testing.T) {
	setOrchestratorEnv(t)
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	previousProxy := proxyOrchestratorDocumentAction
	t.Cleanup(func() {
		proxyOrchestratorDocumentAction = previousProxy
	})

	proxyOrchestratorDocumentAction = func(ctx context.Context, path string, body []byte) (orchestratorDocumentActionResponse, error) {
		if path != "/documents/reprocess" {
			t.Fatalf("expected reprocess path, got %q", path)
		}

		var request map[string]any
		if err := json.Unmarshal(body, &request); err != nil {
			t.Fatalf("expected valid proxied json: %v", err)
		}
		if request["documentId"] != "doc-1" {
			t.Fatalf("expected document id doc-1, got %#v", request["documentId"])
		}

		return orchestratorDocumentActionResponse{
			statusCode:  http.StatusAccepted,
			contentType: "application/json",
			body:        []byte(`{"status":"queued","documentId":"doc-1","processingVersion":5,"sourceUri":"s3://documents/runbooks/process.md"}`),
		}, nil
	}

	request := httptest.NewRequest(http.MethodPost, "/api/documents/reprocess", strings.NewReader(`{"documentId":"doc-1"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	documentReprocessHandler(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"processingVersion":5`) {
		t.Fatalf("expected orchestrator body to be returned, got %q", recorder.Body.String())
	}
}

func TestDocumentScanBucketHandlerProxiesToOrchestrator(t *testing.T) {
	setOrchestratorEnv(t)
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	previousProxy := proxyOrchestratorDocumentAction
	t.Cleanup(func() {
		proxyOrchestratorDocumentAction = previousProxy
	})

	proxyOrchestratorDocumentAction = func(ctx context.Context, path string, body []byte) (orchestratorDocumentActionResponse, error) {
		if path != "/documents/scan-bucket" {
			t.Fatalf("expected scan path, got %q", path)
		}

		var request map[string]any
		if err := json.Unmarshal(body, &request); err != nil {
			t.Fatalf("expected valid proxied json: %v", err)
		}
		if request["prefix"] != "runbooks/" {
			t.Fatalf("expected prefix runbooks/, got %#v", request["prefix"])
		}
		if request["maxKeys"] != float64(50) {
			t.Fatalf("expected maxKeys 50, got %#v", request["maxKeys"])
		}

		return orchestratorDocumentActionResponse{
			statusCode:  http.StatusOK,
			contentType: "application/json",
			body:        []byte(`{"status":"scanned","bucket":"documents","prefix":"runbooks/","scanned":2,"queued":1,"skipped":1}`),
		}, nil
	}

	request := httptest.NewRequest(http.MethodPost, "/api/documents/scan-bucket", strings.NewReader(`{"prefix":"runbooks/","maxKeys":50}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	documentScanBucketHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"queued":1`) {
		t.Fatalf("expected orchestrator body to be returned, got %q", recorder.Body.String())
	}
}

func TestDocumentControlHandlerRequiresConfiguredOrchestrator(t *testing.T) {
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	request := httptest.NewRequest(http.MethodPost, "/api/documents/curation", strings.NewReader(`{"documentId":"doc-1","metadata":{"tag":"runbook"}}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	documentCurationHandler(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", recorder.Code)
	}
}

func TestDocumentControlHandlerValidatesJSON(t *testing.T) {
	setOrchestratorEnv(t)
	base.ServiceName = "external_test_" + t.Name()
	prometheusutil.Start(http.NewServeMux())

	request := httptest.NewRequest(http.MethodPost, "/api/documents/reprocess", strings.NewReader(`{`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	documentReprocessHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestLocalQueryEmbeddingFallback(t *testing.T) {
	t.Setenv("EMBEDDING_ENDPOINT", "")
	t.Setenv("EMBEDDING_MODEL", "local-embeddings")

	embedding, model, err := getQueryEmbedding(context.Background(), "Astra keeps field notes")
	if err != nil {
		t.Fatalf("expected local embedding to succeed: %v", err)
	}
	if model != "local-embeddings" {
		t.Fatalf("expected local model, got %q", model)
	}
	if len(embedding) != 384 {
		t.Fatalf("expected 384-dimensional embedding, got %d", len(embedding))
	}
	if !embeddingsConfigured() {
		t.Fatal("expected local embeddings to count as configured")
	}
}

func setMinIOEnv(t *testing.T) {
	t.Helper()
	t.Setenv("MINIO_ENDPOINT", "svartalfheim:9000")
	t.Setenv("MINIO_ACCESS_KEY", "test-access")
	t.Setenv("MINIO_SECRET_KEY", "test-secret")
	t.Setenv("MINIO_BUCKET", "documents")
}

func setEmbeddingEnv(t *testing.T) {
	t.Helper()
	t.Setenv("EMBEDDING_ENDPOINT", "http://embeddings.homelab.svc.cluster.local/v1/embeddings")
	t.Setenv("EMBEDDING_MODEL", "local-embeddings")
}

func setPostgresEnv(t *testing.T) {
	t.Helper()
	t.Setenv("POSTGRES_HOST", "app-db-rw.data.svc.cluster.local")
	t.Setenv("POSTGRES_USER", "app")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_DATABASE", "app")
}

func setOrchestratorEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ORCHESTRATOR_BASE_URL", "http://homelab-orchestrator.homelab.svc.cluster.local")
}

func minioObjectInfo(key string, contentType string, lastModified time.Time) minio.ObjectInfo {
	return minio.ObjectInfo{
		Key:          key,
		ContentType:  contentType,
		LastModified: lastModified,
	}
}
