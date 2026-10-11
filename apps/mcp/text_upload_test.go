package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestTextUploadPreservesContentAtExternalBoundary(t *testing.T) {
	for _, text := range []string{"  indented paragraph\n\n", "\tfirst\r\nsecond\t ", "\u2003“Unicode” and a backslash \\\n\u00a0"} {
		t.Run(text, func(t *testing.T) {
			var called atomic.Bool
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called.Store(true)
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if r.Method != http.MethodPut || r.URL.Path != "/api/documents/object" || body["objectKey"] != "fixture/exact.txt" || body["text"] != text || body["contentType"] != "text/plain; charset=utf-8" {
					t.Errorf("text upload changed at upstream boundary: %#v", body)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"stored":true}`))
			}))
			defer upstream.Close()
			t.Setenv("API_BASE_URL", upstream.URL)
			request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			status, response := handleMCPRequest(context.Background(), request, jsonRPCRequest{
				JSONRPC: "2.0", ID: "1", Method: "tools/call",
				Params: mustMarshalParams(t, map[string]any{"name": "minio.documents.putTextObject", "arguments": map[string]any{
					"objectKey": "fixture/exact.txt", "body": map[string]any{"text": text},
				}}),
			})
			if status != http.StatusOK || response == nil || response.Error != nil || !called.Load() {
				t.Fatalf("upload did not reach external: status=%d response=%#v called=%v", status, response, called.Load())
			}
		})
	}
}

func TestDocumentResourcePreservesUpstreamBytes(t *testing.T) {
	for _, tc := range []struct{ contentType, body string }{
		{"text/plain", "\t  exact Unicode “text”\r\n\n"},
		{"text/plain", " \t\r\n"},
		{"text/plain", ""},
		{"application/octet-stream", "\t\x00\xff\r\n "},
	} {
		t.Run(tc.contentType+tc.body, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer upstream.Close()
			t.Setenv("API_BASE_URL", upstream.URL)
			request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			_, response := handleMCPRequest(context.Background(), request, jsonRPCRequest{
				JSONRPC: "2.0", ID: "1", Method: "resources/read",
				Params: mustMarshalParams(t, map[string]any{"uri": "homelab://mcp/minio/documents/objects/fixture/exact.txt"}),
			})
			if response == nil || response.Error != nil {
				t.Fatalf("resource read failed: %#v", response)
			}
			item := response.Result.(map[string]any)["contents"].([]map[string]any)[0]
			if tc.contentType == "text/plain" {
				if item["text"] != tc.body {
					t.Fatalf("text changed: %#v", item)
				}
			} else if item["blob"] != base64.StdEncoding.EncodeToString([]byte(tc.body)) {
				t.Fatalf("binary bytes changed: %#v", item)
			}
			_, body, rpcErr := proxyAPIRequest(context.Background(), request, http.MethodGet, "/api/documents/object", nil)
			if rpcErr != nil || body != tc.body {
				t.Fatalf("HTTP proxy changed bytes: %q, %#v", body, rpcErr)
			}
		})
	}
}
