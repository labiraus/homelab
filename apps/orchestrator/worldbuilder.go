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
	"pkg/canonsearch"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

type canonEvent struct {
	EventID       string `json:"eventId"`
	WorldID       string `json:"worldId"`
	Revision      int64  `json:"revision"`
	SchemaVersion int    `json:"schemaVersion"`
}

// relayCanonOnce marks delivery only after a JetStream acknowledgement. A crash between
// acknowledgement and commit resends the same event ID; consumers must be idempotent.
func relayCanonOnce(ctx context.Context, pool *pgxpool.Pool, publish func(string, []byte) error) (bool, error) {
	return relayWorldbuilderOutboxOnce(ctx, pool, "outbox", publish)
}
func relayWorldbuilderOutboxOnce(ctx context.Context, pool *pgxpool.Pool, table string, publish func(string, []byte) error) (bool, error) {
	if table != "outbox" && table != "generation_outbox" {
		return false, fmt.Errorf("unsupported Worldbuilder outbox")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var id string
	var payload []byte
	err = tx.QueryRow(ctx, `SELECT id::text,payload FROM worldbuilder.`+table+` WHERE delivered_at IS NULL ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err = publish(id, payload); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE worldbuilder.`+table+` SET delivered_at=now() WHERE id=$1`, id); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

// One deadline-bearing context handles both shutdown and the acknowledgement bound.
// JetStream rejects combining a per-call Context with AckWait.
func publishWorldbuilderEvent(ctx context.Context, js nats.JetStreamContext, msg *nats.Msg) (*nats.PubAck, error) {
	publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return js.PublishMsg(msg, nats.Context(publishCtx))
}

func startWorldbuilder(ctx context.Context) error {
	if !strings.EqualFold(os.Getenv("WORLDBUILDER_RELAY_ENABLED"), "true") {
		if strings.EqualFold(os.Getenv("WORLDBUILDER_JOBS_ENABLED"), "true") {
			return fmt.Errorf("generation jobs require the Worldbuilder relay")
		}
		return nil
	}
	dsn := strings.TrimSpace(os.Getenv("WORLDBUILDER_RELAY_DATABASE_URL"))
	endpoint := strings.TrimRight(os.Getenv("WORLDBUILDER_OPENSEARCH_URL"), "/")
	if dsn == "" || endpoint == "" {
		return fmt.Errorf("Worldbuilder relay requires dedicated database and OpenSearch configuration")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("invalid relay database configuration")
	}
	var user string
	var elevated bool
	if err = pool.QueryRow(ctx, `SELECT current_user,(SELECT rolsuper OR rolcreaterole FROM pg_roles WHERE rolname=current_user)`).Scan(&user, &elevated); err != nil || user != "worldbuilder_relay" || elevated {
		pool.Close()
		return fmt.Errorf("nonprivileged worldbuilder_relay login required")
	}
	nc, err := nats.Connect(os.Getenv("NATS_URLS"))
	if err != nil {
		pool.Close()
		return err
	}
	js, err := nc.JetStream()
	if err != nil {
		pool.Close()
		nc.Close()
		return err
	}
	// Reuse an existing compatible stream; never overwrite live retention/settings implicitly.
	info, err := js.StreamInfo("worldbuilder-canon")
	if errors.Is(err, nats.ErrStreamNotFound) {
		_, err = js.AddStream(&nats.StreamConfig{Name: "worldbuilder-canon", Subjects: []string{"worldbuilder.canon.published"}, Storage: nats.FileStorage, Retention: nats.LimitsPolicy, Replicas: 3, MaxAge: 7 * 24 * time.Hour})
	} else if err == nil && (len(info.Config.Subjects) != 1 || info.Config.Subjects[0] != "worldbuilder.canon.published") {
		err = fmt.Errorf("incompatible worldbuilder-canon stream")
	}
	if err != nil {
		pool.Close()
		nc.Close()
		return err
	}
	index := &canonIndex{pool: pool, endpoint: endpoint, client: &http.Client{Timeout: 10 * time.Second}}
	index.vector, err = canonsearch.FromConfig(os.Getenv, endpoint)
	if err != nil {
		pool.Close()
		nc.Close()
		return err
	}
	if err = index.ensureWithRetry(ctx, 12, 5*time.Second); err != nil {
		pool.Close()
		nc.Close()
		return err
	}
	sub, err := js.PullSubscribe("worldbuilder.canon.published", "worldbuilder-index-v1", nats.BindStream("worldbuilder-canon"), nats.ManualAck(), nats.AckExplicit())
	if err != nil {
		pool.Close()
		nc.Close()
		return err
	}
	if err = startGenerationJobs(ctx, pool, js); err != nil {
		pool.Close()
		nc.Close()
		return err
	}
	go func() { <-ctx.Done(); nc.Close(); pool.Close() }()
	go func() {
		for ctx.Err() == nil {
			sent, err := relayCanonOnce(ctx, pool, func(id string, body []byte) error {
				msg := nats.NewMsg("worldbuilder.canon.published")
				msg.Data = body
				msg.Header.Set("Nats-Msg-Id", id)
				_, err := publishWorldbuilderEvent(ctx, js, msg)
				return err
			})
			if err != nil {
				slog.ErrorContext(ctx, "canon outbox delivery failed", "error", err)
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
		if strings.EqualFold(os.Getenv("WORLDBUILDER_REBUILD_ON_START"), "true") {
			if err := index.rebuild(ctx); err != nil {
				slog.ErrorContext(ctx, "canon index rebuild failed", "error", err)
			}
		}
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
			for _, msg := range messages {
				var event canonEvent
				if err = json.Unmarshal(msg.Data, &event); err == nil {
					if event.SchemaVersion != 1 || event.Revision < 1 || event.EventID == "" {
						err = fmt.Errorf("unsupported canon event")
					} else {
						err = projectWithHeartbeat(ctx, msg.InProgress, func(work context.Context) error { return index.project(work, event.WorldID, event.Revision) })
					}
				}
				if err != nil {
					slog.ErrorContext(ctx, "canon projection failed", "error", err)
					_ = msg.NakWithDelay(5 * time.Second)
				} else {
					_ = msg.AckSync()
				}
			}
		}
	}()
	return nil
}

type canonIndex struct {
	pool     *pgxpool.Pool
	endpoint string
	client   *http.Client
	vector   *canonsearch.Index
}

// Long CPU projections retain their broker lease. Losing the heartbeat cancels
// inference; the event remains retryable and external versioning fences writes.
func projectWithHeartbeat(ctx context.Context, heartbeat func(...nats.AckOpt) error, project func(context.Context) error) error {
	return projectWithLease(ctx, func(work context.Context) error { return heartbeat(nats.Context(work)) }, project, 10*time.Second)
}
func projectWithLease(ctx context.Context, heartbeat func(context.Context) error, project func(context.Context) error, interval time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- project(ctx) }()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			<-done
			return ctx.Err()
		case <-ticker.C:
			if err := heartbeat(ctx); err != nil {
				cancel()
				<-done
				return fmt.Errorf("canon projection heartbeat failed: %w", err)
			}
		}
	}
}

func (i *canonIndex) request(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, i.endpoint+path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if user := os.Getenv("OPENSEARCH_USERNAME"); user != "" {
		req.SetBasicAuth(user, os.Getenv("OPENSEARCH_PASSWORD"))
	}
	resp, err := i.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 65536))
	return resp.StatusCode, raw, err
}

// OpenSearch and its network route may become ready after this Pod. Keep startup
// unready and retry within a finite budget before starting any worker goroutines.
func (i *canonIndex) ensureWithRetry(ctx context.Context, attempts int, delay time.Duration) error {
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err = i.ensure(ctx); err == nil {
			return nil
		}
		if attempt+1 == attempts {
			break
		}
		slog.WarnContext(ctx, "canon index startup check failed", "error", err, "attempt", attempt+1)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("canon index unavailable after %d attempts: %w", attempts, err)
}

func (i *canonIndex) ensure(ctx context.Context) error {
	if i.vector != nil {
		if err := i.vector.Ensure(ctx); err != nil {
			return err
		}
	}
	status, _, err := i.request(ctx, "HEAD", "/worldbuilder-canon-v1", nil)
	if err != nil {
		return err
	}
	if status == 200 {
		return nil
	}
	if status != 404 {
		return fmt.Errorf("canon index check returned %d", status)
	}
	mapping := []byte(`{"mappings":{"dynamic":"strict","properties":{"worldId":{"type":"keyword"},"revision":{"type":"long"},"visibility":{"type":"keyword"},"records":{"type":"nested","properties":{"id":{"type":"keyword"},"kind":{"type":"keyword"},"timeline":{"type":"keyword"},"text":{"type":"text"}}}}}}`)
	status, body, err := i.request(ctx, "PUT", "/worldbuilder-canon-v1", mapping)
	if err != nil {
		return err
	}
	if status != 200 && status != 201 && !bytes.Contains(body, []byte("resource_already_exists_exception")) {
		return fmt.Errorf("canon index creation returned %d", status)
	}
	return nil
}
func projection(world string, revision int64, raw []byte) ([]byte, error) {
	// Projection text is derived evidence: retain original number tokens instead
	// of rounding signed dates through generic float64 values.
	var snapshot map[string]map[string]json.RawMessage
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, err
	}
	records := []map[string]any{}
	for id, r := range snapshot {
		content, _ := json.Marshal(r)
		records = append(records, map[string]any{"id": id, "kind": r["kind"], "timeline": r["timeline"], "text": string(content)})
	}
	return json.Marshal(map[string]any{"worldId": world, "revision": revision, "visibility": "canon", "records": records})
}
func (i *canonIndex) project(ctx context.Context, world string, revision int64) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	// Serialize background projections across rolling replicas, independently of
	// the interactive per-user allowance. The transaction releases on owner loss.
	if i.vector != nil {
		tx, err := i.pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() {
			cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			_ = tx.Rollback(cleanup)
		}()
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('worldbuilder-background-embedding',0))`); err != nil {
			return err
		}
		// Probe the owning session while inference is running. A lost database
		// connection releases its lock, so its old owner must stop sending work.
		return projectWithLease(ctx, func(work context.Context) error {
			probe, stop := context.WithTimeout(work, 2*time.Second)
			defer stop()
			_, err := tx.Exec(probe, `SELECT 1`)
			return err
		}, func(work context.Context) error { return i.projectSnapshot(work, world, revision) }, time.Second)
	}
	return i.projectSnapshot(ctx, world, revision)
}
func (i *canonIndex) projectSnapshot(ctx context.Context, world string, revision int64) error {
	var raw []byte
	if err := i.pool.QueryRow(ctx, `SELECT snapshot FROM worldbuilder.revision WHERE world_id=$1 AND revision=$2`, world, revision).Scan(&raw); err != nil {
		return err
	}
	body, err := projection(world, revision, raw)
	if err != nil {
		return err
	}
	// External versioning makes retries and out-of-order delivery safe.
	status, _, err := i.request(ctx, "PUT", "/worldbuilder-canon-v1/_doc/"+url.PathEscape(world)+"?version="+strconv.FormatInt(revision+1, 10)+"&version_type=external_gte", body)
	if err != nil {
		return err
	}
	if status != 200 && status != 201 && status != 409 {
		return fmt.Errorf("canon indexing returned %d", status)
	}
	if i.vector != nil {
		requestID := fmt.Sprintf("canon-index:%s:%d", world, revision)
		usage, err := i.vector.Publish(ctx, world, revision, raw, requestID)
		slog.InfoContext(ctx, "canon vector projection", "requestId", requestID, "model", usage.Model, "requests", usage.Requests, "inputTokens", usage.InputTokens, "latencyMs", usage.LatencyMS, "valid", err == nil)
		return err
	}
	return nil
}
func (i *canonIndex) rebuild(ctx context.Context) error {
	rows, err := i.pool.Query(ctx, `SELECT world_id::text,max(revision) FROM worldbuilder.revision GROUP BY world_id`)
	if err != nil {
		return err
	}
	type item struct {
		world    string
		revision int64
	}
	items := []item{}
	for rows.Next() {
		var x item
		if err = rows.Scan(&x.world, &x.revision); err != nil {
			rows.Close()
			return err
		}
		items = append(items, x)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, x := range items {
		if err = i.project(ctx, x.world, x.revision); err != nil {
			return err
		}
	}
	return nil
}
