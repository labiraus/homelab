export interface ProcessorConfig {
	port: number;
	natsServers: string[];
	streamName: string;
	subject: string;
	eventsStreamName: string;
	eventsSubject: string;
	consumerName: string;
	minioEndpoint: string;
	minioUseSSL: boolean;
	minioRegion: string;
	minioBucket: string;
	minioAccessKey: string;
	minioSecretKey: string;
	openSearchBaseUrl: string;
	openSearchUsername: string;
	openSearchPassword: string;
	openSearchIndex: string;
	openSearchIngestPipeline: string;
	openSearchSearchPipeline: string;
	openSearchModelId: string;
	openSearchVectorDimensions: number;
	openSearchChunkTokenLimit: number;
	openSearchChunkOverlapRate: number;
	postgresConnectionString: string;
}

function requireEnv(name: string): string {
	const value = process.env[name]?.trim();
	if (!value) {
		throw new Error(`${name} is required`);
	}
	return value;
}

export function loadConfig(): ProcessorConfig {
	const openSearchModelId =
		process.env.OPENSEARCH_RAG_MODEL_ID?.trim() ||
		process.env.AI_EMBEDDING_MODEL?.trim() ||
		process.env.EMBEDDING_MODEL?.trim() ||
		"";
	return {
		port: Number(process.env.PORT ?? "8080"),
		natsServers: requireEnv("NATS_URLS").split(",").map((value) => value.trim()).filter(Boolean),
		streamName: process.env.NATS_STREAM?.trim() || "documents",
		subject: process.env.NATS_SUBJECT?.trim() || "documents.ingest",
		eventsStreamName: process.env.NATS_EVENTS_STREAM?.trim() || "document-events",
		eventsSubject: process.env.NATS_EVENTS_SUBJECT?.trim() || "documents.events.>",
		consumerName: process.env.NATS_CONSUMER?.trim() || "processor",
		minioEndpoint: requireEnv("MINIO_ENDPOINT"),
		minioUseSSL: process.env.MINIO_USE_SSL?.trim().toLowerCase() === "true",
		minioRegion: process.env.MINIO_REGION?.trim() || "",
		minioBucket: process.env.MINIO_BUCKET?.trim() || "documents",
		minioAccessKey: requireEnv("MINIO_ACCESS_KEY"),
		minioSecretKey: requireEnv("MINIO_SECRET_KEY"),
		openSearchBaseUrl: requireEnv("OPENSEARCH_BASE_URL"),
		openSearchUsername: process.env.OPENSEARCH_USERNAME?.trim() || "",
		openSearchPassword: process.env.OPENSEARCH_PASSWORD?.trim() || "",
		openSearchIndex: process.env.OPENSEARCH_RAG_INDEX?.trim() || "rag-documents",
		openSearchIngestPipeline: process.env.OPENSEARCH_RAG_INGEST_PIPELINE?.trim() || "rag-native-ingest",
		openSearchSearchPipeline: process.env.OPENSEARCH_RAG_SEARCH_PIPELINE?.trim() || "rag-neural-search",
		openSearchModelId: openSearchModelId || requireEnv("OPENSEARCH_RAG_MODEL_ID"),
		openSearchVectorDimensions: Number(process.env.OPENSEARCH_VECTOR_DIMENSIONS ?? "1024"),
		openSearchChunkTokenLimit: Number(process.env.OPENSEARCH_CHUNK_TOKEN_LIMIT ?? "384"),
		openSearchChunkOverlapRate: Number(process.env.OPENSEARCH_CHUNK_OVERLAP_RATE ?? "0.15"),
		postgresConnectionString: buildPostgresConnectionString(),
	};
}

function buildPostgresConnectionString(): string {
	const connectionString = process.env.POSTGRES_CONNECTION_STRING?.trim();
	if (connectionString) {
		return connectionString;
	}

	const host = requireEnv("POSTGRES_HOST");
	const port = process.env.POSTGRES_PORT?.trim() || "5432";
	const user = requireEnv("POSTGRES_USER");
	const password = requireEnv("POSTGRES_PASSWORD");
	const database = requireEnv("POSTGRES_DATABASE");
	const sslmode = process.env.POSTGRES_SSLMODE?.trim() || "disable";

	return `postgres://${user}:${password}@${host}:${port}/${database}?sslmode=${sslmode}`;
}
