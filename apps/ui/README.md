# Worldbuilder frontend

## Current experience correction — 26 October 2026

Read the [experience contract](../../docs/worldbuilder/UX-EXPERIENCE-CONTRACT.md)
and [six journeys](../../docs/worldbuilder/UX-JOURNEYS.md) before UX changes.
The [audit and verification record](../../docs/worldbuilder/UX-AUDIT-2026-10-26.md)
tracks the current slice separately from historical checkpoints below.

Generated entities have inline name, description and property editing; detailed
fields reuse `WorldRecordEditor` inside the response. Explicit refinement sends
the edited operations as unapproved context through the existing bounded reasoning
question and presents a separate candidate for comparison. Saving uses the actual
change-set API and original revision, with distinct device-queued/server receipt
labels and a direct review handoff. Unsaved candidate edits survive section/thread
navigation and shared-result refresh, but not page refresh or world/account exit.
They are not server autosaves. The existing device queue still governs explicit saves.

Answers resolve up to four source previews at their original revision/timeline/time.
Source follow-up preserves saved conversation pins; a different scope starts a
separate thread and leaves the origin available in history. Secondary sections
offer **Back to conversation**. Shared conversations and SSE progress are used only
with server-advertised capability; private Play stays separate. No provider calls,
new endpoints, approval shortcuts or backend/schema changes are introduced.

Current PWA behavior and governance are documented in [PWA implementation](../../docs/worldbuilder/PWA-IMPLEMENTATION.md) and [design](../../docs/worldbuilder/PWA-DESIGN.md). These supersede the historical single-owner/superuser-self-review and ephemeral-chat descriptions below. Run `npm run test:pwa` after `npm run build` for production-worker lifecycle checks and responsive screenshots; the owned loopback fixture is distinct from `test:browser` against disposable PostgreSQL.

Worlds open into a conversation workspace, with Explore, Drafts & Proposals, Stories and World Settings alongside it. The compact header holds the world selector and world search; the collapsible rail/mobile drawer holds section navigation and in-memory conversation history. The full composer reserves its own bottom row while messages scroll. Resumed stories open directly on their saved scene, with a pin-scoped character picker and explicit provisional turn saves. The existing Vite/React stack, host theme and OAuth session remain. See the [AI-first specification](../../docs/worldbuilder/05-AI-FIRST-UI.md) and [implementation and validation](../../docs/worldbuilder/UI-IMPLEMENTATION.md).

Container builds copy `package-lock.json` and run `npm ci`, matching CI's dependency resolution. Local `node_modules`, generated output and environment files are excluded from the Docker context.

**Worlds** opens a library at `#/worlds`, with searchable loaded world names, access/archive labels and links into each workspace. The button stays available on narrow screens. **World menu** has a stationary header button and a separate action panel for browsing worlds, opening the creation form and account administration. Escape returns focus to the button; outside clicks, focus leaving the panel and navigation dismiss it. Creation failures keep the entered name and show the error beside the action. Ordinary accounts see an explanation of administration access instead of an empty disclosure. These source changes are tracked separately from deployment in [the October 15 review](../../docs/worldbuilder/UI-CORRECTIONS.md#world-library-and-menu-review--15-october-2026).

Before chart publication, CI runs `browser/image-smoke.py` against the published image with networking disabled, checking Nginx configuration and fetching the actual index and compiled JS/CSS over the container's own loopback. Both responsive workspace journeys and browser/database acceptance are required pre-build gates, with failed workspace traces/screenshots retained.

Worldbuilder date reads and edits preserve the complete signed 64-bit tick range. The UI retains large dates as decimal strings and requires browser JSON reviver source-context support for numeric date responses; unsupported browsers show an update-required error. See the [date wire contract](../../docs/worldbuilder/API.md).

Wiki links to unloaded records retain the displayed revision, timeline and exact submitted fictional-time filter. Unsaved filter edits do not change the scope of a record being inspected.

`WorldTimelineBranch.jsx` prepares a branch proposal from the displayed parent timeline/revision, pins the destination base before editing and preserves exact uncertain retries. All parent timeline records/dates are included regardless of wiki filters, within the 99-record limit; shared world catalog/settings remain shared. The ordinary review queue controls publication. Readers can inspect audited creation origin at the displayed historical revision. The third disposable Chromium flow covers creation, review, parent divergence and scoped origin. See [branch contracts](../../docs/worldbuilder/TIMELINE-BRANCHES.md).

Record and branch identifiers use browser-native character validation as well as the authoritative server checks. Chromium acceptance explicitly rejects embedded spaces; the regex hyphen is escaped for Unicode-set pattern compilation.

Expand **Relationship graph** or **Chronology** in the wiki to explore the loaded published scope. Graph arrows follow typed subject/target relationships, including reverse/self edges; choose an entity to focus, and follow node/edge/source links into the same inspected scope. The diagram caps at 24 nodes/36 edges for readability; its table retains every loaded relationship in the chosen focus. Chronology orders events by exact signed start tick and stable ID, with half-open end dates and source/participant links. Both views show loaded/total coverage and search/time/revision/timeline scope. **Load more canon records** extends them; editor drafts, private stories and the independently loaded editor reference catalog do not enter these views. Dates use the calendar metadata from the same published scope.

The authenticated Worlds tab (`Worldbuilder.jsx`) provides world creation, membership, wiki/history, structured record staging, independent review/publication, pinned story sessions, turn persistence and outcome proposals. AI candidates remain explicit drafts or provisional turns. The UI reports unavailable or unauthorized API capabilities; source availability does not establish deployed health. Retained Labiraus document tools and Overview remain available in the header account menu’s Document tools disclosure.

`WorldHistory.jsx` adds paginated published history, complete record diffs, compensating restore drafts and explicit original/current/proposed reconciliation for proposals and private story outcomes. The preview's revision and story version are preserved on save; conflicts require a choice and fresh independent review. Manual staged changes retain their original revision when canon is refreshed. `WorldRecordEditor.jsx` loads complete existing records, edits multi-field schemas and relationship declarations, preserves attributes and provides typed fact/rule inputs. Editor reference choices load all bounded reference pages at one revision independently of wiki filters. Load-more controls cover worlds, canon, proposals, sessions and turns; links resolve unloaded records through the scoped record API.

This is the Vite/React frontend for Worldbuilder and retained Labiraus operator/document tools.

Worldbuilder is the worldbuilding product within the existing Labiraus host and authentication shell. Keep infrastructure names and legacy route identifiers stable.

## What It Does

The UI exposes:

- a Worldbuilder header with world selection/search, a World menu for creation/account administration and account/theme controls; Overview and retained Document tools are in the account menu, and authenticated root visits open Worlds
- a retained Document assistant backed by `/api/assistant/...` for RAG chat, explicit memories, file proposals, and audit/revert views
- an auth status menu backed by `/api/auth/status` and `/api/auth/providers`
- sign-in and sign-out actions from the header auth menu
- `/api/users/count`
- an authenticated Inventory tab backed by `/api/documents/inventory` for Postgres document state
- an Inventory Scan Bucket action backed by `/api/documents/scan-bucket`
- an authenticated Search tab backed by `/api/documents/search` for document chunk retrieval (lexical by default)
- a Search-tab context panel backed by `/api/documents/context` for cited context blocks
- a Search-tab lifecycle panel backed by `/api/documents/history` for retrieved documents
- a Search-tab document actions panel backed by `/api/documents/curation`, `/api/documents/edit-text`, and `/api/documents/reprocess`
- an authenticated hash-routed documents page that browses `/api/documents/tree`, `/api/documents/object`, and `/api/documents/upload`
- a global toast stack backed by `/api/documents/events` for document lifecycle notifications
- static public legal pages at `/privacy-policy.html` and `/terms-of-service.html`

The results are shown in the page so it is easy to confirm browser-to-gateway-to-service routing and inspect the current auth state seen by the API.

In the current repo choice, the shared-host browser path is fronted by `oauth2-proxy`: `/` is proxied to `ui`, `/api/...` is proxied to `external`, and the login URL published by `/api/auth/providers` points at the local `oauth2-proxy` start endpoint rather than directly at Google.

When `authStatus.valid === true`, the app opens an `EventSource` to `/api/documents/events`, shows toast notifications for lifecycle updates, and tears the stream down again on sign-out or unmount. Toast copy is derived from the document lifecycle subject and document identifiers, with success, info, and error tones.

The authenticated Inventory tab lists `rag.documents` state through `/api/documents/inventory`. It can filter by lifecycle status, exact document ID, object-key prefix, or one exact metadata key/value pair, and renders current/desired processing versions, latest lifecycle summary fields, timestamps, errors, source links, and curated metadata. The tab can ask `orchestrator` to scan the current prefix filter through `/api/documents/scan-bucket`, then refresh the same inventory filters. Each inventory row can load durable lifecycle history through `/api/documents/history` without leaving the Inventory tab. Inventory rows can update curated metadata through `/api/documents/curation`, and text inventory rows with source object keys can load raw source text through `/api/documents/object`, save guarded text edits through `/api/documents/edit-text`, or queue reprocessing through `/api/documents/reprocess`; successful queue responses update the row's desired processing version and load version-specific lifecycle history.

The authenticated Assistant tab lists saved conversation trails, sends chat messages through `/api/assistant/chat`, saves explicit user-approved memories through `/api/assistant/memories`, and manages file proposals through `/api/assistant/proposals`. Proposal approvals are handled by processor-owned assistant workflows and routed to `orchestrator`; the UI can also inspect assistant audit rows and stage MinIO-version-backed reverts through `/api/documents/revert`.

Assistant UX work should keep this approval boundary visible: model-suggested file changes remain proposals until an authenticated browser user approves or rejects them, and audit/revert views should make the resulting MinIO and Postgres changes inspectable.

The authenticated Search tab submits natural-language queries to the public API, optionally narrows by object-key prefix or one exact metadata key/value pair, and renders the top matching processed chunks with ranking scores and download links. The same filters can assemble a compact cited context block through `/api/documents/context`. Each search result can also load its durable lifecycle history from `/api/documents/history`, showing processing versions, timestamps, lifecycle subjects, and recorded payload details. Search results can be selected for document actions that update curated metadata, load and save guarded raw text edits, or queue reprocessing through `external`.

Text editing is limited to selected text documents with source object keys. The UI loads the current text through `/api/documents/object`, requires the user to change the text and confirm the raw overwrite, then sends the replacement to `/api/documents/edit-text` so `orchestrator` writes MinIO and queues the next processing version.

When reprocess or text-edit actions return a queued processing version, the UI automatically loads `/api/documents/history` for that exact version so the lifecycle panel shows the queued/start/complete trail for the action the user just triggered.

## Local Development

Install dependencies:

```bash
npm install
```

Start the dev server:

```bash
npm run dev
```

The Vite dev server listens on `0.0.0.0:3000`.

If you need to target an API running on a different origin, set:

```bash
VITE_API_BASE_URL=http://host-or-gateway
```

## Tests

Run the test suite with:

```bash
npm test
```

## Production Build

Create the static production build:

```bash
npm run build
```

The generated `dist/` output is local build artifact only. It should not be committed; CI and the container build generate production assets from source.

## Container Build

The production image is built from `apps/ui/dockerfile`. It builds the Vite app with Node and serves the output with Nginx.

`WorldAdministration.jsx` lists members and provides reasoned, version-checked owner actions for membership, transfer and archive/reopen. Names/default timelines/settings use the structured canonical record editor and independent publication review. Archived worlds retain readable canon and story history. Newly opened worlds use the published default timeline.

Dependency versions and security checks are recorded in [Worldbuilder dependency validation](../../docs/worldbuilder/DEPENDENCIES.md). CI runs build/tests and rejects high/critical npm advisories before publishing.

The rule editor supports scoped relationship cardinality, exclusive boolean predicates and life-bound exception predicates with fictional validity intervals. These remain proposed canon until reviewed and published.

Wiki queries use the membership-scoped search API with pinned pagination. If the server advertises semantic search, **Search method** offers words-and-meaning queries against current canon. Each explicit search/page uses the approved AI allowance; continuation retains the submitted scope/ranking token. Save-triggered refreshes use lexical reads. See [semantic search](../../docs/worldbuilder/SEMANTIC-SEARCH.md). AI answers display selected/total evidence counts when only part of a large timeline was inspected.

Generation details show the request correlation, reported model, duration and allocated allowance. The allowance is explicitly distinguished from a provider invoice.

`WorldStorySetup.jsx` creates/reuses private immutable definitions at their canon pin, preserves IDs on uncertain retries, and retains generated premise scope. Sessions display provisional summary coverage and offered choices. Generated turns retain the inspected session version when saved.

The workspace’s **Long-running generation** disclosure lists private durable generations, queues up to four narrative parts, refreshes progress, cancels remaining work and inspects preserved partial results. Background generation must be enabled server-side. Retried queue submissions retain their UUID; inspected results retain their original canon/session versions before human save.

The review queue explains independent human approval and offers proposal consistency checks at the inspected base revision. Advisory findings show severity, uncertainty, scoped evidence and deterministic-check metadata; partial evidence and an empty finding list never claim complete semantic consistency.

Wiki connection browsing follows explicit forward/back references with pinned revision/timeline/fictional time and pagination. Browser acceptance uses pinned Playwright Chromium with the production UI build and disposable real canon API/PostgreSQL, driving independent editor/reviewer publication and private story resume. It also reconciles a persisted provisional story event into a draft, requires both configured reviewers, finds the published outcome in scoped wiki search and preserves the private story's original pin. See [offline acceptance](../../docs/worldbuilder/OFFLINE-ACCEPTANCE.md); test identities exist only in Go test code, and browser requests are restricted to the fixture origin. Narration in this flow is human-entered; actual model quality remains separate.

The canonical settings editor supports reviewed fictional calendars with custom months, repeating leap cycles, weekdays and exact subday ticks. Wiki/graph/chronology dates use calendar metadata from their pinned canon response and show original ticks. Typed date facts use signed decimal strings. See [calendar contracts and examples](../../docs/worldbuilder/CALENDARS.md).

The settings editor configures 1–5 independent human approvals through review. The queue displays each proposal’s required/current count, immutable decisions and invalidated membership authority, and disables publication below quorum. Three-identity browser acceptance proves a two-reviewer change cannot publish after only one vote. See [review policy](../../docs/worldbuilder/REVIEW-POLICY.md).

World menu → **Account administration** includes **Superuser · Manage users** for the operator-granted class. It registers Google emails as ordinary users; world membership is assigned separately. Superusers can explicitly approve their own proposals, with a required reason and a visible self-approval label. Configured approval counts still apply, and ordinary users retain independent review. The UI reflects server permission; it never grants authority itself. See [superuser accounts](../../docs/worldbuilder/API.md#superuser-accounts-schema-8).

## AI workspace and local acceptance

Ask/Create/Change/Play share one composer. Explicit intent can override conservative intent detection; ambiguous assertions ask an inline clarification. Cited answers open a scoped source inspector. **Ask AI about this** attaches a removable record reference. Relationships/chronology reuse the loaded published scope. Type-filtered word browsing uses the existing canon endpoint; ranked/hybrid search is available without a type filter.

Conversations are held only in memory while a world stays mounted, cleared by world/account changes and reloads. Only the last three user requests and up to eight explicit references are added to the bounded reasoning question; AI prose is never canonical evidence. There is no world chat persistence/streaming API. **Stop waiting** cancels the browser request, not a guarantee of upstream cancellation or an allowance refund. Durable generations retain their existing server cancellation and uncertainty semantics.

Generated structured changes can be edited using the existing record editor or explicitly saved as immutable drafts. The original base revision and before-state are retained. Review, approval and publication remain separate server-authorized actions; stale proposals require reconciliation. Narration can be edited and saved as a private turn at the generated session version, with choices and freeform continuation in the workspace. Use Stories to resume durable sessions and reconcile outcomes.

Hash links use `#/worlds/{world}/{workspace|explore|drafts|stories|settings}`. Source links include `record`, `revision`, `timeline` and optional exact `at`; proposal links use `proposal`; story links use `session`. Authentication loading preserves the link. In-memory conversation state survives section navigation, but is not a shareable conversation URL.

Run from `apps/ui`:

```sh
npm ci
npm test
npm run build
npm run test:workspace
```

`test:workspace` starts a loopback Vite server and runs desktop/mobile HTTP-fixture Chromium journeys. Install the pinned Playwright Chromium and its OS dependencies first. It covers conversation/citations, generated draft editing/submission, ambiguity/failure, isolation and private story resume. Focused composer resizing also covers a reduced 390×500 viewport: the focused form is brought above the visible bottom edge after resize, with scroll measurements kept current. This is not real-device keyboard evidence.

To repeat the same fixtures against deployed assets, start an owned loopback-only forward to the verified UI Pod, then use `WORLDBUILDER_BROWSER_URL=http://127.0.0.1:19473 npm run test:workspace`. This selects the served bundle instead of starting Vite. External fixture URLs are rejected. Close the forward afterward. These fixtures do not authenticate against or exercise production APIs. For actual API/SQL governance, build first, then run from the repository root:

```sh
WORLDBUILDER_TEST_TRANSPORT=exec WORLDBUILDER_BROWSER_TEST=1 bash scripts/worldbuilder/test-postgres.sh
```

This uses only owned disposable PostgreSQL and test identities, never a LAN DSN. Browser screenshots/traces go to `WORLDBUILDER_BROWSER_ARTIFACTS` or `/tmp/worldbuilder-browser-artifacts`. The workspace suite also checks actual click targets after desktop scrolling/resizing: sidebar and inspector bounds account for the measured header/footer, preventing controls from slipping under the sticky header at page end. Production model quality, actual human accounts and provider cancellation require separate LAN checks; see [publication and deployed acceptance](../../docs/worldbuilder/UI-IMPLEMENTATION.md#publication-and-deployed-acceptance--2026-10-12) for exact release evidence and browser-session limits.

## Correction acceptance — 13 October 2026

See [frontend corrections](../../docs/worldbuilder/UI-CORRECTIONS.md) for the implementation map, actual checks/screenshots and remaining contracts. World configuration edits stage ordinary reviewed drafts and preserve unrelated settings. Dedicated world/style guides are unsupported; private story premise/style are separate. Manual narration is under authoring controls; **Your action** uses real `story_turn` reasoning and requires explicit candidate save.

`test:workspace` uses local HTTP fixtures and Vite. `test:browser` selects the three actual API journeys and must run through the owned disposable PostgreSQL harness shown above. A resized desktop Chromium viewport verifies layout and focus, not a real mobile software keyboard or screen reader. These source changes are not a deployment.

## Independent inference

Inference result provenance distinguishes free local usage, estimated USD cloud costs, outstanding reservations and historical allowances. Provider/profile selection is deployment configuration; there is no browser cloud toggle. See [operator procedures](../../docs/Inference.md).
