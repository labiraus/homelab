package mcpauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// No fixture identity or credential store is compiled into the service.
type browserStore struct {
	Store
	mu      sync.Mutex
	client  Client
	pending map[string]Authorization
}

func (s *browserStore) Client(_ context.Context, id string) (Client, error) {
	if id != s.client.ID {
		return Client{}, ErrDenied
	}
	return s.client, nil
}
func (s *browserStore) SaveAuthorization(_ context.Context, a Authorization) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[a.IDHash] = a
	return nil
}
func (s *browserStore) Consent(_ context.Context, id, csrf, email, code string, allow bool) (Authorization, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.pending[id]
	if !ok || a.CSRFHash != csrf || a.Email != email {
		return Authorization{}, ErrDenied
	}
	delete(s.pending, id)
	return a, nil
}

func TestBrowserConsent(t *testing.T) {
	if os.Getenv("MCP_OAUTH_BROWSER_TEST") != "1" {
		t.Skip("opt-in disposable Chromium consent acceptance")
	}
	callback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Referer() != "" || r.URL.Path != "/callback" || r.URL.Query().Get("state") != "browser-state" || (r.URL.Query().Get("code") == "" && r.URL.Query().Get("error") != "access_denied") {
			http.Error(w, "invalid fixture callback", 400)
			return
		}
		w.Write([]byte("Returned to fixture client"))
	}))
	defer callback.Close()
	store := &browserStore{client: Client{ID: "browser-client", Name: "Disposable browser client", Redirects: []string{callback.URL + "/callback"}}, pending: map[string]Authorization{}}
	mux := http.NewServeMux()
	server := httptest.NewTLSServer(mux)
	defer server.Close()
	s, err := New(server.URL, store, func(r *http.Request) (string, bool) { return "browser@example.test", true })
	if err != nil {
		t.Fatal(err)
	}
	s.Register(mux)
	q := url.Values{"response_type": {"code"}, "scope": {"mcp"}, "resource": {s.Resource}, "client_id": {store.client.ID}, "redirect_uri": {callback.URL + "/callback"}, "state": {"browser-state"}, "code_challenge_method": {"S256"}, "code_challenge": {pkce(strings.Repeat("a", 43))}}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "browser-consent.cjs")
	cmd.Env = append(os.Environ(), "MCP_OAUTH_BROWSER_URL="+server.URL+AuthorizePath+"?"+q.Encode(), "MCP_OAUTH_BROWSER_CALLBACK="+callback.URL+"/callback")
	output, err := cmd.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
}
