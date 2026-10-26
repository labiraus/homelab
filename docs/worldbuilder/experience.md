# Worldbuilder experience

This is the canonical product and behavioral acceptance specification. It preserves
requirements that are not yet implemented; the source boundary below and
[status](../status.md) distinguish capability from acceptance. Server contracts in
[API](api.md) govern permissions, persistence, pins and publication.

## Product intent

Worldbuilder is a conversational creative workspace. Ask, Create, Change, Explore
and Play are intentions within one experience, not five disconnected forms. Canon
is authoritative; AI is an interface and collaborator. Users must be able to act on
a useful result without reconstructing their task in another screen.

| Intent | Useful result | Canon impact |
| --- | --- | --- |
| Ask | Readable grounded answer, uncertainty, linked source/entity previews | None |
| Create | Editable draft, comparison/refinement, explicit save/propose | Draft only |
| Change | Clarification when ambiguous, affected facts, before/after and review | Governed proposal |
| Explore | Focused relationships, chronology, source detail and return path | None |
| Play | Private scene, choices/freeform actions, versioned save/resume | Provisional session only |

Never imply that generated prose is created canon, that a green deterministic check
proves semantic truth, or that a queued device save is confirmed server persistence.
A consequential ambiguous assertion needs clarification before a write proposal.
Direct structured editing and read-only browsing must remain available without AI.

## Experience

Worldbuilder is an editorial studio: warm neutral light surfaces, ink-dark mode,
restrained blue actions, serif headings and readable sans-serif controls. The
five sections remain Workspace, Explore, Drafts & Proposals, Stories and World
Settings. A compact header contains world selection, search, activity and account
controls. The conversation scrolls above a reserved composer; narrow layouts use
focus-managed drawers and sheets. Existing hash links and exact scope survive.

Content states (Published canon, Established account, Local draft, Suggestion,
Proposal, Hypothetical, Private session) are separate from synchronization states
(Saved on this device, Waiting to sync, Synced, Needs sign-in, Needs resolution).

## Shared visual and interaction specification

| Token | Light | Dark |
| --- | --- | --- |
| Canvas | `#f6f3ed` | `#15191e` |
| Surface | `#fffdf8` | `#20262e` |
| Muted surface | `#f3efe6` | `#191f26` |
| Main text | `#24282e` | `#f1eee8` |
| Secondary text | `#62615e` | `#b4b2ad` |
| Border | `#d8d2c7` | `#414951` |
| Action/focus | `#285ea0` | `#8bb9ef` |

Bundle IBM Plex Serif for editorial headings and IBM Plex Sans for controls and
body copy, with Georgia/system fallbacks. Prose uses 1.7 line height and a 74ch
maximum measure. Controls have 44px minimum height; visible keyboard focus uses a
3px outline with 3px offset. Compact secondary labels do not replace accessible
names. Status is expressed in text as well as color. Respect reduced motion.

Cards share surface, border and spacing. Buttons use action verbs and one primary
action per task. Editors retain entered values on failure. Comparisons label the
original, current and proposed versions. Deterministic validation and advisory AI
findings remain separate. Loading uses a status announcement; errors explain a
recoverable action; empty screens explain what can be created or downloaded.

| Screen | Primary workflow | Context and secondary interactions |
| --- | --- | --- |
| World library | Find/open/create a world | Role and archive labels, recent public activity, cross-world search, return to library |
| Workspace | Ask/Create/Change in shared threads; Play privately | Attributed messages, explicit source references, pinned revision/timeline/time, progress, editable candidates and scoped citations |
| Explore | Read/search canon and inspect sources | Structured editor entry, graph, chronology, calendars, branch origin, established-account lineage and original attribution |
| Drafts & Proposals | Stage, save, submit, review and publish | Before/after, distinct findings, effective quorum, reasoned override, restore and three-way reconciliation |
| Stories | Start or resume a private story | Definition, cast, scene, choices, manual turns, location/inventory, Sandbox label and reviewed outcome proposals |
| World Settings | Review guide/schema/calendar changes | Membership, administrative publication policy, archive/reopen and permitted inference settings |
| Account and device | Install, download and recover work | Theme, quota/usage, persistence request, export/import, synchronization, explicit local-data clearing and Administrator registry |

Keep the desktop world rail and inspector in the existing shell. At narrow widths,
use the same section destinations in a navigation drawer and focus-managed sheets.
The header and underlying canvas become inert behind modal sheets. Escape closes
the sheet and restores its invoking control. Account/device controls scroll inside
a viewport-bounded popup. Only the transcript scrolls above the reserved composer.
World, timeline, exact fictional time and revision stay visible and survive links.

## Response artifacts and navigation

Use grounded answers, entity previews, editable drafts, proposal comparisons,
bounded timelines, focused relationship views, private story scenes and source
inspectors where each serves the user's intention. Every applicable artifact shows
title/type, authority status, world/timeline/revision, provenance, primary action,
inspect/edit options and explicit loading/error/retry/save state. Keep raw JSON,
model usage and technical configuration behind advanced disclosure.

The desktop conversation takes precedence (roughly 60–70% where practical); the
world rail and optional inspector support it. Narrow screens use one working
surface and focus-managed sheets. Preserve existing hash routes, thread state,
selected context and a visible return to the originating conversation or story.

- Explore presents readable canon first, with scoped history, source links,
  graph/chronology and an Ask-about-this handoff. Graphs must be bounded and legible.
- Drafts & Proposals distinguishes My drafts, Needs review, Conflicts, Published
  and Rejected. Show original/current/proposed, deterministic findings separately
  from AI advice, effective policy and reasons an action is unavailable.
- Stories offers Start/Resume/Create, scene prose, choices, freeform actions and
  disclosed cast/location/inventory. Show immutable pin and Sandbox/Canon-eligible
  mode. Outcomes enter normal proposal governance; playing alone cannot publish.
- World Settings progressively discloses reviewed guides, schemas and calendars,
  collective membership, Administrator publication policies and permitted inference
  settings. Ordinary accounts need an explanation when administration is unavailable.

Preserve complete record fields in structured edits and use independent catalogs
pinned to the inspected revision rather than a filtered wiki page. Source previews,
follow-ups and character selection retain the original scope. A saved conversation
cannot be repinned; different scope creates another thread while retaining origin.

Header triggers stay in normal layout; only their popup is positioned. Closing a
sheet removes inert state before restoring its original trigger. Verify Escape,
outside dismissal, keyboard order, actual click targets, document width and complete
composer bounds. DOM visibility and auto-scrolling clicks alone are insufficient.
The account/device panel must remain scrollable at small widths. Mobile scene and
return controls share the existing toolbar space rather than pushing content away.

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

## Current source capability and limits

The source implements shared attributed conversations with immutable pins and saved
final results; SSE reports progress before a validated final result, not unrestricted
token streaming. Play/private context cannot enter shared threads. Completed results
are recovered without re-inference; running, failed or uncertain work is not replayed
automatically.

The 26 October Ask/Create slice adds up to four named source previews, inline draft
fields, the detailed editor in the same card, generated/working comparison, explicit
refinement, stable shared-result IDs and save/review/return handoffs. Refinement uses
the existing 4,000-byte question bound and refuses oversized context before inference.
Changed payloads receive new request IDs; unchanged uncertain retries retain theirs.

Unsaved candidate edits and comparisons remain memory-only across mounted thread/
section navigation and polling. Refresh/world exit can lose them. Explicit device
saves use the separate durable queue; receipts distinguish queued from server-saved.
No durable candidate-version or task-to-proposal-link endpoint is implied.

Still-required experience work includes subject-focused natural-language graph
composition, full consequential-change presentation in the originating response,
complete selection/return continuity, durable candidate history and task links.
Broader semantic consequence/uncertainty quality requires a real model. The six
journeys below are acceptance requirements, not blanket claims of completion.

## Behavioral acceptance journeys

### Journey 1 — Understand a world in conversation

**Starting point:** Reader opens The Shattered Empire, primary timeline, published revision 128.

**User:** "Why did House Veyr abandon Ashfall?"

**Expected UI:** The workspace answers in readable prose with linked/clickable sources and explicitly distinguishes witnessed accounts from inferred motives. A compact House Veyr card offers Open, Show relationships and Ask about this. The current timeline and revision remain visible. Following a citation opens a source inspector pinned to revision 128; returning preserves the question and answer.

**Avoid:** An unsourced answer; a full table of entities; a generic link to a separate wiki search with no retained context.

**Acceptance:** The sources resolve in the same world and scope; absent evidence produces honest uncertainty; readers cannot mutate canon.
### Journey 2 — Co-create a character without losing the draft

**User:** "Create a young diplomat from House Veyr who secretly supports its enemies."

**Expected UI:** The assistant produces an editable draft with proposed name, description, motivations, key facts and relevant relationships. The user can say "Make her older and have her loyalty be to Ashfall rather than the rival house" and the same working artefact is updated or a clear candidate revision is presented for review. The draft remains conspicuously noncanonical. An author can save it and enter the existing change-set review process; a reader can suggest where permissions allow.

**Avoid:** A long AI paragraph followed only by "Copy" or "Open editor"; silently inserting canonical entities; losing edits when opening a related entity.

**Acceptance:** Generated claims are not shown as published, validation is distinct from AI commentary, and save/refresh guarantees match real persistence capabilities.
### Journey 3 — Propose a consequential change

**User:** "Actually, King Aldric survived the Battle of Ashfall."

**Expected UI:** The assistant treats this as ambiguous until the user confirms they mean to propose a canon change rather than discuss an alternative scenario. The proposal explains affected death, succession, burial and chronology facts with human-readable original/proposed comparisons and cited dependencies. It explicitly separates hard rule violations from uncertain narrative implications. The user can adjust the proposed operations before review. Publication invokes the existing server-side approval/quorum, revision and atomicity rules.

**Avoid:** Flipping a deceased field directly; showing a green "success" state before publication; treating a historical established account as freely editable.

**Acceptance:** Stale revision or missing authority has a recoverable path and never bypasses publication safeguards.
### Journey 4 — Explore relationships visually

**User:** "Show me which factions influence the Council of Ashfall."

**Expected UI:** A focused relationship view appears as an answer artefact, showing a legible subset with named edges and supporting source links. The user can select a faction to view its facts, expand the neighbourhood or switch to a dedicated Explore view; a Back to conversation action restores the originating task and selected entities.

**Avoid:** Unbounded node hairballs, unlabeled edges, or a graph disconnected from sources.

**Acceptance:** Relationships respect world, timeline, revision and permission scope.
### Journey 5 — Play, resume, and propose an outcome

**User:** "Start an adventure as a courier entering Ashfall during the siege."

**Expected UI:** The story begins as an immersive narrative scene with choices and a freeform action, along with subtle cast/location/session context and explicit private/provisional status. A saved session can be resumed with its actual version and pinned canon context. At a meaningful outcome, the user can review candidate changes and submit them through ordinary proposal governance. Nothing played is canon by default.

**Avoid:** A standalone chat pretending its turns are durable; silently treating narrated events as official history; exposing private sessions in world activity.

**Acceptance:** Turn version conflicts and offline limitations are visible, with retries preserving user-authored text.
### Journey 6 — Recover gracefully when the backend cannot do it

**Scenario:** The user refreshes a conversation or goes offline during drafting.

**Expected UI:** Previously committed story and proposal state is still available according to the API; ephemeral chat is not misrepresented as server-persistent. Local draft/offline status is explicitly labeled and governed by the PWA offline contract. The UI offers export/recovery when supported and does not automatically invoke a paid AI endpoint after reconnecting.

**Avoid:** Fake saved indicators, unconditional "Resume chat" claims, or silently replaying an approval/publication request.

**Acceptance:** Actual online/offline identity, pinned revisions, saved versus device-local state and reconciliation semantics are tested distinctly.
### Cross-journey visual verification

For each journey, verify desktop at 1440px and responsive states at 1024px, 768px, 390px and 320px in both themes. Verify keyboard and focus order, visible status text, bounded scrolling, the composer remaining reachable, error recovery and accessible action names. Use real Android keyboard evidence before claiming Android acceptance.
### Evidence template

For each test record: journey; source revision/commit; API versus fixture; viewport/device; screenshots before/action/after; successful assertions; known blocked behaviours; canonical state before/after; and whether the result is source-only, local integration, or deployed evidence.

## Feature decision and release checks

Before changing a feature, state its user intention, direct path to a manipulable
result, authoritative/provisional boundary, real supporting API, persistence across
navigation/failure/refresh and keyboard/narrow-screen behavior. If users must start
over or the API cannot support the claimed outcome, rendering is not completion.

Require both themes at 320, 390, 768, 1024 and 1440px, plus reduced-height desktop,
comfortable actual hit targets, contained scrolling and reachable composer. Respect
reduced motion, accessible names and visible focus. Use exact accessible combobox
names for the world picker; a partial Open-world label can match Reopen-world options.
Keep shared IDs stable and never weaken pin, privacy, quorum or conflict checks to
make a visual journey pass.

Test real worker/offline restart, transactional ordering and account-bound replay,
lease ownership, quota failure, export/import, sign-out and update survival. Real
Android installation/window/keyboard/dictation/TalkBack, production Google sessions,
live model quality and large-world performance require separate evidence. Desktop
Chromium resizing is not Android acceptance; iOS is a subsequent release gate.
See [testing](../testing.md) and the retained [screenshots](screenshots/ux-2026-10-26/README.md).
