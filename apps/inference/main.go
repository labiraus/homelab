package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	c, e := loadConfig(os.Getenv("INFERENCE_CONFIG"))
	if e != nil {
		log.Fatal(e)
	}
	b, e := os.ReadFile(os.Getenv("INFERENCE_CLIENTS"))
	if e != nil {
		log.Fatal("client credentials unavailable")
	}
	var clients []Client
	if json.Unmarshal(b, &clients) != nil || validateClients(clients) != nil {
		log.Fatal("invalid client credentials")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	var ledger Ledger
	if dsn := os.Getenv("INFERENCE_DATABASE_URL"); dsn != "" {
		pool, e := pgxpool.New(ctx, dsn)
		if e != nil {
			log.Fatal("invalid inference database configuration")
		}
		defer pool.Close()
		ledger = PostgresLedger{pool}
	}
	if c.CloudEnabled && ledger == nil {
		log.Fatal("cloud requires durable accounting")
	}
	s := NewServer(c, clients, ledger)
	address := os.Getenv("INFERENCE_LISTEN")
	if address == "" {
		address = ":8080"
	}
	httpServer := &http.Server{Addr: address, Handler: s, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 75 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		<-ctx.Done()
		s.drained.Store(true)
		shutdown, cancel := context.WithTimeout(context.Background(), 70*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdown)
	}()
	if e = httpServer.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Fatal(e)
	}
	<-drained
}
