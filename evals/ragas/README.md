# RAGAS Chunking Evaluation

This harness scores the quality of the current processed document chunks by running representative queries against live Postgres-backed retrieval data and evaluating the retrieved chunks with RAGAS.

Create a local gold case file:

```bash
cp evals/ragas/chunking_cases.example.jsonl evals/ragas/chunking_cases.jsonl
```

`chunking_cases.jsonl` is intentionally ignored because cases can include private source excerpts. Each JSONL row supports:

- `query`: required natural-language query
- `reference_context_ids`: stable chunk citation IDs, such as `s3://documents/path/to/source.md#chunk-0`
- `reference_contexts`: source passage text for non-LLM RAGAS context precision/recall
- `prefix`, `documentId`, `metadata`, `limit`: optional retrieval filters matching `documents.search`

Install and run:

```bash
python3 -m venv .venv-ragas
. .venv-ragas/bin/activate
python3 -m pip install -r evals/ragas/requirements.txt
make ragas-chunking-eval RAGAS_ARGS="--min-id-recall 0.8 --min-context-recall 0.8"
```

The Make target reads the CNPG app credentials from the cluster secret and opens a temporary `kubectl port-forward` before running the eval; it does not rewrite `.devcontainer/.env`. If you run the Python script directly, it reads Postgres connection details from `--postgres-dsn`, `DB_*`, `POSTGRES_*`, `PG*`, `POSTGRES_CONNECTION_STRING`, or `DATABASE_URL`; direct script runs need their own working Postgres route.

## Gold Case Guidance

Prefer 5-10 private gold cases before changing chunking, embedding dimensions, ranking, or metadata-filter behavior. A useful starter set includes:

- two exact-identity questions where the answer should live in one known source document
- two relationship questions where the right chunk mentions both entities or the key link between them
- one metadata-filtered query that proves curated metadata narrowing still returns the expected chunk
- one negative or near-neighbor query where the expected source should outrank a similarly named document

Use `documents.search` or the UI Search tab to capture the stable citation ID for each expected chunk. The ID format used by this harness is `sourceUri#chunk-N`, matching the citation IDs returned by the browser and MCP retrieval surfaces.

The JSON report intentionally includes citation-confidence fields for every retrieved chunk:

- `source_uri`
- `object_key`
- `content_type`
- `chunk_index`
- `processing_version`
- `metadata`
- `citation.id`, `citation.label`, and the source/chunk/version fields used to render it

Keep the first private baseline permissive while the gold set is still small, for example:

```bash
make ragas-chunking-eval RAGAS_ARGS="--min-id-recall 0.6 --min-context-recall 0.6 --output evals/ragas/baseline.report.json"
```

Once the cases represent real operator questions, raise the regular gate toward `--min-id-recall 0.8 --min-context-recall 0.8`. Treat a missing expected citation ID or a dropped processing version/source URI as a retrieval-quality regression, even when the rendered answer text still looks plausible.

## Local Embeddings

The default `local-embeddings` mode uses the same deterministic 384-dimensional token-hash embedding shape as the processor fallback. It stores and queries `vector(384)` rows so the harness can run without an external embedding service. These scores are useful for regression checks against the current fallback behavior, but they should not be interpreted as a production-quality semantic benchmark for a future embedding model. When `EMBEDDING_ENDPOINT` and a non-local `EMBEDDING_MODEL` are supplied, the case file and thresholds should be baselined separately for that model.
