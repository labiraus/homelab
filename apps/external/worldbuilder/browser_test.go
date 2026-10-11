package worldbuilder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Only compiled into tests. Never register a fixture identity in a production binary.
func TestBrowserAcceptance(t *testing.T) {
	if os.Getenv("WORLDBUILDER_BROWSER_TEST") != "1" {
		t.Skip("opt-in disposable Chromium acceptance")
	}
	h := setup(t)
	if _, err := h.pool.Exec(context.Background(), `INSERT INTO worldbuilder.superuser(principal,granted_by) VALUES('browser-superuser@example.test','offline browser fixture')`); err != nil {
		t.Fatal(err)
	}
	world := h.world()
	h.call("owner@example.test", "POST", "/"+world+"/members", map[string]string{"principal": "second@example.test", "role": "reviewer"}, 200)
	ops := append(fixture(),
		Operation{Action: "put", ID: "alternate", Record: &Record{ID: "alternate", Kind: "timeline", Label: "Alternate"}},
		Operation{Action: "put", ID: "alternate-era", Record: &Record{ID: "alternate-era", Kind: "event", Label: "Alternate era", Timeline: "alternate", Source: "source", From: ptr(0)}},
		Operation{Action: "put", ID: "alternate-edge", Record: &Record{ID: "alternate-edge", Kind: "relationship", Subject: "friend", Target: "hero", Predicate: "friend", Timeline: "alternate", Source: "source"}},
	)
	h.publish(world, h.draft(world, 0, ops))
	ui, err := filepath.Abs("../../ui")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(ui, "dist/index.html")); err != nil {
		t.Fatal("build UI before browser acceptance")
	}
	mux := http.NewServeMux()
	mux.Handle("/api/worlds", h.handler)
	mux.Handle("/api/worlds/", h.handler)
	mux.HandleFunc("/api/auth/status", func(w http.ResponseWriter, r *http.Request) {
		actor := r.Header.Get("Test-Actor")
		json.NewEncoder(w).Encode(map[string]any{"valid": actor == "editor@example.test" || actor == "reviewer@example.test" || actor == "second@example.test" || actor == "browser-superuser@example.test", "email": actor, "mode": "offline-test"})
	})
	mux.HandleFunc("/api/auth/providers", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"providers": []any{}})
	})
	mux.Handle("/", http.FileServer(http.Dir(filepath.Join(ui, "dist"))))
	server := httptest.NewServer(mux)
	defer server.Close()
	cmd := exec.Command("npm", "run", "test:browser")
	cmd.Dir = ui
	cmd.Env = append(os.Environ(), "WORLDBUILDER_BROWSER_URL="+server.URL, "WORLDBUILDER_BROWSER_WORLD="+world)
	output, err := cmd.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
}
