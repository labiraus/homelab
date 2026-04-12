import http from "node:http";

import { AckPolicy, RetentionPolicy, StorageType, connect } from "nats";
import { Pool } from "pg";

import { chunkText } from "./chunking.js";
import { loadConfig } from "./config.js";
import { ensureSchema, persistDocument } from "./db.js";
import { fetchEmbedding } from "./embedding.js";
import type { DocumentEvent, EmbeddingResult } from "./types.js";

async function main(): Promise<void> {
	const config = loadConfig();
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
			await ensureConsumer(jsm, config.streamName, config.consumerName, config.subject);

			const js = nc.jetstream();
			const consumer = await js.consumers.get(config.streamName, config.consumerName);
			const messages = await consumer.consume();

			ready = true;

			for await (const message of messages) {
				try {
					const event = JSON.parse(message.string()) as DocumentEvent;
					const chunks = chunkText(event.text, {
						chunkSize: config.chunkSize,
						chunkOverlap: config.chunkOverlap,
					});
					const embeddings: EmbeddingResult[] = [];
					for (const chunk of chunks) {
						const embedding = await fetchEmbedding(config.embeddingEndpoint, config.embeddingModel, chunk.text);
						embeddings.push(embedding);
					}

					await persistDocument(pool, event, chunks, embeddings);
					message.ack();
				} catch (error) {
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

main().catch((error) => {
	console.error(error);
	process.exit(1);
});
