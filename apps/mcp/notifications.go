package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"pkg/documentevents"
	"pkg/natsutil"
)

var (
	mcpKeepaliveInterval time.Duration = 20 * time.Second
	mcpEventIDCounter    uint64
)

type sessionStreamOptions struct {
	Endpoint        string
	UseMessageEvent bool
}

func startDocumentNotifications(ctx context.Context) error {
	if !natsConfigured() {
		slog.InfoContext(ctx, "nats config not provided; MCP document notifications will be unavailable")
		return nil
	}

	if err := natsutil.Start(ctx, natsutil.NATSConfig{
		Servers: strings.Split(strings.TrimSpace(os.Getenv("NATS_URLS")), ","),
		Streams: map[string]natsutil.Stream{
			documentevents.StreamID: {
				Name:      documentEventsStreamName(),
				Subject:   documentEventsSubject(),
				Replicas:  3,
				Retention: natsutil.RetentionLimits,
			},
		},
	}); err != nil {
		return err
	}

	_, err := natsutil.Subscribe(documentevents.StreamID, func(message natsutil.Message) {
		var event documentevents.LifecycleEvent
		if decodeErr := json.Unmarshal(message.Data, &event); decodeErr != nil {
			slog.ErrorContext(ctx, "failed to decode document lifecycle event", "error", decodeErr)
			return
		}
		sessionRegistry.notifyResourceUpdated(event.DocumentID)
	})
	return err
}

func natsConfigured() bool {
	return strings.TrimSpace(os.Getenv("NATS_URLS")) != ""
}

func documentEventsStreamName() string {
	stream := strings.TrimSpace(os.Getenv("NATS_EVENTS_STREAM"))
	if stream == "" {
		return documentevents.DefaultStreamName
	}
	return stream
}

func documentEventsSubject() string {
	subject := strings.TrimSpace(os.Getenv("NATS_EVENTS_SUBJECT"))
	if subject == "" {
		return documentevents.DefaultStreamSubject
	}
	return subject
}

func serveSessionStream(w http.ResponseWriter, r *http.Request, sessionID string) error {
	return serveSessionStreamWithOptions(w, r, sessionID, sessionStreamOptions{})
}

func serveSessionStreamWithOptions(w http.ResponseWriter, r *http.Request, sessionID string, options sessionStreamOptions) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming is unavailable")
	}

	stream, ok := sessionRegistry.registerStream(sessionID)
	if !ok {
		return fmt.Errorf("session is not available")
	}
	defer sessionRegistry.unregisterStream(sessionID, stream)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if options.Endpoint != "" {
		if _, err := fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", options.Endpoint); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
			return err
		}
	}
	flusher.Flush()

	ticker := time.NewTicker(mcpKeepaliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return err
			}
			flusher.Flush()
		case message, ok := <-stream.messages:
			if !ok {
				return nil
			}
			eventName := ""
			if options.UseMessageEvent {
				eventName = "event: message\n"
			}
			if _, err := fmt.Fprintf(w, "id: %d\n%sdata: %s\n\n", nextMCPEventID(), eventName, message); err != nil {
				return err
			}
			flusher.Flush()
		}
	}
}

func nextMCPEventID() uint64 {
	return atomic.AddUint64(&mcpEventIDCounter, 1)
}
