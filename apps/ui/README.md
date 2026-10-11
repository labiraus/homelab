# ui

Worldbuilder date reads and edits preserve the complete signed 64-bit tick range. The UI retains large dates as decimal strings and requires browser JSON reviver source-context support for numeric date responses; unsupported browsers show an update-required error. See the [date wire contract](../../docs/worldbuilder/API.md).

Wiki links to unloaded records retain the displayed revision, timeline and exact submitted fictional-time filter. Unsaved filter edits do not change the scope of a record being inspected.

The authenticated Worlds tab (`Worldbuilder.jsx`) provides world creation, membership, wiki/history, structured record staging, independent review/publication, pinned story sessions, turn persistence and outcome proposals. AI candidates remain explicit drafts or provisional turns. The API is disabled until its LAN gate, and the UI reports that condition. Existing Labiraus document tools below remain available.

`WorldHistory.jsx` adds paginated published history, complete record diffs, compensating restore drafts and explicit original/current/proposed reconciliation for proposals and private story outcomes. The preview's revision and story version are preserved on save; conflicts require a choice and fresh independent review. Manual staged changes retain their original revision when canon is refreshed. `WorldRecordEditor.jsx` loads complete existing records, edits multi-field schemas and relationship declarations, preserves attributes and provides typed fact/rule inputs. Editor reference choices load all bounded reference pages at one revision independently of wiki filters. Load-more controls cover worlds, canon, proposals, sessions and turns; links resolve unloaded records through the scoped record API.

This is the Vite/React frontend for Worldbuilder and retained Labiraus operator/document tools.

Worldbuilder is the worldbuilding product within the existing Labiraus host and authentication shell. Keep infrastructure names and legacy route identifiers stable.

## What It Does

The UI exposes:

- a GitHub-style top bar with an Overview tab and a Documents tab for authenticated users
- an authenticated Assistant tab backed by `/api/assistant/...` for RAG chat, explicit memories, file proposals, and audit/revert views
- an auth status menu backed by `/api/auth/status` and `/api/auth/providers`
- sign-in and sign-out actions from the header auth menu
- `/api/users/count`
- an authenticated Inventory tab backed by `/api/documents/inventory` for Postgres document state
- an Inventory Scan Bucket action backed by `/api/documents/scan-bucket`
- an authenticated Search tab backed by `/api/documents/search` for semantic chunk retrieval
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

The authenticated Search tab submits natural-language queries to the public API, optionally narrows by object-key prefix or one exact metadata key/value pair, and renders the top matching processed chunks with similarity scores and download links. The same filters can assemble a compact cited context block through `/api/documents/context`. Each search result can also load its durable lifecycle history from `/api/documents/history`, showing processing versions, timestamps, lifecycle subjects, and recorded payload details. Search results can be selected for document actions that update curated metadata, load and save guarded raw text edits, or queue reprocessing through `external`.

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

Wiki queries use the membership-scoped search API with pinned pagination. AI answers display selected/total evidence counts when only part of a large timeline was inspected.

Generation details show the request correlation, reported model, duration and allocated allowance. The allowance is explicitly distinguished from a provider invoice.

`WorldStorySetup.jsx` creates/reuses private immutable definitions at their canon pin, preserves IDs on uncertain retries, and retains generated premise scope. Sessions display provisional summary coverage and offered choices. Generated turns retain the inspected session version when saved.

The Worlds reasoning panel lists private durable generations, queues up to four narrative parts, refreshes progress, cancels remaining work and inspects preserved partial results. Background generation must be enabled server-side. Retried queue submissions retain their UUID; inspected results retain their original canon/session versions before human save.

The review queue explains independent human approval and offers proposal consistency checks at the inspected base revision. Advisory findings show severity, uncertainty, scoped evidence and deterministic-check metadata; partial evidence and an empty finding list never claim complete semantic consistency.

Wiki connection browsing follows explicit forward/back references with pinned revision/timeline/fictional time and pagination. Browser acceptance uses pinned Playwright Chromium with the production UI build and disposable real canon API/PostgreSQL, driving independent editor/reviewer publication and private story resume. See [offline acceptance](../../docs/worldbuilder/OFFLINE-ACCEPTANCE.md); test identities exist only in Go test code, and browser requests are restricted to the fixture origin.
