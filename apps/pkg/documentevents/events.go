package documentevents

import "time"

const (
	StreamID             = "document-events"
	DefaultStreamName    = "document-events"
	DefaultStreamSubject = "documents.events.>"

	SubjectMinIOStored        = "documents.events.minio.stored"
	SubjectProcessorQueued    = "documents.events.processor.queued"
	SubjectProcessorStarted   = "documents.events.processor.started"
	SubjectProcessorCompleted = "documents.events.processor.completed"
	SubjectProcessorFailed    = "documents.events.processor.failed"
)

type LifecycleEvent struct {
	Subject           string `json:"subject"`
	DocumentID        string `json:"documentId"`
	Bucket            string `json:"bucket,omitempty"`
	ObjectKey         string `json:"objectKey,omitempty"`
	ContentType       string `json:"contentType,omitempty"`
	ProcessingVersion int    `json:"processingVersion,omitempty"`
	OccurredAt        string `json:"occurredAt"`
	Error             string `json:"error,omitempty"`
}

func NewLifecycleEvent(subject string, documentID string, bucket string, objectKey string, contentType string, processingVersion int) LifecycleEvent {
	return LifecycleEvent{
		Subject:           subject,
		DocumentID:        documentID,
		Bucket:            bucket,
		ObjectKey:         objectKey,
		ContentType:       contentType,
		ProcessingVersion: processingVersion,
		OccurredAt:        time.Now().UTC().Format(time.RFC3339),
	}
}
