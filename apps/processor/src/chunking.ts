import type { Chunk, ChunkMetadata, ExtractedSegment } from "./types.js";

export interface ChunkingConfig {
	chunkSize: number;
	chunkOverlap: number;
}

export function chunkText(input: string | ExtractedSegment[], config: ChunkingConfig): Chunk[] {
	const normalized = normalizeChunkInput(input);
	if (!normalized) {
		return [];
	}

	const chunkSize = Math.max(config.chunkSize, 1);
	const overlap = Math.max(Math.min(config.chunkOverlap, chunkSize - 1), 0);
	const chunks: Chunk[] = [];

	let start = 0;
	let index = 0;
	while (start < normalized.text.length) {
		const end = Math.min(start + chunkSize, normalized.text.length);
		const rawValue = normalized.text.slice(start, end);
		const leadingTrim = rawValue.length - rawValue.trimStart().length;
		const trailingTrim = rawValue.length - rawValue.trimEnd().length;
		const value = rawValue.trim();
		if (value) {
			const chunkStart = start + leadingTrim;
			const chunkEnd = end - trailingTrim;
			chunks.push({
				index,
				text: value,
				tokenCount: countTokens(value),
				metadata: mergedChunkMetadata(normalized.spans, chunkStart, chunkEnd),
			});
			index += 1;
		}
		if (end === normalized.text.length) {
			break;
		}
		start = Math.max(end - overlap, start + 1);
	}

	return chunks;
}

function countTokens(text: string): number {
	return text.split(/\s+/).filter(Boolean).length;
}

interface SegmentSpan {
	start: number;
	end: number;
	metadata?: ChunkMetadata;
}

function normalizeChunkInput(input: string | ExtractedSegment[]): { text: string; spans: SegmentSpan[] } | null {
	const segments = typeof input == "string" ? [{ text: input }] : input;
	const spans: SegmentSpan[] = [];
	let text = "";

	for (const segment of segments) {
		const value = segment.text.trim();
		if (!value) {
			continue;
		}
		if (text) {
			text += "\n\n";
		}
		const start = text.length;
		text += value;
		spans.push({
			start,
			end: text.length,
			metadata: cloneChunkMetadata(segment.metadata),
		});
	}

	if (!text) {
		return null;
	}
	return { text, spans };
}

function mergedChunkMetadata(spans: SegmentSpan[], start: number, end: number): ChunkMetadata | undefined {
	const overlaps = spans
		.map((span) => {
			const overlap = Math.min(end, span.end) - Math.max(start, span.start);
			return overlap > 0 ? { span, overlap } : null;
		})
		.filter(Boolean) as { span: SegmentSpan; overlap: number }[];
	if (overlaps.length == 0) {
		return undefined;
	}

	const dominant = overlaps.reduce((selected, current) => {
		if (current.overlap > selected.overlap) {
			return current;
		}
		if (current.overlap == selected.overlap && current.span.end >= selected.span.end) {
			return current;
		}
		return selected;
	});

	const metadata = cloneChunkMetadata(dominant.span.metadata) ?? {};
	const warnings = new Set<string>(metadata.warnings ?? []);
	for (const overlap of overlaps) {
		for (const warning of overlap.span.metadata?.warnings ?? []) {
			warnings.add(warning);
		}
		if (metadata.title == undefined && overlap.span.metadata?.title) {
			metadata.title = overlap.span.metadata.title;
		}
		if (metadata.sourceType == undefined && overlap.span.metadata?.sourceType) {
			metadata.sourceType = overlap.span.metadata.sourceType;
		}
	}
	if (warnings.size > 0) {
		metadata.warnings = [...warnings];
	}
	return Object.keys(metadata).length > 0 ? metadata : undefined;
}

function cloneChunkMetadata(metadata?: ChunkMetadata): ChunkMetadata | undefined {
	if (!metadata) {
		return undefined;
	}
	return {
		sourceType: metadata.sourceType,
		title: metadata.title,
		headingPath: metadata.headingPath ? [...metadata.headingPath] : undefined,
		warnings: metadata.warnings ? [...metadata.warnings] : undefined,
	};
}
