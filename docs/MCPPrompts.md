# Labiraus MCP Prompts

This document tracks the prompt catalog that the Labiraus MCP server publishes through `prompts/list` and `prompts/get`.

Use it alongside [.codex/REPO_PLAN.md](/workspaces/homelab/.codex/REPO_PLAN.md) when shaping future MCP capabilities so the prompt surface grows in step with the actual repo plan.

## Current Prompt Catalog

The manifest currently lists these prompt names:

- `documents.submit.example`
- `documents.scanBucket.plan`
- `postgres.auth.userCount.prompt`
- `minio.documents.browse.prompt`
- `documents.notifications.subscribe.prompt`

## Prompt Intent

### `documents.submit.example`

- lifecycle: `live`
- purpose: show a ready-to-run example for the current `documents.submit` tool
- arguments:
  - `documentId` required
  - `sourceUri` required
  - `contentType` required

Use this when:

- an agent needs a concrete ingestion payload shape
- you want prompt-first guidance before calling the live tool

The current example should point at a MinIO-backed reference payload with:

- `bucket`
- `objectKey`
- `sourceUri`
- `contentType`

This live ingestion path currently supports `text/*` documents only.

### `documents.scanBucket.plan`

- lifecycle: `planned`
- purpose: describe the future MinIO scan and Postgres reconciliation flow
- arguments:
  - `prefix` optional

Use this when:

- planning the first bucket-reconciliation implementation
- reviewing how the future `documents.scanBucket` tool should behave

### `postgres.auth.userCount.prompt`

- lifecycle: `live`
- purpose: explain the quickest Postgres-backed auth-health read available today
- arguments: none

Use this when:

- verifying the Labiraus Postgres capability surface
- checking whether the deployed MCP server can execute a simple auth query

### `minio.documents.browse.prompt`

- lifecycle: `live`
- purpose: show how to browse the documents bucket with the current folder-aware MinIO tools and resources
- arguments:
  - `prefix` optional

Use this when:

- exploring MinIO-backed document storage through MCP
- narrowing reads before fetching specific objects

### `documents.notifications.subscribe.prompt`

- lifecycle: `planned`
- purpose: define the future document-lifecycle subscription flow driven by NATS JetStream
- arguments:
  - `documentId` required

Use this when:

- designing the notification resource and subscription behavior
- validating that event naming stays aligned across `orchestrator`, `processor`, and `mcp`

## Notification Pattern

The planned document notification flow is:

1. `documents.events.minio.stored`
2. `documents.events.processor.queued`
3. `documents.events.processor.started`
4. `documents.events.processor.completed`

The intended shape is:

- `orchestrator` and the MinIO-ingest boundary emit the lifecycle events onto NATS JetStream
- `mcp` subscribes to that event stream
- `mcp` forwards document-specific updates to MCP subscribers for a resource shaped like `homelab://mcp/documents/notifications/{documentId}`

This notification surface is still planned, but the prompt exists now so the repo can build toward it intentionally.
