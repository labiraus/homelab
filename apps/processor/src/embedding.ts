import type { EmbeddingResult } from "./types.js";

const LOCAL_EMBEDDING_MODEL = "local-embeddings";
const LOCAL_EMBEDDING_DIMENSIONS = 384;

interface EmbeddingApiResponse {
	data?: Array<{ embedding: number[] }>;
	model?: string;
}

export async function fetchEmbedding(
	endpoint: string,
	model: string,
	input: string,
): Promise<EmbeddingResult> {
	if (!endpoint && model === LOCAL_EMBEDDING_MODEL) {
		return {
			model,
			vector: embedText(input),
		};
	}
	if (!endpoint) {
		throw new Error("EMBEDDING_ENDPOINT is required for non-local embedding models");
	}

	const response = await fetch(endpoint, {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify({
			model,
			input,
		}),
	});

	if (!response.ok) {
		throw new Error(`embedding request failed: ${response.status} ${response.statusText}`);
	}

	const payload = (await response.json()) as EmbeddingApiResponse;
	const vector = payload.data?.[0]?.embedding;
	if (!vector) {
		throw new Error("embedding response did not include a vector");
	}

	return {
		model: payload.model ?? model,
		vector,
	};
}

export function embedText(input: string): number[] {
	const vector = Array.from({ length: LOCAL_EMBEDDING_DIMENSIONS }, () => 0);
	const tokens = tokenize(input);
	if (tokens.length === 0) {
		tokens.push(input.trim().toLowerCase());
	}

	for (const token of tokens) {
		const hash = fnv1a(token);
		const index = hash % LOCAL_EMBEDDING_DIMENSIONS;
		vector[index] += (hash & 0x80000000) !== 0 ? -1 : 1;
	}

	normalize(vector);
	return vector;
}

function tokenize(input: string): string[] {
	return input.toLowerCase().match(/[a-z0-9]+/g) ?? [];
}

function fnv1a(value: string): number {
	let hash = 2166136261;
	for (let index = 0; index < value.length; index += 1) {
		hash ^= value.charCodeAt(index);
		hash = Math.imul(hash, 16777619) >>> 0;
	}
	return hash;
}

function normalize(vector: number[]): void {
	const magnitude = Math.sqrt(vector.reduce((sum, value) => sum + value * value, 0));
	if (magnitude === 0) {
		return;
	}

	for (let index = 0; index < vector.length; index += 1) {
		vector[index] /= magnitude;
	}
}
