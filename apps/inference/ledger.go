package main

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errBudget = errors.New("monthly budget exhausted")
var errDuplicate = errors.New("request already submitted; never automatically replay")
var errCapacity = errors.New("inference capacity exhausted")

type Totals struct {
	Settled   int64 `json:"settled_micros"`
	Reserved  int64 `json:"reserved_micros"`
	Limit     int64 `json:"limit_micros"`
	Remaining int64 `json:"remaining_micros"`
}
type Ledger interface {
	Reserve(context.Context, string, string, string, Model, int64, int64, int) error
	Settle(context.Context, string, int64, int64, int64) error
	Totals(context.Context, int64) (Totals, error)
}
type PostgresLedger struct{ Pool *pgxpool.Pool }

func (l PostgresLedger) validate(ctx context.Context) error {
	var version int
	var user string
	var elevated bool
	err := l.Pool.QueryRow(ctx, `SELECT current_user,(SELECT rolsuper OR rolcreaterole OR rolbypassrls FROM pg_roles WHERE rolname=current_user),max(version) FROM inference.schema_version`).Scan(&user, &elevated, &version)
	if err != nil || version != 1 || user != "inference_runtime" || elevated {
		return errors.New("isolated inference schema/identity unavailable")
	}
	return nil
}
func (l PostgresLedger) Reserve(ctx context.Context, id, client, digest string, m Model, amount, limit int64, concurrent int) error {
	if err := l.validate(ctx); err != nil {
		return err
	}
	tx, e := l.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	// One global lock covers all clients, months, model profiles and replicas.
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(719341025)`); e != nil {
		return e
	}
	var duplicate bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT FROM inference.attempt WHERE id=$1)`, id).Scan(&duplicate); e != nil {
		return e
	}
	if duplicate {
		return errDuplicate
	}
	var spent int64
	var active int
	e = tx.QueryRow(ctx, `SELECT COALESCE(sum(COALESCE(settled_micros,reserved_micros)) FILTER(WHERE period=date_trunc('month',now() AT TIME ZONE 'UTC')::date),0),count(*) FILTER(WHERE settled_micros IS NULL AND lease_until>now()) FROM inference.attempt`).Scan(&spent, &active)
	if e != nil {
		return e
	}
	if active >= concurrent {
		return errCapacity
	}
	if amount > limit || spent > limit-amount {
		return errBudget
	}
	_, e = tx.Exec(ctx, `INSERT INTO inference.attempt(id,client_id,request_hash,profile,profile_revision,model,price_version,input_rate,output_rate,reserved_micros,lease_until) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,now()+interval '120 seconds')`, id, client, digest, m.Alias, m.Revision, m.Model, m.PriceVersion, m.InputRate, m.OutputRate, amount)
	var pe *pgconn.PgError
	if errors.As(e, &pe) && pe.Code == "23505" {
		return errDuplicate
	}
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (l PostgresLedger) Settle(ctx context.Context, id string, cost, input, output int64) error {
	// The immutable reservation must cover usage; an anomaly never refunds it.
	tag, e := l.Pool.Exec(ctx, `UPDATE inference.attempt SET settled_micros=$2,input_tokens=$3,output_tokens=$4,finished_at=now() WHERE id=$1 AND settled_micros IS NULL AND reserved_micros >= $2`, id, cost, input, output)
	if e == nil && tag.RowsAffected() != 1 {
		return errors.New("settlement refused")
	}
	return e
}
func (l PostgresLedger) Totals(ctx context.Context, limit int64) (Totals, error) {
	if err := l.validate(ctx); err != nil {
		return Totals{}, err
	}
	t := Totals{Limit: limit}
	e := l.Pool.QueryRow(ctx, `SELECT COALESCE(sum(settled_micros),0),COALESCE(sum(reserved_micros) FILTER(WHERE settled_micros IS NULL),0) FROM inference.attempt WHERE period=date_trunc('month',now() AT TIME ZONE 'UTC')::date`).Scan(&t.Settled, &t.Reserved)
	t.Remaining = limit - t.Settled - t.Reserved
	return t, e
}
func settlementContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
