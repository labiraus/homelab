package main

import (
	"net/http"
	"pkg/prometheusutil"
	"time"
)

type worldbuilderStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *worldbuilderStatusWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *worldbuilderStatusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

// Labels are fixed at registration, never derived from private paths or identities.
func observeWorldbuilder(label string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		out := &worldbuilderStatusWriter{ResponseWriter: w}
		prometheusutil.IncrementProcessed(label, "call")
		completed := false
		defer func() {
			prometheusutil.OpDuration(label, time.Since(start))
			state := "success"
			switch {
			case !completed || out.status >= 500:
				state = "error"
			case out.status == 429:
				state = "budget"
			case out.status == 409:
				state = "conflict"
			case out.status == 401 || out.status == 403 || out.status == 404:
				state = "denied"
			case out.status >= 400:
				state = "invalid"
			}
			prometheusutil.IncrementProcessed(label, state)
		}()
		next.ServeHTTP(out, r)
		completed = true
	})
}
