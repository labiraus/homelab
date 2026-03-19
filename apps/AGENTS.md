## Scope

This file applies to everything under `apps/`.

## Purpose

`apps/` contains the example and utility services that run inside the cluster plus shared Go packages under `apps/pkg/`.

The goal here is clarity and operability, not unnecessary abstraction.

## Working Rules

- Keep each app independently runnable, testable, and understandable.
- Prefer shared code in `apps/pkg/` only when logic is genuinely reused across services.
- Preserve the current implementation choices of each app unless the task explicitly changes them.
- When changing app behavior, also update the app-level README and any affected Helm chart values or manifests.
- When changing HTTP routes, configuration expectations, or health behavior, check both the service code and the corresponding deployment/chart config.

## Repo-Specific Notes

- `goapi/` is the Go service
- `pythonapi/` is the Flask service
- `reactapp/` is the Vite frontend
- `pkg/` contains shared Go helpers used by application code

## Safety Notes

- Do not hardcode real credentials, tokens, or cluster-specific endpoints in app source or tests.
- Keep local-only secrets in ignored files or runtime configuration, not tracked source.

## Self-Learning Loop

If a repeated app-level convention emerges, record it here.

If a lesson applies only to one app, add a more specific `AGENTS.md` under that app directory later rather than overloading this file.
