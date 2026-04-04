export interface ProcessorConfig {
	port: number;
	kafkaBrokers: string[];
	inputTopic: string;
	consumerGroup: string;
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
		kafkaBrokers: requireEnv("KAFKA_BROKERS").split(",").map((value) => value.trim()).filter(Boolean),
		inputTopic: process.env.KAFKA_TOPIC?.trim() || "documents.ingest",
		consumerGroup: process.env.KAFKA_GROUP?.trim() || "processor",
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
