# File-Type Expansion Policy

The current stable ingestion baseline is UTF-8 `text/*`, with HTML-aware extraction enabled for `text/html`.

Non-text objects discovered during bucket reconciliation must remain visible in `rag.documents` with status `unsupported` until an extraction policy exists for that type. Do not hide unsupported files from inventory, and do not queue them into `processor` just because a parser package exists.

## Ownership

- MinIO remains the canonical raw object store for every file type.
- `orchestrator` owns reconciliation, support decisions, processing-version selection, queueing, and lifecycle history.
- `processor` owns extraction and OpenSearch indexing after `orchestrator` queues supported work.
- Postgres remains the source of truth for inventory rows and lifecycle events; OpenSearch owns derived chunks, embeddings, and retrieval metadata.

## Support States

Use these meanings consistently:

- `unsupported`: the object was reconciled, but its content type or extension is outside the current extraction policy.
- `pending`: the object is supported and queued or waiting to be claimed for a specific processing version.
- `processing`: a worker has claimed the queued processing version.
- `processed`: extraction, OpenSearch indexing, and document state updates completed for the current processing version.
- `failed`: extraction or processing was attempted and failed in a way that should be visible to operators.

Unsupported objects should keep bucket, object key, source URI, content type, size, last modified time, ETag/version marker, reconciliation metadata, and latest lifecycle summary fields.

## Supported HTML Policy

### HTML

- `text/html` and `.html`/`.htm` are now the first implemented expansion after plain text.
- Extract visible text, title, headings, link text, and meaningful alt text.
- Remove scripts, styles, navigation boilerplate, and repeated layout chrome where practical.
- Preserve source URI, object key, processing version, and section-ish anchors in OpenSearch chunk metadata so retrieval surfaces can render richer citation labels.
- Failed parsing should mark the processing version `failed`; malformed but partially readable HTML can be `processed` only when the extracted text is non-empty and the lifecycle payload records the extraction warning.

## Candidate Type Policies

### PDF

- Treat `application/pdf` and `.pdf` as a separate expansion because page numbers matter for citations.
- Choose a parser that can return text with page boundaries before enabling PDF support.
- Citation labels should include page ranges when available, for example `source.pdf p. 3 chunk 0`.
- Scanned-image PDFs should either remain `unsupported` or become `failed` with an explicit OCR-required error until OCR is intentionally added.
- Do not add OCR runtimes, Tesseract data, Poppler, or heavyweight system packages without documenting image size and CPU/memory impact first.

### Office Documents

- Treat `.docx`, `.pptx`, and `.xlsx` as later work, not the first expansion.
- Prefer extracting text and document structure from the zipped XML formats directly or through a narrow dependency with clear security posture.
- Track worksheet, slide, or heading context in chunk metadata once chunk metadata exists.
- Macro-enabled formats should stay `unsupported` unless there is a deliberate sandboxing policy.

## Citation Rules

Every new extractor must preserve the current citation baseline:

- `sourceUri`
- `objectKey`
- `chunkIndex`
- `processingVersion`
- stable citation ID in the form `sourceUri#chunk-N`

When a type has intrinsic locations, add them to chunk metadata and rendered labels:

- PDF: page or page range
- HTML: title, heading path, or fragment-like section
- Office: slide number, worksheet name, heading path, or paragraph index

Do not change existing search/context clients to depend on type-specific metadata until older text chunks still render correctly without those fields.

## Runtime Dependency Checklist

Before adding a parser to `processor`:

- document supported MIME types and extensions
- document extraction failure modes and operator-facing error messages
- document CPU, memory, image-size, and startup impact
- add tests for success, unsupported, partial/failure, and citation metadata behavior
- update `apps/processor/README.md`, `docs/async-ingestion.md`, `docs/RAG.md`, and this policy
- run `make ragas-chunking-eval` against the private gold set if chunking, ranking, embeddings, or citation fields change

## Initial Implementation Shape

When implementation begins, prefer a small extractor boundary inside `processor`:

- input: bucket, object key, content type, raw bytes or stream
- output: extracted text plus optional source-location spans
- no global lifecycle ownership inside the extractor
- no direct Postgres writes from extractor code

`orchestrator` should continue to decide whether a discovered object is supportable and queueable. `processor` should still fail loudly if it receives a job whose content type no longer matches the extractor policy.
