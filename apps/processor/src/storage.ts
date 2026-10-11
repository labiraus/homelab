import { GetObjectCommand, S3Client, S3ServiceException } from "@aws-sdk/client-s3";

import type { ProcessorConfig } from "./config.js";

export interface DocumentStorage {
	readDocument(bucket: string, objectKey: string, reference?: { versionMarker?: string; etag?: string }): Promise<Buffer>;
}

export function createDocumentStorage(config: Pick<ProcessorConfig,
	"minioEndpoint" | "minioUseSSL" | "minioAccessKey" | "minioSecretKey" | "minioRegion" | "minioBucket"
>): DocumentStorage {
	const endpoint = parseMinioEndpoint(config.minioEndpoint, config.minioUseSSL);
	const origin = `${config.minioUseSSL ? "https" : "http"}://${endpoint.endPoint}${endpoint.port ? `:${endpoint.port}` : ""}`;
	const client = new S3Client({
		endpoint: origin,
		forcePathStyle: true,
		region: config.minioRegion || "us-east-1",
		credentials: {
			accessKeyId: config.minioAccessKey,
			secretAccessKey: config.minioSecretKey,
		},
		maxAttempts: 3,
	});

	return {
		async readDocument(bucket: string, objectKey: string, reference?: { versionMarker?: string; etag?: string }): Promise<Buffer> {
			const resolvedBucket = bucket || config.minioBucket;
			const version = reference?.versionMarker?.trim() || undefined;
			const etag = reference?.etag?.trim();
			try {
				const response = await client.send(new GetObjectCommand({
					Bucket: resolvedBucket, Key: objectKey, VersionId: version,
					IfMatch: etag ? JSON.stringify(etag.replace(/^"|"$/g, "")) : undefined,
				}));
				if (!response.Body) throw new Error("S3 response did not include an object body");
				return Buffer.from(await response.Body.transformToByteArray());
			} catch (error) {
				// Preserve the adapter's S3 error code for callers without exposing
				// the SDK's response-body/transport types through our interface.
				if (error instanceof S3ServiceException) Object.assign(error, { code: error.name });
				throw error;
			}
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
	if (parsed.protocol !== (useSSL ? "https:" : "http:") || parsed.username || parsed.password ||
		(parsed.pathname !== "" && parsed.pathname !== "/") || parsed.search || parsed.hash) {
		throw new Error("MINIO_ENDPOINT must be an HTTP(S) origin matching MINIO_USE_SSL, without credentials, path or query");
	}
	const port = parsed.port ? Number(parsed.port) : undefined;

	return {
		endPoint: parsed.hostname,
		port: Number.isFinite(port) ? port : undefined,
	};
}
