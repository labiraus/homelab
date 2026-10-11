package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

type generationStartupProbe struct {
	nats.JetStreamContext
	called bool
	err    error
}

func (p *generationStartupProbe) StreamInfo(string, ...nats.JSOpt) (*nats.StreamInfo, error) {
	p.called = true
	return nil, p.err
}

func TestGenerationStartupAcceptsInstalledSchema(t *testing.T) {
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
	t.Setenv("WORLDBUILDER_JOBS_ENABLED", "true")
	t.Setenv("WORLDBUILDER_JOB_API_URL", "http://127.0.0.1:1")
	t.Setenv("WORLDBUILDER_JOB_TOKEN", strings.Repeat("k", 32))
	// Stop at broker discovery: this exercises the real startup/schema query
	// without launching workers or touching a live endpoint.
	probe := &generationStartupProbe{err: errors.New("offline broker probe")}
	if err := startGenerationJobs(ctx, pool, probe); !errors.Is(err, probe.err) || !probe.called {
		t.Fatalf("worker cannot start against the migrated schema: %v", err)
	}
}

func TestGenerationOutboxRetainsUndeliveredOnBrokerFailure(t *testing.T) {
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
	world, job, event := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err = pool.Exec(ctx, `INSERT INTO worldbuilder.world(id,name) VALUES($1,'job relay fixture');`, world); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO worldbuilder.generation_job(world_id,id,owner,request,parts) VALUES($1,$2,'offline@test','{}',1)`, world, job); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(generationEvent{event, world, job, 1})
	if _, err = pool.Exec(ctx, `INSERT INTO worldbuilder.generation_outbox(id,world_id,job_id,payload) VALUES($1,$2,$3,$4)`, event, world, job, payload); err != nil {
		t.Fatal(err)
	}
	var failed string
	_, err = relayWorldbuilderOutboxOnce(ctx, pool, "generation_outbox", func(id string, _ []byte) error { failed = id; return errors.New("broker unavailable") })
	if err == nil {
		t.Fatal("broker error ignored")
	}
	var pending bool
	pool.QueryRow(ctx, `SELECT delivered_at IS NULL FROM worldbuilder.generation_outbox WHERE id=$1`, failed).Scan(&pending)
	if !pending {
		t.Fatal("marked delivered before broker acknowledgement")
	}
	sent, err := relayWorldbuilderOutboxOnce(ctx, pool, "generation_outbox", func(id string, raw []byte) error {
		if id != failed {
			t.Fatal("lost stable event id on retry")
		}
		var body map[string]any
		json.Unmarshal(raw, &body)
		for _, key := range []string{"question", "owner", "results"} {
			if _, ok := body[key]; ok {
				t.Fatal("private content in event")
			}
		}
		return nil
	})
	if err != nil || !sent {
		t.Fatalf("retry: %v", err)
	}
}

func TestGenerationLiveTransportCheckpointAcknowledgements(t *testing.T) {
	endpoint := os.Getenv("WORLDBUILDER_TEST_NATS_URL")
	if endpoint == "" {
		t.Skip("disposable NATS not configured")
	}
	nc, err := nats.Connect(endpoint, nats.Timeout(3*time.Second), nats.NoReconnect())
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	name := "generation-test-" + uuid.NewString()
	subject := "offline." + name
	if _, err = js.AddStream(&nats.StreamConfig{Name: name, Subjects: []string{subject}, Storage: nats.MemoryStorage, Replicas: 1}); err != nil {
		t.Fatal(err)
	}
	defer js.DeleteStream(name)
	sub, err := js.PullSubscribe(subject, "worker", nats.BindStream(name), nats.ManualAck(), nats.AckExplicit())
	if err != nil {
		t.Fatal(err)
	}
	event := generationEvent{uuid.NewString(), uuid.NewString(), uuid.NewString(), 1}
	body, _ := json.Marshal(event)
	msg := nats.NewMsg(subject)
	msg.Data = body
	msg.Header.Set("Nats-Msg-Id", event.EventID)
	if _, err = js.PublishMsg(msg); err != nil {
		t.Fatal(err)
	}
	ack, err := js.PublishMsg(msg)
	if err != nil || !ack.Duplicate {
		t.Fatal("stable event did not deduplicate")
	}
	calls := 0
	token := strings.Repeat("k", 32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/api/worldbuilder-jobs/run" || r.Header.Get("Authorization") != "Bearer "+token {
			t.Fatal("incorrect worker boundary")
		}
		var ref map[string]string
		json.NewDecoder(r.Body).Decode(&ref)
		if len(ref) != 2 || ref["worldId"] != event.WorldID || ref["jobId"] != event.JobID {
			t.Fatal("worker sent more than saved intent IDs")
		}
		json.NewEncoder(w).Encode(map[string]bool{"done": calls == 2})
	}))
	defer server.Close()
	for i := 0; i < 2; i++ {
		messages, e := sub.Fetch(1, nats.MaxWait(2*time.Second))
		if e != nil {
			t.Fatal(e)
		}
		done, e := runGenerationEvent(context.Background(), messages[0].Data, server.URL, token, server.Client())
		if e != nil || done != (i == 1) {
			t.Fatalf("checkpoint ack: %t %v", done, e)
		}
		if done {
			if e = messages[0].AckSync(); e != nil {
				t.Fatal(e)
			}
		} else {
			if e = messages[0].NakWithDelay(20 * time.Millisecond); e != nil {
				t.Fatal(e)
			}
		}
	}
	if done, err := runGenerationEvent(context.Background(), []byte(`{"schemaVersion":99}`), server.URL, token, server.Client()); !done || err == nil || calls != 2 {
		t.Fatal("poison event reached runner")
	}
}
