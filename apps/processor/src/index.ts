import http from "node:http";

import { Kafka } from "kafkajs";
import { Pool } from "pg";

import { chunkText } from "./chunking.js";
import { loadConfig } from "./config.js";
import { ensureSchema, persistDocument } from "./db.js";
import { fetchEmbedding } from "./embedding.js";
import type { DocumentEvent, EmbeddingResult } from "./types.js";

async function main(): Promise<void> {
	const config = loadConfig();
	const pool = new Pool({ connectionString: config.postgresConnectionString });
	await ensureSchema(pool);

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

	const kafka = new Kafka({
		brokers: config.kafkaBrokers,
	});
	const consumer = kafka.consumer({ groupId: config.consumerGroup });
	await consumer.connect();
	await consumer.subscribe({ topic: config.inputTopic, fromBeginning: false });

	ready = true;

	await consumer.run({
		eachMessage: async ({ message }) => {
			if (!message.value) {
				return;
			}

			const event = JSON.parse(message.value.toString()) as DocumentEvent;
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
		},
	});
}

main().catch((error) => {
	console.error(error);
	process.exit(1);
});
