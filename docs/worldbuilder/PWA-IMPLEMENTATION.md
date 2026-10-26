# Worldbuilder PWA implementation and release gates

## Experience correction — 26 October 2026

The [experience audit](UX-AUDIT-2026-10-26.md) records the Ask/Create vertical slice
under the [UX contract](UX-EXPERIENCE-CONTRACT.md) and [journeys](UX-JOURNEYS.md).
Inline candidate edits/refinement and exact source follow-ups reuse the existing
reasoning, conversation and change-set contracts. Shared result identities survive
polling; saved pins cannot be silently changed. Proposal receipts distinguish
device-local queuing from server saves. Card edits themselves remain memory-only
until an explicit save; ordinary PWA device writing/queue rules remain unchanged.
Existing authority, immutable accounts, private scenes and separate review gates
are retained. No new deployment, backend migration or device acceptance is claimed.

## Current source scope — 19 October 2026

This checkout extends the existing React/Vite and external Go application using
schema **009**. It is not evidence that a migration, image, chart or PWA has been
published or deployed. Earlier deployment checkpoints remain in
[implementation status](IMPLEMENTATION-STATUS.md). The approved screen and
interaction design is [PWA-DESIGN.md](PWA-DESIGN.md).

The editorial shell keeps the five existing sections and hash routes. Locally
bundled IBM Plex Serif/Sans fonts, warm neutral and ink-dark tokens, 44px primary
controls, reserved composer space and existing focus-managed sheets apply across
worlds. World library discovery includes membership-filtered search/activity.
Account controls include installation, device storage, downloads, synchronization,
export/import and clearing. Retained document tools require online authentication.

## Governance and established canon

Author and Reader replace the five legacy world roles. Administrators use the
existing `superuser` account field for wire compatibility and have Author access
to every world; they cannot inspect another person's private story or job.
Authors collectively manage membership and archive/reopen. No transfer-owner
workflow remains. The final Author cannot be removed or demoted.

Administrators set a versioned per-world `publicationPolicy` through the audited
administration endpoint: independent (default one, configurable 1–5), direct, or
unanimous. Independent approvals exclude the proposer, including an Administrator.
Unanimous endorsement includes every current assigned Author and an Author
proposer. Global Administrators can contribute reviews but cannot replace missing
assigned Authors under unanimous policy. A reasoned Administrator publication
override skips approval counts only. Stale revisions, stale policy pins,
validation, rejection, archive and immutable accounts still block publication.
Policy changes require fresh proposals; membership changes reevaluate current
authority and unanimous membership. Published approval lists remain historical.

`established_account` records have title (`label`), `synopsis`, `timeline`, optional
exact `from`/`until`, `references` to events/sources, and optional `supersedes` or
`clarifies`. Propose and publish through ordinary governance. The ledger is
append-only; publication inserts ledger entries through a fixed-search-path
trigger, alongside the revision, projection, audit and outbox transaction. Runtime
has SELECT only on the ledger. Revision insertion independently refuses changing
or omitting a prior account, including compensating restores. Clarifications and
supersessions get new identities. The original remains readable and citable;
unique successor links, cycle and timeline validation apply.

Only the account is immutable. Sources, events and interpretations remain editable.
The review response flags changes to referenced records. This is a structural
warning, not a claim to understand prose. AI consistency findings remain advisory.
Reasoning context includes scoped accounts and reviewed guides, instructs the
model to prefer the effective successor and distinguish conflicting reports, and
returns server-derived effective-account citations separately from model prose.
Sources can explicitly say `evidenceStatus: reported` or `unspecified`; existing
sources retain unspecified status. Branch-copy remaps account identities and links
while the existing immutable branch audit retains origin.

## API additions

All routes below are under `/api/worlds`, authenticated at external Go. World
members may read shared content; only the attributed requesting human can start a
saved message's inference. Existing CSRF/origin and request-size checks apply.

| Route | Contract |
| --- | --- |
| `GET /:world` | Adds `publicationPolicy`, `policyVersion`, effective role and capabilities. |
| `POST /:world/administration` | `set_policy` accepts `policy`, Administrator only; Author membership/archive operations keep stable request ID and expected administration version. |
| `POST /administrators` | Registered `principal`, `enabled`, required `reason`; protects the final Administrator and writes immutable account audit. |
| `POST /:world/changes` and `/submit` | Readers can create/submit their own suggestions; Author review/publication remains separate. |
| `GET /:world/changes/:id` | Adds policy pin, effective approval count, publication readiness and established-account structural warnings. |
| `POST /:world/changes/:id/publish` | Optional `overrideReason`, Administrator only. Ordinary publication uses `{}`. |
| `GET /:world/established-accounts?revision=&timeline=` | Exact-snapshot records, successor map and ledger publication origins. Generic canon/record/MCP reads also support the new kind. |
| `GET/POST /:world/conversations` | Paginated shared threads; create with stable UUID, title, revision, timeline and optional exact `at`. Pins are immutable. |
| `GET/POST /:world/conversations/:id/messages` | Ordered, attributed messages; POST `{id,request}` is idempotent and cannot contain private session context. |
| `POST /:world/conversations/:id/messages/:message/generate` | SSE progress, then a validated final result. Completed retries return the result without inference. Running/failed/uncertain requests never auto-repeat. GET messages recovers saved results. |
| `POST /:world/sessions` | `mode: canon_eligible` (default) or `sandbox`, part of immutable creation identity. |
| `POST /:world/sessions/:id/turns` | Optional `scene: {location, inventory:[{entityId,quantity}]}` saved atomically with the version-checked turn and exact retry receipt. |
| `GET /search?q=` | Authorized worlds' default timelines, bounded world/result coverage, scoped citations. |
| `GET /activity` | Authorized world creation/submission/publication metadata only; no private story/job payloads. |

Reviewed `worldGuide` and `styleGuide` belong to world settings and stay pinned to
requested canon revisions. Sandbox bypasses fictional consistency checks only;
shape, references, privacy, concurrency and established accounts remain enforced.
Publishing Sandbox outcomes uses ordinary validation and explicit reconciliation.
Reasoning accepts exact signed-int64 `at`; numeric and quoted wire forms preserve
identity. Generation and story pins are never substituted with refreshed versions.

## Device lifecycle

The custom `injectManifest` worker precaches only build assets and icons. Root
navigation is network-first, with an offline fallback only for transport failure.
It does not intercept API, OAuth, MCP, document or mutation requests, and HTTP
authentication refusals never become cached application success. New workers wait
for the user's update action and completion of registered local-writing saves.
See [Vite PWA update guidance](https://vite-pwa-org.netlify.app/guide/prompt-for-update).

Offline access is opt-in, per authenticated account and device. IndexedDB holds
complete selected timeline snapshots, sources/schemas/calendars/accounts, downloaded
conversation history, private stories, local writing and immutable queued requests.
Downloads retain exact revision and coverage. On missing scope, the UI reports
unavailable content rather than fetching a substitute snapshot. AI prompts can be
autosaved but are never automatically sent.

With offline writing enabled, explicit draft and manual-turn saves enter the
durable queue before delivery, including while connected. Story queue insertion,
local version, monotonic sequence and receipt commit in one transaction. Sequence
numbers preserve dependency order even when saves share a timestamp.
Foreground synchronization authenticates the same account, checks world capabilities
and archive state, acquires a renewable transactional lease and uses the unchanged
request ID/payload. A lost acknowledgement retries the same request. Stale drafts
pause for Original / Current / Proposed choices; divergent story turns and their
dependencies pause for inspection and explicit reauthoring. No Background Sync
support is required; the app must be open. See the
[Background Synchronization API](https://developer.mozilla.org/en-US/docs/Web/API/Background_Synchronization_API).

Sign-out stops new replay, clears in-memory content across tabs and hides retained
account data. Downloads are not encrypted; shared-device protection depends on the
OS/browser. The UI requests persistent storage, reports origin usage, exposes quota
failures and supports account-bound export/import. Browsers can still clear stored
data; important unsynced writing should be exported. See
[browser storage behavior](https://developer.mozilla.org/en-US/docs/Web/API/Storage_API/Storage_quotas_and_eviction_criteria).

## Operator procedure

Back up canon and the existing identity registry. Rehearse populated migration and
restore in the disposable harness. Apply numbered migrations through `009_up.sql`
with the canon migration owner, then refresh `grants.sql`; retain the separately
owned `auth-grants.sql` grants. Never apply the destructive legacy auth seed.
Publish external and orchestrator builds that require version 9 alongside the new
UI. Do not activate against a version-8 database. Preserve deployment evidence
separately; this source task does not perform those steps.

Rollback disables the affected feature/service while retaining populated schema
and ledger. `009_down.sql` is only for an empty database. A full restore must retain
all 22 Worldbuilder tables plus `auth.users`; replacing only the current-record
projection is insufficient. Uncertain conversation generation is inspected through
saved message status; a new attempt requires a deliberate human action and may
consume another allowance.

## Validation and remaining release evidence

See [retained local acceptance and screenshots](PWA-ACCEPTANCE.md) for the completed
checks and their limits.

Run `npm test`, `npm run build`, `npm run test:workspace` and `npm run test:pwa` from
`apps/ui`. `test:pwa` serves the production bundle on an owned loopback server and
exercises a real worker; its API is an explicitly isolated fixture. It retains
light/dark screenshots at 320/390/768/1024/1440px. The disposable
`scripts/worldbuilder/test-postgres.sh` covers actual SQL/Go API, migrations,
revocation, ledger protection and backup/restore. Add `WORLDBUILDER_BROWSER_TEST=1`
for actual API Chromium journeys. These evidence categories are not interchangeable.

Real Android installation, installed-window launch, software keyboard/dictation,
TalkBack/screen-reader interaction, production Google authentication and live model
semantic quality require separate device/deployment evidence. Desktop emulation
does not establish these gates. iOS acceptance is a subsequent release gate.
