export interface DocumentEvent {
	documentId: string;
	bucket: string;
	objectKey: string;
	sourceUri: string;
	contentType: string;
	versionMarker?: string;
	etag?: string;
	sizeBytes?: number;
	lastModified?: string;
	metadata?: Record<string, unknown>;
	requestedAt: string;
	processingVersion?: number;
}

export interface DocumentLifecycleEvent {
	subject: string;
	documentId: string;
	bucket?: string;
	objectKey?: string;
	contentType?: string;
	processingVersion?: number;
	occurredAt: string;
	error?: string;
	warnings?: string[];
}

export interface ChunkMetadata {
	sourceType?: string;
	title?: string;
	headingPath?: string[];
	warnings?: string[];
}

export interface ExtractedSegment {
	text: string;
	metadata?: ChunkMetadata;
}

export interface Chunk {
	index: number;
	text: string;
	tokenCount: number;
	metadata?: ChunkMetadata;
}

export interface EmbeddingResult {
	model: string;
	vector: number[];
}

export interface DocumentState {
	documentPk: number;
	status: string;
	desiredProcessingVersion: number;
	currentProcessingVersion: number;
}

export interface DocumentClaimResult {
	kind: "claimed" | "noop" | "retry";
	documentPk?: number;
	reason: string;
}
