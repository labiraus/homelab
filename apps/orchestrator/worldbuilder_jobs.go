package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

const generationSubject = "worldbuilder.generation.requested"
const generationStream = "worldbuilder-generation"

type generationEvent struct {
	EventID       string `json:"eventId"`
	WorldID       string `json:"worldId"`
	JobID         string `json:"jobId"`
	SchemaVersion int    `json:"schemaVersion"`
}

// A true done with an error denotes a poison event, never a failed provider job.
func runGenerationEvent(ctx context.Context, body []byte, endpoint, token string, client *http.Client) (done bool, err error) {
	var event generationEvent
	if json.Unmarshal(body, &event) != nil || event.SchemaVersion != 1 {
		return true, fmt.Errorf("unsupported generation event")
	}
	for _, id := range []string{event.EventID, event.WorldID, event.JobID} {
		if _, err = uuid.Parse(id); err != nil {
			return true, fmt.Errorf("invalid generation event identity")
		}
	}
	raw, _ := json.Marshal(map[string]string{"worldId": event.WorldID, "jobId": event.JobID})
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(endpoint, "/")+"/api/worldbuilder-jobs/run", bytes.NewReader(raw))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return false, fmt.Errorf("generation runner returned status %d", response.StatusCode)
	}
	raw, err = io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil || len(raw) > 4096 {
		return false, fmt.Errorf("invalid generation acknowledgement size")
	}
	var result struct {
		Done *bool `json:"done"`
	}
	if json.Unmarshal(raw, &result) != nil || result.Done == nil {
		return false, fmt.Errorf("invalid generation acknowledgement")
	}
	return *result.Done, nil
}

func startGenerationJobs(ctx context.Context, pool *pgxpool.Pool, js nats.JetStreamContext) error {
	if !strings.EqualFold(os.Getenv("WORLDBUILDER_JOBS_ENABLED"), "true") {
		return nil
	}
	endpoint := os.Getenv("WORLDBUILDER_JOB_API_URL")
	token := os.Getenv("WORLDBUILDER_JOB_TOKEN")
	target, err := url.Parse(endpoint)
	if err != nil || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") || target.User != nil || target.RawQuery != "" || (target.Path != "" && target.Path != "/") || len(token) < 32 {
		return fmt.Errorf("generation jobs require an internal external API URL and dedicated token")
	}
	var version int
	if err = pool.QueryRow(ctx, `SELECT max(version) FROM worldbuilder.schema_version`).Scan(&version); err != nil || version != 8 {
		return fmt.Errorf("generation jobs require schema version 8")
	}
	if err = ensureGenerationStream(js); err != nil {
		return err
	}
	sub, err := subscribeGeneration(js)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 40 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	go func() {
		for ctx.Err() == nil {
			sent, err := relayWorldbuilderOutboxOnce(ctx, pool, "generation_outbox", func(id string, body []byte) error {
				msg := nats.NewMsg(generationSubject)
				msg.Data = body
				msg.Header.Set("Nats-Msg-Id", id)
				_, err := js.PublishMsg(msg, nats.Context(ctx), nats.AckWait(5*time.Second))
				return err
			})
			if err != nil {
				slog.ErrorContext(ctx, "generation outbox delivery failed", "error", err)
			}
			if err != nil || !sent {
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
				}
			}
		}
	}()
	go func() {
		for ctx.Err() == nil {
			messages, err := sub.Fetch(1, nats.MaxWait(time.Second))
			if errors.Is(err, nats.ErrTimeout) {
				continue
			}
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
				}
				continue
			}
			for _, message := range messages {
				done, err := runGenerationEvent(ctx, message.Data, endpoint, token, client)
				if err != nil {
					slog.ErrorContext(ctx, "generation dispatch failed", "error", err)
				}
				if done && err != nil {
					_ = message.Term()
				} else if done {
					_ = message.AckSync()
				} else {
					_ = message.NakWithDelay(2 * time.Second)
				}
			}
		}
	}()
	return nil
}

func ensureGenerationStream(js nats.JetStreamContext) error {
	info, err := js.StreamInfo(generationStream)
	if errors.Is(err, nats.ErrStreamNotFound) {
		_, err = js.AddStream(&nats.StreamConfig{Name: generationStream, Subjects: []string{generationSubject}, Storage: nats.FileStorage, Retention: nats.LimitsPolicy, Replicas: 3, MaxAge: 7 * 24 * time.Hour})
	} else if err == nil && (len(info.Config.Subjects) != 1 || info.Config.Subjects[0] != generationSubject ||
		info.Config.Storage != nats.FileStorage || info.Config.Replicas != 3 ||
		info.Config.Retention != nats.LimitsPolicy || info.Config.MaxAge != 7*24*time.Hour) {
		err = fmt.Errorf("incompatible generation stream")
	}
	return err
}

func subscribeGeneration(js nats.JetStreamContext) (*nats.Subscription, error) {
	return js.PullSubscribe(generationSubject, "worldbuilder-generation-v1", nats.BindStream(generationStream), nats.ManualAck(), nats.AckExplicit(), nats.AckWait(2*time.Minute), nats.MaxAckPending(10), nats.ConsumerReplicas(3))
}
