package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pkg/minioutil"
)

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
