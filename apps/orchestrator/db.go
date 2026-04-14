package main

import (
	"context"
	"fmt"

	"pkg/postgresutil"

	"github.com/jackc/pgx/v5"
)

type txContextKey struct{}

var (
	runDocumentTx       = withDocumentTx
	upsertPendingRecord = upsertPendingDocument
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
			updated_at,
			last_error
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::timestamptz, 'pending', $10, $11, NOW(), NULL)
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
