package worldbuilder

import (
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
	world := h.world()
	h.publish(world, h.draft(world, 0, fixture()))
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
		json.NewEncoder(w).Encode(map[string]any{"valid": actor == "editor@example.test" || actor == "reviewer@example.test", "email": actor, "mode": "offline-test"})
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
