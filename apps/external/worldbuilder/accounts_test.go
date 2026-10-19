package worldbuilder

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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func superuserFixture(t *testing.T) (*harness, *pgxpool.Pool, string) {
	admin := setup(t)
	ctx := context.Background()
	actor := "super-" + uuid.NewString() + "@example.test"
	if _, err := admin.pool.Exec(ctx, `INSERT INTO worldbuilder.superuser(principal,granted_by) VALUES($1,'fixture operator')`, actor); err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(os.Getenv("WORLDBUILDER_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	config.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		_, e := c.Exec(ctx, "SET ROLE worldbuilder_external")
		return e
	}
	runtime, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	h := &harness{t, admin.pool, Handler(&Store{Pool: runtime}, func(r *http.Request) (string, bool) { a := r.Header.Get("Test-Actor"); return a, a != "" })}
	return h, runtime, actor
}

func TestSuperuserRegistersUsersWithoutGrantingAuthority(t *testing.T) {
	h, runtime, actor := superuserFixture(t)
	target := "new-" + uuid.NewString() + "@example.test"
	body := map[string]any{"email": "  " + strings.ToUpper(target) + "  ", "displayName": "New user"}
	h.call("", "POST", "/users", body, 401)
	h.call("ordinary@example.test", "POST", "/users", body, 403)
	h.call("ordinary@example.test", "GET", "/users", nil, 403)
	raw := h.call(actor, "POST", "/users", body, 200)
	var a Account
	if err := json.Unmarshal(raw, &a); err != nil || a.Email != target || a.Superuser {
		t.Fatalf("registration %+v %v", a, err)
	}
	h.call(actor, "POST", "/users", body, 200)
	h.call(actor, "GET", "/users?limit=1", nil, 200)
	h.call(actor, "POST", "/users", map[string]any{"email": target, "displayName": "Escalate", "superuser": true}, 422)
	h.call(actor, "POST", "/users", map[string]any{"email": "Name <x@example.test>", "displayName": "Invalid"}, 422)
	r := httptest.NewRequest("POST", "/api/worlds/users", strings.NewReader(`{"email":"cross@example.test","displayName":"Cross"}`))
	r.Header.Set("Test-Actor", actor)
	r.Header.Set("Origin", "https://untrusted.example")
	w := httptest.NewRecorder()
	h.handler.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-origin registration accepted", w.Code)
	}
	var count int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM worldbuilder.account_audit WHERE action='user.register' AND principal=$1`, target).Scan(&count); err != nil || count != 1 {
		t.Fatal("registration audit lost or duplicated", count, err)
	}
	for _, q := range []string{
		`INSERT INTO worldbuilder.superuser(principal,granted_by) VALUES('escalate@example.test','runtime')`,
		`DELETE FROM worldbuilder.superuser`,
		`UPDATE worldbuilder.superuser SET granted_by='runtime'`,
		`DELETE FROM worldbuilder.account_audit`,
		`UPDATE auth.users SET display_name='changed'`,
	} {
		if _, err := runtime.Exec(context.Background(), q); err == nil {
			t.Fatal("runtime authority bypass", q)
		}
	}
	if _, err := h.pool.Exec(context.Background(), `DELETE FROM worldbuilder.superuser WHERE principal=$1`, actor); err != nil {
		t.Fatal(err)
	}
	h.call(actor, "POST", "/users", body, 403)
}

func TestAdministratorOverrideRevocationAndPublication(t *testing.T) {
	h, _, actor := superuserFixture(t)
	w := h.world()
	p := "/" + w
	id := uuid.NewString()
	route := p + "/changes/" + id
	h.call(actor, "GET", p, nil, 200) // Global authority requires no per-world membership.
	h.call(actor, "POST", p+"/changes", Change{ID: id, BaseRevision: 0, Reason: "Witness notes", Operations: put(Record{ID: "notes", Kind: "source", Label: "Notes"})}, 200)
	h.call(actor, "POST", route+"/submit", map[string]any{}, 200)
	h.call(actor, "POST", route+"/review", reviewBody("approved", "Self"), 403)
	h.call(actor, "POST", route+"/publish", map[string]any{}, 409)
	h.call("owner@example.test", "POST", route+"/publish", map[string]string{"overrideReason": "Not an administrator"}, 403)
	h.call(actor, "POST", route+"/publish", map[string]string{"overrideReason": "Session witnessed and approved by administrator"}, 200)
	var reason string
	if err := h.pool.QueryRow(context.Background(), `SELECT detail->>'overrideReason' FROM worldbuilder.audit WHERE world_id=$1 AND action='change.publish'`, w).Scan(&reason); err != nil || reason == "" {
		t.Fatal("missing override audit", err)
	}
	if _, err := h.pool.Exec(context.Background(), `DELETE FROM worldbuilder.superuser WHERE principal=$1`, actor); err != nil {
		t.Fatal(err)
	}
	h.call(actor, "GET", p, nil, 404)
}

func TestSuperuserGrantLockSerializesRevocation(t *testing.T) {
	h, runtime, actor := superuserFixture(t)
	ctx := context.Background()
	tx, err := runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = requireSuperuser(ctx, tx, actor); err != nil {
		t.Fatal(err)
	}
	revokeCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	if _, err = h.pool.Exec(revokeCtx, `DELETE FROM worldbuilder.superuser WHERE principal=$1`, actor); err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("grant revocation did not wait for active transaction", err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = h.pool.Exec(ctx, `DELETE FROM worldbuilder.superuser WHERE principal=$1`, actor); err != nil {
		t.Fatal(err)
	}
	h.call(actor, "GET", "/users", nil, 403)
}
