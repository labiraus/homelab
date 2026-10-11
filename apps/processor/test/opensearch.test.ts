import assert from "node:assert/strict";
import { test } from "node:test";

import type { ProcessorConfig } from "../src/config.js";
import {
	buildIndexPayload,
	buildIngestPipelinePayload,
	buildOpenSearchDocument,
	buildSearchPipelinePayload,
	ensureOpenSearchResources,
} from "../src/opensearch.js";

test("builds an OpenSearch index with nested vector chunks", () => {
	const payload = buildIndexPayload(testConfig());

	assert.equal((payload.settings as any).index.knn, true);
	assert.equal((payload.mappings as any).properties.passage_chunk.type, "nested");
	assert.equal((payload.mappings as any).properties.passage_chunk.properties.embedding.type, "knn_vector");
	assert.equal((payload.mappings as any).properties.passage_chunk.properties.embedding.dimension, 1024);
});

test("builds ingest and search pipelines using OpenSearch-native chunking and inference", () => {
	const config = testConfig();
	const ingest = buildIngestPipelinePayload(config);
	const search = buildSearchPipelinePayload(config);
	const ingestText = JSON.stringify(ingest);
	const searchText = JSON.stringify(search);

	assert.match(ingestText, /text_chunking/);
	assert.match(ingestText, /ml_inference/);
	assert.match(ingestText, /model-123/);
	assert.match(searchText, /ml_inference/);
	assert.match(searchText, /query_text/);
	assert.match(searchText, /query_embedding/);
	const inference = (ingest.processors as any[]).find((step) => step.foreach).foreach.processor.ml_inference;
	assert.deepEqual(inference.input_map, [{ input: "_ingest._value.text" }]);
	assert.deepEqual(inference.output_map, [{ "_ingest._value.embedding": "data[0].embedding" }]);
	const queryInference = (search.request_processors as any[])[0].ml_inference;
	assert.deepEqual(queryInference.input_map, [{ input: "ext.ml_inference.params.query_text" }]);
	assert.deepEqual(queryInference.output_map, [{ "ext.ml_inference.params.query_embedding": "data[0].embedding" }]);
});

test("refuses chunk settings that cannot make bounded Unicode-safe progress", () => {
	for (const charLimit of [0, 1, 3.5, NaN]) {
		assert.throws(() => buildIngestPipelinePayload({ ...testConfig(), openSearchChunkCharLimit: charLimit }), /character limit/);
	}
	for (const overlap of [-0.1, 0.51, 1, NaN]) {
		assert.throws(() => buildIngestPipelinePayload({ ...testConfig(), openSearchChunkOverlapRate: overlap }), /overlap/);
	}
});

test("builds an OpenSearch document from extracted segments", () => {
	const document = buildOpenSearchDocument(
		{
			documentId: "s3://documents/runbooks/process.md",
			bucket: "documents",
			objectKey: "runbooks/process.md",
			sourceUri: "s3://documents/runbooks/process.md",
			contentType: "text/markdown",
			metadata: { tag: "runbook" },
			requestedAt: "2026-04-14T12:00:00Z",
			processingVersion: 3,
		},
		[{ text: "First section" }, { text: "Second section" }],
	);

	assert.equal(document.status, "processed");
	assert.equal(document.processingVersion, 3);
	assert.equal(document.body, "First section\n\nSecond section");
	assert.deepEqual(document.metadata, { tag: "runbook" });
});

test("ensureOpenSearchResources tolerates an existing index", async () => {
	const originalFetch = globalThis.fetch;
	const calls: string[] = [];
	globalThis.fetch = (async (input: string | URL | Request) => {
		calls.push(String(input));
		if (calls.length === 1) {
			return new Response(JSON.stringify({ model_state: "DEPLOYED" }), { status: 200 });
		}
		if (calls.length === 2) {
			return new Response(
				JSON.stringify({
					error: {
						type: "resource_already_exists_exception",
					},
				}),
				{ status: 400, statusText: "Bad Request" },
			);
		}
		return new Response("{}", { status: 200 });
	}) as typeof fetch;
	try {
		await ensureOpenSearchResources(testConfig());
	} finally {
		globalThis.fetch = originalFetch;
	}

	assert.equal(calls.length, 4);
	assert.match(calls[0], /_plugins\/_ml\/models\/model-123$/);
	assert.match(calls[1], /rag-documents$/);
});

test("an absent or undeployed model prevents pipeline creation and worker readiness", async () => {
	const originalFetch = globalThis.fetch;
	try {
		for (const [status, state] of [[404, undefined], [200, "REGISTERED"], [200, "DEPLOY_FAILED"], [200, undefined]] as const) {
			let calls = 0;
			globalThis.fetch = (async () => {
				calls++;
				return new Response(JSON.stringify({ model_state: state }), { status });
			}) as typeof fetch;
			await assert.rejects(ensureOpenSearchResources(testConfig()), /embedding model/);
			assert.equal(calls, 1);
		}
	} finally {
		globalThis.fetch = originalFetch;
	}
});

function testConfig(): ProcessorConfig {
	return {
		port: 8080,
		natsServers: ["nats://nats:4222"],
		streamName: "documents",
		subject: "documents.ingest",
		eventsStreamName: "document-events",
		eventsSubject: "documents.events.>",
		consumerName: "processor",
		minioEndpoint: "svartalfheim:9000",
		minioUseSSL: false,
		minioRegion: "",
		minioBucket: "documents",
		minioAccessKey: "access",
		minioSecretKey: "secret",
		openSearchBaseUrl: "http://opensearch:9200",
		openSearchUsername: "",
		openSearchPassword: "",
		openSearchIndex: "rag-documents",
		openSearchIngestPipeline: "rag-native-ingest",
		openSearchSearchPipeline: "rag-neural-search",
		openSearchModelId: "model-123",
		openSearchVectorDimensions: 1024,
		openSearchChunkTokenLimit: 384,
		openSearchChunkOverlapRate: 0.15,
		postgresConnectionString: "postgres://app:secret@app-db/app",
	};
}
