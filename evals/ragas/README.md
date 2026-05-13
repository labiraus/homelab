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
