package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pkg/minioutil"

	"github.com/minio/minio-go/v7"
)

type scanBucketRequest struct {
	Bucket            string `json:"bucket,omitempty"`
	Prefix            string `json:"prefix,omitempty"`
	MaxKeys           int    `json:"maxKeys,omitempty"`
	ProcessingVersion int    `json:"processingVersion,omitempty"`
}

type scanBucketResponse struct {
	Status      string               `json:"status"`
	Bucket      string               `json:"bucket"`
	Prefix      string               `json:"prefix,omitempty"`
	Scanned     int                  `json:"scanned"`
	Created     int                  `json:"created"`
	Updated     int                  `json:"updated"`
	Queued      int                  `json:"queued"`
	Skipped     int                  `json:"skipped"`
	Unsupported int                  `json:"unsupported"`
	Failed      int                  `json:"failed"`
	Results     []scanDocumentResult `json:"results,omitempty"`
}

type scanDocumentResult struct {
	DocumentID  string `json:"documentId"`
	ObjectKey   string `json:"objectKey"`
	ContentType string `json:"contentType,omitempty"`
	Action      string `json:"action"`
	Error       string `json:"error,omitempty"`
}

var (
	listBucketObjectsForScan = listMinIOObjectsForScan
	findInventoryRecord      = findDocumentByObject
	upsertInventoryRecord    = upsertReconciledDocument
)

func scanBucketHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var request scanBucketRequest
	if err := decodeOptionalJSONBody(r.Body, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	if request.MaxKeys < 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "maxKeys must be greater than or equal to zero"})
		return
	}

	response, err := scanBucket(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to scan bucket"})
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func decodeOptionalJSONBody(body io.Reader, target any) error {
	if body == nil {
		return nil
	}

	decoder := json.NewDecoder(body)
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

func scanBucket(ctx context.Context, request scanBucketRequest) (scanBucketResponse, error) {
	bucket := strings.TrimSpace(request.Bucket)
	if bucket == "" {
		bucket = documentsBucket()
	}

	prefix := normalizeObjectPrefix(request.Prefix)
	objects, err := listBucketObjectsForScan(ctx, bucket, prefix, request.MaxKeys)
	if err != nil {
		return scanBucketResponse{}, err
	}

	response := scanBucketResponse{
		Status: "scanned",
		Bucket: bucket,
		Prefix: prefix,
	}

	for _, object := range objects {
		objectKey := strings.TrimSpace(object.Key)
		if objectKey == "" || strings.HasSuffix(objectKey, "/") {
			response.Skipped++
			continue
		}

		response.Scanned++
		event := eventFromObject(bucket, object, request.ProcessingVersion)
		record, found, err := findInventoryRecord(ctx, bucket, objectKey)
		if err != nil {
			response.Failed++
			response.Results = append(response.Results, scanDocumentResult{
				DocumentID: event.DocumentID,
				ObjectKey:  objectKey,
				Action:     "failed",
				Error:      err.Error(),
			})
			continue
		}
		if found {
			event.DocumentID = record.DocumentID
		}

		if !supportedTextContentType(event.ContentType) {
			if err := upsertInventoryRecord(ctx, event, "unsupported", false); err != nil {
				response.Failed++
				response.Results = append(response.Results, scanDocumentResult{
					DocumentID:  event.DocumentID,
					ObjectKey:   objectKey,
					ContentType: event.ContentType,
					Action:      "failed",
					Error:       err.Error(),
				})
				continue
			}

			response.Unsupported++
			countInventoryMutation(&response, found)
			response.Results = append(response.Results, scanDocumentResult{
				DocumentID:  event.DocumentID,
				ObjectKey:   objectKey,
				ContentType: event.ContentType,
				Action:      "unsupported",
			})
			continue
		}

		if found && !documentNeedsQueue(record, event) {
			if err := upsertInventoryRecord(ctx, event, "pending", true); err != nil {
				response.Failed++
				response.Results = append(response.Results, scanDocumentResult{
					DocumentID:  event.DocumentID,
					ObjectKey:   objectKey,
					ContentType: event.ContentType,
					Action:      "failed",
					Error:       err.Error(),
				})
				continue
			}

			response.Skipped++
			response.Results = append(response.Results, scanDocumentResult{
				DocumentID:  event.DocumentID,
				ObjectKey:   objectKey,
				ContentType: event.ContentType,
				Action:      "skipped",
			})
			continue
		}

		if err := queueDocument(ctx, event); err != nil {
			response.Failed++
			response.Results = append(response.Results, scanDocumentResult{
				DocumentID:  event.DocumentID,
				ObjectKey:   objectKey,
				ContentType: event.ContentType,
				Action:      "failed",
				Error:       err.Error(),
			})
			continue
		}

		response.Queued++
		countInventoryMutation(&response, found)
		response.Results = append(response.Results, scanDocumentResult{
			DocumentID:  event.DocumentID,
			ObjectKey:   objectKey,
			ContentType: event.ContentType,
			Action:      "queued",
		})
	}

	return response, nil
}

func countInventoryMutation(response *scanBucketResponse, found bool) {
	if found {
		response.Updated++
		return
	}
	response.Created++
}

func listMinIOObjectsForScan(ctx context.Context, bucket string, prefix string, maxKeys int) ([]minio.ObjectInfo, error) {
	return minioutil.ListObjectInfoInBucket(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}, maxKeys)
}

func eventFromObject(bucket string, object minio.ObjectInfo, processingVersion int) documentEvent {
	objectKey := strings.TrimSpace(object.Key)
	sourceURI := fmt.Sprintf("s3://%s/%s", bucket, objectKey)
	contentType := textContentTypeForObject(object)

	return documentEvent{
		DocumentID:        sourceURI,
		Bucket:            bucket,
		ObjectKey:         objectKey,
		SourceURI:         sourceURI,
		ContentType:       contentType,
		VersionMarker:     strings.TrimSpace(object.VersionID),
		ETag:              strings.TrimSpace(object.ETag),
		SizeBytes:         object.Size,
		LastModified:      formatObjectTime(object.LastModified),
		RequestedAt:       time.Now().UTC().Format(time.RFC3339),
		ProcessingVersion: defaultProcessingVersion(processingVersion),
		Metadata: map[string]interface{}{
			"reconciledBy": "orchestrator.scanBucket",
		},
	}
}

func documentNeedsQueue(record documentInventoryRecord, event documentEvent) bool {
	if record.Status == "unsupported" {
		return true
	}
	if documentSourceChanged(record, event) {
		return true
	}

	processingVersion := defaultProcessingVersion(event.ProcessingVersion)
	if record.Status == "pending" || record.Status == "processing" {
		return processingVersion > record.DesiredProcessingVersion
	}

	return record.CurrentProcessingVersion < processingVersion
}

func documentSourceChanged(record documentInventoryRecord, event documentEvent) bool {
	if event.VersionMarker != "" && event.VersionMarker != record.VersionMarker {
		return true
	}
	if event.ETag != "" && event.ETag != record.ETag {
		return true
	}
	if event.SizeBytes > 0 && event.SizeBytes != record.SizeBytes {
		return true
	}

	eventModified, err := time.Parse(time.RFC3339Nano, event.LastModified)
	if err == nil && record.HasLastModified && !eventModified.Equal(record.LastModified) {
		return true
	}
	if err == nil && !record.HasLastModified {
		return true
	}

	return false
}

func textContentTypeForObject(object minio.ObjectInfo) string {
	contentType := strings.TrimSpace(object.ContentType)
	if supportedTextContentType(contentType) {
		return contentType
	}

	extension := strings.ToLower(filepath.Ext(object.Key))
	switch extension {
	case ".md", ".markdown":
		return "text/markdown; charset=utf-8"
	}

	if extensionType := mime.TypeByExtension(extension); supportedTextContentType(extensionType) {
		return extensionType
	}

	if contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}

func formatObjectTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func normalizeObjectPrefix(prefix string) string {
	prefix = strings.TrimSpace(strings.ReplaceAll(prefix, "\\", "/"))
	prefix = strings.TrimPrefix(prefix, "/")
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return prefix
}

func documentsBucket() string {
	bucket := strings.TrimSpace(os.Getenv("MINIO_BUCKET"))
	if bucket == "" {
		return "documents"
	}
	return bucket
}
