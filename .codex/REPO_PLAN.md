# Repo Plan For Codex — Homelab Migration to a RAG System

Use this file as the working objective-state document for the repository as it evolves from “platform + example apps” into a functional Retrieval-Augmented Generation (RAG) system.

REPO_MAP.md explains where things are.
This file explains where the repo is trying to go, what is in flight, and which gaps should shape future work.

## Executive Summary

This repo already includes:
- a Vite React UI (apps/reactapp)
- a Go API (apps/goapi)
- a Python API (apps/pythonapi)
- Helm charts for apps/data/observability and shared chart templates
- Postgres via CNPG, with the vector extension enabled at bootstrap
- MinIO as an object store hosted on `svartalfheim`, with state intentionally managed via Ansible outside Kubernetes manifests

Goal: add a new RAG slice (ragapi + ingestion/indexing) while preserving the existing example UI + APIs.

## Target State

The repo should converge on a clean homelab platform with:

- Terraform responsible for provisioning and lifecycle of Kubernetes-capable infrastructure
- Helm + reconciliation (Flux preferred) responsible for cluster bootstrap and steady-state application deployment
- Ansible responsible for external storage-host services on `svartalfheim`, including MinIO service state intentionally managed outside Kubernetes manifests
- Small example and utility applications deployable through the same delivery path as platform workloads
- A RAG system that can ingest documents, build embeddings, perform retrieval, and generate answers via an LLM
- Operator workflows safe to run from the devcontainer with minimal host-specific setup
- Repo documentation that stays aligned with actual layout, workflows, and security expectations

## RAG Architectural Intent

### Data plane

- MinIO is the landing zone for raw documents and (optionally) derived ingestion artifacts.
- Postgres (CNPG) is the system-of-record for RAG metadata and embeddings using the vector extension.
- The RAG schema lives under its own Postgres schema namespace (e.g., rag.*) for clean rollback/cleanup.

### Control plane

- Ingestion/indexing runs as a Kubernetes Job/CronJob:
  - detects new/changed documents
  - extracts text
  - chunks content using a deterministic strategy
  - creates embeddings using the chosen embedding model
  - upserts chunk records + embeddings into Postgres
- Query-time retrieval/generation runs as a dedicated service (ragapi):
  - accepts queries from the UI
  - performs vector similarity retrieval from Postgres
  - constructs prompts with retrieved context
  - calls an LLM (in-cluster or external, chosen explicitly)
  - returns answer + source citations (source_uri + chunk identifiers)

### App surface

- The existing UI continues to call /go/hello and /python/hello.
- The UI adds a RAG panel that calls /rag/query.
- The RAG response includes citations suitable for display (source + chunk ranges).

## Phased Migration Plan

### Phase 0 — Decisions and scope (gate)
Deliverables:
- Define corpus sources and update mechanism (MinIO bucket naming, allowed file types, expected volumes)
- Choose embedding model and dimension
- Choose LLM hosting approach (external API vs in-cluster CPU vs in-cluster GPU)
- Define basic SLOs (acceptable latency, freshness, cost/compute envelope)

### Phase 1 — Storage primitives (Postgres + MinIO)
Deliverables:
- SQL: create rag schema and tables (documents, chunks, embeddings)
- Ansible: create MinIO bucket(s) and dedicated IAM/policies for rag ingestion + ragapi reads
- Kubernetes secrets: apply generated secrets for MinIO access (never commit credentials)

### Phase 2 — Ingestion/indexing workload
Deliverables:
- New ingestion container app (apps/rag_ingest)
- Helm: CronJob or Job templates to run ingestion
- Idempotency: content hashing to avoid re-embedding unchanged docs
- Basic reconciliation: rebuild mode that can re-index all documents

### Phase 3 — Query-time RAG API (ragapi)
Deliverables:
- New ragapi service (apps/ragapi) with:
  - POST /rag/query
  - GET /readiness and /liveness
  - optional admin endpoints guarded by auth/policy
- Helm: deploy ragapi behind /rag ingress path with network policies and service mesh policy consistent with other apps

### Phase 4 — UI integration
Deliverables:
- UI panel/page for RAG Q&A
- Source citation rendering (source_uri + chunk identifiers)
- Basic UI tests

### Phase 5 — Operational hardening
Deliverables:
- Monitoring: metrics + dashboards + alerts for ragapi and ingestion
- Tracing: integrate with cluster tracing if configured
- Security review: secrets, network policies, RBAC, data handling boundaries
- Rollback drill and runbooks

## Security and Operations

- Never commit secrets.
- Prefer additive changes with reversible toggles (e.g., CronJob suspend=true by default).
- Keep RAG schema and resources isolated so rollback does not impact existing app schemas.
- Align docs with behaviour changes whenever endpoints or deployment layouts change.

## Known Gaps / Questions

- Flux is the authoritative reconciliation mechanism; keep migration work and docs aligned to that path.
- What is the source corpus for RAG (MinIO bucket only, Git, web, other)?
- What embedding/LLM models and dimensions are required?
- What are the hardware constraints (CPU-only vs GPU nodes)?
- What is the acceptable operational budget (latency, storage growth, re-index frequency)?

## Change Heuristics

Prefer changes that:
- Move the repo toward the target state above
- Reduce stale documentation
- Reduce duplicated conventions
- Make workflows safer and more explicit
- Preserve clear ownership boundaries between Terraform, Helm/reconciliation, apps, SQL, and Ansible

Be cautious with changes that:
- Introduce new operators/datastores without clear need
- Add secrets or credentials into git history
- Mix MinIO state management between Ansible and Kubernetes manifests without a clear boundary
