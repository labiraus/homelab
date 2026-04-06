package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"pkg/base"

	"github.com/google/uuid"
)

func Start(ctx context.Context, mux *http.ServeMux, port int, middlewares ...func(http.Handler) http.Handler) <-chan struct{} {
	mux.HandleFunc("/readiness", readinessHandler)
	mux.HandleFunc("/liveness", livelinessHandler)

	handler := http.Handler(mux)
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] == nil {
			continue
		}
		handler = middlewares[i](handler)
	}
	handler = traceIDMiddleware(handler)
	handler = contextMiddleware(ctx, handler)
	handler = securityHeadersMiddleware(handler)

	done := make(chan struct{})
	srv := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: handler,
	}

	go func() {
		defer close(done)

		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			slog.ErrorContext(ctx, "ListenAndServe: "+err.Error())
		}
	}()

	go func() {
		<-ctx.Done()
		if err := srv.Shutdown(ctx); err != nil {
			slog.ErrorContext(ctx, "Shutdown: "+err.Error())
		}
	}()
	return done
}

func traceIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceIDHeader := http.CanonicalHeaderKey(string(base.TraceID))
		traceID := r.Header[traceIDHeader]
		if len(traceID) != 0 {
			r = r.WithContext(context.WithValue(r.Context(), base.TraceID, traceID[0]))
		} else {
			r = r.WithContext(context.WithValue(r.Context(), base.TraceID, "new-trace"+uuid.NewString()))
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestIsHTTPS(r) {
			setHeaderIfAbsent(w.Header(), "Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		setHeaderIfAbsent(w.Header(), "Content-Security-Policy", "default-src 'self'; base-uri 'self'; frame-ancestors 'self'; object-src 'none'")
		setHeaderIfAbsent(w.Header(), "X-XSS-Protection", "1; mode=block")
		setHeaderIfAbsent(w.Header(), "X-Frame-Options", "SAMEORIGIN")
		setHeaderIfAbsent(w.Header(), "X-Content-Type-Options", "nosniff")

		next.ServeHTTP(w, r)
	})
}

func setHeaderIfAbsent(header http.Header, key string, value string) {
	if header.Get(key) != "" {
		return
	}

	header.Set(key, value)
}

func requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}

	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}

	for _, forwarded := range r.Header.Values("Forwarded") {
		if strings.Contains(strings.ToLower(forwarded), "proto=https") {
			return true
		}
	}

	return false
}

func contextMiddleware(ctx context.Context, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rctx, cancel := context.WithCancel(r.Context())
		context.AfterFunc(ctx, cancel)
		next.ServeHTTP(w, r.WithContext(rctx))
	})
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	<-base.Ready
	w.WriteHeader(http.StatusOK)
}

func livelinessHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
