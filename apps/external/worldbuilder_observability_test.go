package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"pkg/base"
	"pkg/prometheusutil"
	"strings"
	"testing"
)

// Emit actual handler/registry output only in an explicitly invoked offline test.
// No server, database, production authentication switch or runtime flag is added.
func TestWorldbuilderObservabilityFixture(t *testing.T) {
	base.ServiceName = "external"
	mux := http.NewServeMux()
	prometheusutil.Start(mux)
	for _, label := range []string{"worldbuilder", "worldbuilder_tools", "worldbuilder_worker"} {
		for _, status := range []int{200, 400, 401, 409, 429, 500} {
			handler := observeWorldbuilder(label, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/worlds/private-world", nil))
		}
	}
	metrics := httptest.NewRecorder()
	mux.ServeHTTP(metrics, httptest.NewRequest("GET", "/metrics", nil))
	body := metrics.Body.String()
	if !strings.Contains(body, `external_ops_total{method="worldbuilder",state="call"} 6`) ||
		!strings.Contains(body, `external_processing_duration_seconds_count{method="worldbuilder"} 6`) ||
		strings.Contains(body, "private-world") {
		t.Fatal("metrics fixture does not match the real public metric contract")
	}
	if path := os.Getenv("WORLDBUILDER_METRICS_FIXTURE"); path != "" {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
