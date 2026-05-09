package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"pkg/documentevents"
	"pkg/natsutil"
	"pkg/prometheusutil"
)

const (
	documentEventsLabel = "documentEventsHandler"
)

var documentEventKeepaliveInterval = 20 * time.Second

type documentEventHub struct {
	mu      sync.RWMutex
	nextID  uint64
	clients map[uint64]chan documentevents.LifecycleEvent
}

func newDocumentEventHub() *documentEventHub {
	return &documentEventHub{
		clients: map[uint64]chan documentevents.LifecycleEvent{},
	}
}

func (hub *documentEventHub) subscribe() (<-chan documentevents.LifecycleEvent, func()) {
	id := atomic.AddUint64(&hub.nextID, 1)
	ch := make(chan documentevents.LifecycleEvent, 16)

	hub.mu.Lock()
	hub.clients[id] = ch
	hub.mu.Unlock()

	return ch, func() {
		hub.mu.Lock()
		if existing, ok := hub.clients[id]; ok {
			delete(hub.clients, id)
			close(existing)
		}
		hub.mu.Unlock()
	}
}

func (hub *documentEventHub) publish(event documentevents.LifecycleEvent) {
	hub.mu.RLock()
	defer hub.mu.RUnlock()

	for _, ch := range hub.clients {
		select {
		case ch <- event:
		default:
		}
	}
}

var eventsHub = newDocumentEventHub()

func documentEventsHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	startTime := time.Now()
	prometheusutil.IncrementProcessed(documentEventsLabel, "call")

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
			w.WriteHeader(http.StatusInternalServerError)
		}
		if err != nil {
			slog.ErrorContext(r.Context(), err.Error())
			prometheusutil.IncrementProcessed(documentEventsLabel, "error")
		}
		prometheusutil.OpDuration(documentEventsLabel, time.Since(startTime))
	}()

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream") {
		w.WriteHeader(http.StatusNotAcceptable)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "Accept must include text/event-stream"})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "streaming is unavailable"})
		return
	}

	eventCh, unsubscribe := eventsHub.subscribe()
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ticker := time.NewTicker(documentEventKeepaliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, writeErr := fmt.Fprint(w, ": keepalive\n\n"); writeErr != nil {
				err = writeErr
				return
			}
			flusher.Flush()
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			payload, marshalErr := json.Marshal(event)
			if marshalErr != nil {
				err = marshalErr
				return
			}
			if _, writeErr := fmt.Fprintf(w, "event: document\ndata: %s\n\n", payload); writeErr != nil {
				err = writeErr
				return
			}
			flusher.Flush()
		}
	}
}

func startDocumentEvents(ctx context.Context) error {
	if !natsConfigured() {
		slog.InfoContext(ctx, "nats config not provided; document event stream will be unavailable")
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
		eventsHub.publish(event)
	})
	return err
}

var publishStoredDocumentEvent = publishMinIOStoredDocumentEvent

func publishMinIOStoredDocumentEvent(ctx context.Context, uploaded uploadedDocument) error {
	if !natsConfigured() {
		return nil
	}

	event, ok := storedDocumentLifecycleEvent(uploaded)
	if !ok {
		return nil
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return natsutil.PublishSubject(ctx, documentevents.StreamID, event.Subject, payload)
}

func storedDocumentLifecycleEvent(uploaded uploadedDocument) (documentevents.LifecycleEvent, bool) {
	bucket := documentsBucket()
	objectKey := strings.TrimSpace(uploaded.ObjectKey)
	if bucket == "" || objectKey == "" {
		return documentevents.LifecycleEvent{}, false
	}

	event := documentevents.NewLifecycleEvent(
		documentevents.SubjectMinIOStored,
		fmt.Sprintf("s3://%s/%s", bucket, objectKey),
		bucket,
		objectKey,
		uploaded.ContentType,
		0,
	)

	return event, true
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
