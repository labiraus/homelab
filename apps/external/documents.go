package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"pkg/base"
	"pkg/minioutil"
	"pkg/prometheusutil"

	"github.com/minio/minio-go/v7"
)

const (
	documentsTreeLabel  = "documentsTreeHandler"
	documentObjectLabel = "documentObjectHandler"
	documentUploadLabel = "documentUploadHandler"
	maxUploadSize       = 32 << 20
)

type uploadedDocument struct {
	ObjectKey   string
	SizeBytes   int64
	ContentType string
}

var (
	listDocumentFolder = func(ctx context.Context, prefix string, maxKeys int) ([]minioutil.FolderEntry, error) {
		return minioutil.ListFolderEntriesInBucket(ctx, documentsBucket(), prefix, maxKeys)
	}
	readDocumentObject = func(ctx context.Context, objectKey string) (minioutil.Object, error) {
		return minioutil.ReadObjectFromBucket(ctx, documentsBucket(), objectKey)
	}
	putDocumentObject = func(ctx context.Context, objectKey string, body []byte, contentType string) (uploadedDocument, error) {
		info, err := minioutil.PutObjectBytesToBucket(ctx, documentsBucket(), objectKey, body, minio.PutObjectOptions{
			ContentType: contentType,
		})
		if err != nil {
			return uploadedDocument{}, err
		}
		return uploadedDocument{
			ObjectKey:   info.Key,
			SizeBytes:   info.Size,
			ContentType: defaultString(strings.TrimSpace(info.ContentType), contentType),
		}, nil
	}
)

func documentsTreeHandler(w http.ResponseWriter, r *http.Request) {
	handleDocumentAPI(documentsTreeLabel, w, r, func() error {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return nil
		}

		prefix := normalizePrefix(r.URL.Query().Get("prefix"))
		maxKeys := 0
		if raw := strings.TrimSpace(r.URL.Query().Get("maxKeys")); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 0 {
				w.WriteHeader(http.StatusBadRequest)
				return json.NewEncoder(w).Encode(ErrorResponse{Error: "maxKeys must be a positive integer"})
			}
			maxKeys = value
		}

		entries, err := listDocumentFolder(r.Context(), prefix, maxKeys)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return json.NewEncoder(w).Encode(ErrorResponse{Error: "could not list documents"})
		}

		response := DocumentTreeResponse{
			Bucket:      documentsBucket(),
			Prefix:      prefix,
			Breadcrumbs: buildBreadcrumbs(prefix),
			Entries:     toDocumentEntries(entries),
		}

		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(response)
	})
}

func documentObjectHandler(w http.ResponseWriter, r *http.Request) {
	handleDocumentAPI(documentObjectLabel, w, r, func() error {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return nil
		}

		objectKey := normalizeObjectKey(r.URL.Query().Get("objectKey"))
		if objectKey == "" {
			w.WriteHeader(http.StatusBadRequest)
			return json.NewEncoder(w).Encode(ErrorResponse{Error: "objectKey is required"})
		}

		object, err := readDocumentObject(r.Context(), objectKey)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return json.NewEncoder(w).Encode(ErrorResponse{Error: "could not read document"})
		}

		contentType := defaultString(strings.TrimSpace(object.Info.ContentType), http.DetectContentType(object.Body))
		filename := path.Base(objectKey)
		disposition := "inline"
		if r.URL.Query().Get("download") == "1" {
			disposition = "attachment"
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", strconv.FormatInt(int64(len(object.Body)), 10))
		w.Header().Set("Last-Modified", object.Info.LastModified.UTC().Format(http.TimeFormat))
		w.Header().Set("Content-Disposition", contentDisposition(disposition, filename))
		_, writeErr := w.Write(object.Body)
		return writeErr
	})
}

func documentUploadHandler(w http.ResponseWriter, r *http.Request) {
	handleDocumentAPI(documentUploadLabel, w, r, func() error {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return nil
		}

		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return json.NewEncoder(w).Encode(ErrorResponse{Error: "could not parse upload payload"})
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return json.NewEncoder(w).Encode(ErrorResponse{Error: "file is required"})
		}
		defer file.Close()

		body, err := io.ReadAll(io.LimitReader(file, maxUploadSize+1))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return json.NewEncoder(w).Encode(ErrorResponse{Error: "could not read uploaded file"})
		}
		if int64(len(body)) > maxUploadSize {
			w.WriteHeader(http.StatusBadRequest)
			return json.NewEncoder(w).Encode(ErrorResponse{Error: "uploaded file exceeds size limit"})
		}

		objectKey, err := resolveUploadObjectKey(r.FormValue("objectKey"), r.FormValue("prefix"), header.Filename)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		}

		contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
		if contentType == "" || strings.EqualFold(contentType, "application/octet-stream") {
			contentType = http.DetectContentType(body)
		}

		uploaded, err := putDocumentObject(r.Context(), objectKey, body, contentType)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return json.NewEncoder(w).Encode(ErrorResponse{Error: "could not upload document"})
		}
		if err := publishStoredDocumentEvent(r.Context(), uploaded); err != nil {
			slog.ErrorContext(r.Context(), "failed to publish stored document lifecycle notification", "error", err, "objectKey", uploaded.ObjectKey)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		return json.NewEncoder(w).Encode(DocumentUploadResponse{
			ObjectKey:   uploaded.ObjectKey,
			SizeBytes:   uploaded.SizeBytes,
			ContentType: uploaded.ContentType,
		})
	})
}

func handleDocumentAPI(label string, w http.ResponseWriter, r *http.Request, fn func() error) {
	var err error
	startTime := time.Now()
	prometheusutil.IncrementProcessed(label, "call")

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
		if err != nil {
			slog.ErrorContext(r.Context(), err.Error())
			prometheusutil.IncrementProcessed(label, "error")
		}
		prometheusutil.OpDuration(label, time.Since(startTime))
	}()

	if !minioConfigured() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "document storage is unavailable"})
		return
	}

	err = fn()
}

func toDocumentEntries(entries []minioutil.FolderEntry) []DocumentEntry {
	result := make([]DocumentEntry, 0, len(entries))
	for _, entry := range entries {
		documentEntry := DocumentEntry{
			Name:        entry.Name,
			Type:        entry.Type,
			ObjectKey:   entry.ObjectKey,
			Prefix:      entry.Prefix,
			SizeBytes:   entry.SizeBytes,
			ContentType: entry.ContentType,
		}
		if !entry.LastModified.IsZero() {
			documentEntry.LastModified = entry.LastModified.UTC().Format(time.RFC3339)
		}
		result = append(result, documentEntry)
	}
	return result
}

func buildBreadcrumbs(prefix string) []DocumentBreadcrumb {
	breadcrumbs := []DocumentBreadcrumb{{Name: documentsBucket(), Prefix: ""}}
	trimmed := strings.TrimSuffix(prefix, "/")
	if trimmed == "" {
		return breadcrumbs
	}

	currentPrefix := ""
	for _, segment := range strings.Split(trimmed, "/") {
		currentPrefix += segment + "/"
		breadcrumbs = append(breadcrumbs, DocumentBreadcrumb{
			Name:   segment,
			Prefix: currentPrefix,
		})
	}

	return breadcrumbs
}

func resolveUploadObjectKey(rawObjectKey string, rawPrefix string, filename string) (string, error) {
	objectKey := normalizeObjectKey(rawObjectKey)
	if objectKey != "" {
		return objectKey, nil
	}

	prefix := normalizePrefix(rawPrefix)
	filename = strings.TrimSpace(strings.ReplaceAll(filename, "\\", "/"))
	filename = path.Base(filename)
	filename = strings.Trim(strings.TrimSpace(filename), "/")
	if filename == "" || filename == "." {
		return "", fmt.Errorf("upload filename is required")
	}

	return prefix + filename, nil
}

func normalizePrefix(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = strings.Trim(value, "/")
	if value == "" {
		return ""
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return ""
	}
	return cleaned + "/"
}

func normalizeObjectKey(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = strings.Trim(value, "/")
	if value == "" {
		return ""
	}

	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return ""
	}
	return cleaned
}

func documentsBucket() string {
	return base.GetEnv("MINIO_BUCKET", "documents")
}

func contentDisposition(disposition string, filename string) string {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = "document"
	}
	return fmt.Sprintf("%s; filename=%q", disposition, filename)
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
