# Repo Plan For Codex

Use this file as the working objective-state document for the repository.

`REPO_MAP.md` explains where things are.

This file explains where the repo is trying to go, what is in flight, and which gaps should shape future work.

## Target State

The repo should converge on a clean homelab platform with:

- Terraform responsible for provisioning and lifecycle of Kubernetes-capable infrastructure
- Helm and Flux responsible for cluster bootstrap and steady-state application deployment
- Ansible responsible only for MinIO service state that is intentionally managed outside Kubernetes manifests
- small example and utility applications that are deployable through the same delivery path as platform workloads
- operator workflows that are safe to run from the devcontainer with minimal host-specific setup
- repo documentation that stays aligned with the actual layout, workflows, and security expectations

## Architectural Intent

### Provisioning

- `components/` should remain the entry point layer for Terraform operations
- `modules/` should hold reusable infrastructure primitives
- environment and node overlays under `etc/` should stay explicit and composable

### Cluster Delivery

- Flux should remain the bootstrap and reconciliation mechanism
- charts under `helm/` should be organized by responsibility: bootstrap, apps, data, infra, observability, workloads, libraries
- chart interfaces should stay consistent enough that future changes can be made without rediscovering conventions

### App Surface

- app code under `apps/` should remain intentionally small and readable
- shared logic should move into `apps/pkg/` only when reused or clearly cross-cutting
- each app should stay testable and independently buildable

### Operations And Security

- local-only secrets must stay out of git
- generated artifacts should be reproducible or clearly marked as generated
- operator tasks should prefer the devcontainer toolchain and documented Makefile/script entry points

## Current Priorities

- keep repo-local agent guidance and docs aligned with the current directory structure
- make the repo easier for future agents and humans to navigate without stale assumptions
- keep secret-handling expectations explicit after the recent repository cleanup
- improve the quality of local operating guidance before expanding the platform further

## Known Gaps

- repo-level objective state and navigation guidance were previously too implicit
- local conventions differ across Terraform, Helm, apps, SQL, and Ansible but were not documented near the work
- some human docs can drift behind current implementation details if not updated with each change

## Change Heuristics

Prefer changes that move the repo toward the target state above:

- reduce stale documentation
- reduce duplicated conventions
- make workflows safer and more explicit
- preserve clear ownership boundaries between Terraform, Helm/Flux, apps, SQL, and Ansible

Be cautious with changes that:

- blur tool ownership boundaries
- add new deployment paths without a strong reason
- introduce secrets or local state into tracked files
- create one-off patterns that cannot be reused across the repo

## Self-Learning Loop

When a task reveals a durable lesson:

- update `REPO_MAP.md` if discovery/navigation changed
- update this file if priorities, gaps, or target-state assumptions changed
- update the nearest `AGENTS.md` if a local rule should persist

If repeated work suggests the repo is missing an explicit milestone or objective, add it here instead of leaving it as conversational context only.
