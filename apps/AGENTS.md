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
- UI styling lives in `apps/ui/src/styles.css`; there is no `App.css`. Scoped canon views consume published records and submitted filters, never staged edits, unsaved inputs or the editor's independent reference catalog.
- `pkg/` contains shared Go helpers used by application code

## Safety Notes

- Do not hardcode real credentials, tokens, or cluster-specific endpoints in app source or tests.
- Keep local-only secrets in ignored files or runtime configuration, not tracked source.

## Self-Learning Loop

If a repeated app-level convention emerges, record it here.

If a lesson applies only to one app, add a more specific `AGENTS.md` under that app directory later rather than overloading this file.

## Build and canon boundaries

- There is no root `go.mod` or `go.work`. Run Go checks from each module directory (for example `cd apps/external && go test ./...`); CI must use the same working directory.
- When a tool's working directory is already an app directory, pass app-relative paths to formatters and scripts; do not repeat the repository's `apps/<name>/` prefix.
- Worldbuilder canon credentials belong only to external. Processor and MCP must not receive the schema owner/runtime login or human publication authority. Mesh identity/header isolation must be validated before enabling the feature.
- A successful source or artifact publication is not evidence of cluster readiness.
- Istio native sidecars appear in `initContainers` with `restartPolicy: Always`; inspecting only `containers` can falsely suggest missing injection. Check sidecar status and the actual workload certificate before judging mesh identity.
- Fictional record `from`/`until` and query `at` ticks span signed int64. Preserve original JSON number tokens at JavaScript and generic Go boundaries; JavaScript carries unsafe ticks as decimal strings. Never restrict canon to JS safe integers or convert already-rounded Numbers to BigInt. Keep numeric persisted history/hash encoding stable; test both extremes, equivalent quoted/numeric retries and half-open historical reads when changing date handling.
- AI proposal candidates must retain their generation `baseRevision` when persisted; never replace it with a newly fetched revision to avoid a conflict. Rebase explicitly and obtain fresh review.
- Manual editor staging must also retain the displayed revision and original before-state. Refreshing canon must not silently change a draft's base revision. Reconciliation creates a fresh draft with explicit conflict choices and independent approval.

- Generated story candidates must retain their inspected sessionVersion when saved; do not substitute a refreshed session version. Definitions and narrative premises retain their original revision/timeline pin. Compare persisted JSONB semantically in tests, since PostgreSQL may reorder object keys without changing content.

- Browser acceptance may use fixture identities only in Go `_test.go` code serving loopback/disposable data. Never add a runtime authentication bypass; restrict fixture browser requests to the local origin. For native selects, prefer their accessible combobox role when exact label matching includes option text.
- Interactive SVG links need comfortable hit areas and keyboard access. Verify actual browser clicks without forced actions; a clickable-looking thin path alone does not prove usable navigation.

- Storage SDK replacements must preserve application interfaces, `MINIO_*` compatibility keys, private endpoint/credential scope and error propagation. Use signed HTTP boundary tests plus the disposable actual-client SeaweedFS gate; do not mask vulnerable transitive packages with incompatible module/API overrides or treat a client replacement as storage cutover.
- Document/citation `processingVersion` is a derived-index generation, never an S3 version ID. Current lifecycle constructors omit S3 markers; audit actual stored legacy/extended JSON and explicit `version_marker` columns when preparing migration coverage. Historical audits can survive deleted inventory rows, so do not limit reference export to joins against current documents.
- Node application images must copy the committed lockfile and use `npm ci` (with `--omit=dev` for production dependencies). Exclude local `node_modules` and generated output from the context so a later `COPY` cannot overwrite the tested dependency tree. Validate complete images when changing their dependency installation.

- Calendar presentation uses the requested canon/search snapshot metadata, including filtered pages. Never derive it from current world metadata or staged settings. Full-range date facts use the `tick` type and canonical int64 strings; legacy numeric date facts retain safe-integer semantics. Audit SQL projection constraints and columns when extending Record fields or value types; snapshots alone do not establish database compatibility.
- When advancing the Worldbuilder schema, search every app's startup checks, including orchestrator generation workers, and exercise them against the migrated disposable database. External publication tests alone do not cover relay/worker activation compatibility.

- Review policies are canon settings. Pin each proposal to the stricter base/proposed quorum and count distinct humans. Ordinary users cannot self-review; the user explicitly authorised operator-granted superusers to review their own proposals and register users. Bind self-reviews to both membership and superuser authority, record the exception in immutable review/audit history, and invalidate it on either revocation. Superusers still require world membership and count as one human. Runtime credentials and user-registration requests cannot grant the class. Never resurrect old approvals after role restoration or mutate published approval identities; reconcile into a fresh proposal when content/policy pins change.

- Exercise the production Worldbuilder publish helper in disposable broker tests. JetStream rejects passing both `nats.Context` and publish `nats.AckWait`; use a timeout-bearing context so bounded acknowledgement and shutdown cancellation share one option. Direct test-only publishes missed this live relay failure.

- Startup dependency routes can lag workload readiness. Keep canon index initialization bounded and cancellation-aware, with workers/readiness withheld until it succeeds; verify persistent refusal as well as recovery.
- Inventory scans need provider version listings and `IsLatest`; ordinary listings omit version IDs, and MinIO HEAD can omit the header for a literal-null version. Retain the explicit `null` marker, exclude deleted/history entries, and apply object limits after filtering. Reconcile mutable inventory through its normal workflow without rewriting immutable audit markers.
- Queued document reads must pass both the source version and ETag to S3. Preserve opaque/literal-null IDs, use conditional reads, and never fall back to the newest object after a pinned read fails. Drain source work before cutover and reconcile current destination identities through the ordinary inventory workflow.
- Keep model citation IDs distinct from record IDs. A generated put can carry one record identity and derive its redundant outer target; explicit mismatches must still fail. Factual abstention cannot be applied unchanged to requested provisional drafts/stories, and a single successful factual smoke is not broad workflow acceptance.
