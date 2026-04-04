package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"pkg/api"
	"pkg/base"
	"pkg/kafkautil"
	"pkg/prometheusutil"
)

func main() {
	ctx := base.Start("orchestrator")

	if err := startKafka(ctx); err != nil {
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

func startKafka(ctx context.Context) error {
	return kafkautil.Start(ctx, kafkautil.KafkaConfig{
		Brokers: strings.Split(strings.TrimSpace(os.Getenv("KAFKA_BROKERS")), ","),
		Topics: map[string]kafkautil.Topic{
			"documents": {
				Name:                   kafkaTopic(),
				CreateTopic:            true,
				Partitions:             1,
				ReplicationFactor:      1,
				BatchSize:              1,
				BatchTimeoutMs:         1000,
				ReadLagIntervalSeconds: -1,
			},
		},
	})
}
