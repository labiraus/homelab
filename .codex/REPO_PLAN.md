# Repo Plan For Codex — Homelab Async Document Platform

Use this file as the working objective-state document for the repository.

`REPO_MAP.md` explains where things are.
This file explains the architecture the repo is converging on, what is active now, and which gaps should drive future changes.

## Executive Summary

This repo is no longer planning around separate `rag_ingest` and `ragapi` applications.
The chosen application boundary is:

- `ui`: React frontend
- `external`: public Go API for browser clients
- `mcp`: Go MCP server for AI-native access
- `orchestrator`: internal Go control-plane API
- `processor`: TypeScript NATS JetStream worker for document operations

The intended platform shape is:

- MinIO on `svartalfheim` remains the canonical raw object store
- Postgres via CNPG plus pgvector is the system of record for metadata, workflow state, chunks, and embeddings
- NATS JetStream plus KEDA provide asynchronous execution and worker scaling
- Redis is available but is not yet a core architectural dependency
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
- the `ui`, `external`, `mcp`, `orchestrator`, and `processor` apps under `apps/`
- SQL bootstrap under `sql/`
- external MinIO management through Ansible on `svartalfheim`
- orchestrator-backed `documents.scanBucket` reconciliation that inventories MinIO objects into Postgres and queues new or changed text objects for processing
- a built-in deterministic `local-embeddings` fallback used by processor, `external`, and `mcp` when no external embedding endpoint is configured

Documentation must stay aligned with implementation as these pieces evolve.

## Chosen Architecture

## App Boundaries

- `ui` is the browser-facing React application.
- `external` is the stable public API for the UI. It serves Postgres-backed data and triggers orchestrator actions without exposing internal pipeline details.
- `mcp` is the stable AI-native front door for agents and MCP-compatible clients.
- the Labiraus MCP server is a primary product surface of this repo, not a sidecar utility, and should be evaluated alongside the rest of the deployed stack
- `orchestrator` is the internal control plane. It owns workflow state transitions, reconciliation, and task dispatch decisions.
- `processor` is the stateless data-plane worker. It performs extraction, chunking, embedding, and persistence work when triggered asynchronously.

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
- `mcp` has no currently advertised planned operations in the manifest catalog

### Phase 5 — Editing, Reprocessing, Citations, And Richer Context

Deliverables:

- editing and curation flows
- explicit reprocessing/versioning support
- citation UX in the UI and API responses
- richer context assembly and later graph-style or CAG capabilities layered on top of the document/chunk/embedding foundation

Current status:

- `orchestrator` exposes `POST /documents/curation` for metadata-only curation of existing inventory rows
- `orchestrator` exposes `POST /documents/reprocess` for queueing an existing inventory document at a newer processing version
- `mcp` exposes that curation flow as `documents.curation.update`
- `mcp` exposes that reprocessing flow as `documents.reprocess`
- `mcp` inventory reads include the document metadata object
- `external` and `mcp` search the current processed chunk version and include citation objects; the UI displays citation labels with source and download actions

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
