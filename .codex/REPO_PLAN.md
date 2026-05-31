# Repo Plan For Codex — Homelab Async Document Platform

Use this file as the working objective-state document for the repository.

`REPO_MAP.md` explains where things are.
This file explains the architecture the repo is converging on, what is active now, and which gaps should drive future changes.

## Executive Summary

This repo is no longer planning around separate ingestion and query applications for RAG.
The chosen application boundary is:

- `ui`: React frontend
- `external`: public Go API for browser clients
- `assistant`: browser-first LLM chat, user memory, proposal, and audit service
- `mcp`: Go MCP server for AI-native access
- `orchestrator`: internal Go control-plane API
- `processor`: TypeScript NATS JetStream worker for document operations

The intended platform shape is:

- MinIO on `svartalfheim` remains the canonical raw object store
- Postgres via CNPG plus pgvector is the system of record for metadata, workflow state, chunks, and embeddings
- NATS JetStream plus KEDA provide asynchronous execution and worker scaling
- Redis is available but is not yet a core architectural dependency
- local vLLM is the first LLM runtime target, scheduled onto the GPU worker path and fronted by Envoy AI Gateway resources
- Mongo is not part of the active plan
- browser authentication currently standardizes on `oauth2-proxy + Google` on the shared public host
- certificate authentication is already part of the Labiraus MCP access story and currently sits alongside the shared Google-backed path as the all-or-nothing access choice for deployed MCP clients
- the Labiraus MCP server targets native client compatibility across Streamable HTTP clients and older HTTP+SSE clients while keeping `/mcp` as the primary endpoint

## Current Repo Reality

The repo already includes:

- Flux plus Helm as the delivery path for cluster workloads
- Envoy AI Gateway infra chart under `helm/infra/envoy-ai-gateway/` for future AI traffic routing
- app charts under `helm/apps/`
- data/platform charts under `helm/data/` and `helm/bootstrap/`
- the `ui`, `external`, `assistant`, `mcp`, `orchestrator`, and `processor` apps under `apps/`
- SQL bootstrap under `sql/`
- external MinIO management through Ansible on `svartalfheim`
- orchestrator-backed `documents.scanBucket` reconciliation that inventories MinIO objects into Postgres and queues new or changed text objects for processing
- browser uploads emit `documents.events.minio.stored` after raw objects are written to MinIO
- a built-in deterministic `local-embeddings` fallback used by processor, `external`, and `mcp` when no external embedding endpoint is configured
- durable document lifecycle history in `rag.document_lifecycle_events`, exposed through `external` and `mcp`
- the MCP prompt catalog includes prompt-first guidance for ingestion, scan planning, inventory reads, metadata curation, guarded text editing, reprocessing, retrieval, context assembly, lifecycle history, auth health, MinIO browsing, and document notification subscriptions, with prompt arguments aligned to the live tool schemas for common filters and control fields, omitted optional arguments rendered without leaking template placeholders, and unknown prompt argument names rejected
- `mcp` advertises strict top-level tool input schemas and rejects unknown top-level, missing required, and wrong-typed advertised `tools/call` arguments before execution so misspelled filters, malformed limits, or incomplete calls do not silently fall back to broader default reads
- browser-facing document inventory reads through `/api/documents/inventory`, backed by `rag.documents`
- browser-facing bucket reconciliation through `/api/documents/scan-bucket`, proxied to `orchestrator`
- the UI Search tab can load that durable lifecycle history for a retrieved document through `/api/documents/history`
- the UI Search tab can assemble citation-backed context blocks through `/api/documents/context`
- the UI Inventory tab can ask `orchestrator` to reconcile the current prefix filter and refresh inventory rows
- the UI Inventory tab can inspect document status, versions, latest lifecycle summary fields, and metadata without needing a matching retrieval chunk
- the UI Inventory tab can curate metadata for inventory rows through `external` proxy routes backed by `orchestrator`
- the UI Inventory tab can perform guarded raw text edits for text inventory rows through `external` proxying to `orchestrator`
- the UI Inventory tab can queue reprocessing for text inventory rows and immediately load the queued version's lifecycle history
- the UI Search tab can update curated metadata and queue reprocessing for a retrieved document through `external` proxy routes backed by `orchestrator`
- the UI Search tab can perform guarded raw text edits for selected text documents through `external` proxying to `orchestrator`
- browser document actions that queue processing automatically load durable lifecycle history for the returned processing version
- the `assistant` app stores conversations, messages, per-user approved memories, tool calls, file proposals, and proposal decisions in Postgres
- the Assistant browser tab can ask RAG-backed questions, save approved user memories, stage create/edit proposals, approve or reject proposals, and inspect audit rows
- approved assistant create/edit proposals call `orchestrator`, write text objects to MinIO, queue reingestion, and append `rag.document_change_audits`
- `orchestrator` exposes text create and MinIO-version-backed revert endpoints in addition to guarded text editing
- Ansible bucket management enables versioning on the external `documents` bucket while leaving current-object lifecycle expiration disabled
- RAGAS-based chunking evaluation lives under `evals/ragas` and can score live processed chunks against gold retrieval cases through Postgres

Documentation must stay aligned with implementation as these pieces evolve.

## Chosen Architecture

## App Boundaries

- `ui` is the browser-facing React application.
- `external` is the stable public API for the UI. It serves Postgres-backed data and triggers orchestrator actions without exposing internal pipeline details.
- `assistant` is the browser-first LLM and memory service. It owns conversation state, explicit per-user memories, read-only MCP tool-call allowlisting, file-change proposals, approval state, and assistant audit views.
- `mcp` is the stable AI-native front door for agents and MCP-compatible clients.
- the Labiraus MCP server is a primary product surface of this repo, not a sidecar utility, and should be evaluated alongside the rest of the deployed stack
- `orchestrator` is the internal control plane. It owns workflow state transitions, reconciliation, and task dispatch decisions.
- `processor` is the stateless data-plane worker. It performs extraction, chunking, embedding, and persistence work when triggered asynchronously.
- `vllm` is the local OpenAI-compatible model runtime, initially scheduled onto `helheim` through `node-llm=gpu` and one `nvidia.com/gpu`.

## Data And Control Model

- MinIO is the canonical source for raw objects and source documents.
- Postgres is the source of truth for document inventory, workflow state, metadata, chunks, and embeddings.
- NATS JetStream is transport and execution infrastructure, not the source of truth.
- KEDA scales workers from JetStream consumer lag and should remain an execution concern rather than a state-management concern.

## Architectural Principles

- Prefer async-first workflows.
- Prefer controller/reconciliation plus state-machine thinking over request-coupled pipelines.
- Keep public and AI-facing APIs stable while hiding ingestion and processing internals behind them.
- Keep raw storage in MinIO and derived state in Postgres.
- Treat reprocessing and versioning as first-class future concerns even when early implementations stay small.
- keep browser login at the edge and avoid embedding provider-specific browser OAuth logic directly into app services
- keep write-like LLM outcomes as durable proposals until the authenticated user approves them in the browser
- keep CAG memory explicit, user-approved, and scoped by authenticated email rather than inferred globally

## Service Roles

### `orchestrator`

`orchestrator` is the control plane.

It should:

- reconcile MinIO object inventory into Postgres
- decide what requires processing or reprocessing
- own durable workflow transitions
- enqueue asynchronous work on NATS JetStream when needed

It should not:

- do chunking or embedding itself
- treat NATS JetStream as the authoritative state store
- leak internal pipeline mechanics into the public API surface

### `processor`

`processor` is the stateless data-plane worker.

It should:

- consume NATS JetStream jobs
- fetch or receive document content for processing
- perform extraction, chunking, and embedding
- write results back to Postgres

It should not:

- own global document lifecycle state
- become the system-of-record for reconciliation
- replace orchestrator as the workflow owner

## Phased Plan

### Phase 1 — Document Inventory And Reconciliation Schema

Deliverables:

- establish a dedicated Postgres schema namespace for document/control-plane state
- add a `documents` table suitable for MinIO reconciliation
- include fields for bucket, object key, etag or version marker, size, last modified time, status, and timestamps
- make idempotent upsert and reconciliation practical
- leave explicit room for future processing-version and reprocessing tracking

### Phase 2 — Orchestrator-Triggered MinIO Scan And Document Upsert

Deliverables:

- orchestrator gains the first reconciliation flow
- MinIO object inventory is scanned through the existing external MinIO boundary
- document metadata and lifecycle state are upserted into Postgres
- no heavy processing logic is moved into orchestrator

### Phase 3 — NATS JetStream Job Contracts And Processor Execution

Deliverables:

- define NATS JetStream job contracts owned by the orchestrator/processor boundary
- orchestrator emits processing work based on Postgres-backed state
- processor consumes jobs and performs extraction, chunking, embedding, and persistence
- processing results are written back to Postgres, not treated as JetStream-owned state
- define and emit document lifecycle notification subjects so MCP and the browser-facing API can forward NATS-backed updates to subscribers

### Phase 4 — Retrieval Through `external` And `mcp`

Deliverables:

- `external` exposes stable retrieval/query capabilities for the UI
- `mcp` exposes the same capabilities in an AI-native shape for agents
- both surfaces read from Postgres-backed state and hide pipeline internals
- `mcp` publishes prompt guidance for the current capability surface
- `mcp` maintains a subscription pattern for document lifecycle notifications sourced from NATS JetStream

Current status:

- `external` exposes `/api/documents/search` for UI retrieval
- `mcp` exposes `documents.inventory.list` and `documents.search` for agent-facing inventory and semantic retrieval
- search responses now include citation objects that identify source URI and chunk identity, and the UI renders those citations with source links
- `mcp` prompt guidance covers the current live document inventory, retrieval, curation, reprocessing, history, notification, and storage capability surface
- `mcp` has no currently advertised planned operations in the manifest catalog

### Phase 5 — Editing, Reprocessing, Citations, And Richer Context

Deliverables:

- editing and curation flows
- explicit reprocessing/versioning support
- citation UX in the UI and API responses
- richer context assembly and later graph-style or CAG capabilities layered on top of the document/chunk/embedding foundation

Current status:

- `orchestrator` exposes `POST /documents/curation` for metadata-only curation of existing inventory rows
- `orchestrator` exposes `POST /documents/edit-text` for text-only edits of existing inventory rows that overwrite the raw MinIO object and queue a newer processing version
- `orchestrator` exposes `POST /documents/reprocess` for queueing an existing inventory document at a newer processing version
- `external` exposes `POST /api/documents/curation`, `POST /api/documents/edit-text`, `POST /api/documents/reprocess`, and `POST /api/documents/scan-bucket` as browser-facing proxy routes to the orchestrator control plane
- `mcp` exposes that curation flow as `documents.curation.update`
- `mcp` exposes that text editing flow as `documents.editText`
- `mcp` exposes that reprocessing flow as `documents.reprocess`
- `mcp` inventory reads include the document metadata object
- `external` and `mcp` search the current processed chunk version and include citation objects; the UI displays citation labels with source and download actions
- `external` and `mcp` expose read-only context assembly that packages current-version retrieval hits into cited context blocks
- `external` and `mcp` retrieval can be narrowed by exact-match curated metadata filters without adding a separate graph datastore
- the `rag` schema includes a JSONB GIN index for metadata containment filters used by retrieval
- `external` emits the ingest-boundary `documents.events.minio.stored` notification for successful browser uploads
- the UI Search tab exposes curation, guarded text editing, and reprocess actions for selected retrieval results

### Phase 6 — Durable Processing History

Deliverables:

- persist queue/start/complete/fail lifecycle events in Postgres so reprocessing has an auditable trail
- keep `rag.documents.last_event_*` as the fast inventory summary while storing full event payloads separately
- expose lifecycle history through both the browser-facing public API and the MCP tool surface
- keep notification delivery best-effort and avoid making NATS the system of record

Current status:

- `rag.document_lifecycle_events` stores document ID, subject, processing version, original event payload, and occurrence time
- `orchestrator` appends `documents.events.processor.queued` after successful queue commits
- `processor` appends started, completed, and failed lifecycle events after notification publish succeeds
- `external` exposes `POST /api/documents/history`
- `mcp` exposes `documents.history.list` plus `documents.history.prompt`

### Phase 7 — Browser Retrieval History UX

Deliverables:

- expose durable document history in the browser near retrieval results
- keep history read-only and backed by the existing browser-facing `/api/documents/history` route
- show processing versions, timestamps, lifecycle subjects, and recorded payload details without introducing new backend storage

Current status:

- the Search tab includes a History action for each result
- the lifecycle panel calls `POST /api/documents/history` for the selected result's `documentId`
- UI tests cover the history request body and rendered lifecycle events

### Phase 8 — Browser Context Assembly UX

Deliverables:

- expose citation-backed context assembly in the browser near retrieval results
- reuse the existing browser-facing `/api/documents/context` route rather than adding new backend storage or workflow state
- keep query, prefix, and metadata filtering aligned with semantic search and MCP `documents.context`
- render assembled context, citation references, and truncation state in a read-only panel

Current status:

- the Search tab includes an Assemble Context action beside semantic search
- the context panel calls `POST /api/documents/context` with the current query, prefix, metadata filter, limit, and character budget
- UI tests cover the context request body, rendered context text, rendered citation label, and truncation state

### Phase 9 — Browser Document Control Actions UX

Deliverables:

- expose metadata curation and reprocess actions in the browser near retrieval results
- keep browser requests flowing through `external` while preserving `orchestrator` as the workflow/control-plane owner
- proxy only the already-defined curation and reprocess contracts in this slice
- avoid browser text overwrite/editing until there is a safer preview/diff/confirmation UX
- keep Kubernetes network policy and runtime config aligned with the new `external -> orchestrator` dependency

Current status:

- `external` proxies `POST /api/documents/curation` to `orchestrator` `POST /documents/curation`
- `external` proxies `POST /api/documents/reprocess` to `orchestrator` `POST /documents/reprocess`
- the Search tab includes a Document actions panel selected from retrieval results
- the panel displays current document metadata, updates one metadata key/value pair, supports full metadata replacement when explicitly checked, and queues the next processing version
- tests cover the public API proxy behavior, validation/config errors, and the browser request bodies for curation and reprocess

### Phase 10 — Browser Guarded Text Editing UX

Deliverables:

- expose existing orchestrator text-edit capability in the browser without bypassing the control plane
- keep raw text loading through the existing authenticated document object route
- require an explicit confirmation before overwriting the raw MinIO text object
- queue a newer processing version through `orchestrator` after the edit
- keep editing limited to selected text documents with known object keys

Current status:

- `external` proxies `POST /api/documents/edit-text` to `orchestrator` `POST /documents/edit-text`
- the Search tab Document actions panel can load the current raw text for a selected text result
- the edit form requires a changed body plus explicit overwrite confirmation before saving
- successful saves show the queued processing version returned by the orchestrator
- tests cover the public API proxy and the browser load/edit/save request bodies

### Phase 11 — Browser Action Lifecycle Follow-Up UX

Deliverables:

- connect browser-triggered processing actions to the existing durable lifecycle history panel
- use the processing version returned by orchestrator action responses
- keep the lifecycle view read-only and backed by `POST /api/documents/history`
- avoid adding new workflow state or duplicating lifecycle decisions in the browser

Current status:

- reprocess actions automatically load lifecycle history for the queued processing version
- guarded text edits automatically load lifecycle history for the queued processing version
- the history panel labels version-specific action timelines distinctly from broad document history
- UI tests cover the version-specific history request bodies after reprocess and text edit actions

### Phase 12 — Browser Inventory State UX

Deliverables:

- expose Postgres-backed document inventory through the browser-facing `external` API
- keep inventory read-only and sourced from `rag.documents`
- align browser filters with MCP inventory where practical: status, object-key prefix, exact document ID, exact-match metadata, and limit
- show processing status, current and desired processing versions, latest lifecycle summary fields, errors, timestamps, source links, and curated metadata in the UI

Current status:

- `external` exposes `POST /api/documents/inventory`
- the endpoint reads `rag.documents` and returns document identity, status, metadata, processing versions, last event summary, last processed/reconciled timestamps, and last error
- `mcp` inventory now also accepts exact-match metadata filters against `rag.documents.metadata`
- the UI includes an authenticated Inventory tab with status, exact document ID, prefix, and one metadata key/value filter
- UI and API tests cover the inventory request body, response rendering, and validation path

### Phase 13 — Browser Inventory Lifecycle Drilldown UX

Deliverables:

- connect inventory rows to the existing durable lifecycle history route
- keep the drilldown read-only and backed by `POST /api/documents/history`
- reuse the existing lifecycle panel shape so Search and Inventory explain processing timelines consistently
- avoid adding new workflow state or duplicating lifecycle ownership in the browser

Current status:

- Inventory rows include a History action that calls `POST /api/documents/history` for the selected `documentId`
- the Inventory tab renders the shared lifecycle panel under the inventory results
- the existing Search lifecycle panel now uses the same shared component
- UI tests cover the inventory-to-history request body and rendered lifecycle event

### Phase 14 — Browser Inventory Reprocess Follow-Up UX

Deliverables:

- expose existing reprocess control from Inventory rows without adding a new backend route
- keep queueing backed by `POST /api/documents/reprocess`, with `orchestrator` choosing the next processing version
- keep reprocessing limited to text documents with source object keys
- update the selected row with the accepted desired processing version and load version-specific lifecycle history

Current status:

- Inventory rows include a Queue Reprocess action for text documents with object keys
- successful queue responses update the row's desired processing version from the orchestrator response
- Inventory reprocess actions automatically load `POST /api/documents/history` for the queued processing version
- UI tests cover the inventory reprocess request body, row version update, and version-specific lifecycle event rendering

### Phase 15 — Browser Inventory Metadata Curation UX

Deliverables:

- expose existing metadata curation from Inventory rows without adding a new backend route
- keep curation backed by `POST /api/documents/curation`, with `orchestrator` owning metadata persistence
- show current row metadata before submitting a focused one-key update
- update the selected inventory row from the accepted metadata response

Current status:

- Inventory rows include a Curate Metadata action that selects the row into a metadata curation panel
- the panel submits one key/value update through `POST /api/documents/curation`
- successful responses update the selected inventory row metadata and clear the edit fields
- UI tests cover the inventory curation request body and rendered metadata update

### Phase 16 — Browser Inventory Guarded Text Editing UX

Deliverables:

- expose existing guarded text editing from Inventory rows without adding a new backend route
- keep raw text loading through the existing authenticated `/api/documents/object` route
- keep saves backed by `POST /api/documents/edit-text`, with `orchestrator` owning raw object writes and version queueing
- require explicit confirmation before overwriting raw source text
- update the selected inventory row with the accepted desired processing version and load version-specific lifecycle history

Current status:

- Inventory rows include an Edit Text action for text documents with object keys
- the Inventory tab renders a guarded text edit panel that loads the current raw source text before editing
- successful saves update the row's desired processing version from the orchestrator response
- Inventory text edit actions automatically load `POST /api/documents/history` for the queued processing version
- UI tests cover the inventory object load, edit-text request body, row version update, and version-specific lifecycle event rendering

### Phase 17 — Browser Inventory Bucket Scan UX

Deliverables:

- expose existing orchestrator bucket reconciliation through the browser-facing `external` API
- keep scan execution backed by `POST /api/documents/scan-bucket`, with `orchestrator` owning MinIO reconciliation and queueing
- let Inventory operators scan the current object-key prefix filter without leaving the Inventory tab
- refresh inventory rows after a successful scan so reconciled state is visible immediately

Current status:

- `external` proxies `POST /api/documents/scan-bucket` to `orchestrator` `POST /documents/scan-bucket`
- the Inventory tab includes a Scan Bucket action that sends the current prefix filter and a bounded scan limit
- successful scan responses render scanned, queued, skipped, unsupported, and failed counts
- successful scans reload the current inventory filters
- external and UI tests cover the scan proxy request body, browser request body, scan summary, and inventory refresh

### Phase 18 — MCP Prompt Coverage For Inventory And Control Actions

Deliverables:

- keep the Labiraus MCP prompt catalog aligned with live inventory and control-plane tools
- add prompt-first guidance for `documents.inventory.list`, `documents.curation.update`, and `documents.reprocess`
- keep prompts grounded in the existing Postgres/orchestrator ownership model rather than introducing separate workflow state
- update MCP prompt docs and manifest tests with the new prompt names and arguments

Current status:

- `mcp` publishes `documents.inventory.prompt`, `documents.curation.update.prompt`, and `documents.reprocess.prompt`
- the inventory prompt explains status, prefix, document ID, and exact-match metadata filters plus when to scan before listing
- the curation prompt explains targeted metadata merge versus full replacement
- the reprocess prompt explains orchestrator-owned next-version selection and following the returned version through durable history
- MCP tests cover the new prompt catalog entries and rendered control prompt arguments

### Phase 19 — MCP Prompt Argument Rendering Completeness

Deliverables:

- keep `prompts/list` argument metadata aligned with live MCP tool schemas for scan, edit, retrieval, context, and history workflows
- render optional prompt arguments in `prompts/get` so prompt-first calls preserve bucket scan bounds, edit metadata/versioning, retrieval filters/limits, context budgets, and lifecycle history filters
- keep raw document replacement text as an explicit `documents.editText` tool payload rather than embedding large source bodies into prompt templates
- update MCP prompt docs and manifest tests with the expanded argument coverage

Current status:

- `documents.scanBucket.plan` renders optional bucket, prefix, maxKeys, and processingVersion fields
- `documents.editText.prompt` renders optional contentType, metadata, and processingVersion fields while requiring documentId
- `documents.search.prompt` and `documents.context.prompt` render query, prefix, documentId, metadata, and limit filters; context also renders maxChars
- `documents.history.prompt` renders documentId, processingVersion, and limit
- MCP tests cover the expanded prompt argument catalog and rendered prompt messages

### Phase 20 — MCP Prompt Optional Argument Resilience

Deliverables:

- keep `prompts/get` responses clean when callers provide only required arguments
- replace omitted optional prompt placeholders with an explicit neutral value instead of exposing template syntax to MCP clients
- adjust prompt wording so supplied and omitted optional values read naturally across scan, curation, edit, retrieval, inventory, history, and MinIO browsing prompts
- cover the required-only retrieval path with MCP transport tests

Current status:

- the prompt renderer fills every declared optional argument with `not supplied` when the caller omits it
- prompt copy now describes optional values as supplied fields, so required-only calls do not produce awkward examples
- MCP tests verify that omitted retrieval filters do not leave raw `{{placeholder}}` text in rendered prompt messages

### Phase 21 — MCP Prompt Argument Name Validation

Deliverables:

- keep `prompts/get` strict about the argument names advertised by `prompts/list`
- reject unknown prompt argument names with an invalid-params error before rendering messages
- prevent misspelled optional filters from silently becoming `not supplied`
- document the prompt argument validation contract

Current status:

- `prompts/get` validates submitted prompt argument names against the selected prompt definition
- unknown prompt arguments return `-32602` with the unknown argument name
- MCP tests cover a misspelled optional retrieval argument so typos cannot be silently ignored

### Phase 22 — MCP Tool Top-Level Argument Validation

Deliverables:

- keep `tools/call` strict about top-level argument names advertised by each tool input schema
- reject unknown top-level tool argument names with an invalid-params error before executing local adapters or orchestrator proxies
- prevent misspelled tool filters from silently broadening reads, while leaving nested `body` and `metadata` payload validation to the owning tool/upstream contract
- document the tool argument validation contract

Current status:

- `tools/call` validates submitted top-level argument names against the selected tool schema
- unknown top-level tool arguments return `-32602` with the unknown argument name
- MCP tests cover a misspelled MinIO folder prefix argument and verify execution is skipped

### Phase 23 — MCP Tool Required Argument Validation

Deliverables:

- validate required top-level `tools/call` arguments against each advertised tool input schema before execution
- reject incomplete tool calls with invalid-params errors rather than empty adapter/upstream error bodies
- cover both local adapter tools and orchestrator-proxied tools so validation stays aligned across MCP execution modes
- keep nested `body` and `metadata` field validation owned by the relevant tool or upstream service

Current status:

- `tools/call` returns `-32602` when a required top-level argument is missing or an advertised required string argument is blank
- local MinIO tools skip execution when a required top-level object key argument is missing
- orchestrator-proxied tools skip execution when the required top-level `body` argument is missing
- MCP tests cover missing required top-level arguments for both MinIO move and document reprocess calls

### Phase 24 — MCP Tool Top-Level Type Validation

Deliverables:

- validate top-level `tools/call` argument types against each advertised tool input schema before execution
- reject wrong-typed primitive filters such as string limits for integer fields
- reject wrong-typed top-level object payloads such as string `body` values for orchestrator-proxied tools
- keep nested `body` and `metadata` field validation owned by the relevant tool or upstream service

Current status:

- top-level schema types `string`, `integer`, `boolean`, and `object` are validated before tool execution
- malformed top-level tool arguments return `-32602` with a field-specific type error
- MCP tests cover wrong-typed MinIO `maxKeys` and wrong-typed orchestrator proxy `body` arguments, and verify execution is skipped

### Phase 25 — MCP Tool Nested Body Schema Validation

Deliverables:

- validate advertised nested object fields in `tools/call` payloads before execution
- reject missing required nested `body` fields such as `body.documentId` before calling orchestrator-backed tools
- reject wrong-typed advertised nested fields such as string `body.processingVersion` values for integer fields
- leave unknown nested body and metadata fields to the owning upstream service contract

Current status:

- nested schema paths are reported in invalid-params errors, for example `body.documentId` and `body.processingVersion`
- orchestrator-proxied tools skip execution when advertised nested body requirements are not met
- MCP tests cover missing nested required fields and wrong-typed nested fields for document reprocess calls

### Phase 26 — MCP Tool Schema Strictness Advertising

Deliverables:

- keep `tools/list` schemas aligned with the `tools/call` top-level validation contract
- advertise `additionalProperties: false` on top-level tool input schemas so clients can see unknown top-level arguments are rejected
- preserve upstream-owned nested payload flexibility by not advertising nested `body` objects as strict unless the schema explicitly says so
- cover strict top-level schema advertising in MCP manifest tests

Current status:

- explicit and generated tool input schemas advertise strict top-level argument objects
- nested orchestrator `body` schemas continue to allow service-owned extension fields
- MCP tests verify strict top-level advertising for retrieval and orchestrator-proxied tools

### Phase 27 — MCP Notification Resource URI Robustness

Deliverables:

- preserve full document identifiers when matching notification resource URIs
- keep scanned document IDs such as `s3://documents/...` usable with `resources/read`, `resources/subscribe`, and `resources/unsubscribe`
- retain nested MinIO object-key resource matching while avoiding lossy path normalization for final URI template parameters
- cover scheme-style document IDs in MCP transport regression tests

Current status:

- final resource-template path parameters are matched from the raw URI tail so embedded `://` sequences are preserved
- notification subscriptions accept scanned source-URI document IDs without collapsing `s3://` to `s3:/`
- MCP tests cover both the matcher and the session-bound subscription path for scheme-style document IDs

### Phase 28 — Browser LLM Assistant, Memory, And File Audit

Deliverables:

- add a browser-first `assistant` app for RAG-backed chat, per-user memories, conversation trails, file proposals, and audit views
- deploy local vLLM on the GPU node path with Envoy AI Gateway resources for OpenAI-compatible routing
- keep model-readable tool use limited to allowlisted read-only MCP/RAG context calls
- require authenticated user approval before create/edit writes reach `orchestrator`
- append `rag.document_change_audits` rows for create, edit, and revert operations
- enable MinIO object versioning on the external `documents` bucket so revert operations can restore prior raw content

Current status:

- `assistant` exposes conversation, chat, memory, proposal, approval, and audit endpoints under `/assistant/...`
- `external` proxies `/api/assistant/...` to the assistant service and forwards the authenticated email identity
- the UI has an Assistant tab for conversation trails, RAG-backed chat, explicit memory saves, proposal approval/rejection, and audit-driven revert staging
- `orchestrator` supports `/documents/create-text`, `/documents/edit-text`, and `/documents/revert`, all of which queue reingestion and write audit metadata when used by approved assistant proposals or browser actions
- `sql/assistant/schema.pgsql` defines assistant-owned state, while `sql/rag/schema.pgsql` includes `rag.document_change_audits`
- `helm/apps/assistant` and `helm/apps/vllm` are wired into the Flux app release plan, with vLLM pinned to the `helheim` GPU path by default
- the default vLLM model is a small unquantized Qwen instruct model so the `helheim` runtime path can be smoke-tested before larger quantized models are promoted
- Ansible now marks the `documents` bucket as versioned and enforces that versioning through the MinIO bucket role

### Phase 29 — vLLM And Envoy AI Gateway Runtime Validation

Deliverables:

- validate the local vLLM runtime path on `helheim` through the Envoy AI Gateway route used by `assistant`
- prove the current small Qwen instruct model can start reliably, answer OpenAI-compatible chat requests, and tolerate first-load model download and CUDA setup time
- document the minimum operator checks for pod scheduling, GPU visibility, model cache state, startup probe behavior, and gateway routing
- keep larger or quantized model promotion gated on measured startup behavior, memory pressure, response quality, and tool-call compatibility

Next status target:

- a repeatable smoke-test path exists for `LLM_BASE_URL=http://homelab-vllm-ai-gateway.envoy-gateway-system.svc.cluster.local/v1`
- runtime docs identify what to inspect when `helheim` scheduling, model loading, or Envoy AI Gateway routing fails
- the default model remains conservative until the local GPU path is proven stable

### Phase 30 — Assistant Quality, Safety, Memory, And Audit UX

Deliverables:

- improve assistant answer quality while keeping RAG grounding through the allowlisted read-only MCP context call
- keep memory explicit, user-approved, and scoped to the authenticated email
- make proposal approval and rejection flows easier to audit from the browser
- improve revert staging around `rag.document_change_audits` and MinIO version markers without giving the model direct write tools
- document assistant safety expectations so future LLM tool-use changes preserve the current proposal-before-write boundary

Next status target:

- assistant docs describe the supported chat, memory, proposal, approval, audit, and revert paths as current product surfaces
- tests cover the high-risk proposal and identity-scoping behavior when assistant workflows change
- any expanded tool-use behavior remains explicitly allowlisted and read-only unless a browser approval path exists

### Phase 31 — Retrieval Quality And Citation Confidence

Deliverables:

- expand the RAGAS gold-case set under `evals/ragas` with representative private queries and expected context IDs
- use RAGAS runs to compare chunking, metadata filters, and citation recall before changing retrieval behavior
- improve citation confidence in UI, MCP, and assistant contexts by keeping source URI, chunk identity, processing version, and metadata visible
- document acceptable baseline thresholds once the first real gold set is stable

Next status target:

- `make ragas-chunking-eval` is the preferred quality gate for retrieval changes that affect chunking, embeddings, ranking, or metadata filtering
- docs explain how local deterministic embeddings relate to stored `vector(384)` rows
- citation regressions are treated as retrieval-quality bugs, not only UI presentation issues

### Phase 32 — Corpus And File-Type Expansion

Deliverables:

- plan expansion beyond the current `text/*` ingestion path without weakening the MinIO, Postgres, and orchestrator ownership model
- choose extraction rules for the next file types, including how unsupported or partially extracted objects should be represented in inventory and lifecycle history
- keep non-text ingestion idempotent, versioned, and auditable through the existing processing-version model
- document any extra extraction dependencies or container runtime requirements before adding them to `processor`

Next status target:

- current `text/*` behavior remains the stable baseline
- future PDF, HTML, Office, or other file-type work has explicit extraction, failure, and citation policies before implementation
- unsupported objects continue to be visible in inventory rather than disappearing from reconciliation results

### Phase 33 — Operations Hardening And Retention Policy

Deliverables:

- add operator-facing runbooks for document processing failures, stuck lifecycle states, NATS/KEDA worker lag, vLLM startup failures, and assistant proposal recovery
- add dashboards or checks for ingestion throughput, processor failures, retrieval latency, assistant proposal outcomes, and vLLM health
- decide retention policy for MinIO object versions, lifecycle history, assistant conversations, memories, and audit rows
- rehearse backup and recovery for Postgres RAG state, external MinIO documents, and generated Kubernetes secrets
- keep the MCP and browser notification paths best-effort while preserving Postgres as the durable audit trail

Next status target:

- common day-two failures have documented diagnosis and recovery steps
- retention and rollback expectations are explicit before enabling destructive cleanup or lifecycle expiration
- observability work reinforces the existing service boundaries instead of adding a separate workflow engine

## Near-Term Non-Goals

For now, do not introduce:

- Mongo
- a separate graph database
- an over-engineered workflow engine unless there is a clear later justification

CAG and semantic graph ambitions remain later phases, not the initial implementation target.

## Delivery And Operations Reality

- Flux plus Helm remain the authoritative delivery path for in-cluster workloads.
- MinIO remains externally managed through Ansible on `svartalfheim`, not reintroduced as an in-cluster default.
- Postgres changes under `sql/` should stay idempotent and operator-friendly.
- operator access to the CNPG Postgres cluster should prefer the local `make postgres` port-forward workflow over permanent TCP exposure through the cluster edge
- Repo documentation must be updated in the same task as meaningful architecture or workflow changes.

## Immediate Priorities

- keep docs aligned with the chosen app boundaries
- keep the chosen `oauth2-proxy + Google` browser-auth path aligned across Helm, `ui`, `external`, and docs
- keep the client-certificate MCP access story aligned with the manifest metadata and auth docs
- keep MCP client compatibility broad across Codex, Claude, VS Code/Copilot, Cursor, Windsurf, and legacy SSE clients without weakening Origin validation or edge auth assumptions
- keep the `rag` Postgres schema aligned with document inventory, chunk, embedding, and retrieval behavior
- keep the `assistant` Postgres schema, browser API contract, and UI trail/proposal/audit UX aligned as the LLM surface evolves
- validate the vLLM/Envoy AI Gateway runtime path on `helheim` before promoting larger models or wider tool-use behavior
- grow retrieval quality through RAGAS gold cases before changing chunking, embedding dimensions, or ranking behavior
- plan file-type expansion beyond `text/*` before adding extraction dependencies to `processor`
- harden operations with runbooks, metrics, retention decisions, and recovery drills
- shape orchestrator and processor around clean control-plane versus data-plane boundaries
- keep public access flowing through `external` and `mcp`
- build the Labiraus prompt and notification surfaces in step with the repo plan rather than as disconnected demos
- avoid premature datastore or workflow-engine expansion

## Change Heuristics

Prefer changes that:

- reinforce Postgres as the source of truth
- keep orchestrator as the workflow owner
- keep processor stateless and job-driven
- improve idempotency and future reprocessing support
- reduce drift between docs, SQL, Helm, and app behavior

Be cautious with changes that:

- make NATS JetStream carry durable state
- push document lifecycle ownership into workers
- expose internal pipeline details through public APIs
- add new datastores or operators without a clear architectural need
