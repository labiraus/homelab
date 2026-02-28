package api

import (
	"context"
	"log/slog"
	"net/http"

	"go-common/base"
)

func Init(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	srv := &http.Server{Addr: "0.0.0.0:8080"}
	http.HandleFunc("/readiness", readinessHandler)
	http.HandleFunc("/liveness", livelinessHandler)

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			panic("ListenAndServe: " + err.Error())
		}
	}()

	go func() {
		defer close(done)

		<-ctx.Done()
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("Shutdown: " + err.Error())
		}
	}()
	return done
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	<-base.Ready
	w.WriteHeader(http.StatusOK)
}

func livelinessHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
