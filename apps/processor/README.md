# processor

Both container stages consume `package-lock.json` with `npm ci`; the runtime stage omits development dependencies. Local dependencies, generated output and environment files are excluded from the Docker context, keeping the published dependency tree aligned with CI tests.

Before chart publication, CI runs `test/image-smoke.mjs` inside the published image with networking disabled. It compares the image lockfile hash to source, validates installed production package versions/dependency health, imports compiled storage/reasoning code, and checks exact date parsing on the actual image runtime.

Worldbuilder evidence and model candidates preserve original JSON date tokens. Large signed ticks stay decimal strings in prompts/results instead of passing through rounded JavaScript numbers; external validates them and remains the sole canon writer. See the [date wire contract](../../docs/worldbuilder/API.md).

Worldbuilder LangGraph workflows live in `src/worldbuilder.ts`: canon query/proposal, advisory consistency check and story-turn candidates. `/worldbuilder/generate` requires a private `WORLDBUILDER_AI_TOKEN` shared only with external. Evidence is supplied by external; processor has no canon DB access or publication tools. Inference uses only `AI_GATEWAY_BASE_URL`, caps context/response bytes, 768 output tokens, 20 requests/minute and two concurrent calls per process. These are local limits, not a provider billing guarantee. Model usage is returned for external audit. No token means this endpoint is disabled. Existing document/assistant APIs below remain separate.

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

The worker publishes `documents.events.processor.started`, `documents.events.processor.completed`, and `documents.events.processor.failed` lifecycle notifications on a best-effort basis. After a notification publish succeeds, it also appends the event payload to `rag.document_lifecycle_events` and updates the quick summary columns on `rag.documents`.

OpenSearch stores nested chunk text and vectors. `OPENSEARCH_RAG_MODEL_ID` should identify an OpenSearch ML Commons model whose connector routes inference through Envoy AI Gateway.

The current ingestion baseline is UTF-8 `text/*`, with HTML-aware extraction for `text/html`. OpenSearch-native chunking may initially carry less precise HTML heading metadata than the previous processor chunker until section metadata is mapped into the ingest pipeline.

Future file-type expansion must still follow `docs/FileTypeExpansion.md`: document extraction rules, failure handling, citation policy, tests, and runtime dependencies before changing the worker container.

Dependency versions and security checks are recorded in [Worldbuilder dependency validation](../../docs/worldbuilder/DEPENDENCIES.md). CI runs build/tests and rejects moderate/high/critical npm advisories before publishing.

The storage adapter uses the pinned AWS SDK v3 S3 client with path-style requests and explicit application credentials. Existing `MINIO_*` configuration names and the `DocumentStorage` interface remain compatible. `MINIO_REGION` defaults to `us-east-1`; configure it explicitly for stores using another signing region. `MINIO_ENDPOINT` accepts a bare host/port or an HTTP(S) origin matching `MINIO_USE_SSL`; embedded credentials, paths, queries and conflicting TLS settings are rejected. Reads preserve binary/empty content and S3 error codes; redirects do not forward credentials and truncated streams fail instead of indexing partial content. This adapter supports both the retained MinIO source and the SeaweedFS target; no public endpoint or environment-key rename accompanies the SDK change.

Worldbuilder requests can mark `contextScope=selected_records`. The graph explicitly treats this as partial canon, cannot claim global absence/consistency, and still accepts only supplied canonical citation IDs. External owns selection and validates returned scope/operations.

Worldbuilder correlation IDs supplied by external are forwarded to Envoy as `X-Request-ID`. Responses preserve the gateway-reported model when available. Durable per-actor allowance decisions belong to external before this endpoint; processor’s local burst/concurrency bounds remain an additional guard.

`narrative_generate` returns scenario prose without mutations. `story_turn` consumes a scoped private definition, bounded summary/recent-turn context and canonical evidence, and may propose bounded summaries/choices. Provisional memory cannot supply canonical citation IDs or expand permissions.

Durable generation checkpoints carry bounded previous provisional prose and part metadata through the same authenticated reasoning endpoint. Continuation text is untrusted context and cannot supply citations or publication authority. External owns private jobs, allowances and checkpoint persistence; processor remains a bounded per-part inference worker.

`consistency_check` may return bounded findings with severity, uncertainty and published citations. Proposal operations are separate untrusted review context. Strict response validation rejects unknown fields, forged citations and approval authority; external repeats evidence checks and owns deterministic validation.

Eight synthetic Worldbuilder evaluation cases run in `npm test` from `test/fixtures/worldbuilder-evaluation.json`; they exercise actual validator boundaries with scripted model output. [Evaluation guidance](../../evals/worldbuilder/README.md) separates these checks from live semantic quality and billing acceptance.

SeaweedFS compatibility is tested through the actual `createDocumentStorage` adapter in the disposable [storage acceptance harness](../../docs/StorageVersionMigration.md). Run `scripts/seaweedfs-smoke.py --application-clients` using the documented Python environment; it creates Go-written UTF-8 fixtures and checks Node reads, default-bucket selection, deleted objects and cross-bucket denial. The integration-only test is skipped by ordinary `npm test` and explicitly required by the CI storage gate before publication.

Canon candidate guidance preserves declared `tick` date facts as exact signed int64 decimal strings. External validates these against the pinned schema and life-bound rules; processor receives no calendar publication authority. See [calendar/date contracts](../../docs/worldbuilder/CALENDARS.md).
