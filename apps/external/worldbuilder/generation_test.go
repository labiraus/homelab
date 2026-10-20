package worldbuilder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func generationStore(h *harness, fn Reasoner) *Store {
	store := &Store{Pool: h.pool, Reasoner: fn, JobsEnabled: true}
	h.handler = Handler(store, func(r *http.Request) (string, bool) { a := r.Header.Get("Test-Actor"); return a, a != "" })
	return store
}
func activeGenerationActor(context.Context, string) (bool, string, error) { return true, "", nil }

func TestGenerationPinsProfileAcrossOperatorChange(t *testing.T) {
	h := setup(t)
	w := h.world()
	store := generationStore(h, func(_ context.Context, r ReasonRequest) (ReasonResult, error) {
		if r.InferenceProfile == nil || r.InferenceProfile.Alias != "local-default" || r.InferenceProfile.Revision != "local-r1" {
			t.Fatal("queued profile changed", r.InferenceProfile)
		}
		return ReasonResult{Answer: "Provisional", Unknown: true, Model: "local", Inference: &ProxyInference{RequestID: r.RequestID, Profile: "local-default", ProfileRevision: "local-r1", Provider: "local", AccountingStatus: "local"}}, nil
	})
	store.Budget = &InferenceBudget{Pool: h.pool, Mode: "proxy", Profile: "local-default", ProfileRevision: "local-r1", Concurrent: 2, Currency: "USD", GatewayRoute: "inference"}
	b := GenerationRequest{ID: uuid.NewString(), Parts: 1, ReasonInput: ReasonInput{Workflow: "narrative_generate", Question: "Scene", Timeline: "main", Revision: 0}}
	h.call("reader@example.test", "POST", "/"+w+"/generations", b, 200)
	store.Budget.Profile = "bedrock-sonnet"
	store.Budget.ProfileRevision = "sonnet-r1"
	h.call("reader@example.test", "POST", "/"+w+"/generations", b, 200)
	state, e := store.RunGeneration(context.Background(), GenerationRef{WorldID: w, JobID: b.ID}, activeGenerationActor)
	if e != nil || state.Status != "succeeded" {
		t.Fatal(state, e)
	}
}

func TestGenerationCheckpointsPrivacyAndNoCanonicalMutation(t *testing.T) {
	h := setup(t)
	w := h.world()
	calls := 0
	store := generationStore(h, func(_ context.Context, r ReasonRequest) (ReasonResult, error) {
		calls++
		if r.Generation == nil || r.Generation.Part != calls || len(r.Generation.Previous) != calls-1 {
			t.Fatalf("checkpoint context: %+v", r.Generation)
		}
		return ReasonResult{Answer: "Provisional section", Unknown: true}, nil
	})
	store.Budget = &InferenceBudget{Pool: h.pool, DailyRequests: 100, Concurrent: 2, RequestMicros: 100, DailyMicros: 10000, Currency: "TST", GatewayRoute: "offline-fixture"}
	request := GenerationRequest{ID: uuid.NewString(), Parts: 3, ReasonInput: ReasonInput{Workflow: "narrative_generate", Question: "Tell a story", Timeline: "main", Revision: 0}}
	p := "/" + w + "/generations"
	h.call("reader@example.test", "POST", p, request, 200)
	h.call("reader@example.test", "POST", p, request, 200)
	changed := request
	changed.Question = "Different"
	h.call("reader@example.test", "POST", p, changed, 409)
	h.call("owner@example.test", "GET", p+"/"+request.ID, nil, 404)
	var outboxCount int
	h.pool.QueryRow(context.Background(), `SELECT count(*) FROM worldbuilder.generation_outbox WHERE world_id=$1`, w).Scan(&outboxCount)
	if outboxCount != 1 {
		t.Fatal("duplicate queue event")
	}
	ref := GenerationRef{WorldID: w, JobID: request.ID}
	for i := 1; i <= 3; i++ {
		// Fresh store instance models worker/API restart between checkpoints.
		restarted := *store
		state, err := restarted.RunGeneration(context.Background(), ref, activeGenerationActor)
		if err != nil || state.Step != i || state.Done != (i == 3) {
			t.Fatalf("checkpoint %d: %+v %v", i, state, err)
		}
	}
	if _, err := store.RunGeneration(context.Background(), ref, activeGenerationActor); err != nil || calls != 3 {
		t.Fatal("terminal delivery repeated provider work")
	}
	var job Generation
	json.Unmarshal(h.call("reader@example.test", "GET", p+"/"+request.ID, nil, 200), &job)
	if len(job.Results) != 3 || job.Status != "succeeded" {
		t.Fatalf("checkpoint lost: %+v", job)
	}
	for i, result := range job.Results {
		if result.Provenance.GenerationID != job.ID || result.Provenance.GenerationPart != i+1 {
			t.Fatal("lost per-part allocation attribution")
		}
	}
	var revision, changes int
	h.pool.QueryRow(context.Background(), `SELECT revision FROM worldbuilder.world WHERE id=$1`, w).Scan(&revision)
	h.pool.QueryRow(context.Background(), `SELECT count(*) FROM worldbuilder.change_set WHERE world_id=$1`, w).Scan(&changes)
	if revision != 0 || changes != 0 {
		t.Fatal("job mutated canon or proposed automatically")
	}
}

func TestGenerationQueueRollbackAndStaleStory(t *testing.T) {
	h := setup(t)
	w := h.world()
	calls := 0
	store := generationStore(h, func(context.Context, ReasonRequest) (ReasonResult, error) {
		calls++
		return ReasonResult{Answer: "Scene", Unknown: true}, nil
	})
	_, err := h.pool.Exec(context.Background(), `CREATE FUNCTION worldbuilder.test_generation_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected generation outbox failure'; END $$; CREATE TRIGGER test_generation_failure BEFORE INSERT ON worldbuilder.generation_outbox FOR EACH ROW EXECUTE FUNCTION worldbuilder.test_generation_failure()`)
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		h.pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS test_generation_failure ON worldbuilder.generation_outbox; DROP FUNCTION IF EXISTS worldbuilder.test_generation_failure()`)
	}
	t.Cleanup(cleanup)
	b := GenerationRequest{ID: uuid.NewString(), Parts: 1, ReasonInput: ReasonInput{Workflow: "narrative_generate", Question: "Scene", Timeline: "main"}}
	h.call("reader@example.test", "POST", "/"+w+"/generations", b, 500)
	var count int
	h.pool.QueryRow(context.Background(), `SELECT count(*) FROM worldbuilder.generation_job WHERE world_id=$1`, w).Scan(&count)
	if count != 0 {
		t.Fatal("failed dispatch insert left orphan job")
	}
	cleanup()
	id := uuid.NewString()
	h.call("reader@example.test", "POST", "/"+w+"/sessions", map[string]any{"id": id, "title": "Journey", "pinnedRevision": 0, "timeline": "main"}, 200)
	b.ReasonInput = ReasonInput{Workflow: "story_turn", Question: "Continue", Timeline: "main", SessionID: id, ExpectedVersion: ptr(0)}
	h.call("reader@example.test", "POST", "/"+w+"/generations", b, 200)
	h.call("reader@example.test", "POST", "/"+w+"/sessions/"+id+"/turns", Turn{RequestID: uuid.NewString(), Narration: "Manual next turn"}, 200)
	state, err := store.RunGeneration(context.Background(), GenerationRef{w, b.ID}, activeGenerationActor)
	if err != nil || state.Status != "failed" || calls != 0 {
		t.Fatal("stale story spent inference")
	}
}

func TestGenerationDuplicateWorkersAndExpiredClaims(t *testing.T) {
	h := setup(t)
	w := h.world()
	calls := 0
	store := generationStore(h, func(context.Context, ReasonRequest) (ReasonResult, error) {
		calls++
		return ReasonResult{Answer: "Scene", Unknown: true}, nil
	})
	b := GenerationRequest{ID: uuid.NewString(), Parts: 1, ReasonInput: ReasonInput{Workflow: "narrative_generate", Question: "Scene", Timeline: "main"}}
	h.call("reader@example.test", "POST", "/"+w+"/generations", b, 200)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.RunGeneration(context.Background(), GenerationRef{w, b.ID}, activeGenerationActor)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("duplicate work: %d", calls)
	}
	b.ID = uuid.NewString()
	h.call("reader@example.test", "POST", "/"+w+"/generations", b, 200)
	h.pool.Exec(context.Background(), `UPDATE worldbuilder.generation_job SET status='running',lease_token=$3,lease_until=now()-interval '1 second' WHERE world_id=$1 AND id=$2`, w, b.ID, uuid.NewString())
	state, err := store.RunGeneration(context.Background(), GenerationRef{w, b.ID}, activeGenerationActor)
	if err != nil || !state.Done || state.Status != "uncertain" || calls != 1 {
		t.Fatalf("expired work retried automatically: %+v %v", state, err)
	}
}

func TestGenerationCancellationRevocationAndServiceBoundary(t *testing.T) {
	h := setup(t)
	w := h.world()
	calls := 0
	store := generationStore(h, func(context.Context, ReasonRequest) (ReasonResult, error) {
		calls++
		return ReasonResult{Answer: "Scene", Unknown: true}, nil
	})
	b := GenerationRequest{ID: uuid.NewString(), Parts: 1, ReasonInput: ReasonInput{Workflow: "narrative_generate", Question: "Scene", Timeline: "main"}}
	p := "/" + w + "/generations"
	h.call("reader@example.test", "POST", p, b, 200)
	h.call("reader@example.test", "POST", p+"/"+b.ID+"/cancel", map[string]any{}, 200)
	state, err := store.RunGeneration(context.Background(), GenerationRef{w, b.ID}, activeGenerationActor)
	if err != nil || state.Status != "cancelled" || calls != 0 {
		t.Fatal("cancelled work ran")
	}
	b.ID = uuid.NewString()
	h.call("reader@example.test", "POST", p, b, 200)
	h.call("owner@example.test", "POST", "/"+w+"/administration", AdminRequest{RequestID: uuid.NewString(), ExpectedVersion: 3, Action: "remove_member", Principal: "reader@example.test", Reason: "Revoke"}, 200)
	state, err = store.RunGeneration(context.Background(), GenerationRef{w, b.ID}, activeGenerationActor)
	if err != nil || !state.Done || calls != 0 {
		t.Fatal("revoked member work ran")
	}
	handler := GenerationHandler(store, strings.Repeat("k", 32), activeGenerationActor)
	req := httptest.NewRequest("POST", GenerationRunPath, strings.NewReader(`{"worldId":"`+w+`","jobId":"`+b.ID+`"}`))
	req.Header.Set("X-Forwarded-Email", "owner@example.test")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != 401 {
		t.Fatal("raw human identity reached service runner")
	}
}

func TestGenerationRecoveryChecksLedgerAndActualCheckpoint(t *testing.T) {
	h := setup(t)
	w := h.world()
	calls := 0
	store := generationStore(h, func(context.Context, ReasonRequest) (ReasonResult, error) {
		calls++
		return ReasonResult{Answer: "Scene", Unknown: true}, nil
	})
	store.Budget = &InferenceBudget{Pool: h.pool, DailyRequests: 100, Concurrent: 2, RequestMicros: 100, DailyMicros: 10000, Currency: "TST", GatewayRoute: "offline-fixture"}
	ctx := context.Background()
	b := GenerationRequest{ID: uuid.NewString(), Parts: 2, ReasonInput: ReasonInput{Workflow: "narrative_generate", Question: "Scene", Timeline: "main"}}
	h.call("reader@example.test", "POST", "/"+w+"/generations", b, 200)
	ref := GenerationRef{w, b.ID}
	oldToken := uuid.NewString()
	if _, err := h.pool.Exec(ctx, `UPDATE worldbuilder.generation_job SET status='running',lease_token=$3,lease_until=now()-interval '1 second' WHERE world_id=$1 AND id=$2`, w, b.ID, oldToken); err != nil {
		t.Fatal(err)
	}
	state, err := store.RunGeneration(ctx, ref, activeGenerationActor)
	if err != nil || state.Done || state.Status != "queued" || calls != 0 {
		t.Fatalf("unstarted recovery: %+v %v", state, err)
	}
	state, err = store.RunGeneration(ctx, ref, activeGenerationActor)
	if err != nil || state.Step != 1 || state.Done || calls != 1 {
		t.Fatalf("checkpoint: %+v %v", state, err)
	}
	// Simulate error handling after a successful commit whose acknowledgement was lost.
	state, err = store.failGeneration(ref, oldToken, "failed", "lost_ack")
	if err != nil || state.Done || state.Status != "queued" || state.Step != 1 {
		t.Fatalf("committed work was acknowledged terminal: %+v %v", state, err)
	}
	_, err = store.Budget.reserve(ctx, "reader@example.test", "", ReasonRequest{WorldID: w, Workflow: "narrative_generate", Timeline: "main", Generation: &GenerationContext{ID: b.ID, Part: 2, Parts: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.pool.Exec(ctx, `UPDATE worldbuilder.generation_job SET status='running',lease_token=$3,lease_until=now()-interval '1 second' WHERE world_id=$1 AND id=$2`, w, b.ID, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	state, err = store.RunGeneration(ctx, ref, activeGenerationActor)
	if err != nil || !state.Done || state.Status != "uncertain" || state.Step != 1 || calls != 1 {
		t.Fatalf("possibly billed work repeated: %+v %v", state, err)
	}
}
