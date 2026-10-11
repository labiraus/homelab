package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"pkg/minioutil"
	"pkg/postgresutil"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
)

func TestDocumentReconciliationKeepsCompletedGeneration(t *testing.T) {
	dsn := os.Getenv("WORLDBUILDER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("disposable PostgreSQL not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	// Use the repository document DDL, excluding unrelated vector/other tables.
	schema, err := os.ReadFile("../../sql/rag/schema.pgsql")
	if err != nil {
		t.Fatal(err)
	}
	ddl := strings.Split(strings.TrimPrefix(string(schema), "CREATE EXTENSION IF NOT EXISTS vector;"), "CREATE TABLE IF NOT EXISTS rag.document_lifecycle_events")[0]
	if _, err = tx.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	if _, err = tx.Exec(ctx, `INSERT INTO rag.documents(document_id,source_uri,status,current_processing_version,desired_processing_version) VALUES($1,'s3://documents/fixture.txt','processed',9,9)`, id); err != nil {
		t.Fatal(err)
	}
	original := postgresutil.Exec
	postgresutil.Exec = tx.Exec
	t.Cleanup(func() { postgresutil.Exec = original })
	event := documentEvent{DocumentID: id, Bucket: "documents", ObjectKey: "fixture.txt", SourceURI: "s3://documents/fixture.txt", ContentType: "text/plain", VersionMarker: "null", ProcessingVersion: 1}
	if err = upsertReconciledDocument(ctx, event, "pending", true); err != nil {
		t.Fatal(err)
	}
	var current, desired int
	var status string
	read := func() {
		t.Helper()
		if err = tx.QueryRow(ctx, `SELECT current_processing_version,desired_processing_version,status FROM rag.documents WHERE document_id=$1`, id).Scan(&current, &desired, &status); err != nil {
			t.Fatal(err)
		}
	}
	read()
	if current != 9 || desired != 9 || status != "processed" {
		t.Fatalf("unchanged reconciliation regressed inventory: %d %d %s", current, desired, status)
	}
	event.ProcessingVersion = 10
	if err = upsertPendingDocument(context.WithValue(ctx, txContextKey{}, tx), event); err != nil {
		t.Fatal(err)
	}
	read()
	if current != 9 || desired != 10 || status != "pending" {
		t.Fatalf("new generation did not become eligible: %d %d %s", current, desired, status)
	}
}

func TestScanPreservesAndAdvancesProcessingGenerations(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		current, desired, asked int
		status, oldVersion      string
		oldSize                 int64
		want                    int
		queued                  bool
	}{
		{"legacy unpinned completed source", 9, 9, 0, "processed", "", 0, 10, true},
		{"changed source while work is pending", 9, 12, 0, "pending", "old", 0, 13, true},
		{"explicit newer generation", 9, 9, 15, "processed", "old", 0, 15, true},
		{"unchanged completed source", 9, 9, 0, "processed", "null", 0, 9, false},
		{"unchanged pending source", 9, 10, 0, "pending", "null", 0, 10, false},
		{"unchanged explicit rebuild", 9, 9, 11, "processed", "null", 0, 11, true},
		{"repair lowered desired generation", 9, 1, 0, "processed", "null", 0, 9, false},
		{"previously unsupported source", 1, 1, 0, "unsupported", "null", 0, 2, true},
		{"source becomes empty", 9, 9, 0, "processed", "null", 100, 10, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			originalList, originalFind, originalUpsert, originalQueue := listBucketObjectsForScan, findInventoryRecord, upsertInventoryRecord, queueDocument
			t.Cleanup(func() {
				listBucketObjectsForScan, findInventoryRecord, upsertInventoryRecord, queueDocument = originalList, originalFind, originalUpsert, originalQueue
			})
			listBucketObjectsForScan = func(context.Context, string, string, int) ([]minio.ObjectInfo, error) {
				return []minio.ObjectInfo{{Key: "source.txt", VersionID: "null", ETag: "etag"}}, nil
			}
			findInventoryRecord = func(context.Context, string, string) (documentInventoryRecord, bool, error) {
				return documentInventoryRecord{DocumentID: "retained-id", Status: tc.status, VersionMarker: tc.oldVersion,
					ETag: "etag", SizeBytes: tc.oldSize, CurrentProcessingVersion: tc.current, DesiredProcessingVersion: tc.desired}, true, nil
			}
			var event documentEvent
			queued := false
			upsertInventoryRecord = func(_ context.Context, e documentEvent, _ string, preserve bool) error {
				if !preserve {
					t.Fatal("unchanged source lost status")
				}
				event = e
				return nil
			}
			queueDocument = func(_ context.Context, e documentEvent) error { event = e; queued = true; return nil }
			result, err := scanBucket(context.Background(), scanBucketRequest{Bucket: "documents", ProcessingVersion: tc.asked})
			if err != nil || result.Failed != 0 || queued != tc.queued || event.ProcessingVersion != tc.want || event.DocumentID != "retained-id" || event.VersionMarker != "null" {
				t.Fatalf("generation reconciliation: queued=%v event=%+v result=%+v err=%v", queued, event, result, err)
			}
			if queued && event.ProcessingVersion <= tc.current {
				t.Fatal("processor would discard completed generation")
			}
		})
	}
}

func TestScanRetainsCurrentVersionIdentity(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(200)
			return
		}
		requests++
		if r.URL.Path != "/documents/" || !r.URL.Query().Has("versions") {
			t.Errorf("unexpected listing request: %s", r.URL.Path)
			http.Error(w, "version listing required", 400)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<ListVersionsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>documents</Name><IsTruncated>false</IsTruncated>
<Version><Key>a.txt</Key><VersionId>old-a</VersionId><IsLatest>false</IsLatest><Size>5</Size></Version>
<Version><Key>a.txt</Key><VersionId>null</VersionId><IsLatest>true</IsLatest><Size>7</Size></Version>
<DeleteMarker><Key>b.txt</Key><VersionId>deleted-b</VersionId><IsLatest>true</IsLatest></DeleteMarker>
<Version><Key>b.txt</Key><VersionId>old-b</VersionId><IsLatest>false</IsLatest><Size>9</Size></Version>
<Version><Key>c.txt</Key><VersionId>opaque-c</VersionId><IsLatest>true</IsLatest><Size>11</Size></Version>
</ListVersionsResult>`)
	}))
	defer server.Close()
	if err := minioutil.Init(context.Background(), map[string]minioutil.Config{"scan-fixture": {
		Endpoint: strings.TrimPrefix(server.URL, "http://"), Bucket: "documents", Region: "us-east-1",
		AccessKey: "fixture", SecretKey: "fixture-secret",
	}}); err != nil {
		t.Fatal(err)
	}
	objects, err := listMinIOObjectsForScan(context.Background(), "documents", "", 2)
	if err != nil || len(objects) != 2 {
		t.Fatalf("current objects: %d, %v", len(objects), err)
	}
	if objects[0].Key != "a.txt" || objects[0].VersionID != "null" || objects[1].Key != "c.txt" || objects[1].VersionID != "opaque-c" {
		t.Fatal("historical/delete entries displaced current version identities")
	}
	if eventFromObject("documents", objects[0], 1).VersionMarker != "null" || requests != 1 {
		t.Fatal("literal null marker lost or unexpected extra listing")
	}
}

func TestScanRefusesMissingVersionAndListingFailure(t *testing.T) {
	for _, status := range []int{200, 403} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodHead {
					w.WriteHeader(200)
					return
				}
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(status)
				if status == 403 {
					fmt.Fprint(w, `<Error><Code>AccessDenied</Code><Message>denied</Message></Error>`)
				} else {
					fmt.Fprint(w, `<ListVersionsResult><Name>documents</Name><IsTruncated>false</IsTruncated><Version><Key>a.txt</Key><IsLatest>true</IsLatest><Size>7</Size></Version></ListVersionsResult>`)
				}
			}))
			defer server.Close()
			if err := minioutil.Init(context.Background(), map[string]minioutil.Config{"scan-fixture": {
				Endpoint: strings.TrimPrefix(server.URL, "http://"), Bucket: "documents", Region: "us-east-1",
				AccessKey: "fixture", SecretKey: "fixture-secret",
			}}); err != nil {
				t.Fatal(err)
			}
			if _, err := listMinIOObjectsForScan(context.Background(), "documents", "", 0); err == nil {
				t.Fatal("unverifiable inventory accepted")
			}
		})
	}
}
