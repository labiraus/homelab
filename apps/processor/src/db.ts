import { Pool } from "pg";

import type { Chunk, DocumentEvent, EmbeddingResult } from "./types.js";

const statements = [
	"CREATE EXTENSION IF NOT EXISTS vector",
	"CREATE SCHEMA IF NOT EXISTS rag_pipeline",
	`CREATE TABLE IF NOT EXISTS rag_pipeline.documents (
		id BIGSERIAL PRIMARY KEY,
		document_id TEXT NOT NULL UNIQUE,
		source_uri TEXT NOT NULL,
		content_type TEXT NOT NULL,
		metadata JSONB,
		requested_at TIMESTAMPTZ NOT NULL,
		processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS rag_pipeline.chunks (
		id BIGSERIAL PRIMARY KEY,
		document_id BIGINT NOT NULL REFERENCES rag_pipeline.documents(id) ON DELETE CASCADE,
		chunk_index INTEGER NOT NULL,
		chunk_text TEXT NOT NULL,
		token_count INTEGER NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE (document_id, chunk_index)
	)`,
	`CREATE TABLE IF NOT EXISTS rag_pipeline.embeddings (
		id BIGSERIAL PRIMARY KEY,
		chunk_id BIGINT NOT NULL UNIQUE REFERENCES rag_pipeline.chunks(id) ON DELETE CASCADE,
		model TEXT NOT NULL,
		vector vector,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
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
	try {
		await client.query("BEGIN");
		const documentResult = await client.query(
			`INSERT INTO rag_pipeline.documents (document_id, source_uri, content_type, metadata, requested_at, processed_at)
			 VALUES ($1, $2, $3, $4, $5, NOW())
			 ON CONFLICT (document_id)
			 DO UPDATE SET
			   source_uri = EXCLUDED.source_uri,
			   content_type = EXCLUDED.content_type,
			   metadata = EXCLUDED.metadata,
			   requested_at = EXCLUDED.requested_at,
			   processed_at = NOW()
			 RETURNING id`,
			[event.documentId, event.sourceUri, event.contentType, event.metadata ?? null, event.requestedAt],
		);

		const documentId = documentResult.rows[0]?.id as number;
		await client.query("DELETE FROM rag_pipeline.embeddings WHERE chunk_id IN (SELECT id FROM rag_pipeline.chunks WHERE document_id = $1)", [documentId]);
		await client.query("DELETE FROM rag_pipeline.chunks WHERE document_id = $1", [documentId]);

		for (const chunk of chunks) {
			const embedding = embeddings[chunk.index];
			const chunkResult = await client.query(
				`INSERT INTO rag_pipeline.chunks (document_id, chunk_index, chunk_text, token_count)
				 VALUES ($1, $2, $3, $4)
				 RETURNING id`,
				[documentId, chunk.index, chunk.text, chunk.tokenCount],
			);

			const chunkId = chunkResult.rows[0]?.id as number;
			await client.query(
				`INSERT INTO rag_pipeline.embeddings (chunk_id, model, vector)
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
