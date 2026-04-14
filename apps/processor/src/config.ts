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
	embeddingEndpoint: string;
	embeddingModel: string;
	chunkSize: number;
	chunkOverlap: number;
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
		embeddingEndpoint: requireEnv("EMBEDDING_ENDPOINT"),
		embeddingModel: process.env.EMBEDDING_MODEL?.trim() || "local-embeddings",
		chunkSize: Number(process.env.CHUNK_SIZE ?? "1200"),
		chunkOverlap: Number(process.env.CHUNK_OVERLAP ?? "200"),
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
