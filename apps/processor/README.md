# processor

Both container stages consume `package-lock.json` with `npm ci`; the runtime stage omits development dependencies. Local dependencies, generated output and environment files are excluded from the Docker context, keeping the published dependency tree aligned with CI tests.

Before chart publication, CI runs `test/image-smoke.mjs` inside the published image with networking disabled. It compares the image lockfile hash to source, validates installed production package versions/dependency health, imports compiled storage/reasoning code, and checks exact date parsing on the actual image runtime.

Worldbuilder LangGraph workflows live in `src/worldbuilder.ts`: canon query/proposal, advisory consistency check, story-turn candidates and narrative generation. `/worldbuilder/generate` requires a private `WORLDBUILDER_AI_TOKEN` shared only with external. Evidence is supplied by external; processor has no canon DB access or publication tools. Inference uses `AI_GATEWAY_BASE_URL`, caps context/response bytes and 768 output tokens, and permits two concurrent calls per process. The current handler has no minute quota. Proxy mode requires pinned profile/revision plus `INFERENCE_TOKEN`; external and the proxy enforce their separate durable allowances. These are local limits, not a provider billing guarantee. Model usage is returned for external audit. No token means this endpoint is disabled. Existing document/assistant APIs below remain separate.

`processor` is the internal TypeScript service for processor-owned assistant workflows and document-processing jobs.

For synchronous browser assistant requests, it exposes internal `/assistant/...` routes called by `external`. It owns conversation state, explicit memories, proposal state, prompt construction, RAG context lookup through `external`, LangGraph-based chat orchestration, model calls through Envoy AI Gateway, and tool-call logging in Postgres.

For asynchronous document jobs, it consumes NATS JetStream events, claims pending document work from Postgres, fetches S3-backed text documents, extracts text, and indexes derived retrieval state into OpenSearch.

`processor` is part of the AI/data plane. It should not own global document lifecycle state; that remains the job of `orchestrator` and Postgres-backed control-plane records.

The current lifecycle for a claimed job is:

1. claim `pending -> processing`
2. load the referenced supported object through the S3 storage adapter
3. extract chunkable text from the raw object, including HTML-specific visible-text extraction for `text/html`
4. index the extracted text into OpenSearch through the configured ingest pipeline
5. finalize the owning `rag.documents` row as `processed`

If work fails after claim, the worker records `last_error`, returns the document to `pending`, and lets JetStream retry the message.

Each claimed document holds a PostgreSQL session advisory lock until its checkpoint is saved. Worker death releases ownership, allowing another worker to recover a retained `processing` row; active owners still exclude duplicate work. Losing that database session aborts pinned storage reads and index requests. Reads/indexing have a ten-minute bound, active messages send progress acknowledgements, and retries use a delay. Completed or superseded generations cannot be reset by late success/error writes, and OpenSearch `external_gte` versions prevent an older generation overwriting newer chunks. Drain and pause the document consumer during the first transition from workers without these locks; resume only after every consumer uses the new build.

`bash scripts/worldbuilder/test-document-claims.sh` proves concurrent exclusion, actual owning-session termination, abandoned-claim recovery, completed/superseded deduplication and late checkpoint refusal using an owned PostgreSQL fixture. CI requires this separately from ordinary unit tests. The live synthetic search gate also proves stale OpenSearch version refusal and an exact current-generation retry.

The worker publishes `documents.events.processor.started`, `documents.events.processor.completed`, and `documents.events.processor.failed` lifecycle notifications on a best-effort basis. After a notification publish succeeds, it also appends the event payload to `rag.document_lifecycle_events` and updates the quick summary columns on `rag.documents`.

OpenSearch stores nested chunk text and vectors. `OPENSEARCH_RAG_MODEL_ID` should identify an OpenSearch ML Commons model whose connector routes inference through Envoy AI Gateway.

Startup now requires that exact model to exist in `DEPLOYED` state before creating pipelines, starting document consumption or reporting readiness. A placeholder, unavailable or undeployed model keeps startup in its existing retry loop. This check establishes model registration/deployment only; the operator acceptance below must still prove actual inference and indexing.

The connector accepts OpenAI-compatible embeddings (`input` → `data[0].embedding`). `OPENSEARCH_CHUNK_CHAR_LIMIT` defaults to 480 before native word splitting. The Unicode-safe Painless stage preserves surrogate pairs; the word stage disables excess-text merging and Painless wrapping preserves raw strings. Provider truncation is refused. See [document retrieval](../../docs/documents.md#retrieval-and-embeddings) for connector registration, batch bounds, text integrity and actual indexing acceptance.

The current ingestion baseline is UTF-8 `text/*`, with HTML-aware extraction for `text/html`. OpenSearch-native chunking may initially carry less precise HTML heading metadata than the previous processor chunker until section metadata is mapped into the ingest pipeline.

Future file-type expansion must still follow `docs/documents.md`: document extraction rules, failure handling, citation policy, tests, and runtime dependencies before changing the worker container.

Dependency versions and security checks are recorded in [Worldbuilder dependency validation](../../docs/testing.md). CI runs build/tests and rejects moderate/high/critical npm advisories before publishing.

The storage adapter uses the pinned AWS SDK v3 S3 client with path-style requests and explicit application credentials. Existing `MINIO_*` configuration names and the `DocumentStorage` interface remain compatible. `MINIO_REGION` defaults to `us-east-1`; configure it explicitly for stores using another signing region. `MINIO_ENDPOINT` accepts a bare host/port or an HTTP(S) origin matching `MINIO_USE_SSL`; embedded credentials, paths, queries and conflicting TLS settings are rejected. Reads preserve binary/empty content and S3 error codes; redirects do not forward credentials and truncated streams fail instead of indexing partial content. This adapter supports both the retained MinIO source and the SeaweedFS target; no public endpoint or environment-key rename accompanies the SDK change.

Eight synthetic Worldbuilder evaluation cases run in `npm test` from `test/fixtures/worldbuilder-evaluation.json`; they exercise actual validator boundaries with scripted model output. [Evaluation guidance](../../evals/worldbuilder/README.md) separates these checks from live semantic quality and billing acceptance.

SeaweedFS compatibility is tested through the actual `createDocumentStorage` adapter in the disposable [storage acceptance harness](../../docs/storage.md). Run `scripts/seaweedfs-smoke.py --application-clients` using the documented Python environment; it creates Go-written UTF-8 fixtures and checks Node reads, default-bucket selection, deleted objects and cross-bucket denial. The integration-only test is skipped by ordinary `npm test` and explicitly required by the CI storage gate before publication.

Storage reads preserve queued `VersionId` and `If-Match`, including literal `null`, without falling back to latest. Legacy unpinned events retain compatibility. Drain source jobs and reconcile destination-native IDs at [storage cutover](../../docs/storage.md).

## Independent inference

With the explicit inference activation overlay, chat uses the independent private policy proxy and `INFERENCE_TOKEN`. Trusted Worldbuilder requests carry the profile/revision pinned by external; processor forwards these and retains returned accounting provenance. The two-call concurrency limit remains; no processor daily/minute quota is added in proxy mode. Cloud structured output is normalized by the proxy, then independently validated here. Embedding configuration remains unchanged. See [operator procedures](../../docs/inference.md).


## Local checks and contracts

Run `npm ci`, `npm test` and `npm run build` from this directory. The actual-client
storage gate requires the integration-only test instead of accepting its ordinary
unit-test skip. Keep inference profile/accounting validation independent of the
proxy's structured-output checks.

[Worldbuilder API](../../docs/worldbuilder/api.md) owns exact int64 wire handling,
workflow/citation validation, account/guide context, request pins and private story
semantics. [Documents](../../docs/documents.md) owns extraction and retrieval policy.
Processor receives scoped evidence and produces candidates; it cannot publish canon.
