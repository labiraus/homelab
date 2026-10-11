package main

import (
	"net/http"
	"net/http/httptest"
	"pkg/base"
	"pkg/prometheusutil"
	"strings"
	"testing"
)

func TestWorldbuilderMetricsPreserveStatusAndExcludePrivateLabels(t *testing.T) {
	base.ServiceName = "external_test_" + t.Name()
	mux := http.NewServeMux()
	prometheusutil.Start(mux)
	handler := observeWorldbuilder("worldbuilder", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(409)
		w.Write([]byte(`{"code":"conflict"}`))
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("POST", "/api/worlds/private-world/changes/private-change/publish", nil))
	if response.Code != 409 || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("observability changed HTTP contract")
	}
	metrics := httptest.NewRecorder()
	mux.ServeHTTP(metrics, httptest.NewRequest("GET", "/metrics", nil))
	body := metrics.Body.String()
	if !strings.Contains(body, `method="worldbuilder",state="conflict"} 1`) || !strings.Contains(body, `method="worldbuilder",state="call"} 1`) {
		t.Fatal("missing conflict/request metrics")
	}
	if strings.Contains(body, "private-world") || strings.Contains(body, "private-change") {
		t.Fatal("private identifiers leaked in metric labels")
	}
}
