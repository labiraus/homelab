import { Client } from "minio";

import type { ProcessorConfig } from "./config.js";

export interface DocumentStorage {
	readDocument(bucket: string, objectKey: string): Promise<Buffer>;
}

export function createDocumentStorage(config: ProcessorConfig): DocumentStorage {
	const endpoint = parseMinioEndpoint(config.minioEndpoint, config.minioUseSSL);
	const client = new Client({
		endPoint: endpoint.endPoint,
		port: endpoint.port,
		useSSL: config.minioUseSSL,
		accessKey: config.minioAccessKey,
		secretKey: config.minioSecretKey,
		region: config.minioRegion || undefined,
	});

	return {
		async readDocument(bucket: string, objectKey: string): Promise<Buffer> {
			const resolvedBucket = bucket || config.minioBucket;
			const stream = await client.getObject(resolvedBucket, objectKey);
			const chunks: Buffer[] = [];

			for await (const chunk of stream) {
				chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
			}

			return Buffer.concat(chunks);
		},
	};
}

export function decodeUtf8Text(value: Buffer): string {
	return new TextDecoder("utf-8", { fatal: true }).decode(value);
}

export function parseMinioEndpoint(
	endpoint: string,
	useSSL: boolean,
): { endPoint: string; port?: number } {
	const normalized = endpoint.includes("://")
		? endpoint
		: `${useSSL ? "https" : "http"}://${endpoint}`;
	const parsed = new URL(normalized);
	const port = parsed.port ? Number(parsed.port) : undefined;

	return {
		endPoint: parsed.hostname,
		port: Number.isFinite(port) ? port : undefined,
	};
}
