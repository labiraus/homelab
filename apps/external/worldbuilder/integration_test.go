package worldbuilder

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type harness struct {
	t       *testing.T
	pool    *pgxpool.Pool
	handler http.Handler
}

func setup(t *testing.T) *harness {
	t.Helper()
	dsn := os.Getenv("WORLDBUILDER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("WORLDBUILDER_TEST_DATABASE_URL not set: disposable PostgreSQL integration test")
	}
	p, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return &harness{t, p, Handler(&Store{Pool: p}, func(r *http.Request) (string, bool) { a := r.Header.Get("Test-Actor"); return a, a != "" })}
}
func (h *harness) request(actor, method, path string, body any) (int, []byte) {
	h.t.Helper()
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, "/api/worlds"+path, bytes.NewReader(raw))
	r.Header.Set("Test-Actor", actor)
	w := httptest.NewRecorder()
	h.handler.ServeHTTP(w, r)
	return w.Code, w.Body.Bytes()
}
func (h *harness) call(actor, method, path string, body any, want int) []byte {
	h.t.Helper()
	code, raw := h.request(actor, method, path, body)
	if code != want {
		h.t.Fatalf("%s %s as %s: want %d got %d: %s", method, path, actor, want, code, raw)
	}
	return raw
}
func (h *harness) world() string {
	h.t.Helper()
	id := uuid.NewString()
	h.call("owner@example.test", "POST", "", map[string]any{"id": id, "name": "Test world"}, 200)
	for actor, role := range map[string]string{"editor@example.test": "editor", "reviewer@example.test": "reviewer", "reader@example.test": "reader"} {
		h.call("owner@example.test", "POST", "/"+id+"/members", map[string]string{"principal": actor, "role": role}, 200)
	}
	return id
}
func fixture() []Operation {
	records := []Record{
		{ID: "person", Kind: "type", Label: "Person", Fields: map[string]string{"age": "number", "alive": "boolean"}, Relations: map[string]string{"friend": "person"}},
		{ID: "source", Kind: "source", Label: "Author notes"},
		{ID: "hero", Kind: "entity", Type: "person", Label: "Hero"},
		{ID: "friend", Kind: "entity", Type: "person", Label: "Friend"},
		{ID: "age", Kind: "fact", Subject: "hero", Predicate: "age", ValueType: "number", Value: float64(20), Timeline: "main", Source: "source", From: ptr(10), Until: ptr(30)},
		{ID: "edge", Kind: "relationship", Subject: "hero", Target: "friend", Predicate: "friend", Timeline: "main", Source: "source"},
	}
	ops := []Operation{}
	for _, r := range records {
		copy := r
		ops = append(ops, Operation{Action: "put", ID: r.ID, Record: &copy})
	}
	return ops
}
func ptr(n int64) *int64 { return &n }
func (h *harness) draft(world string, rev int64, ops []Operation) string {
	h.t.Helper()
	id := uuid.NewString()
	h.call("editor@example.test", "POST", "/"+world+"/changes", map[string]any{"id": id, "baseRevision": rev, "reason": "fixture", "operations": ops}, 200)
	return id
}
func (h *harness) approve(world, id string) {
	h.t.Helper()
	h.call("editor@example.test", "POST", "/"+world+"/changes/"+id+"/submit", map[string]any{}, 200)
	h.call("reviewer@example.test", "POST", "/"+world+"/changes/"+id+"/review", map[string]string{"decision": "approved", "reason": "Reviewed evidence"}, 200)
}
func (h *harness) publish(world, id string) {
	h.t.Helper()
	h.approve(world, id)
	h.call("reviewer@example.test", "POST", "/"+world+"/changes/"+id+"/publish", map[string]any{}, 200)
}
func TestPublicationIsolationHistoryAndTemporalQueries(t *testing.T) {
	h := setup(t)
	w := h.world()
	path := "/" + w
	id := h.draft(w, 0, fixture())
	change := path + "/changes/" + id
	h.call("", "GET", path, nil, 401)
	h.call("stranger@example.test", "GET", path, nil, 404)
	h.call("reader@example.test", "POST", path+"/changes", map[string]any{"id": uuid.NewString(), "baseRevision": 0, "reason": "bad", "operations": fixture()}, 403)
	h.call("reviewer@example.test", "POST", change+"/publish", map[string]any{}, 409)
	h.call("editor@example.test", "POST", change+"/submit", map[string]any{}, 200)
	h.call("editor@example.test", "POST", change+"/review", map[string]string{"decision": "approved", "reason": "self"}, 403)
	h.call("reviewer@example.test", "POST", change+"/review", map[string]string{"decision": "approved", "reason": "ok"}, 200)
	h.call("reviewer@example.test", "POST", change+"/publish", map[string]any{}, 200)
	h.call("reviewer@example.test", "POST", change+"/publish", map[string]any{}, 200)
	var count int
	err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM worldbuilder.outbox WHERE world_id=$1`, w).Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("outbox %d: %v", count, err)
	}
	raw := h.call("reader@example.test", "GET", path+"/canon?timeline=main&revision=0", nil, 200)
	if bytes.Contains(raw, []byte(`"hero"`)) {
		t.Fatal("history changed")
	}
	raw = h.call("reader@example.test", "GET", path+"/canon?timeline=main&revision=1&at=30", nil, 200)
	var result struct {
		Records   []Record
		Citations []Evidence
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	for _, r := range result.Records {
		if r.ID == "age" {
			t.Fatal("half-open temporal query leaked expired fact")
		}
	}
	h.call("reader@example.test", "GET", path+"/canon?timeline=missing", nil, 422)
	for _, c := range result.Citations {
		if c.WorldID != w || c.Revision != 1 || c.Timeline != "main" {
			t.Fatal("unscoped evidence")
		}
	}
	w2 := h.world()
	raw = h.call("reader@example.test", "GET", path+"/canon?timeline=main&revision=1&kind=type,source&limit=1&offset=1", nil, 200)
	var catalog struct {
		Records []Record
		Total   int
	}
	if err = json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Total != 2 || len(catalog.Records) != 1 || catalog.Records[0].Kind != "source" {
		t.Fatalf("bad filtered catalog: %s", raw)
	}
	h.call("reader@example.test", "GET", path+"/canon?timeline=main&kind=private", nil, 422)
	cross := Record{ID: "foreign", Kind: "relationship", Subject: "hero", Target: "friend", Predicate: "friend", Source: "source", Timeline: "main"}
	h.call("editor@example.test", "POST", "/"+w2+"/changes", map[string]any{"id": uuid.NewString(), "baseRevision": 0, "reason": "cross-world", "operations": []Operation{{Action: "put", ID: cross.ID, Record: &cross}}}, 422)
	if _, err = h.pool.Exec(context.Background(), `UPDATE worldbuilder.revision SET reason='rewrite' WHERE world_id=$1`, w); err == nil {
		t.Fatal("history UPDATE was allowed")
	}
}
func TestConcurrentPublicationAndRevokedApproval(t *testing.T) {
	h := setup(t)
	w := h.world()
	a := h.draft(w, 0, fixture())
	b := h.draft(w, 0, fixture())
	h.approve(w, a)
	h.approve(w, b)
	results := make(chan int, 2)
	var wg sync.WaitGroup
	for _, id := range []string{a, b} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			code, _ := h.request("reviewer@example.test", "POST", "/"+w+"/changes/"+id+"/publish", map[string]any{})
			results <- code
		}(id)
	}
	wg.Wait()
	close(results)
	got := map[int]int{}
	for c := range results {
		got[c]++
	}
	if got[200] != 1 || got[409] != 1 {
		t.Fatalf("concurrent publication: %v", got)
	}
	c := h.draft(w, 1, []Operation{{Action: "delete", ID: "edge"}})
	h.approve(w, c)
	h.call("owner@example.test", "POST", "/"+w+"/members", map[string]string{"principal": "reviewer@example.test", "role": "reader"}, 200)
	h.call("owner@example.test", "POST", "/"+w+"/changes/"+c+"/publish", map[string]any{}, 409)
}
func TestAtomicFailureAndCompensation(t *testing.T) {
	h := setup(t)
	w := h.world()
	c := h.draft(w, 0, fixture())
	h.approve(w, c)
	// A real DB failure after projection/history writes must roll back the entire transaction.
	ctx := context.Background()
	_, err := h.pool.Exec(ctx, `CREATE OR REPLACE FUNCTION worldbuilder.test_outbox_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.world_id::text = TG_ARGV[0] THEN RAISE EXCEPTION 'injected outbox failure'; END IF; RETURN NEW; END $$;`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.pool.Exec(ctx, `CREATE TRIGGER test_outbox_fail BEFORE INSERT ON worldbuilder.outbox FOR EACH ROW EXECUTE FUNCTION worldbuilder.test_outbox_fail('`+w+`')`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		h.pool.Exec(ctx, `DROP TRIGGER IF EXISTS test_outbox_fail ON worldbuilder.outbox; DROP FUNCTION IF EXISTS worldbuilder.test_outbox_fail()`)
	})
	h.call("reviewer@example.test", "POST", "/"+w+"/changes/"+c+"/publish", map[string]any{}, 500)
	var rev int64
	var count int
	if err = h.pool.QueryRow(ctx, `SELECT revision FROM worldbuilder.world WHERE id=$1`, w).Scan(&rev); err != nil || rev != 0 {
		t.Fatalf("partial revision %d %v", rev, err)
	}
	if err = h.pool.QueryRow(ctx, `SELECT count(*) FROM worldbuilder.record WHERE world_id=$1`, w).Scan(&count); err != nil || count != 2 {
		t.Fatalf("partial projection %d %v", count, err)
	}
	h.pool.Exec(ctx, `DROP TRIGGER test_outbox_fail ON worldbuilder.outbox`)
	h.call("reviewer@example.test", "POST", "/"+w+"/changes/"+c+"/publish", map[string]any{}, 200)
	inverse := []Operation{}
	for _, op := range fixture() {
		inverse = append(inverse, Operation{Action: "delete", ID: op.ID})
	}
	rollback := h.draft(w, 1, inverse)
	h.publish(w, rollback)
	raw := h.call("reader@example.test", "GET", "/"+w+"/canon?timeline=main&revision=1", nil, 200)
	if !bytes.Contains(raw, []byte(`"hero"`)) {
		t.Fatal("compensation destroyed history")
	}
}
func TestStoryPersistenceConcurrencyAndOutcome(t *testing.T) {
	h := setup(t)
	w := h.world()
	c := h.draft(w, 0, fixture())
	h.publish(w, c)
	id := uuid.NewString()
	path := "/" + w + "/sessions/" + id
	h.call("editor@example.test", "POST", "/"+w+"/sessions", map[string]any{"id": id, "title": "Adventure", "pinnedRevision": 1, "timeline": "main"}, 200)
	r := Record{ID: "age", Kind: "fact", Subject: "hero", Predicate: "age", ValueType: "number", Value: float64(21), Timeline: "main", Source: "source", From: ptr(10), Until: ptr(30)}
	turn := Turn{RequestID: uuid.NewString(), Narration: "A year passes", Operations: []Operation{{Action: "put", ID: "age", Record: &r}}}
	h.call("editor@example.test", "POST", path+"/turns", turn, 200)
	h.call("editor@example.test", "POST", path+"/turns", turn, 200)
	h2 := setup(t)
	raw := h2.call("editor@example.test", "GET", path, nil, 200)
	var s Session
	if err := json.Unmarshal(raw, &s); err != nil || s.Version != 1 || s.State["age"].Value != float64(21) {
		t.Fatalf("persistent story: %s %v", raw, err)
	}
	h.call("reviewer@example.test", "GET", path, nil, 404)
	canon := h.call("editor@example.test", "GET", "/"+w+"/canon?timeline=main&revision=1&q=age", nil, 200)
	if bytes.Contains(canon, []byte(`"value":21`)) {
		t.Fatal("story mutated canon")
	}
	turn.RequestID = uuid.NewString()
	h.call("editor@example.test", "POST", path+"/turns", turn, 409)
	turn.ExpectedVersion = 1
	results := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			copy := turn
			copy.RequestID = uuid.NewString()
			code, _ := h.request("editor@example.test", "POST", path+"/turns", copy)
			results <- code
		}()
	}
	wg.Wait()
	close(results)
	got := map[int]int{}
	for code := range results {
		got[code]++
	}
	if got[200] != 1 || got[409] != 1 {
		t.Fatalf("turn race: %v", got)
	}
	outcome := Outcome{ID: uuid.NewString(), ExpectedVersion: 2, BaseRevision: 1, Reason: "Adopt story outcome"}
	h.call("editor@example.test", "POST", path+"/outcomes", outcome, 200)
	h.call("editor@example.test", "POST", path+"/outcomes", outcome, 200)
	h.publish(w, outcome.ID)
	outcome.ID = uuid.NewString()
	h.call("editor@example.test", "POST", path+"/outcomes", outcome, 409)
}
func TestValidationAndRequestBoundaries(t *testing.T) {
	before := Snapshot{"main": {ID: "main", Kind: "timeline", Label: "Main"}}
	s, err := Apply(before, fixture())
	if err != nil {
		t.Fatal(err)
	}
	duplicate := s["age"]
	duplicate.ID = "age2"
	if _, err = Apply(s, []Operation{{Action: "put", ID: "age2", Record: &duplicate}}); err == nil {
		t.Fatal("overlapping fact accepted")
	}
	duplicate.From = ptr(30)
	duplicate.Until = nil
	if _, err = Apply(s, []Operation{{Action: "put", ID: "age2", Record: &duplicate}}); err != nil {
		t.Fatal(err)
	}
	duplicate.Value = "not a number"
	if _, err = Apply(s, []Operation{{Action: "put", ID: "age2", Record: &duplicate}}); err == nil {
		t.Fatal("untyped fact accepted")
	}
	if _, err = Apply(s, []Operation{{Action: "delete", ID: "hero"}}); err == nil {
		t.Fatal("dangling relationships accepted")
	}
	h := setup(t)
	w := h.world()
	h.call("owner@example.test", "POST", "/"+w+"/changes", map[string]any{"id": uuid.NewString(), "publish": true}, 422)
	r := httptest.NewRequest("POST", "/api/worlds", bytes.NewBufferString(`{}`))
	r.Header.Set("Test-Actor", "owner@example.test")
	r.Header.Set("Origin", "https://attacker.test")
	resp := httptest.NewRecorder()
	h.handler.ServeHTTP(resp, r)
	if resp.Code != 403 {
		t.Fatal("cross-site write accepted")
	}
}

func TestLifeBoundsAndScopedCitations(t *testing.T) {
	s, err := Apply(Snapshot{"main": {ID: "main", Kind: "timeline", Label: "Main"}}, fixture())
	if err != nil {
		t.Fatal(err)
	}
	person := s["person"]
	person.Fields["death"] = "number"
	s["person"] = person
	s["death"] = Record{ID: "death", Kind: "fact", Subject: "hero", Predicate: "death", ValueType: "number", Value: float64(50), Source: "source", Timeline: "main"}
	s["life"] = Record{ID: "life", Kind: "rule", Label: "Life bounds", Attributes: map[string]string{"check": "life_bounds", "birth": "birth", "death": "death"}}
	event := Record{ID: "late", Kind: "event", Label: "Hero acts", Subject: "hero", Source: "source", Timeline: "main", From: ptr(51)}
	if _, err = Apply(s, []Operation{{Action: "put", ID: "late", Record: &event}}); err == nil {
		t.Fatal("dead participant event accepted")
	}
	event.From = ptr(49)
	if _, err = Apply(s, []Operation{{Action: "put", ID: "late", Record: &event}}); err != nil {
		t.Fatal(err)
	}
}

func TestDedicatedRuntimeRoleCanPublish(t *testing.T) {
	admin := setup(t)
	config, err := pgxpool.ParseConfig(os.Getenv("WORLDBUILDER_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE worldbuilder_external")
		return err
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	h := &harness{t, admin.pool, Handler(&Store{Pool: pool}, func(r *http.Request) (string, bool) { a := r.Header.Get("Test-Actor"); return a, a != "" })}
	w := h.world()
	c := h.draft(w, 0, fixture())
	h.publish(w, c)
	var name string
	var elevated bool
	var version int
	if err = pool.QueryRow(context.Background(), `SELECT current_user,(SELECT rolsuper OR rolcreaterole FROM pg_roles WHERE rolname=current_user),max(version) FROM worldbuilder.schema_version`).Scan(&name, &elevated, &version); err != nil || name != "worldbuilder_external" || elevated || version != 6 {
		t.Fatalf("runtime identity check: %s %t %d %v", name, elevated, version, err)
	}
	if _, err = pool.Exec(context.Background(), `DELETE FROM worldbuilder.revision WHERE world_id=$1`, w); err == nil {
		t.Fatal("runtime could delete history")
	}
}

func TestGroundingAPIScopeAndProposalRevision(t *testing.T) {
	h := setup(t)
	w := h.world()
	c := h.draft(w, 0, fixture())
	h.publish(w, c)
	calls := 0
	h.handler = Handler(&Store{Pool: h.pool, Reasoner: func(ctx context.Context, r ReasonRequest) (ReasonResult, error) {
		calls++
		if r.WorldID != w || r.Revision != 1 || r.Timeline != "main" {
			t.Fatal("reasoner received wrong canon scope")
		}
		return ReasonResult{Answer: "Hero exists", Citations: []string{w + ":1:main:hero"}, Model: "fixture", Usage: map[string]any{"total_tokens": 10}}, nil
	}}, func(r *http.Request) (string, bool) { a := r.Header.Get("Test-Actor"); return a, a != "" })
	path := "/" + w + "/reason"
	body := map[string]any{"workflow": "canon_proposal", "question": "Add a fact", "timeline": "main", "revision": 0}
	h.call("editor@example.test", "POST", path, body, 409)
	if calls != 0 {
		t.Fatal("stale proposal triggered inference")
	}
	body["revision"] = 1
	h.call("stranger@example.test", "POST", path, body, 404)
	if calls != 0 {
		t.Fatal("unauthorised inference")
	}
	raw := h.call("editor@example.test", "POST", path, body, 200)
	var result ReasonResult
	if err := json.Unmarshal(raw, &result); err != nil || result.BaseRevision != 1 {
		t.Fatalf("lost proposal base revision: %s %v", raw, err)
	}
	var count int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM worldbuilder.change_set WHERE world_id=$1`, w).Scan(&count); err != nil || count != 1 {
		t.Fatalf("inference created or published a proposal: %d %v", count, err)
	}
	h.call("reader@example.test", "GET", "/"+w+"/records/hero?timeline=main&revision=1", nil, 200)
	h.call("reader@example.test", "GET", "/"+w+"/records/hero?timeline=main&revision=0", nil, 404)
}
