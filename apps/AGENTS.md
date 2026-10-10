## Scope

This file applies to everything under `apps/`.

## Purpose

`apps/` contains Worldbuilder and retained document services plus shared Go packages under `apps/pkg/`. Canon publication belongs only to external; processor can generate candidates, never approve or publish.

The goal here is clarity and operability, not unnecessary abstraction.

## Working Rules

- Keep each app independently runnable, testable, and understandable.
- Prefer shared code in `apps/pkg/` only when logic is genuinely reused across services.
- Preserve the current implementation choices of each app unless the task explicitly changes them.
- When changing app behavior, also update the app-level README and any affected Helm chart values or manifests.
- When changing HTTP routes, configuration expectations, or health behavior, check both the service code and the corresponding deployment/chart config.

## Repo-Specific Notes

- `external/` is the public Go service
- `mcp/` is the Go MCP service
- `orchestrator/` is the internal Go document control plane and optional canon outbox/index worker
- `processor/` is the TypeScript Node HTTP/LangGraph service and NATS JetStream worker
- `ui/` is the Vite frontend
- `pkg/` contains shared Go helpers used by application code

## Safety Notes

- Do not hardcode real credentials, tokens, or cluster-specific endpoints in app source or tests.
- Keep local-only secrets in ignored files or runtime configuration, not tracked source.

## Self-Learning Loop

If a repeated app-level convention emerges, record it here.

If a lesson applies only to one app, add a more specific `AGENTS.md` under that app directory later rather than overloading this file.

## Build and canon boundaries

- There is no root `go.mod` or `go.work`. Run Go checks from each module directory (for example `cd apps/external && go test ./...`); CI must use the same working directory.
- Worldbuilder canon credentials belong only to external. Processor and MCP must not receive the schema owner/runtime login or human publication authority. Mesh identity/header isolation must be validated before enabling the feature.
- A successful source or artifact publication is not evidence of cluster readiness.
- AI proposal candidates must retain their generation `baseRevision` when persisted; never replace it with a newly fetched revision to avoid a conflict. Rebase explicitly and obtain fresh review.
- Manual editor staging must also retain the displayed revision and original before-state. Refreshing canon must not silently change a draft's base revision. Reconciliation creates a fresh draft with explicit conflict choices and independent approval.
