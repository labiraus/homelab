import { Pool } from "pg";

import type {
	Chunk,
	DocumentClaimResult,
	DocumentEvent,
	DocumentState,
	EmbeddingResult,
} from "./types.js";

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
		last_event_subject TEXT,
		last_event_at TIMESTAMPTZ,
		last_error TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	"ALTER TABLE IF EXISTS rag.documents ADD COLUMN IF NOT EXISTS last_event_subject TEXT",
	"ALTER TABLE IF EXISTS rag.documents ADD COLUMN IF NOT EXISTS last_event_at TIMESTAMPTZ",
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
	`CREATE INDEX IF NOT EXISTS rag_embeddings_model_idx
		ON rag.embeddings (model)`,
	`CREATE INDEX IF NOT EXISTS rag_embeddings_vector_cosine_idx
		ON rag.embeddings
		USING hnsw (vector vector_cosine_ops)
		WHERE vector IS NOT NULL`,
];

export async function ensureSchema(pool: Pool): Promise<void> {
	for (const statement of statements) {
		await pool.query(statement);
	}
}

export async function claimDocumentForProcessing(
	pool: Pool,
	event: DocumentEvent,
): Promise<DocumentClaimResult> {
	const client = await pool.connect();
	const processingVersion = event.processingVersion ?? 1;
	try {
		await client.query("BEGIN");
		const result = await client.query<DocumentState>(
			`SELECT
				id AS "documentPk",
				status,
				desired_processing_version AS "desiredProcessingVersion",
				current_processing_version AS "currentProcessingVersion"
			FROM rag.documents
			WHERE document_id = $1
			FOR UPDATE`,
			[event.documentId],
		);

		const state = result.rows[0];
		const outcome = resolveClaimResult(state ?? null, processingVersion);
		if (outcome.kind !== "claimed") {
			await client.query("ROLLBACK");
			return outcome;
		}

		const claimed = await client.query(
			`UPDATE rag.documents
			SET status = 'processing',
				updated_at = NOW(),
				last_error = NULL
			WHERE id = $1
			  AND desired_processing_version = $2`,
			[outcome.documentPk, processingVersion],
		);
		if (claimed.rowCount !== 1) {
			await client.query("ROLLBACK");
			return { kind: "retry", reason: "document row changed before claim" };
		}

		await client.query("COMMIT");
		return outcome;
	} catch (error) {
		await client.query("ROLLBACK");
		throw error;
	} finally {
		client.release();
	}
}

export function resolveClaimResult(
	state: DocumentState | null,
	processingVersion: number,
): DocumentClaimResult {
	if (!state) {
		return { kind: "retry", reason: "document row is not visible yet" };
	}

	if (state.desiredProcessingVersion < processingVersion) {
		return { kind: "retry", reason: "document version is not committed yet" };
	}

	if (state.desiredProcessingVersion > processingVersion) {
		return { kind: "noop", reason: "document was superseded by a newer version" };
	}

	if (state.currentProcessingVersion >= processingVersion) {
		return { kind: "noop", reason: "document version is already processed" };
	}

	if (state.status === "processing") {
		return { kind: "retry", reason: "document is already being processed" };
	}

	return {
		kind: "claimed",
		documentPk: state.documentPk,
		reason: "document claimed for processing",
	};
}

export async function persistProcessedDocument(
	pool: Pool,
	documentPk: number,
	event: DocumentEvent,
	chunks: Chunk[],
	embeddings: EmbeddingResult[],
): Promise<void> {
	const client = await pool.connect();
	const processingVersion = event.processingVersion ?? 1;
	try {
		await client.query("BEGIN");
		await client.query(
			"DELETE FROM rag.embeddings WHERE chunk_id IN (SELECT id FROM rag.chunks WHERE document_pk = $1 AND processing_version = $2)",
			[documentPk, processingVersion],
		);
		await client.query("DELETE FROM rag.chunks WHERE document_pk = $1 AND processing_version = $2", [
			documentPk,
			processingVersion,
		]);

		for (const chunk of chunks) {
			const embedding = embeddings[chunk.index];
			const chunkResult = await client.query(
				`INSERT INTO rag.chunks (document_pk, processing_version, chunk_index, chunk_text, token_count, content_hash)
				 VALUES ($1, $2, $3, $4, $5, $6)
				 RETURNING id`,
				[documentPk, processingVersion, chunk.index, chunk.text, chunk.tokenCount, null],
			);

			const chunkId = chunkResult.rows[0]?.id as number;
			await client.query(
				`INSERT INTO rag.embeddings (chunk_id, model, vector)
				 VALUES ($1, $2, $3::vector)`,
				[chunkId, embedding.model, toVectorLiteral(embedding.vector)],
			);
		}

		await client.query(
			`UPDATE rag.documents
			SET current_processing_version = GREATEST(current_processing_version, $2),
				status = CASE
					WHEN desired_processing_version > $2 THEN 'pending'
					ELSE 'processed'
				END,
				last_processed_at = NOW(),
				updated_at = NOW(),
				last_error = NULL
			WHERE id = $1`,
			[documentPk, processingVersion],
		);

		await client.query("COMMIT");
	} catch (error) {
		await client.query("ROLLBACK");
		throw error;
	} finally {
		client.release();
	}
}

export async function markDocumentPendingWithError(
	pool: Pool,
	event: DocumentEvent,
	message: string,
): Promise<void> {
	await pool.query(
		`UPDATE rag.documents
		SET status = CASE
			WHEN desired_processing_version > $2 THEN status
			ELSE 'pending'
		END,
			last_error = $3,
			updated_at = NOW()
		WHERE document_id = $1`,
		[event.documentId, event.processingVersion ?? 1, message],
	);
}

export async function recordLifecycleEvent(
	pool: Pool,
	documentId: string,
	subject: string,
	occurredAt: string,
): Promise<void> {
	await pool.query(
		`UPDATE rag.documents
		SET last_event_subject = $2,
			last_event_at = $3::timestamptz,
			updated_at = NOW()
		WHERE document_id = $1`,
		[documentId, subject, occurredAt],
	);
}

export function toVectorLiteral(vector: number[]): string {
	return `[${vector.join(",")}]`;
}
