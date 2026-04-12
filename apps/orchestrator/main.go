package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"pkg/api"
	"pkg/base"
	"pkg/natsutil"
	"pkg/prometheusutil"
)

func main() {
	ctx := base.Start("orchestrator")

	if err := startNATS(ctx); err != nil {
		slog.ErrorContext(ctx, err.Error())
		return
	}

	mux := http.NewServeMux()
	prometheusutil.Start(mux)
	mux.HandleFunc("/documents", documentsHandler)

	done := api.Start(ctx, mux, 8080)

	close(base.Ready)
	<-done
}

func startNATS(ctx context.Context) error {
	return natsutil.Start(ctx, natsutil.NATSConfig{
		Servers: strings.Split(strings.TrimSpace(os.Getenv("NATS_URLS")), ","),
		Streams: map[string]natsutil.Stream{
			"documents": {
				Name:     streamName(),
				Subject:  subjectName(),
				Replicas: 3,
			},
		},
	})
}
