package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCanonIndexStartupRecoversAfterUnavailableService(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.URL.Path != "/worldbuilder-canon-v1" {
			t.Errorf("unexpected startup request %s %s", r.Method, r.URL.Path)
		}
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()
	index := &canonIndex{endpoint: server.URL, client: server.Client()}
	if err := index.ensureWithRetry(context.Background(), 3, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatalf("expected recovery after three real HTTP checks; got %d", calls.Load())
	}
}

func TestCanonIndexStartupRefusesPersistentFailureAndCancels(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	index := &canonIndex{endpoint: server.URL, client: server.Client()}
	if err := index.ensureWithRetry(context.Background(), 2, time.Millisecond); err == nil || calls.Load() != 2 {
		t.Fatalf("persistent refusal must exhaust finite attempts: %v, %d", err, calls.Load())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := index.ensureWithRetry(ctx, 12, time.Hour); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown must interrupt startup delay: %v", err)
	}
	before := calls.Load()
	if err := index.ensureWithRetry(ctx, 12, time.Hour); !errors.Is(err, context.DeadlineExceeded) || calls.Load() != before {
		t.Fatalf("canceled startup issued another request: %v", err)
	}
}

func TestCanonProjectionRetainsScope(t *testing.T) {
	raw, err := projection("world", 3, []byte(`{"hero":{"id":"hero","kind":"entity","label":"Hero"},"fact":{"id":"fact","kind":"fact","timeline":"alternate","value":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err = json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result["worldId"] != "world" || result["revision"] != float64(3) || result["visibility"] != "canon" {
		t.Fatal(string(raw))
	}
}

func TestCanonProjectionRetainsExactDates(t *testing.T) {
	raw, err := projection("world", 3, []byte(`{"epoch":{"id":"epoch","kind":"event","timeline":"main","from":-9223372036854775808,"until":9223372036854775807}}`))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Records []struct {
			Kind     string `json:"kind"`
			Timeline string `json:"timeline"`
			Text     string `json:"text"`
		} `json:"records"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || len(result.Records) != 1 {
		t.Fatalf("invalid projection: %s %v", raw, err)
	}
	record := result.Records[0]
	if record.Kind != "event" || record.Timeline != "main" || !strings.Contains(record.Text, `"from":-9223372036854775808`) || !strings.Contains(record.Text, `"until":9223372036854775807`) {
		t.Fatalf("rounded or incorrectly scoped index content: %s", raw)
	}
}
func TestCanonOutboxRetryAndIndexRebuild(t *testing.T) {
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
	// Keep this test in its own database; pending events from external's integration fixtures are valid too.
	w := uuid.NewString()
	event := uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO worldbuilder.world(id,name,revision) VALUES($1,'relay test',1)`, w)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO worldbuilder.revision(world_id,revision,snapshot,actor,reason) VALUES($1,1,'{"hero":{"id":"hero","kind":"entity","label":"Hero"}}','test','relay')`, w)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(canonEvent{event, w, 1, 1})
	_, err = pool.Exec(ctx, `INSERT INTO worldbuilder.outbox(id,world_id,revision,payload) VALUES($1,$2,1,$3)`, event, w, payload)
	if err != nil {
		t.Fatal(err)
	}
	var failedID string
	sent, err := relayCanonOnce(ctx, pool, func(id string, _ []byte) error { failedID = id; return errors.New("broker unavailable") })
	if err == nil || sent {
		t.Fatal("failed publish marked delivered")
	}
	var delivered bool
	if err = pool.QueryRow(ctx, `SELECT delivered_at IS NOT NULL FROM worldbuilder.outbox WHERE id=$1`, failedID).Scan(&delivered); err != nil || delivered {
		t.Fatalf("delivery %v %v", delivered, err)
	}
	sent, err = relayCanonOnce(ctx, pool, func(id string, _ []byte) error {
		if id != failedID {
			t.Fatal("retry event ID changed")
		}
		return nil
	})
	if err != nil || !sent {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(wr http.ResponseWriter, r *http.Request) {
		requests++
		if !strings.HasPrefix(r.URL.Path, "/worldbuilder-canon-v1/_doc/") || r.URL.Query().Get("version_type") != "external_gte" {
			t.Error(r.URL.String())
		}
		wr.WriteHeader(201)
	}))
	defer server.Close()
	index := &canonIndex{pool, server.URL, server.Client()}
	if err = index.project(ctx, w, 1); err != nil {
		t.Fatal(err)
	}
	if err = index.project(ctx, w, 1); err != nil {
		t.Fatal(err)
	}
	if err = index.rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	if requests < 3 {
		t.Fatal("missing projection/rebuild requests")
	}
}

func TestCanonLiveTransportAndIndex(t *testing.T) {
	dsn, natsURL, searchURL := os.Getenv("WORLDBUILDER_TEST_DATABASE_URL"), os.Getenv("WORLDBUILDER_TEST_NATS_URL"), os.Getenv("WORLDBUILDER_TEST_OPENSEARCH_URL")
	if dsn == "" || natsURL == "" || searchURL == "" {
		t.Skip("disposable PostgreSQL/NATS/OpenSearch endpoints not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	stream := "test-" + uuid.NewString()
	subject := "worldbuilder.test." + uuid.NewString()
	if _, err = js.AddStream(&nats.StreamConfig{Name: stream, Subjects: []string{subject}, Storage: nats.FileStorage}); err != nil {
		t.Fatal(err)
	}
	defer js.DeleteStream(stream)
	w, eventID := uuid.NewString(), uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO worldbuilder.world(id,name,revision) VALUES($1,'transport test',2)`, w)
	if err != nil {
		t.Fatal(err)
	}
	for rev := 1; rev <= 2; rev++ {
		_, err = pool.Exec(ctx, `INSERT INTO worldbuilder.revision(world_id,revision,snapshot,actor,reason) VALUES($1,$2,$3,'test','transport')`, w, rev, fmt.Sprintf(`{"hero":{"id":"hero","kind":"entity","label":"Revision %d"}}`, rev))
		if err != nil {
			t.Fatal(err)
		}
	}
	payload, _ := json.Marshal(canonEvent{eventID, w, 2, 1})
	// JetStream acknowledges duplicate message IDs without creating duplicate events.
	msg := nats.NewMsg(subject)
	msg.Data = payload
	msg.Header.Set("Nats-Msg-Id", eventID)
	if _, err = publishWorldbuilderEvent(context.Background(), js, msg); err != nil {
		t.Fatal(err)
	}
	ack, err := publishWorldbuilderEvent(context.Background(), js, msg)
	if err != nil || !ack.Duplicate {
		t.Fatalf("duplicate acknowledgement: %v %v", ack, err)
	}
	sub, err := js.PullSubscribe(subject, "index-test", nats.BindStream(stream))
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()
	messages, err := sub.Fetch(1, nats.MaxWait(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	index := &canonIndex{pool, searchURL, &http.Client{Timeout: 10 * time.Second}}
	if err = index.ensure(ctx); err != nil {
		t.Fatal(err)
	}
	if err = index.project(ctx, w, 2); err != nil {
		t.Fatal(err)
	}
	if err = messages[0].AckSync(); err != nil {
		t.Fatal(err)
	}
	if err = index.project(ctx, w, 1); err != nil {
		t.Fatal(err)
	} // stale event must not replace revision 2
	status, body, err := index.request(ctx, "GET", "/worldbuilder-canon-v1/_doc/"+w, nil)
	if err != nil || status != 200 {
		t.Fatalf("index read: %d %v %s", status, err, body)
	}
	var doc struct {
		Source struct {
			Revision int `json:"revision"`
		} `json:"_source"`
	}
	if err = json.Unmarshal(body, &doc); err != nil || doc.Source.Revision != 2 {
		t.Fatalf("stale overwrite: %s %v", body, err)
	}
	if err = index.rebuild(ctx); err != nil {
		t.Fatal(err)
	}
}
