export interface DocumentEvent {
	documentId: string;
	bucket?: string;
	objectKey?: string;
	sourceUri: string;
	contentType: string;
	versionMarker?: string;
	etag?: string;
	sizeBytes?: number;
	lastModified?: string;
	text: string;
	metadata?: Record<string, unknown>;
	requestedAt: string;
	processingVersion?: number;
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
