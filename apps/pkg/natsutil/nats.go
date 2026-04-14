package natsutil

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

var (
	config NATSConfig
	conn   *nats.Conn
	js     nats.JetStreamContext
	rwMux  sync.RWMutex
)

const defaultFlushTimeout = 5 * time.Second

type NATSConfig struct {
	Servers []string
	Streams map[string]Stream
}

type RetentionPolicy string

const (
	RetentionWorkQueue RetentionPolicy = "workqueue"
	RetentionLimits    RetentionPolicy = "limits"
	RetentionInterest  RetentionPolicy = "interest"
)

type Stream struct {
	Name      string
	Subject   string
	Replicas  int
	Retention RetentionPolicy
}

type Message struct {
	Subject string
	Data    []byte
}

func Start(ctx context.Context, c NATSConfig) error {
	config = c
	config.Servers = cleanServers(config.Servers)
	if len(config.Servers) == 0 {
		return fmt.Errorf("nats servers not configured")
	}

	nc, err := nats.Connect(strings.Join(config.Servers, ","))
	if err != nil {
		return fmt.Errorf("nats.Connect: %w", err)
	}

	flushCtx := ctx
	cancel := func() {}
	if _, ok := ctx.Deadline(); !ok {
		flushCtx, cancel = context.WithTimeout(ctx, defaultFlushTimeout)
	}
	defer cancel()

	if err := nc.FlushWithContext(flushCtx); err != nil {
		return fmt.Errorf("flushing nats connection: %w", err)
	}

	streamContext, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("creating jetstream context: %w", err)
	}

	rwMux.Lock()
	conn = nc
	js = streamContext
	rwMux.Unlock()

	go func() {
		<-ctx.Done()
		nc.Close()
	}()

	slog.Info("initializing nats jetstream", "servers", strings.Join(config.Servers, ","))

	for streamID, streamConfig := range config.Streams {
		if err := ensureStream(streamID, streamConfig); err != nil {
			return err
		}
	}

	return nil
}

func Publish(ctx context.Context, streamID string, value []byte) error {
	streamConfig, err := getStreamConfig(streamID)
	if err != nil {
		return err
	}

	return PublishSubject(ctx, streamID, streamConfig.Subject, value)
}

func PublishSubject(ctx context.Context, streamID string, subject string, value []byte) error {
	if _, err := getStreamConfig(streamID); err != nil {
		return err
	}

	streamContext, err := getJetStreamContext()
	if err != nil {
		return err
	}

	_, err = streamContext.PublishMsg(&nats.Msg{
		Subject: subject,
		Data:    value,
	})
	if err != nil {
		return fmt.Errorf("publishing to subject %s: %w", subject, err)
	}

	return nil
}

func Subscribe(streamID string, handler func(Message)) (func() error, error) {
	streamConfig, err := getStreamConfig(streamID)
	if err != nil {
		return nil, err
	}

	nc, err := getConn()
	if err != nil {
		return nil, err
	}

	subscription, err := nc.Subscribe(streamConfig.Subject, func(msg *nats.Msg) {
		handler(Message{
			Subject: msg.Subject,
			Data:    append([]byte(nil), msg.Data...),
		})
	})
	if err != nil {
		return nil, fmt.Errorf("subscribing to subject %s: %w", streamConfig.Subject, err)
	}

	return subscription.Unsubscribe, nil
}

func getStreamConfig(streamID string) (Stream, error) {
	streamConfig, ok := config.Streams[streamID]
	if !ok {
		return Stream{}, fmt.Errorf("stream %s not configured", streamID)
	}
	if streamConfig.Name == "" {
		return Stream{}, fmt.Errorf("stream %s missing name", streamID)
	}
	if streamConfig.Subject == "" {
		return Stream{}, fmt.Errorf("stream %s missing subject", streamID)
	}
	return streamConfig, nil
}

func getJetStreamContext() (nats.JetStreamContext, error) {
	rwMux.RLock()
	defer rwMux.RUnlock()

	if js == nil {
		return nil, fmt.Errorf("jetstream context not initialized")
	}

	return js, nil
}

func getConn() (*nats.Conn, error) {
	rwMux.RLock()
	defer rwMux.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("nats connection not initialized")
	}

	return conn, nil
}

func ensureStream(streamID string, streamConfig Stream) error {
	streamContext, err := getJetStreamContext()
	if err != nil {
		return err
	}

	desired := &nats.StreamConfig{
		Name:      streamConfig.Name,
		Subjects:  []string{streamConfig.Subject},
		Retention: toRetentionPolicy(streamConfig.Retention),
		Storage:   nats.FileStorage,
		Replicas:  defaultInt(streamConfig.Replicas, 1),
	}

	_, err = streamContext.AddStream(desired)
	if err == nil {
		slog.Info("created jetstream stream", "stream", streamConfig.Name, "subject", streamConfig.Subject)
		return nil
	}
	if !strings.Contains(err.Error(), "stream name already in use") {
		return fmt.Errorf("creating jetstream stream %s: %w", streamConfig.Name, err)
	}

	if _, err := streamContext.UpdateStream(desired); err != nil {
		return fmt.Errorf("updating jetstream stream %s: %w", streamConfig.Name, err)
	}

	slog.Info("updated jetstream stream", "stream", streamConfig.Name, "subject", streamConfig.Subject)
	return nil
}

func defaultInt(value int, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func toRetentionPolicy(policy RetentionPolicy) nats.RetentionPolicy {
	switch policy {
	case RetentionLimits:
		return nats.LimitsPolicy
	case RetentionInterest:
		return nats.InterestPolicy
	default:
		return nats.WorkQueuePolicy
	}
}

func cleanServers(servers []string) []string {
	cleaned := make([]string, 0, len(servers))
	for _, server := range servers {
		server = strings.TrimSpace(server)
		if server == "" {
			continue
		}
		cleaned = append(cleaned, server)
	}
	return cleaned
}
