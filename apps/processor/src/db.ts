import { Pool } from "pg";

import type { Chunk, DocumentEvent, EmbeddingResult } from "./types.js";

const statements = [
	"CREATE EXTENSION IF NOT EXISTS vector",
	"CREATE SCHEMA IF NOT EXISTS rag",
	`CREATE TABLE IF NOT EXISTS rag.documents (
		id BIGSERIAL PRIMARY KEY,
		document_id TEXT NOT NULL UNIQUE,
		bucket_name TEXT,
		object_key TEXT,
		source_uri TEXT NOT NULL,
		content_type TEXT,
		version_marker TEXT,
		etag TEXT,
		size_bytes BIGINT,
		last_modified TIMESTAMPTZ,
		status TEXT NOT NULL DEFAULT 'pending',
		metadata JSONB,
		desired_processing_version INTEGER NOT NULL DEFAULT 1,
		current_processing_version INTEGER NOT NULL DEFAULT 0,
		last_reconciled_at TIMESTAMPTZ,
		last_processed_at TIMESTAMPTZ,
		last_error TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS rag_documents_bucket_object_key_idx
		ON rag.documents (bucket_name, object_key)
		WHERE bucket_name IS NOT NULL AND object_key IS NOT NULL`,
	`CREATE INDEX IF NOT EXISTS rag_documents_status_idx
		ON rag.documents (status)`,
	`CREATE INDEX IF NOT EXISTS rag_documents_processing_version_idx
		ON rag.documents (desired_processing_version, current_processing_version)`,
	`CREATE TABLE IF NOT EXISTS rag.chunks (
		id BIGSERIAL PRIMARY KEY,
		document_pk BIGINT NOT NULL REFERENCES rag.documents(id) ON DELETE CASCADE,
		processing_version INTEGER NOT NULL DEFAULT 1,
		chunk_index INTEGER NOT NULL,
		chunk_text TEXT NOT NULL,
		token_count INTEGER NOT NULL,
		content_hash TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE (document_pk, processing_version, chunk_index)
	)`,
	`CREATE INDEX IF NOT EXISTS rag_chunks_document_pk_idx
		ON rag.chunks (document_pk)`,
	`CREATE TABLE IF NOT EXISTS rag.embeddings (
		id BIGSERIAL PRIMARY KEY,
		chunk_id BIGINT NOT NULL REFERENCES rag.chunks(id) ON DELETE CASCADE,
		model TEXT NOT NULL,
		vector vector,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE (chunk_id, model)
	)`,
	`CREATE INDEX IF NOT EXISTS rag_embeddings_chunk_id_idx
		ON rag.embeddings (chunk_id)`,
];

export async function ensureSchema(pool: Pool): Promise<void> {
	for (const statement of statements) {
		await pool.query(statement);
	}
}

export async function persistDocument(
	pool: Pool,
	event: DocumentEvent,
	chunks: Chunk[],
	embeddings: EmbeddingResult[],
): Promise<void> {
	const client = await pool.connect();
	const processingVersion = event.processingVersion ?? 1;
	try {
		await client.query("BEGIN");
		const documentResult = await client.query(
			`INSERT INTO rag.documents (
			   document_id,
			   bucket_name,
			   object_key,
			   source_uri,
			   content_type,
			   version_marker,
			   etag,
			   size_bytes,
			   last_modified,
			   status,
			   metadata,
			   desired_processing_version,
			   current_processing_version,
			   last_reconciled_at,
			   last_processed_at,
			   updated_at
			 )
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'processed', $10, $11, $11, NULL, NOW(), NOW())
			 ON CONFLICT (document_id)
			 DO UPDATE SET
			   bucket_name = EXCLUDED.bucket_name,
			   object_key = EXCLUDED.object_key,
			   source_uri = EXCLUDED.source_uri,
			   content_type = EXCLUDED.content_type,
			   version_marker = EXCLUDED.version_marker,
			   etag = EXCLUDED.etag,
			   size_bytes = EXCLUDED.size_bytes,
			   last_modified = EXCLUDED.last_modified,
			   status = EXCLUDED.status,
			   metadata = EXCLUDED.metadata,
			   desired_processing_version = EXCLUDED.desired_processing_version,
			   current_processing_version = EXCLUDED.current_processing_version,
			   last_processed_at = NOW(),
			   updated_at = NOW(),
			   last_error = NULL
			 RETURNING id`,
			[
				event.documentId,
				event.bucket ?? null,
				event.objectKey ?? null,
				event.sourceUri,
				event.contentType,
				event.versionMarker ?? null,
				event.etag ?? null,
				event.sizeBytes ?? null,
				event.lastModified ?? null,
				event.metadata ?? null,
				processingVersion,
			],
		);

		const documentId = documentResult.rows[0]?.id as number;
		await client.query("DELETE FROM rag.embeddings WHERE chunk_id IN (SELECT id FROM rag.chunks WHERE document_pk = $1)", [documentId]);
		await client.query("DELETE FROM rag.chunks WHERE document_pk = $1", [documentId]);

		for (const chunk of chunks) {
			const embedding = embeddings[chunk.index];
			const chunkResult = await client.query(
				`INSERT INTO rag.chunks (document_pk, processing_version, chunk_index, chunk_text, token_count, content_hash)
				 VALUES ($1, $2, $3, $4, $5, $6)
				 RETURNING id`,
				[documentId, processingVersion, chunk.index, chunk.text, chunk.tokenCount, null],
			);

			const chunkId = chunkResult.rows[0]?.id as number;
			await client.query(
				`INSERT INTO rag.embeddings (chunk_id, model, vector)
				 VALUES ($1, $2, $3::vector)`,
				[chunkId, embedding.model, toVectorLiteral(embedding.vector)],
			);
		}

		await client.query("COMMIT");
	} catch (error) {
		await client.query("ROLLBACK");
		throw error;
	} finally {
		client.release();
	}
}

export function toVectorLiteral(vector: number[]): string {
	return `[${vector.join(",")}]`;
}
