package worldbuilder

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"sync"
	"testing"
)

func TestInferenceReservationsCoordinateReplicasAndSurviveFailures(t *testing.T) {
	h := setup(t)
	world := h.world()
	actor := uuid.NewString() + "@example.test"
	config := h.pool.Config()
	config.MaxConns = 2
	config.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		_, err := c.Exec(ctx, `SET ROLE worldbuilder_external`)
		return err
	}
	runtime, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	budget := &InferenceBudget{Pool: runtime, DailyRequests: 3, Concurrent: 2, RequestMicros: 100, DailyMicros: 300, Currency: "TST", GatewayRoute: "offline-fixture"}
	request := ReasonRequest{WorldID: world, Timeline: "main", Workflow: "canon_query", Question: "Hero"}
	var wg sync.WaitGroup
	success := make(chan Provenance, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			replica := *budget
			p, err := replica.reserve(context.Background(), actor, "", request)
			if err == nil {
				success <- p
			}
		}()
	}
	wg.Wait()
	close(success)
	reservations := []Provenance{}
	for p := range success {
		reservations = append(reservations, p)
	}
	if len(reservations) != 2 {
		t.Fatalf("concurrency reservations %d", len(reservations))
	}
	for _, p := range reservations {
		if err := budget.finish(p, ReasonResult{}, false); err != nil {
			t.Fatal(err)
		}
	}
	// A fresh instance sees spent allowance; failures do not refund provider risk.
	restarted := *budget
	p, err := restarted.reserve(context.Background(), actor, "", request)
	if err != nil {
		t.Fatal(err)
	}
	if err = budget.finish(p, ReasonResult{Model: "fixture", Usage: map[string]any{"total_tokens": 10}}, true); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.reserve(context.Background(), actor, "", request); err == nil {
		t.Fatal("daily request/currency allowance bypassed")
	}
	var spent int
	h.pool.QueryRow(context.Background(), `SELECT sum(allocated_micros) FROM worldbuilder.inference_attempt WHERE actor=$1`, actor).Scan(&spent)
	if spent != 300 {
		t.Fatalf("lost reservation: %d", spent)
	}
	// Runtime role cannot rewrite or erase allocation evidence.
	tx, err := h.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), `SET LOCAL ROLE worldbuilder_external`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), `UPDATE worldbuilder.inference_attempt SET allocated_micros=0 WHERE actor=$1`, actor); err == nil {
		t.Fatal("runtime can rewrite allocation")
	}
}

func TestFailedInferencePersistsAllowanceOutsideCanonTransaction(t *testing.T) {
	h := setup(t)
	w := h.world()
	budget := &InferenceBudget{Pool: h.pool, DailyRequests: 10, Concurrent: 2, RequestMicros: 100, DailyMicros: 1000, Currency: "TST", GatewayRoute: "offline-fixture"}
	h.handler = Handler(&Store{Pool: h.pool, Budget: budget, Reasoner: func(_ context.Context, r ReasonRequest) (ReasonResult, error) {
		if _, err := uuid.Parse(r.RequestID); err != nil {
			t.Fatal("missing correlation ID")
		}
		return ReasonResult{}, errors.New("provider failed")
	}}, func(r *http.Request) (string, bool) { actor := r.Header.Get("Test-Actor"); return actor, actor != "" })
	h.call("reader@example.test", "POST", "/"+w+"/reason", map[string]any{"workflow": "canon_query", "question": "Where?", "timeline": "main", "revision": 0}, 500)
	var status string
	var allocated int
	err := h.pool.QueryRow(context.Background(), `SELECT status,allocated_micros FROM worldbuilder.inference_attempt WHERE world_id=$1`, w).Scan(&status, &allocated)
	if err != nil || status != "failed" || allocated != 100 {
		t.Fatalf("failure lost allowance: %s %d %v", status, allocated, err)
	}
	var audited int
	h.pool.QueryRow(context.Background(), `SELECT count(*) FROM worldbuilder.audit WHERE world_id=$1 AND action='ai.canon_query'`, w).Scan(&audited)
	if audited != 0 {
		t.Fatal("failed inference left successful canon audit")
	}
}

func TestExpiredInferenceLeaseKeepsItsAllowance(t *testing.T) {
	h := setup(t)
	world := h.world()
	actor := uuid.NewString() + "@example.test"
	budget := &InferenceBudget{Pool: h.pool, DailyRequests: 5, Concurrent: 1, RequestMicros: 100, DailyMicros: 200, Currency: "TST", GatewayRoute: "offline-fixture"}
	request := ReasonRequest{WorldID: world, Timeline: "main", Workflow: "canon_query"}
	first, err := budget.reserve(context.Background(), actor, "", request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = budget.reserve(context.Background(), actor, "", request); err == nil {
		t.Fatal("live lease bypassed")
	}
	if _, err = h.pool.Exec(context.Background(), `UPDATE worldbuilder.inference_attempt SET lease_until=now()-interval '1 second' WHERE id=$1`, first.RequestID); err != nil {
		t.Fatal(err)
	}
	second, err := budget.reserve(context.Background(), actor, "", request)
	if err != nil {
		t.Fatal(err)
	}
	if err = budget.finish(second, ReasonResult{}, false); err != nil {
		t.Fatal(err)
	}
	if _, err = budget.reserve(context.Background(), actor, "", request); err == nil {
		t.Fatal("expired lease refunded allocation")
	}
	changed := *budget
	changed.Currency = "USD"
	if _, err = changed.reserve(context.Background(), actor, "", request); err == nil {
		t.Fatal("mixed daily currencies accepted")
	}
}
