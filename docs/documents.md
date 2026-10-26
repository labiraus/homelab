# Document ingestion and retrieval

The retained document platform operates alongside Worldbuilder. Raw bytes live in
private S3; PostgreSQL owns inventory, lifecycle and audit; OpenSearch is derived.
SeaweedFS is the target and MinIO remains a compatibility/migration source. Document
jobs never publish Worldbuilder canon. The [service READMEs](README.md#component-references)
own exact endpoint and environment inventories.

## Workflow ownership

1. `orchestrator` reconciles bucket/prefix objects into `rag.documents`, including
   unsupported files. Inventory retains bucket/key, source URI, ETag/version,
   content type/size/time, metadata and desired/completed processing generations.
2. New or changed supported objects are queued through JetStream. Request-driven
   writes publish before committing the pending-row transaction; publication
   failure rolls back that write. Workers tolerate a message preceding commit.
3. `processor` claims a generation, reads pinned bytes, extracts text, indexes
   through the configured OpenSearch pipeline and checkpoints completion.
4. Durable lifecycle rows record queued/started/completed/failed states. NATS
   notifications, MCP resource updates and browser SSE are best-effort; their
   failure cannot roll back durable state.

The external API proxies document mutations to orchestrator. Inventory/history
remain readable without a matching search chunk. Curation changes metadata only;
text editing preserves inventory identity, requires explicit overwrite intent,
writes S3 and queues a new generation. Reprocess uses the next generation unless
explicitly supplied. A successful scan/queue response is not completed ingestion.

`documents.events.minio.stored` and `MINIO_*` names are compatibility interfaces;
they do not select the target architecture. Keep existing MCP names stable.

## Version and worker ownership

An S3 version is distinct from a derived `processingVersion`. Scan via provider
version listings and `IsLatest`; retain literal `null` IDs and exclude deleted or
historical entries. Apply scan limits after filtering. Legacy document IDs need
not be `s3://` URIs: resolve bucket/key from inventory, not a stripped ID prefix.

Changed source identities advance beyond both desired and completed generations;
unchanged scans preserve those generations. Queued reads retain `VersionId` and
`If-Match`, refusing changed bytes without a newest-version fallback. Legacy events
without pins retain compatibility behavior. Immutable audit markers are not rewritten.

The worker holds a PostgreSQL session lock through checkpointing and cancels work
when ownership is lost. Generation fencing protects both OpenSearch and PostgreSQL
against stale writes. Inspect current ownership before reprocessing stuck work;
do not clear claims merely because a Pod changed. Drain source work at storage
cutover and reconcile destination-native current IDs through normal inventory flow.

## Extraction and citations

The supported policy is `text/*`. Plain text is decoded as UTF-8; `text/html`
uses the implemented extractor in `apps/processor/src/extractor.ts`. Scans may
infer a supported content type from extension. HTML retains title, heading path,
visible/link text and meaningful alt text while removing scripts, styles and
common navigation/layout containers. Nonempty partial HTML can succeed with
extraction warnings in lifecycle evidence; no readable text fails processing.

| Inventory state | Meaning |
| --- | --- |
| `unsupported` | Reconciled but outside the supported type policy; source identity and lifecycle metadata remain available. |
| `pending` | Supported and queued or waiting for a generation claim. |
| `processing` | A worker claimed that generation. |
| `processed` | Extraction, indexing and state updates completed for the current generation. |
| `failed` | Attempted extraction/processing failed; inspect the recorded error. |

After a claimed job fails, the current worker records `last_error`, returns the
row to `pending` and lets JetStream retry. A failed lifecycle event therefore does
not by itself mean inventory remains in a terminal `failed` state.

Citations retain `sourceUri`, `objectKey`, `chunkIndex`, `processingVersion` and
stable `sourceUri#chunk-N` IDs. HTML adds title/heading context. Future types must
add intrinsic locations without making existing text clients depend on new fields.

### Future file types

PDF is a separate, unimplemented expansion. Choose a parser with page boundaries
and include page ranges in citations. Scanned PDFs remain unsupported or explicitly
fail as OCR-required until OCR is deliberately introduced. Document CPU, memory,
image size and startup effects before adding OCR, Tesseract, Poppler or other
heavyweight packages.

Office formats (`.docx`, `.pptx`, `.xlsx`) follow later. Prefer narrow zipped-XML
extraction with a reviewed security posture; preserve heading, paragraph, slide
or worksheet context. Macro-enabled files stay unsupported without an explicit
sandboxing policy.

Keep extractors limited to bytes/content type in, text/location spans and warnings
out. They do not write PostgreSQL or own lifecycle. Orchestrator decides queueable
types; processor rejects jobs outside its policy. Before adding a parser, document
MIME/extension support, resource cost and failure behavior; test success, unsupported,
partial/failure and citation metadata. Update this guide and the processor README.
Run the private RAGAS gold set when chunking, ranking, embeddings or citations change.

## Retrieval and embeddings

Document search/context uses the configured OpenSearch derived index; canon search
has a separate authorized PostgreSQL reconciliation boundary. The shared query builder
currently emits lexical matching; any configured search-pipeline transformation
needs separate observed acceptance before claiming neural retrieval. `embeddingutil.EmbedText`
is token hashing, not a model inference fallback.

The private CPU embedding route pins TEI CPU 1.9 and BAAI/bge-large-en-v1.5 for
1,024-dimensional normalized vectors. Only the AI gateway may reach it. Its model
cache is reproducible data, not document or database authority. Keep automatic
truncation disabled and test maximum eight-input batches: one vector fitting an
Envoy buffer does not establish batch acceptance. The private gateway's configured
256 KiB buffer addresses the observed default-32-KiB response limit.

Before ingestion, verify actual gateway prediction, register the OpenSearch ML
connector/model, simulate the real pipeline and index/query an owned synthetic
document. Configure `OPENSEARCH_RAG_MODEL_ID` only from that verified registration.
A new OpenSearch cluster needs fresh registration, not a copied historical model ID.

Registration preserves existing trusted endpoint lists and adds only the escaped,
anchored private URL to `plugins.ml_commons.trusted_connector_endpoints_regex` and
`plugins.ml_commons.trusted_connector_private_endpoints_regex`, with private-IP
support explicitly enabled. The scalar connector request is
`{"input":"${parameters.input}","model":"${parameters.model}","encoding_format":"float"}`.
Verify quotes, backslashes and newlines with real simulation. ML Commons requires a
nonempty credential map even for this private unauthenticated route; the documented
`{"local":"unused-private-runtime-placeholder"}` is inert, never transmitted and
confers no authority. Undeploy models before changing their connector actions,
then redeploy and retest.

The processor bounds text with a Unicode-safe Painless script before native word
splitting. Whole-document analysis can exceed the 10,000-token limit, and native
fixed-character splitting can divide UTF-16 surrogate pairs. Require each chunk
to be an exact source substring and preserve coverage through the final character,
including long documents and emoji. Boundary whitespace normalization is separate
from raw-byte/hash comparison; empty source files remain empty indexed documents.

## Evaluation and recovery checks

- `python3 scripts/validation/embedding-endpoint.py --base-url http://127.0.0.1:PORT/v1`
  checks an explicitly owned runtime/gateway forward using synthetic inputs.
- `python3 scripts/validation/document-search.py --base-url http://127.0.0.1:PORT --model-id ID`
  creates owned synthetic OpenSearch resources and validates actual processor/Go
  payloads, prediction, coverage and scoped search, then removes its resources.
  `--chunking-only` does not prove model inference or retrieval.
- `make document-platform-checks` supplies operator diagnostics; inspect whether
  a command touches the estate before running it.
- [RAGAS evaluation](../evals/ragas/README.md) uses authenticated external retrieval
  by default. Legacy pgvector comparison is explicit and does not become authority.
  Run the private gold set when chunking, embeddings, ranking or citations change.

After a rebuild, compare actual source versions/bytes, inventory generations,
chunk coverage and finite normalized vectors. Preserve original lifecycle/audit
fingerprints. Healthy Pods, empty model registries and successful scan responses
are insufficient recovery evidence. [Status](status.md) retains the dated source
rebuild evidence; [storage](storage.md) defines final migration coverage.

## Retained document assistant

The document assistant remains separate from Worldbuilder conversations and canon.
External forwards authenticated browser requests under `/api/assistant/...` to
processor's private `/assistant/...` API. Processor stores conversations, messages,
tool calls, explicit user-approved memories and file-change proposals in the
`assistant` schema, and grounds chat through external's document-context routes.
Model tools are read-only. Memories and raw sources never become canonical facts
without ordinary Worldbuilder proposals and publication.

Write-like outcomes persist as proposals. Explicit browser approval calls
orchestrator create/edit operations; the model cannot write raw files directly.
`rag.document_change_audits` binds actor, conversation/proposal, bucket/key, source
version and processing generation. An approved revert reads a verified earlier
S3 version and writes a new current version before reingestion, preserving history.
Retain scoped audit visibility, explicit memory approval and proposal-before-write
behavior when improving answer quality. [Troubleshooting](../runbooks/troubleshooting.md#assistant-proposal-recovery)
explains failed approval and reprocess diagnosis.

## Remaining product requirements

PDF/OCR and Office extraction require the explicit policies above before activation.
Source-document import into typed canon must generate ordinary reviewed proposals;
it must not turn ingestion into another canon writer. Broader retrieval quality
and realistic corpus performance remain separate from structural fixture acceptance.
