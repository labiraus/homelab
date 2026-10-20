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

Each claimed document holds a PostgreSQL session advisory lock until its checkpoint is saved. Worker death releases ownership, allowing another worker to recover a retained `processing` row; active owners still exclude duplicate work. Losing that database session aborts pinned storage reads and index requests. Reads/indexing have a ten-minute bound, active messages send progress acknowledgements, and retries use a delay. Completed or superseded generations cannot be reset by late success/error writes, and OpenSearch `external_gte` versions prevent an older generation overwriting newer chunks. Drain and pause the document consumer during the first transition from workers without these locks; resume only after every consumer uses the new build.

`bash scripts/worldbuilder/test-document-claims.sh` proves concurrent exclusion, actual owning-session termination, abandoned-claim recovery, completed/superseded deduplication and late checkpoint refusal using an owned PostgreSQL fixture. CI requires this separately from ordinary unit tests. The live synthetic search gate also proves stale OpenSearch version refusal and an exact current-generation retry.

The worker publishes `documents.events.processor.started`, `documents.events.processor.completed`, and `documents.events.processor.failed` lifecycle notifications on a best-effort basis. After a notification publish succeeds, it also appends the event payload to `rag.document_lifecycle_events` and updates the quick summary columns on `rag.documents`.

OpenSearch stores nested chunk text and vectors. `OPENSEARCH_RAG_MODEL_ID` should identify an OpenSearch ML Commons model whose connector routes inference through Envoy AI Gateway.

Startup now requires that exact model to exist in `DEPLOYED` state before creating pipelines, starting document consumption or reporting readiness. A placeholder, unavailable or undeployed model keeps startup in its existing retry loop. This check establishes model registration/deployment only; the operator acceptance below must still prove actual inference and indexing.

The connector contract is OpenAI-compatible embeddings: model input `input`, with output at `data[0].embedding`. Ingest output maps target the destination chunk field; search inference uses root `ext.ml_inference.params`, outside `query`. Text first receives an `OPENSEARCH_CHUNK_CHAR_LIMIT` cap (default 480), then optional further word splitting before the 512-token BGE model. OpenSearch's word tokenizer cannot bound WordPiece tokens, and analyzing an entire long document can exceed its 10,000-token limit. The first Painless stage preserves UTF-16 surrogate pairs; OpenSearch's native character splitter can cut an emoji in half. The later native word stage disables excess-text merging into a final oversized chunk. Raw chunk strings are wrapped with Painless instead of Mustache, which escaped quotes, backslashes and newlines in the live processor. The provider must refuse truncation. Follow [document search recovery](../../docs/DocumentSearchRecovery.md) for actual inference, text integrity and indexed retrieval acceptance.

The current ingestion baseline is UTF-8 `text/*`, with HTML-aware extraction for `text/html`. OpenSearch-native chunking may initially carry less precise HTML heading metadata than the previous processor chunker until section metadata is mapped into the ingest pipeline.

Future file-type expansion must still follow `docs/FileTypeExpansion.md`: document extraction rules, failure handling, citation policy, tests, and runtime dependencies before changing the worker container.

Dependency versions and security checks are recorded in [Worldbuilder dependency validation](../../docs/worldbuilder/DEPENDENCIES.md). CI runs build/tests and rejects moderate/high/critical npm advisories before publishing.

The storage adapter uses the pinned AWS SDK v3 S3 client with path-style requests and explicit application credentials. Existing `MINIO_*` configuration names and the `DocumentStorage` interface remain compatible. `MINIO_REGION` defaults to `us-east-1`; configure it explicitly for stores using another signing region. `MINIO_ENDPOINT` accepts a bare host/port or an HTTP(S) origin matching `MINIO_USE_SSL`; embedded credentials, paths, queries and conflicting TLS settings are rejected. Reads preserve binary/empty content and S3 error codes; redirects do not forward credentials and truncated streams fail instead of indexing partial content. This adapter supports both the retained MinIO source and the SeaweedFS target; no public endpoint or environment-key rename accompanies the SDK change.

Worldbuilder requests can mark `contextScope=selected_records`. The graph explicitly treats this as partial canon, cannot claim global absence/consistency, and still accepts only supplied canonical citation IDs. External owns selection and validates returned scope/operations.

The gateway request supplies a workflow-specific JSON schema: required result fields, exact supplied citation IDs, no mutations for read-only workflows, and scoped optional story/findings fields. Independent response, byte-length, citation and canon-operation validation remains mandatory. This avoids the observed local model returning plausible prose in an incompatible object shape. The deployed vLLM interface is documented in [structured outputs](https://docs.vllm.ai/en/v0.20.2/examples/online_serving/structured_outputs/); actual route/schema/quality acceptance remains a LAN gate.

Worldbuilder correlation IDs supplied by external are forwarded to Envoy as `X-Request-ID`. Responses preserve the gateway-reported model when available. Durable per-actor allowance decisions belong to external before this endpoint; processor’s local burst/concurrency bounds remain an additional guard.

`narrative_generate` returns scenario prose without mutations. `story_turn` consumes a scoped private definition, bounded summary/recent-turn context and canonical evidence, and may propose bounded summaries/choices. Provisional memory cannot supply canonical citation IDs or expand permissions.

Durable generation checkpoints carry bounded previous provisional prose and part metadata through the same authenticated reasoning endpoint. Continuation text is untrusted context and cannot supply citations or publication authority. External owns private jobs, allowances and checkpoint persistence; processor remains a bounded per-part inference worker.

`consistency_check` may return bounded findings with severity, uncertainty and published citations. Proposal operations are separate untrusted review context. Strict response validation rejects unknown fields, forged citations and approval authority; external repeats evidence checks and owns deterministic validation.

Eight synthetic Worldbuilder evaluation cases run in `npm test` from `test/fixtures/worldbuilder-evaluation.json`; they exercise actual validator boundaries with scripted model output. [Evaluation guidance](../../evals/worldbuilder/README.md) separates these checks from live semantic quality and billing acceptance.

SeaweedFS compatibility is tested through the actual `createDocumentStorage` adapter in the disposable [storage acceptance harness](../../docs/StorageVersionMigration.md). Run `scripts/seaweedfs-smoke.py --application-clients` using the documented Python environment; it creates Go-written UTF-8 fixtures and checks Node reads, default-bucket selection, deleted objects and cross-bucket denial. The integration-only test is skipped by ordinary `npm test` and explicitly required by the CI storage gate before publication.

Canon candidate guidance preserves declared `tick` date facts as exact signed int64 decimal strings. External validates these against the pinned schema and life-bound rules; processor receives no calendar publication authority. See [calendar/date contracts](../../docs/worldbuilder/CALENDARS.md).


The default local chat model is Qwen2.5-3B-Instruct-AWQ through Envoy AI Gateway. Canon requests use a workflow-specific JSON schema and an explicit unknown response contract; independent validation still refuses unsupported citations, inconsistent grounding and forbidden operations. See [live gateway acceptance](../../docs/EnvoyAIGateway.md#live-model-acceptance--2026-10-11) for the measured scope and limits.

Factual abstention is scoped to factual queries. Draft edits, advisory checks and provisional stories have separate instructions, with deterministic factual/draft sampling and limited creative sampling. Model input names scoped evidence IDs `citationId` separately from `record.id`. Generated puts carry only the record identity; the adapter derives the existing outer operation ID from it and rejects any explicitly conflicting IDs. Publication remains exclusively external's validated human workflow.

Document workers pass the queued S3 version and ETag to storage. Reads use `VersionId` and `If-Match`, preserving opaque IDs and literal `null` while refusing changed content. They never retry a failed pinned read as a latest-object read. Legacy events without these fields retain their previous behavior; before storage cutover, reconcile current inventory and drain source work so destination jobs carry destination-native current IDs.

Worldbuilder schema-9 context includes reviewed world/style guides, immutable established accounts and explicit story mode/scene. The prompt distinguishes witnessed synopses from reported histories and prioritizes the effective successor at the supplied revision. The external service validates/cites evidence and enforces immutability; prompt compliance is not publication authority or a guarantee of semantic contradiction detection.

## Independent inference

With the explicit inference activation overlay, chat uses the independent private policy proxy and `INFERENCE_TOKEN`. Trusted Worldbuilder requests carry the profile/revision pinned by external; processor forwards these and retains returned accounting provenance. The two-call concurrency limit remains; the former 20/minute processor quota is removed. Cloud structured output is normalized by the proxy, then independently validated here. Embedding configuration remains unchanged. See [operator procedures](../../docs/Inference.md).

The proposal prompt/schema explicitly retain baseline labels and nested attributes. The live quality fixture now uses the actual Go source-record contract (`attributes.description`); top-level descriptions are invalid canon records. The complete corrected eleven-case suite passed through the candidate proxy against Helheim on 19 October 2026, including proposal preservation and concurrent world isolation. Earlier failed reports remain diagnostic history; deployed proxy/cache and each paid model still require their own acceptance.
