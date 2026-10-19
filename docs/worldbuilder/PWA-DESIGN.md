# Worldbuilder progressive web application

This is the approved product specification for the PWA implementation. It extends
[the AI-first design](05-AI-FIRST-UI.md) and supersedes the single-owner,
editor/reviewer and ephemeral-conversation target. Implementation and verification
are recorded separately; this document is not deployment evidence.

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

## Authority

Readers browse, converse, play privately and suggest changes. Authors collectively
own their assigned worlds, manage membership and establish canon. Administrators
are system-wide Authors with account management and explicit approval overrides.
The final Author and final Administrator cannot be removed. Private sessions stay
private even from Administrators.

Administrators select independent Author review (default, 1–5 approvals), direct
Author publication, or unanimous Author endorsement. Overrides require a reason;
they never bypass revisions, validation, established-account protection or audit.
Membership and policy authority are checked again at publication. Existing
published history is immutable.

## Established accounts

An established account records a witnessed occurrence, not an inferred cause.
Its synopsis, timeline and provenance become immutable on publication. Linked
facts and interpretations remain editable. Reviewed clarifications append;
explicit supersession adds a new account and retains the original. One successor
per account, same-timeline links and cycle refusal preserve an unambiguous chain.
History resolves authority at the requested revision. Alternate timelines retain
origin but can diverge. Sandbox cannot modify original canon.

Use a first-class record and an independent append-only ledger written in the
publication transaction. No edit, delete, restore, rebase, Administrator override
or offline replay can erase an established account. Semantic contradictions in
free text are advisory findings, never a claim of complete machine verification.

## Offline contract

Installation caches only application assets. API, OAuth, MCP and document
responses are not service-worker cached. Explicit downloads and local authoring
use account-separated IndexedDB. Downloaded canon retains its exact revision,
timeline, sources, calendars and coverage. Browser storage is not encrypted.
Sign-out hides retained work and stops synchronization until the same user logs
in. Clearing the device is explicit; export protects unsynced writing.

Local autosave never implies a server save. Explicit saves queue immutable UUID
requests. Foreground synchronization verifies identity and permissions, preserves
original pins, retries uncertain acknowledgements exactly, and stops conflicts
for three-way reconciliation. AI calls, approvals, publication, administration
and world creation remain online actions. Never automatically spend an inference
allowance on an offline prompt. Application updates preserve pending work.

## Documentation-to-feature map

| Requirements | Product surface |
| --- | --- |
| Architecture, API, review policy | Server permissions, suggestions, review/publication, audit |
| AI-first specification and correction brief | Shell, rich cards, responsive composer, inspector, accessibility |
| Calendars, timeline branches | Exact dates, scoped chronology, complete branches and origin |
| Semantic search, index acceptance | Optional metered search, scope and coverage, authoritative citations |
| Story/API and generation contracts | Private definitions, explicit saves, Sandbox, state and durable jobs |
| Implementation status, UI corrections, deployed audit | Regression baselines and distinct source/deployed evidence |
| Offline acceptance, dependencies | Disposable real API/database and browser gates, locked dependencies |
| Broker, controller, mesh, backup, Velero and TSDB acceptance | Retained infrastructure boundaries; no PWA deployment claim |
| LAN runbook, public validation, observability, troubleshooting | Staged activation, refusal, operational diagnostics |
| Decisions, documentation audit, migration and archived plan | Compatibility, historical evidence and superseded direction |

## Acceptance

Desktop and Android are first release targets. Require both themes at
320/390/768/1024/1440px, complete composer bounds, keyboard/focus and real Android
keyboard checks. Test governance, immutable accounts/supersession, private stories,
shared conversations, offline restart/replay/conflicts/account isolation/quota,
real service workers and update survival. Keep HTTP fixtures, disposable API/SQL,
actual device, model-quality and production evidence separate. iOS acceptance is
a later milestone, not inferred from desktop WebKit.
