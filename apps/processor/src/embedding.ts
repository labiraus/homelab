import type { EmbeddingResult } from "./types.js";

interface EmbeddingApiResponse {
	data?: Array<{ embedding: number[] }>;
	model?: string;
}

export async function fetchEmbedding(
	endpoint: string,
	model: string,
	input: string,
): Promise<EmbeddingResult> {
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
