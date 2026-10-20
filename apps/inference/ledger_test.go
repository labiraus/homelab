package main

import (
	"context"
	"errors"
	"net/url"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresAccounting(t *testing.T) {
	dsn := os.Getenv("INFERENCE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires owned disposable inference_test database")
	}
	u, e := url.Parse(dsn)
	if e != nil || u.Path != "/inference_test" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "host.docker.internal") {
		t.Fatal("refusing non-fixture database")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	u.User = url.UserPassword("inference_runtime", "offline-inference-only")
	one, e := pgxpool.New(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	defer one.Close()
	two, e := pgxpool.New(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	defer two.Close()
	ledgers := []PostgresLedger{{one}, {two}}
	m := Model{Alias: "bedrock-nova", Revision: "r1", Model: "eu.amazon.nova-2-lite-v1:0", PriceVersion: "price1", InputRate: 1000000, OutputRate: 2000000}
	var wg sync.WaitGroup
	results := make(chan error, 12)
	ids := make(chan string, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := uuid.NewString()
			err := ledgers[i%2].Reserve(ctx, id, "client", "hash", m, 60, 100, 2)
			results <- err
			if err == nil {
				ids <- id
			}
		}(i)
	}
	wg.Wait()
	close(results)
	close(ids)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, errBudget) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("atomic cap failed: %d", success)
	}
	id := <-ids
	if e = ledgers[1].Settle(ctx, id, 10, 5, 2); e != nil {
		t.Fatal(e)
	}
	if e = ledgers[0].Reserve(ctx, id, "other", "different", m, 60, 100, 2); !errors.Is(e, errDuplicate) {
		t.Fatalf("restart/other-client replay: %v", e)
	}
	if _, e = one.Exec(ctx, `UPDATE inference.attempt SET reserved_micros=0 WHERE id=$1`, id); e == nil {
		t.Fatal("runtime changed allocation")
	}
	if e = ledgers[0].Settle(ctx, id, 0, 0, 0); e == nil {
		t.Fatal("settled history mutable")
	}
	id2 := uuid.NewString()
	if e = ledgers[0].Reserve(ctx, id2, "other", "hash", m, 60, 100, 2); e != nil {
		t.Fatal(e)
	}
	totals, e := ledgers[1].Totals(ctx, 100)
	if e != nil || totals.Settled != 10 || totals.Reserved != 60 || totals.Remaining != 30 {
		t.Fatal(totals, e)
	}
	// Previous month's unknown charges stay retained, but do not consume this month's budget.
	_, e = admin.Exec(ctx, `INSERT INTO inference.attempt(id,client_id,request_hash,profile,profile_revision,model,price_version,input_rate,output_rate,reserved_micros,period,lease_until) VALUES($1,'fixture','hash','bedrock-nova','old','model','old-price',1,1,999999,date_trunc('month',now() AT TIME ZONE 'UTC')::date-1,now()-interval '1 day')`, uuid.NewString())
	if e != nil {
		t.Fatal(e)
	}
	totals, e = ledgers[0].Totals(ctx, 100)
	if e != nil || totals.Remaining != 30 {
		t.Fatal("period rollover", totals, e)
	}
	m.PriceVersion = "price2"
	m.InputRate = 2000000
	if e = ledgers[1].Reserve(ctx, uuid.NewString(), "tariff", "hash", m, 31, 100, 2); !errors.Is(e, errBudget) {
		t.Fatal("tariff change ignored held reserves", e)
	}
	newID := uuid.NewString()
	if e = ledgers[1].Reserve(ctx, newID, "tariff", "hash", m, 25, 100, 2); e != nil {
		t.Fatal(e)
	}
	var price string
	var rate int64
	if e = one.QueryRow(ctx, `SELECT price_version,input_rate FROM inference.attempt WHERE id=$1`, id2).Scan(&price, &rate); e != nil || price != "price1" || rate != 1000000 {
		t.Fatal("old tariff changed", price, rate, e)
	}
	if e = one.QueryRow(ctx, `SELECT price_version,input_rate FROM inference.attempt WHERE id=$1`, newID).Scan(&price, &rate); e != nil || price != "price2" || rate != 2000000 {
		t.Fatal("new tariff not captured", price, rate, e)
	}
	if _, e = one.Exec(ctx, `DELETE FROM inference.attempt`); e == nil {
		t.Fatal("runtime deleted accounting")
	}
	if _, e = one.Exec(ctx, `SELECT * FROM worldbuilder.world`); e == nil {
		t.Fatal("runtime accessed canon")
	}
}
