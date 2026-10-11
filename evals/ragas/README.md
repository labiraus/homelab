# Document retrieval evaluation

The default RAGAS harness queries authenticated external `POST /api/documents/search`, exercising the same OpenSearch retrieval and citation construction as the browser. It does not connect directly to OpenSearch, manufacture identity headers, read cluster secrets or open a port-forward. This evaluates retained source-document retrieval; Worldbuilder canon grounding has separate [synthetic fixtures and LAN quality gates](../worldbuilder/README.md).

```bash
python3 -m venv .venv-ragas
. .venv-ragas/bin/activate
python3 -m pip install -r evals/ragas/requirements.txt
cp evals/ragas/chunking_cases.example.jsonl evals/ragas/chunking_cases.jsonl
# In an approved authenticated LAN session, supply RAGAS_API_COOKIE or
# RAGAS_API_AUTHORIZATION through your local secret environment, never source.
export RAGAS_API_URL=https://mcp.labiraus.com
make ragas-chunking-eval RAGAS_ARGS="--min-id-recall 0.8 --min-context-recall 0.8"
```

Use the observed external root if different from this example. HTTPS is required except on loopback; redirects are refused so credentials cannot follow an unexpected login/host redirect. Authentication failure is a failed evaluation, never an empty successful result. No environment files are loaded implicitly; `--env-file` is explicit. Results are capped at 2 MB and require stable citation IDs matching hit processing versions/chunk indexes.

Each private JSONL gold case has `query`, `reference_context_ids` and/or `reference_contexts`, plus optional `prefix`, `documentId`, `metadata`, `limit`. Use the exact citation IDs returned by the API (`sourceUri#chunk-N`) and actual processed text granularity. Include identity, relationship, metadata-filter and negative/near-neighbour cases. Start with 5–10 representative cases and baseline thresholds before tightening them. Markdown/wiki links and whitespace are normalised for text comparison.

`--include-contexts --output evals/ragas/baseline.report.json` records retrieved text for private inspection; the local cases and report files are ignored. Reports retain document/source/object/chunk/version metadata and identify the backend. `server-managed-opensearch` means the API owns retrieval/model configuration; it is not a claim about the embedding model's identity or quality. Missing expected citations or processing versions are regressions even when prose looks plausible.

For an explicitly required historical comparison, `--backend legacy-postgres --postgres-dsn ...` retains the old `rag.chunks`/`rag.embeddings` path. Use an existing read-only local psql route and supply credentials privately. Legacy local hash embeddings are a regression baseline only; they cannot validate current OpenSearch semantic retrieval. The Make target no longer obtains CNPG credentials automatically.

Offline adapter checks (standard library only):

```bash
python3 -m unittest discover -s evals/ragas -p 'test_*.py'
```

These verify filters, authenticated request construction, citation versions and redirect refusal against a disposable local HTTP server. Live RAGAS scores and private gold cases require LAN authentication/data and are not claimed by those checks.
