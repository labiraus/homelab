import http from "node:http";

import { AckPolicy, RetentionPolicy, StorageType, connect } from "nats";
import { Pool } from "pg";

import { chunkText } from "./chunking.js";
import { loadConfig } from "./config.js";
import { extractDocumentContent } from "./extractor.js";
import {
	buildLifecycleEvent,
	DOCUMENT_EVENT_SUBJECTS,
	publishLifecycleEvent,
} from "./events.js";
import {
	claimDocumentForProcessing,
	ensureSchema,
	markDocumentPendingWithError,
	persistProcessedDocument,
	recordLifecycleEvent,
} from "./db.js";
import { fetchEmbedding } from "./embedding.js";
import { createDocumentStorage } from "./storage.js";
import type { DocumentEvent, DocumentLifecycleEvent, EmbeddingResult } from "./types.js";

async function main(): Promise<void> {
	const config = loadConfig();
	const storage = createDocumentStorage(config);
	let ready = false;
	const server = http.createServer((request, response) => {
		if (request.url === "/liveness") {
			response.writeHead(200).end("ok");
			return;
		}
		if (request.url === "/readiness") {
			response.writeHead(ready ? 200 : 503).end(ready ? "ready" : "starting");
			return;
		}
		response.writeHead(404).end("not found");
	});

	server.listen(config.port, "0.0.0.0");

	for (;;) {
		const pool = new Pool({ connectionString: config.postgresConnectionString });
		let nc: Awaited<ReturnType<typeof connect>> | undefined;
		try {
			await ensureSchema(pool);

			nc = await connect({
				servers: config.natsServers,
			});
			const jsm = await nc.jetstreamManager();
			await ensureStream(jsm, config.streamName, config.subject);
			await ensureNotificationStream(jsm, config.eventsStreamName, config.eventsSubject);
			await ensureConsumer(jsm, config.streamName, config.consumerName, config.subject);

			const js = nc.jetstream();
			const consumer = await js.consumers.get(config.streamName, config.consumerName);
			const messages = await consumer.consume();

			ready = true;

			for await (const message of messages) {
				try {
					const event = JSON.parse(message.string()) as DocumentEvent;
					const claim = await claimDocumentForProcessing(pool, event);
					if (claim.kind === "noop") {
						message.ack();
						continue;
					}
					if (claim.kind === "retry" || claim.documentPk == null) {
						message.nak();
						continue;
					}

					await emitLifecycleEventBestEffort(
						pool,
						nc,
						buildLifecycleEvent(DOCUMENT_EVENT_SUBJECTS.processorStarted, event),
					);

					const raw = await storage.readDocument(event.bucket, event.objectKey);
					const extracted = extractDocumentContent(raw, event.contentType);
					const chunks = chunkText(extracted.segments, {
						chunkSize: config.chunkSize,
						chunkOverlap: config.chunkOverlap,
					});
					const embeddings: EmbeddingResult[] = [];
					for (const chunk of chunks) {
						const embedding = await fetchEmbedding(config.embeddingEndpoint, config.embeddingModel, chunk.text);
						embeddings.push(embedding);
					}

					await persistProcessedDocument(pool, claim.documentPk, event, chunks, embeddings);
					await emitLifecycleEventBestEffort(
						pool,
						nc,
						buildLifecycleEvent(DOCUMENT_EVENT_SUBJECTS.processorCompleted, event, {
							warnings: extracted.warnings,
						}),
					);
					message.ack();
				} catch (error) {
					const event = safeParseDocumentEvent(message.string());
					if (event) {
						await markDocumentPendingWithError(pool, event, formatError(error)).catch((persistError) => {
							console.error(persistError);
						});
						await emitLifecycleEventBestEffort(
							pool,
							nc,
							buildLifecycleEvent(DOCUMENT_EVENT_SUBJECTS.processorFailed, event, {
								error: formatError(error),
							}),
						);
					}
					message.nak();
					console.error(error);
				}
			}

			throw new Error("message consumer stopped");
		} catch (error) {
			console.error(error);
		} finally {
			ready = false;
			try {
				await nc?.drain();
			} catch {
				nc?.close();
			}
			await pool.end().catch(() => undefined);
		}

		await delay(5000);
	}
}

async function ensureStream(
	jsm: Awaited<ReturnType<Awaited<ReturnType<typeof connect>>["jetstreamManager"]>>,
	streamName: string,
	subject: string,
): Promise<void> {
	const config = {
		name: streamName,
		subjects: [subject],
		retention: RetentionPolicy.Workqueue,
		storage: StorageType.File,
		num_replicas: 3,
	};

	try {
		await jsm.streams.info(streamName);
		await jsm.streams.update(streamName, config);
	} catch (error) {
		if (!isNotFoundError(error)) {
			throw error;
		}
		await jsm.streams.add(config);
	}
}

async function ensureNotificationStream(
	jsm: Awaited<ReturnType<Awaited<ReturnType<typeof connect>>["jetstreamManager"]>>,
	streamName: string,
	subject: string,
): Promise<void> {
	const config = {
		name: streamName,
		subjects: [subject],
		retention: RetentionPolicy.Limits,
		storage: StorageType.File,
		num_replicas: 3,
	};

	try {
		await jsm.streams.info(streamName);
		await jsm.streams.update(streamName, config);
	} catch (error) {
		if (!isNotFoundError(error)) {
			throw error;
		}
		await jsm.streams.add(config);
	}
}

async function ensureConsumer(
	jsm: Awaited<ReturnType<Awaited<ReturnType<typeof connect>>["jetstreamManager"]>>,
	streamName: string,
	consumerName: string,
	subject: string,
): Promise<void> {
	const config = {
		durable_name: consumerName,
		filter_subject: subject,
		ack_policy: AckPolicy.Explicit,
	};

	try {
		await jsm.consumers.info(streamName, consumerName);
		await jsm.consumers.update(streamName, consumerName, config);
	} catch (error) {
		if (!isNotFoundError(error)) {
			throw error;
		}
		await jsm.consumers.add(streamName, config);
	}
}

function isNotFoundError(error: unknown): boolean {
	return error instanceof Error && "code" in error && error.code === "404";
}

function delay(ms: number): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, ms));
}

function safeParseDocumentEvent(payload: string): DocumentEvent | null {
	try {
		return JSON.parse(payload) as DocumentEvent;
	} catch {
		return null;
	}
}

function formatError(error: unknown): string {
	if (error instanceof Error) {
		return error.message;
	}

	return String(error);
}

async function emitLifecycleEventBestEffort(
	pool: Pool,
	nc: Awaited<ReturnType<typeof connect>> | undefined,
	event: DocumentLifecycleEvent,
): Promise<void> {
	if (!nc) {
		console.error(
			new Error(`nats connection is not available for lifecycle event ${event.subject} (${event.documentId})`),
		);
		return;
	}

	try {
		await publishLifecycleEvent(nc, event);
	} catch (error) {
		console.error(`failed to publish lifecycle event ${event.subject} for ${event.documentId}`, error);
		return;
	}

	try {
		await recordLifecycleEvent(pool, event);
	} catch (error) {
		console.error(`failed to persist lifecycle event ${event.subject} for ${event.documentId}`, error);
	}
}

main().catch((error) => {
	console.error(error);
	process.exit(1);
});
