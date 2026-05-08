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
	t.Cleanup(func() {
		putDocumentObject = previous
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

func minioObjectInfo(key string, contentType string, lastModified time.Time) minio.ObjectInfo {
	return minio.ObjectInfo{
		Key:          key,
		ContentType:  contentType,
		LastModified: lastModified,
	}
}
