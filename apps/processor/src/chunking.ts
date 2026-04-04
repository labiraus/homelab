import type { Chunk } from "./types.js";

export interface ChunkingConfig {
	chunkSize: number;
	chunkOverlap: number;
}

export function chunkText(text: string, config: ChunkingConfig): Chunk[] {
	const normalized = text.trim();
	if (!normalized) {
		return [];
	}

	const chunkSize = Math.max(config.chunkSize, 1);
	const overlap = Math.max(Math.min(config.chunkOverlap, chunkSize - 1), 0);
	const chunks: Chunk[] = [];

	let start = 0;
	let index = 0;
	while (start < normalized.length) {
		const end = Math.min(start + chunkSize, normalized.length);
		const value = normalized.slice(start, end).trim();
		if (value) {
			chunks.push({
				index,
				text: value,
				tokenCount: countTokens(value),
			});
			index += 1;
		}
		if (end === normalized.length) {
			break;
		}
		start = Math.max(end - overlap, start + 1);
	}

	return chunks;
}

function countTokens(text: string): number {
	return text.split(/\s+/).filter(Boolean).length;
}
