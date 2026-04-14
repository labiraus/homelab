import type { NatsConnection } from "nats";

import type { DocumentEvent, DocumentLifecycleEvent } from "./types.js";

export const DOCUMENT_EVENTS_STREAM_NAME = "document-events";
export const DOCUMENT_EVENTS_STREAM_SUBJECT = "documents.events.>";
export const DOCUMENT_EVENT_SUBJECTS = {
	minioStored: "documents.events.minio.stored",
	processorQueued: "documents.events.processor.queued",
	processorStarted: "documents.events.processor.started",
	processorCompleted: "documents.events.processor.completed",
	processorFailed: "documents.events.processor.failed",
} as const;

export function buildLifecycleEvent(
	subject: string,
	event: DocumentEvent,
	overrides: Partial<DocumentLifecycleEvent> = {},
): DocumentLifecycleEvent {
	return {
		subject,
		documentId: event.documentId,
		bucket: event.bucket,
		objectKey: event.objectKey,
		contentType: event.contentType,
		processingVersion: event.processingVersion ?? 1,
		occurredAt: new Date().toISOString(),
		...overrides,
	};
}

export async function publishLifecycleEvent(
	nc: NatsConnection,
	event: DocumentLifecycleEvent,
): Promise<void> {
	nc.publish(event.subject, JSON.stringify(event));
	await nc.flush();
}
