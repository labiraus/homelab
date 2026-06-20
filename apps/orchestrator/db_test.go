package main

import (
	"context"
	"strings"
	"testing"

	"pkg/postgresutil"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestInsertDocumentChangeAuditIncludesAuditID(t *testing.T) {
	originalExec := postgresutil.Exec
	defer func() {
		postgresutil.Exec = originalExec
	}()

	var capturedSQL string
	var capturedArgs []interface{}
	postgresutil.Exec = func(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
		capturedSQL = sql
		capturedArgs = append([]interface{}{}, arguments...)
		return pgconn.CommandTag{}, nil
	}

	err := insertDocumentChangeAudit(context.Background(), documentChangeAudit{
		DocumentID:        "doc-1",
		Bucket:            "documents",
		ObjectKey:         "notes/doc-1.txt",
		Action:            "edit",
		ProcessingVersion: 2,
		Metadata: map[string]interface{}{
			"editedBy": "test",
		},
	})
	if err != nil {
		t.Fatalf("expected audit insert to succeed: %v", err)
	}

	if !strings.Contains(capturedSQL, "audit_id") {
		t.Fatalf("expected audit insert SQL to include audit_id column, got %q", capturedSQL)
	}
	if len(capturedArgs) != 13 {
		t.Fatalf("expected 13 audit insert arguments, got %d (%#v)", len(capturedArgs), capturedArgs)
	}

	auditID, ok := capturedArgs[0].(string)
	if !ok || strings.TrimSpace(auditID) == "" {
		t.Fatalf("expected first audit insert argument to be a non-empty string, got %#v", capturedArgs[0])
	}
	if _, err := uuid.Parse(auditID); err != nil {
		t.Fatalf("expected first audit insert argument to be a UUID, got %q: %v", auditID, err)
	}

	if capturedArgs[1] != "doc-1" {
		t.Fatalf("expected document ID as second audit insert argument, got %#v", capturedArgs[1])
	}
	if capturedArgs[5] != "system" {
		t.Fatalf("expected default actor email to be system, got %#v", capturedArgs[5])
	}
}
