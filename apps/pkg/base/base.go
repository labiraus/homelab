package base

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"runtime"

	"github.com/google/uuid"
)

type ContextKey string

const TraceID ContextKey = "X-Trace-Id"
const UserID ContextKey = "UserID"

var (
	BuildVersion = "local"
	Ready        = make(chan struct{})
	ServiceName  string
)

type customHandler struct {
	slog.Handler
}

func (h *customHandler) Handle(ctx context.Context, r slog.Record) error {
	if traceID, ok := ctx.Value(TraceID).(string); ok {
		r.AddAttrs(slog.String(string(TraceID), traceID))
	} else {
		r.AddAttrs(slog.String(string(TraceID), "no-trace"))
	}
	return h.Handler.Handle(ctx, r)
}

type wrappedHandler struct {
	slog.Handler
}

// Based on https://github.com/golang/go/blob/master/src/log/slog/example_wrap_test.go
func (h *wrappedHandler) Handle(ctx context.Context, r slog.Record) error {
	var pcs [1]uintptr
	runtime.Callers(5, pcs[:])
	pc := pcs[0]
	r.PC = pc

	if traceID, ok := ctx.Value(TraceID).(string); ok {
		r.AddAttrs(slog.String(string(TraceID), traceID))
	} else {
		r.AddAttrs(slog.String(string(TraceID), "no-trace"))
	}

	return h.Handler.Handle(ctx, r)
}

func Start(serviceName string) context.Context {
	ServiceName = serviceName
	baseHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: true})
	handler := &customHandler{Handler: baseHandler.WithGroup(serviceName)}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ctx, ctxCancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, TraceID, uuid.New().String())

	slog.InfoContext(ctx, "starting")
	go func() {
		defer ctxCancel()
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		s := <-c
		slog.InfoContext(ctx, "got signal: ["+s.String()+"] now closing")
	}()

	go func() {
		<-Ready
		slog.InfoContext(ctx, "ready")
	}()

	return ctx
}

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
