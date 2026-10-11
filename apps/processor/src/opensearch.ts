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
	const model = await fetch(
		config.openSearchBaseUrl.replace(/\/+$/, "") + `/_plugins/_ml/models/${encodeURIComponent(config.openSearchModelId)}`,
		{ headers: buildHeaders(config), signal: AbortSignal.timeout(10_000) },
	);
	if (!model.ok) {
		throw new Error(`Configured OpenSearch embedding model is unavailable: HTTP ${model.status}`);
	}
	const modelState = await model.json() as { model_state?: string };
	if (modelState.model_state !== "DEPLOYED") {
		throw new Error("Configured OpenSearch embedding model must be deployed before document consumption");
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
	const charLimit = config.openSearchChunkCharLimit ?? 480;
	if (!Number.isInteger(charLimit) || charLimit < 2 ||
		!Number.isFinite(config.openSearchChunkOverlapRate) || config.openSearchChunkOverlapRate < 0 ||
		config.openSearchChunkOverlapRate > 0.5) {
		throw new Error("Chunk character limit must be an integer >= 2 and overlap must be between 0 and 0.5");
	}
	return {
		description: "Chunk source text and generate chunk embeddings through the configured OpenSearch ML model.",
		processors: [
			{
				script: {
					lang: "painless",
					params: { char_limit: charLimit, overlap_rate: config.openSearchChunkOverlapRate },
					// Bound analyzer input and model tokens without splitting UTF-16
					// surrogate pairs (the native fixed_char_length processor can).
					source: `String text = ctx.body;
int limit = params.char_limit;
int overlap = (int)(limit * params.overlap_rate);
def chunks = new ArrayList();
int start = 0;
while (start < text.length()) {
  int end = (int)Math.min(start + limit, text.length());
  int last = (int)text.charAt(end - 1);
  if (end < text.length() && last >= 55296 && last <= 56319) { end--; }
  chunks.add(text.substring(start, end));
  if (end == text.length()) { break; }
  int next = end - overlap;
  int first = (int)text.charAt(next);
  if (next > start && first >= 56320 && first <= 57343) { next--; }
  start = next > start ? next : end;
}
ctx.bounded_chunks = chunks;`,
				},
			},
			{
				text_chunking: {
					algorithm: {
						fixed_token_length: {
							token_limit: config.openSearchChunkTokenLimit,
							overlap_rate: config.openSearchChunkOverlapRate,
							max_chunk_limit: -1,
						},
					},
					field_map: { bounded_chunks: "passage_chunk" },
				},
			},
			{
				script: {
					lang: "painless",
					// Preserve raw strings: Mustache interpolation escapes quotes,
					// backslashes and newlines before assigning the chunk text.
					source: "ctx.remove('bounded_chunks'); def chunks = new ArrayList(); for (def text : ctx.passage_chunk) { chunks.add(['text': text]); } ctx.passage_chunk = chunks;",
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
									input: "_ingest._value.text",
								},
							],
							output_map: [
								{
									"_ingest._value.embedding": "data[0].embedding",
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
							input: "ext.ml_inference.params.query_text",
						},
					],
					output_map: [
						{
							"ext.ml_inference.params.query_embedding": "data[0].embedding",
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
		const body = await response.text();
		if (response.status === 400 && body.includes("resource_already_exists_exception")) {
			return;
		}
		throw new Error(`OpenSearch request failed: ${response.status} ${response.statusText}: ${body}`);
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
