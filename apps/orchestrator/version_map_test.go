package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"pkg/minioutil"

	"github.com/minio/minio-go/v7"
)

func mapFixture() versionMap {
	return versionMap{SchemaVersion: 1, SourceEndpoint: "http://source.invalid:9000",
		DestinationEndpoint: "http://127.0.0.2:8333", SourceBucket: "documents", DestinationBucket: "documents",
		Entries: []mappedVersion{{Key: "sdk/history.txt", SourceVersion: "legacy-version", DestinationVersion: "new-version",
			SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("original"))), Size: 8}}}
}

func TestVersionMapBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*versionMap)
	}{
		{"wrong schema", func(m *versionMap) { m.SchemaVersion = 2 }},
		{"wrong endpoint", func(m *versionMap) { m.DestinationEndpoint = "http://wrong.invalid" }},
		{"wrong source bucket", func(m *versionMap) { m.SourceBucket = "private" }},
		{"wrong destination bucket", func(m *versionMap) { m.DestinationBucket = "private" }},
		{"missing hash", func(m *versionMap) { m.Entries[0].SHA256 = "" }},
		{"negative size", func(m *versionMap) { m.Entries[0].Size = -1 }},
		{"missing source", func(m *versionMap) { m.Entries[0].SourceVersion = "" }},
		{"null destination", func(m *versionMap) { m.Entries[0].DestinationVersion = "null" }},
		{"duplicate", func(m *versionMap) { m.Entries = append(m.Entries, m.Entries[0]) }},
		{"collision", func(m *versionMap) { m.Entries[0].DestinationVersion = m.Entries[0].SourceVersion }},
		{"duplicate alias", func(m *versionMap) { m.Entries[0].Aliases = []string{m.Entries[0].SourceVersion} }},
		{"alias destination collision", func(m *versionMap) { m.Entries[0].Aliases = []string{m.Entries[0].DestinationVersion} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := mapFixture()
			tc.change(&m)
			body, _ := json.Marshal(m)
			if _, err := parseVersionMap(body, "http://127.0.0.2:8333", "documents"); err == nil {
				t.Fatal("invalid map accepted")
			}
		})
	}
	m := mapFixture()
	m.Entries[0].Aliases = []string{"original-before-recovery"}
	m.Entries = append(m.Entries, mappedVersion{Key: "deleted", SourceVersion: "old-delete", DestinationVersion: "new-delete", DeleteMarker: true})
	body, _ := json.Marshal(m)
	mapping, err := parseVersionMap(body, "http://127.0.0.2:8333", "documents")
	if err != nil {
		t.Fatal(err)
	}
	if entry, found, err := mapping.resolve("documents", "sdk/history.txt", "original-before-recovery"); !found || err != nil || entry.DestinationVersion != "new-version" {
		t.Fatal("composed historical alias did not resolve")
	}
	if _, _, err := mapping.resolve("documents", "deleted", "old-delete"); err == nil {
		t.Fatal("delete marker accepted")
	}
	for _, scope := range [][3]string{{"other", "sdk/history.txt", "legacy-version"}, {"documents", "other", "legacy-version"}, {"documents", "sdk/history.txt", "native-version"}} {
		if _, found, err := mapping.resolve(scope[0], scope[1], scope[2]); found || err != nil {
			t.Fatal("map leaked across scope/native ID")
		}
	}
}

// Invoked by scripts/seaweedfs-smoke.py against its own disposable service only.
func TestSeaweedFSClients(t *testing.T) {
	if os.Getenv("SEAWEEDFS_CLIENT_TEST") != "1" {
		t.Skip("requires disposable SeaweedFS harness")
	}
	ctx := context.Background()
	if err := minioutil.Init(ctx, map[string]minioutil.Config{"documents": {
		Endpoint: "127.0.0.2:8333", AccessKey: "test-documents", SecretKey: "test-secret-documents",
		Region: "us-east-1", Bucket: "documents",
	}}); err != nil {
		t.Fatal(err)
	}
	put := func(key, text string) minio.UploadInfo {
		t.Helper()
		result, err := minioutil.PutObjectToBucket(ctx, "documents", key, strings.NewReader(text), int64(len(text)), minio.PutObjectOptions{
			ContentType: "text/plain", UserMetadata: map[string]string{"probe": "go"},
		})
		if err != nil || result.VersionID == "" {
			t.Fatalf("put: %v, version %q", err, result.VersionID)
		}
		return result
	}
	first := put("sdk/history.txt", "original")
	put("sdk/history.txt", "current")
	m := mapFixture()
	m.Entries[0].DestinationVersion = first.VersionID
	body, _ := json.Marshal(m)
	mapVersion := put(".migration/version-map.json", string(body))
	defer func() { historicalVersions = nil }()
	if err := loadVersionMap(ctx, "http://127.0.0.2:8333", "documents", ".migration/version-map.json", mapVersion.VersionID); err != nil {
		t.Fatal(err)
	}
	got, err := readDocumentObjectVersion(ctx, "documents", "sdk/history.txt", "legacy-version")
	if err != nil || string(got) != "original" {
		t.Fatalf("historical mapped read: %q %v", got, err)
	}
	got, err = readDocumentObjectVersion(ctx, "documents", "sdk/history.txt", first.VersionID)
	if err != nil || string(got) != "original" {
		t.Fatalf("native read: %q %v", got, err)
	}
	entry := historicalVersions.index[[2]string{"sdk/history.txt", "legacy-version"}]
	entry.SHA256 = strings.Repeat("0", 64)
	historicalVersions.index[[2]string{"sdk/history.txt", "legacy-version"}] = entry
	if _, err := readDocumentObjectVersion(ctx, "documents", "sdk/history.txt", "legacy-version"); err == nil {
		t.Fatal("corrupt mapping accepted")
	}
	stat, err := minioutil.StatObjectInBucket(ctx, "documents", "sdk/history.txt", minio.StatObjectOptions{VersionID: first.VersionID})
	if err != nil || stat.Size != 8 || stat.ContentType != "text/plain" || stat.UserMetadata["Probe"] != "go" {
		t.Fatalf("stat: %+v %v", stat, err)
	}
	opts := minio.GetObjectOptions{}
	if err := opts.SetRange(0, 2); err != nil {
		t.Fatal(err)
	}
	object, err := minioutil.GetObjectFromBucket(ctx, "documents", "sdk/history.txt", opts)
	if err != nil {
		t.Fatal(err)
	}
	rangeBody, err := io.ReadAll(object)
	object.Close()
	if err != nil || string(rangeBody) != "cur" {
		t.Fatalf("range: %q %v", rangeBody, err)
	}
	put("sdk/move.txt", "moved")
	if _, err := minioutil.MoveObjectInBucket(ctx, "documents", "sdk/move.txt", "sdk/moved.txt"); err != nil {
		t.Fatal(err)
	}
	versions, err := minioutil.ListObjectInfoInBucket(ctx, "documents", minio.ListObjectsOptions{Recursive: true, WithVersions: true, Prefix: "sdk/"}, 0)
	if err != nil || len(versions) < 5 {
		t.Fatalf("list versions: %d %v", len(versions), err)
	}
	if _, err := minioutil.StatObjectInBucket(ctx, "cnpg-backups", "probe", minio.StatObjectOptions{}); err == nil {
		t.Fatal("cross-bucket access allowed")
	}
	if err := minioutil.DeleteObjectFromBucket(ctx, "documents", "sdk/history.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := minioutil.StatObjectInBucket(ctx, "documents", "sdk/history.txt", minio.StatObjectOptions{}); err == nil {
		t.Fatal("latest survived delete marker")
	}
	got, err = readDocumentObjectVersion(ctx, "documents", "sdk/history.txt", first.VersionID)
	if err != nil || string(got) != "original" {
		t.Fatalf("history after deletion: %q %v", got, err)
	}
	put("sdk/node.txt", "Go → Node: café 🌍")
}
