package base

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
)

var (
	Ready       = make(chan struct{})
	ServiceName string
)

func Init(serviceName string) context.Context {
	ServiceName = serviceName
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: true})).WithGroup(serviceName))
	slog.Info("starting")
	ctx, ctxDone := context.WithCancel(context.Background())

	go func() {
		defer ctxDone()
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		s := <-c
		slog.Info("got signal: [" + s.String() + "] now closing")
	}()

	return ctx
}

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
