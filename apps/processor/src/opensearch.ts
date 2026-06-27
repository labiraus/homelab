import type { ProcessorConfig } from "./config.js";
import type { DocumentEvent, ExtractedSegment } from "./types.js";

export interface OpenSearchDocument {
	documentId: string;
	sourceUri: string;
	objectKey: string;
	bucket: string;
	contentType: string;
	status: "processed";
	metadata: Record<string, unknown>;
	processingVersion: number;
	lastProcessedAt: string;
	body: string;
}

export async function ensureOpenSearchResources(config: ProcessorConfig): Promise<void> {
	if (!config.openSearchModelId) {
		throw new Error("OPENSEARCH_RAG_MODEL_ID or AI_EMBEDDING_MODEL is required");
	}
	await putOpenSearchJSON(config, `/${encodeURIComponent(config.openSearchIndex)}`, buildIndexPayload(config));
	await putOpenSearchJSON(
		config,
		`/_ingest/pipeline/${encodeURIComponent(config.openSearchIngestPipeline)}`,
		buildIngestPipelinePayload(config),
	);
	await putOpenSearchJSON(
		config,
		`/_search/pipeline/${encodeURIComponent(config.openSearchSearchPipeline)}`,
		buildSearchPipelinePayload(config),
	);
}

export async function indexDocumentInOpenSearch(
	config: ProcessorConfig,
	event: DocumentEvent,
	segments: ExtractedSegment[],
): Promise<void> {
	const document = buildOpenSearchDocument(event, segments);
	const path =
		`/${encodeURIComponent(config.openSearchIndex)}/_doc/${encodeURIComponent(event.documentId)}` +
		`?pipeline=${encodeURIComponent(config.openSearchIngestPipeline)}`;
	await putOpenSearchJSON(config, path, document);
}

export function buildOpenSearchDocument(event: DocumentEvent, segments: ExtractedSegment[]): OpenSearchDocument {
	return {
		documentId: event.documentId,
		sourceUri: event.sourceUri,
		objectKey: event.objectKey,
		bucket: event.bucket,
		contentType: event.contentType,
		status: "processed",
		metadata: event.metadata ?? {},
		processingVersion: event.processingVersion ?? 1,
		lastProcessedAt: new Date().toISOString(),
		body: segments
			.map((segment) => segment.text.trim())
			.filter(Boolean)
			.join("\n\n"),
	};
}

export function buildIndexPayload(config: ProcessorConfig): Record<string, unknown> {
	return {
		settings: {
			index: {
				knn: true,
			},
		},
		mappings: {
			dynamic_templates: [
				{
					metadata_strings: {
						path_match: "metadata.*",
						match_mapping_type: "string",
						mapping: {
							type: "keyword",
							ignore_above: 1024,
						},
					},
				},
			],
			properties: {
				documentId: { type: "keyword" },
				sourceUri: { type: "keyword" },
				objectKey: { type: "keyword" },
				bucket: { type: "keyword" },
				contentType: { type: "keyword" },
				status: { type: "keyword" },
				metadata: { type: "object", dynamic: true },
				processingVersion: { type: "integer" },
				lastProcessedAt: { type: "date" },
				body: { type: "text" },
				passage_chunk: {
					type: "nested",
					properties: {
						text: { type: "text" },
						embedding: {
							type: "knn_vector",
							dimension: config.openSearchVectorDimensions,
							method: {
								name: "hnsw",
								space_type: "cosinesimil",
								engine: "lucene",
							},
						},
					},
				},
			},
		},
	};
}

export function buildIngestPipelinePayload(config: ProcessorConfig): Record<string, unknown> {
	return {
		description: "Chunk source text and generate chunk embeddings through the configured OpenSearch ML model.",
		processors: [
			{
				text_chunking: {
					algorithm: {
						fixed_token_length: {
							token_limit: config.openSearchChunkTokenLimit,
							overlap_rate: config.openSearchChunkOverlapRate,
						},
					},
					field_map: {
						body: "passage_chunk",
					},
				},
			},
			{
				foreach: {
					field: "passage_chunk",
					processor: {
						set: {
							field: "_ingest._value",
							value: {
								text: "{{_ingest._value}}",
							},
						},
					},
				},
			},
			{
				foreach: {
					field: "passage_chunk",
					processor: {
						ml_inference: {
							model_id: config.openSearchModelId,
							input_map: [
								{
									inputText: "_ingest._value.text",
								},
							],
							output_map: [
								{
									embedding: "_ingest._value.embedding",
								},
							],
						},
					},
				},
			},
			{
				remove: {
					field: "body",
					ignore_missing: true,
				},
			},
		],
	};
}

export function buildSearchPipelinePayload(config: ProcessorConfig): Record<string, unknown> {
	return {
		description: "Embed neural search queries through the configured OpenSearch ML model.",
		request_processors: [
			{
				ml_inference: {
					model_id: config.openSearchModelId,
					input_map: [
						{
							inputText: "query.ext.ml_inference.params.query_text",
						},
					],
					output_map: [
						{
							embedding: "query.ext.ml_inference.params.query_embedding",
						},
					],
				},
			},
		],
	};
}

async function putOpenSearchJSON(config: ProcessorConfig, path: string, payload: unknown): Promise<void> {
	const baseUrl = config.openSearchBaseUrl.replace(/\/+$/, "");
	const response = await fetch(baseUrl + path, {
		method: "PUT",
		headers: buildHeaders(config),
		body: JSON.stringify(payload),
	});
	if (!response.ok) {
		throw new Error(`OpenSearch request failed: ${response.status} ${response.statusText}: ${await response.text()}`);
	}
}

function buildHeaders(config: ProcessorConfig): Record<string, string> {
	const headers: Record<string, string> = {
		Accept: "application/json",
		"Content-Type": "application/json",
	};
	if (config.openSearchUsername || config.openSearchPassword) {
		headers.Authorization = `Basic ${Buffer.from(`${config.openSearchUsername}:${config.openSearchPassword}`).toString("base64")}`;
	}
	return headers;
}
