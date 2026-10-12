# AI-first frontend implementation

Local source work on 2026-10-12 implements the [AI-first specification](05-AI-FIRST-UI.md) inside the existing Vite/React frontend. This is not a deployment report. No cluster, GitOps, infrastructure, production data or authentication configuration was changed.

## Audit and decisions (UI-0)

The previous `Worldbuilder.jsx` rendered wiki, structured editor, governance, stories and a single replaceable AI result in one long page. `App.jsx` already owned hash navigation, OAuth status, light/dark tokens and retained document tools. There is no router/UI kit dependency to replace. Existing external Go contracts already cover grounded inference, scoped record reads, immutable proposal/review/publication, private definitions/sessions/turns, reconciliation and durable jobs.

Reuse `WorldRecordEditor`, `WorldHistory`, `WorldViews`, `WorldConnections`, `WorldStorySetup`, `WorldGenerations`, `WorldAdministration`, `WorldUsers`, calendar/date helpers and the external API client. Keep exact signed ticks and original generation/review/session pins. No backend/schema changes or provider calls from the browser are required. World chat does not use the document assistant's unrelated conversation API.

## Implemented increments

| Increment | Local implementation |
|---|---|
| UI-1 shell | Authenticated root opens Worlds; opening a world selects Workspace. Persistent world selector, five section links, pending-review count, account/theme controls, secondary Document tools menu. Collapsible tablet/mobile navigation and optional source inspector. Deep links survive authentication loading. |
| UI-2 conversation | In-memory conversations, starters, bounded follow-ups, explicit removable references, editable intent, inline clarification for ambiguous assertions, keyboard send, request progress/error/explicit retry and browser cancellation. Existing durable-generation progress/cancellation remains under a disclosure. |
| UI-3 exploration | Published entity/wiki browsing, kind-filtered word browsing, existing lexical/hybrid search and pagination, reusable relationship/chronology views, revision-pinned source inspector and Ask AI actions. Inspector links retain world/timeline/revision/exact time. |
| UI-4 proposals | Generated operation cards, original-pin draft save or structured editing, human-readable before/after and reconciliation records, proposal filters, stale-revision findings, role/quorum controls and independent submit/review/publish. Human publication remains authoritative. |
| UI-5 stories | Persistent definition/library/resume, freeform and suggested actions in the main conversation, editable candidate narration, explicit version-bound turn save, provisional state/character context and outcome reconciliation into ordinary drafts. |
| UI-6 hardening | Shared theme tokens, readable bounded transcript, responsive single-column diffs, keyboard focus, mobile inspector focus containment/Escape, reduced motion, world/account isolation, cancellation cleanup, component and Chromium journeys. Local test evidence below; live acceptance remains separate. |

## State and API boundaries

`worldWorkspace.js` defines JSDoc UI intent/message/scope contracts and route/citation adapters. `WorldWorkspace.jsx` owns conversations and composer; `WorldResponse.jsx` renders answer/source/operation/narration cards; `WorldInspector.jsx` resolves published evidence. `Worldbuilder.jsx` coordinates retained domain tools. Views and existing conversation cards stay mounted while changing sections/threads so unsaved edits, retry identities and saved status survive local navigation. Switching worlds/accounts or refreshing clears ephemeral chat; private stories, definitions and saved proposals remain in PostgreSQL.

Each generation captures the submitted canon revision/timeline or the resumed story's immutable pin and inspected version. At most three earlier user requests (350 characters each) and eight attached IDs are sent as explicitly noncanonical context within the existing 4,000-byte reasoning input. Previous assistant prose is not recycled as published evidence. Ambiguous assertions require Ask or Change; intent controls never publish. Long-running jobs retain their own request and original result pins.

Only same-world, well-formed server citation IDs become source actions. The inspector fetches the exact source scope, independently of edited/unsaved wiki filters. Asking about an inspected source pins that conversation to the source’s revision/timeline until the user explicitly selects the displayed world context. Complete-record JSON is available under a disclosure; the normal review surface presents named values. Kind filtering uses `GET /canon` (with optional word query), because the ranked lexical/hybrid search contracts do not accept a kind filter.

Generated proposal saves and manual edits preserve the original base revision. Editing a candidate loads its original published snapshot and affected before-states before staging, refusing to overwrite an existing local draft. Mutations use stable request IDs for exact retries. Publication is separately authorized/revalidated by external; the UI disables stale or insufficient-quorum publication. Saved story candidates cannot be applied to a different or advanced session. Their generated `sessionVersion` is never replaced with a refreshed value.

Routes use `#/worlds/{world}/{section}` with `record/revision/timeline/at`, `proposal` or `session` query parameters. They are identifiers, not access grants; external rechecks access. Conversation return paths are section navigation within the mounted world, not persisted chat URLs. World selection beyond the first list page resolves membership-checked metadata before choosing its default timeline.

## Backend dependencies and explicit limits

- There is no world-conversation storage, streaming response or attachment endpoint. Conversations are ephemeral; no simulated persistence, streaming, uploads or fake canon appear in production.
- Browser cancellation stops waiting and ignores a late result; it cannot guarantee cancellation of upstream model work or an allowance refund. Background job cancellation retains the existing server semantics.
- Natural-language exploration returns grounded responses; graph/timeline cards render loaded published records on demand. The current API does not supply arbitrary heterogeneous response blocks or generated layout instructions.
- Stories expose private provisional state and reviewed outcome eligibility. There is no separate persisted Sandbox/Canon-eligible mode flag, equipment/inventory schema or editable inference-budget preference API. These are not invented in the UI. Published rule/guide/calendar/type changes use the existing record editor and review flow; operator inference guardrails remain external configuration.
- Factual model quality, actual independent human accounts, deployed auth/revocation, provider latency/cancellation and representative large-world performance require LAN acceptance. Existing source tests and synthetic browser answers do not prove these gates.

## Local validation

Run commands from `apps/ui` unless explicitly noted:

- `npm test -- --maxWorkers=2 --pool=threads`: component and contract coverage. The final complete run passes 79 tests in 15 files, including historical-source scope and preservation of edited/saved cards when switching conversations.
- `npm run build`: production Vite output using the existing lockfile.
- `npm run test:workspace`: loopback Vite + HTTP-fixture Chromium, desktop and 390-pixel mobile journeys. Covers sourced Ask, edited AI-created draft, ambiguous Change clarification, private Play/save/resume, failure/retry, keyboard navigation and world isolation. Final screenshots/traces are retained in `/tmp/worldbuilder-ui-accepted-artifacts`, with browser output at `/tmp/worldbuilder-ui-accepted-browser.log`. The production build and zero-vulnerability npm audit also pass.
- From the repository root: `WORLDBUILDER_TEST_TRANSPORT=exec WORLDBUILDER_BROWSER_TEST=1 bash scripts/worldbuilder/test-postgres.sh`: owned PostgreSQL, real Go auth/domain logic, three-identity review/publication, direct editing, persisted story-outcome reconciliation/publication, timeline branches and all-table restore. The complete run passes all five browser journeys and restore/security gates; the owned fixture is removed. Exact logs and checksums are recorded in [implementation status](IMPLEMENTATION-STATUS.md#ai-first-frontend-source-implementation--2026-10-12).

The devcontainer's existing cached Chromium lives under `/tmp/homelab-playwright`, with local library/font support under `/tmp/homelab-browser-libs/root/usr/lib/x86_64-linux-gnu` and `/tmp/homelab-browser-fonts.conf`. A standalone actual-browser render/click preflight passed before fixture execution. These local paths are not runtime or CI requirements; CI already installs Playwright dependencies.

## Documentation reconciliation

The original unnumbered UI brief is preserved at the requested numbered specification path, with a compatibility link at its former name. The active UI README and architecture/migration entry points now describe the conversation-first layout. Historical acceptance checkpoints remain unchanged; they do not claim this redesign was published or deployed. Current implementation, decisions, troubleshooting and acceptance references point here.
