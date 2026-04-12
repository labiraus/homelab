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

func setMinIOEnv(t *testing.T) {
	t.Helper()
	t.Setenv("MINIO_ENDPOINT", "svartalfheim:9000")
	t.Setenv("MINIO_ACCESS_KEY", "test-access")
	t.Setenv("MINIO_SECRET_KEY", "test-secret")
	t.Setenv("MINIO_BUCKET", "documents")
}

func minioObjectInfo(key string, contentType string, lastModified time.Time) minio.ObjectInfo {
	return minio.ObjectInfo{
		Key:          key,
		ContentType:  contentType,
		LastModified: lastModified,
	}
}
