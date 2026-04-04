export interface DocumentEvent {
	documentId: string;
	sourceUri: string;
	contentType: string;
	text: string;
	metadata?: Record<string, unknown>;
	requestedAt: string;
}

export interface Chunk {
	index: number;
	text: string;
	tokenCount: number;
}

export interface EmbeddingResult {
	model: string;
	vector: number[];
}
