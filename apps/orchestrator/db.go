package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"pkg/postgresutil"

	"github.com/jackc/pgx/v5"
)

type txContextKey struct{}

type documentInventoryRecord struct {
	DocumentID               string
	ETag                     string
	VersionMarker            string
	SizeBytes                int64
	LastModified             time.Time
	HasLastModified          bool
	Status                   string
	DesiredProcessingVersion int
	CurrentProcessingVersion int
}

type reprocessDocumentRecord struct {
	DocumentID               string
	Bucket                   string
	ObjectKey                string
	SourceURI                string
	ContentType              string
	VersionMarker            string
	ETag                     string
	SizeBytes                int64
	LastModified             time.Time
	HasLastModified          bool
	Metadata                 map[string]interface{}
	DesiredProcessingVersion int
	CurrentProcessingVersion int
}

var (
	runDocumentTx         = withDocumentTx
	upsertPendingRecord   = upsertPendingDocument
	recordLastEvent       = updateLastDocumentEvent
	lookupReprocessRecord = findDocumentForReprocess
)

func withDocumentTx(ctx context.Context, fn func(context.Context) error) error {
	if postgresutil.Begin == nil {
		return fmt.Errorf("postgres is not initialized")
	}

	tx, err := postgresutil.Begin(ctx)
	if err != nil {
		return err
	}

	txCtx := context.WithValue(ctx, txContextKey{}, tx)
	if err := fn(txCtx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

func upsertPendingDocument(ctx context.Context, event documentEvent) error {
	tx, ok := ctx.Value(txContextKey{}).(pgx.Tx)
	if !ok {
		return fmt.Errorf("document transaction is not available")
	}

	processingVersion := defaultProcessingVersion(event.ProcessingVersion)
	_, err := tx.Exec(
		ctx,
		`INSERT INTO rag.documents (
			document_id,
			bucket_name,
			object_key,
			source_uri,
			content_type,
			version_marker,
			etag,
			size_bytes,
			last_modified,
			status,
			metadata,
			desired_processing_version,
			last_reconciled_at,
			updated_at,
			last_error
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::timestamptz, 'pending', $10, $11, NOW(), NOW(), NULL)
		ON CONFLICT (document_id)
		DO UPDATE SET
			bucket_name = EXCLUDED.bucket_name,
			object_key = EXCLUDED.object_key,
			source_uri = EXCLUDED.source_uri,
			content_type = EXCLUDED.content_type,
			version_marker = EXCLUDED.version_marker,
			etag = EXCLUDED.etag,
			size_bytes = EXCLUDED.size_bytes,
			last_modified = EXCLUDED.last_modified,
			status = EXCLUDED.status,
			metadata = EXCLUDED.metadata,
			desired_processing_version = EXCLUDED.desired_processing_version,
			last_reconciled_at = NOW(),
			updated_at = NOW(),
			last_error = NULL`,
		event.DocumentID,
		event.Bucket,
		event.ObjectKey,
		event.SourceURI,
		event.ContentType,
		nullIfEmpty(event.VersionMarker),
		nullIfEmpty(event.ETag),
		nullableInt64(event.SizeBytes),
		nullIfEmpty(event.LastModified),
		event.Metadata,
		processingVersion,
	)
	return err
}

func findDocumentByObject(ctx context.Context, bucket string, objectKey string) (documentInventoryRecord, bool, error) {
	if postgresutil.QueryRow == nil {
		return documentInventoryRecord{}, false, fmt.Errorf("postgres is not initialized")
	}

	var record documentInventoryRecord
	var lastModified sql.NullTime
	err := postgresutil.QueryRow(
		ctx,
		`SELECT
			document_id,
			COALESCE(etag, ''),
			COALESCE(version_marker, ''),
			COALESCE(size_bytes, 0),
			last_modified,
			status,
			desired_processing_version,
			current_processing_version
		FROM rag.documents
		WHERE bucket_name = $1 AND object_key = $2
		ORDER BY id
		LIMIT 1`,
		bucket,
		objectKey,
	).Scan(
		&record.DocumentID,
		&record.ETag,
		&record.VersionMarker,
		&record.SizeBytes,
		&lastModified,
		&record.Status,
		&record.DesiredProcessingVersion,
		&record.CurrentProcessingVersion,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return documentInventoryRecord{}, false, nil
		}
		return documentInventoryRecord{}, false, err
	}

	if lastModified.Valid {
		record.LastModified = lastModified.Time.UTC()
		record.HasLastModified = true
	}
	return record, true, nil
}

func upsertReconciledDocument(ctx context.Context, event documentEvent, status string, preserveStatus bool) error {
	if postgresutil.Exec == nil {
		return fmt.Errorf("postgres is not initialized")
	}

	processingVersion := defaultProcessingVersion(event.ProcessingVersion)
	if status == "" {
		status = "pending"
	}

	_, err := postgresutil.Exec(
		ctx,
		`INSERT INTO rag.documents (
			document_id,
			bucket_name,
			object_key,
			source_uri,
			content_type,
			version_marker,
			etag,
			size_bytes,
			last_modified,
			status,
			metadata,
			desired_processing_version,
			last_reconciled_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::timestamptz, $10, $11, $12, NOW(), NOW())
		ON CONFLICT (document_id)
		DO UPDATE SET
			bucket_name = EXCLUDED.bucket_name,
			object_key = EXCLUDED.object_key,
			source_uri = EXCLUDED.source_uri,
			content_type = EXCLUDED.content_type,
			version_marker = EXCLUDED.version_marker,
			etag = EXCLUDED.etag,
			size_bytes = EXCLUDED.size_bytes,
			last_modified = EXCLUDED.last_modified,
			status = CASE WHEN $13 THEN rag.documents.status ELSE EXCLUDED.status END,
			metadata = EXCLUDED.metadata,
			desired_processing_version = EXCLUDED.desired_processing_version,
			last_reconciled_at = NOW(),
			updated_at = NOW()`,
		event.DocumentID,
		event.Bucket,
		event.ObjectKey,
		event.SourceURI,
		event.ContentType,
		nullIfEmpty(event.VersionMarker),
		nullIfEmpty(event.ETag),
		nullableInt64(event.SizeBytes),
		nullIfEmpty(event.LastModified),
		status,
		event.Metadata,
		processingVersion,
		preserveStatus,
	)
	return err
}

func findDocumentForReprocess(ctx context.Context, documentID string) (reprocessDocumentRecord, bool, error) {
	if postgresutil.QueryRow == nil {
		return reprocessDocumentRecord{}, false, fmt.Errorf("postgres is not initialized")
	}

	var record reprocessDocumentRecord
	var lastModified sql.NullTime
	var metadataRaw string
	err := postgresutil.QueryRow(
		ctx,
		`SELECT
			document_id,
			COALESCE(bucket_name, ''),
			COALESCE(object_key, ''),
			source_uri,
			COALESCE(content_type, ''),
			COALESCE(version_marker, ''),
			COALESCE(etag, ''),
			COALESCE(size_bytes, 0),
			last_modified,
			COALESCE(metadata::text, '{}'),
			desired_processing_version,
			current_processing_version
		FROM rag.documents
		WHERE document_id = $1`,
		documentID,
	).Scan(
		&record.DocumentID,
		&record.Bucket,
		&record.ObjectKey,
		&record.SourceURI,
		&record.ContentType,
		&record.VersionMarker,
		&record.ETag,
		&record.SizeBytes,
		&lastModified,
		&metadataRaw,
		&record.DesiredProcessingVersion,
		&record.CurrentProcessingVersion,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return reprocessDocumentRecord{}, false, nil
		}
		return reprocessDocumentRecord{}, false, err
	}

	if lastModified.Valid {
		record.LastModified = lastModified.Time.UTC()
		record.HasLastModified = true
	}
	if metadataRaw != "" {
		if err := json.Unmarshal([]byte(metadataRaw), &record.Metadata); err != nil {
			return reprocessDocumentRecord{}, false, err
		}
	}

	return record, true, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableInt64(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}

func updateLastDocumentEvent(ctx context.Context, documentID string, subject string, occurredAt string) error {
	if postgresutil.Exec == nil {
		return fmt.Errorf("postgres is not initialized")
	}

	_, err := postgresutil.Exec(
		ctx,
		`UPDATE rag.documents
		SET last_event_subject = $2,
			last_event_at = $3::timestamptz,
			updated_at = NOW()
		WHERE document_id = $1`,
		documentID,
		subject,
		occurredAt,
	)
	return err
}
