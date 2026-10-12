# Worldbuilder AI — AI-First UI Redesign Specification

**Status:** Product specification; local implementation tracked in [UI implementation](UI-IMPLEMENTATION.md)

**Applies to:** Existing Evolve → Worldbuilder AI frontend and its supporting APIs

**Repository path:** `docs/worldbuilder/05-AI-FIRST-UI.md`

**Companion documents:** Worldbuilder architecture, migration plan and remote/LAN deployment runbook

## 1. Product experience

Worldbuilder is **AI-first, not chat-only**. The primary way people interact with their fictional worlds is by speaking or writing naturally to an AI collaborator. The application translates requests into grounded answers, editable creative content, structured world exploration, and reviewable proposals. Conventional forms, wiki pages, graphs and timelines are available when useful, but they support the conversation rather than serving as the main entry point.

**Core principle:** The world is the source of truth; the AI is the conversational interface to it. The assistant never silently alters published canon.

### Experience goals

- Open a world and immediately ask a question, generate content, suggest an edit or start an adventure.
- Preserve conversational context (world, timeline, entities, current task) as users navigate.
- Show grounded citations and allow direct navigation to source facts and entities.
- Make the difference between **Ask**, **Create**, **Change**, and **Play** clear without forcing users into a cumbersome mode-selection flow.
- Present AI outputs as useful interactive results: answer, entity preview, timeline, relationship view, editable draft, or change proposal — not always a wall of text.
- Offer explicit controls and human review before publishing changes to canon.
- Keep the interface welcoming to writers with no technical knowledge.

## 2. Information architecture

### Primary navigation

1. **Workspace (default)** — central AI conversation and generated results; user can ask, create, change, explore and launch stories.
2. **Explore** — searchable wiki and canonical entity/event browsing, plus relationship and timeline views.
3. **Drafts & Proposals** — authored and AI-generated drafts, review queue, differences and publication history.
4. **Stories** — story library, creation, live interactive sessions, past sessions and outcome proposals.
5. **World Settings** — members and roles, world guide, style guide, calendars, timelines and approval rules.

Global world switcher, universal search/command affordance, notifications and account controls live in the top bar. World selection is always visible. Avoid multiple competing navigation schemes and separate chat products.

### Initial landing state

The default world workspace shows:
- The current world and selected timeline/revision context.
- A prominent natural-language composer with placeholder **“Ask, create, change or explore your world…”**.
- A handful of optional starter actions: *Ask about canon*, *Create a character*, *Suggest a change*, *Start an adventure*.
- Resume recent conversations and stories, plus small actionable items (e.g. pending approvals).

The composer is the primary call to action. Do not make the main landing page a dense admin dashboard.

## 3. Desktop layout

Use a responsive three-region layout, but **the conversation is the main canvas**:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ Worldbuilder  [World: The Shattered Empire ▾]   Search   Alerts   Account   │
├───────────────────┬──────────────────────────────────┬──────────────────────┤
│ WORKSPACE         │ AI WORKSPACE                     │ CONTEXT / INSPECTOR  │
│ + New conversation│                                  │                      │
│ Recent chats      │ Conversation thread              │ Relevant canon      │
│                  │ Rich answer and action cards     │ Referenced entities  │
│ Explore           │ Draft previews and diffs         │ Source citations     │
│ Drafts & Proposals│ Graph/timeline on demand         │ Active timeline      │
│ Stories           │                                  │ Draft/story state    │
│ World Settings    │                                  │                      │
│                   │                                  │                      │
│                   │ [Ask, create, change…          ] │                      │
└───────────────────┴──────────────────────────────────┴──────────────────────┘
```

- **Left:** collapsible navigation and conversation/session history.
- **Centre:** 60–70% of available width when possible; conversation and rich results take precedence. Composer is sticky at bottom of this pane, with keyboard access and attachments only if supported.
- **Right:** optional contextual inspector; context changes with selected response, entity, proposed change or story. Closeable; never required for core actions.
- Rich content may open a focused detail view or full-width workspace while retaining a route back to the conversation.

### Responsive behaviour

- **Tablet:** collapse nav to icons/drawer and open inspector on demand.
- **Mobile:** single-column conversation, bottom composer, navigation drawer, context detail as full-screen sheet/route; avoid horizontal overflow and multi-column diff tables.
- Respect reduced motion, keyboard navigation, visible focus, sufficient text contrast and scalable font settings.

## 4. AI interaction contract

The user should not need to understand LangGraph, tool calls or database structure.

| User intent | Expected experience | Canon impact |
|---|---|---|
| **Ask** “Who ruled after Aldric?” | Cited answer; expandable entities, chronology and sources | Read-only |
| **Create** “Invent a rival noble family” | Editable draft with entity cards, relationships and suggested lore | Draft only |
| **Change** “Aldric survived the battle” | Structured proposal, affected facts and detected conflicts | Proposal pending approval |
| **Explore** “Show how the houses connect” | Conversation plus graph/timeline/linked wiki results | Read-only |
| **Play** “Start an adventure in Ashfall” | Create or resume a persistent interactive story | Provisional story state only |

Auto-detect intent where unambiguous. If a request could be interpreted as factual exploration **or** a canonical mutation, ask a short inline clarification before preparing a write proposal; never silently publish. Show detected intent in a small, editable label near results, not a compulsory wizard.

### Conversation and context

- Every conversation is bound to a world; its timeline and canon revision context are visible and controllable.
- Contextual entity references are explicit and removable (e.g. `King Aldric`, `Ashfall`, `Primary timeline`).
- Persist conversations where the backend supports it; distinguish chat history from canonical information.
- Allow follow-up prompts and task-specific actions such as **Open source**, **Show relationships**, **Edit draft**, **Create proposal**, **Continue story**.
- Provide clear progress for long-running generation, retry/cancel where technically supported, and understandable partial/error states.
- Stream narration/answers when supported, but do not present streaming text as validated canon.
- Make token/inference usage optionally available in an advanced details view, not the default creative surface.

## 5. Rich response components

Implement a consistent set of response components; do not treat all responses as generic Markdown.

1. **Grounded answer card:** response text, source chips and links, uncertainties/missing facts, relevant entities and follow-up actions.
2. **Entity card:** name, type, canon status, brief summary, key facts, relationship links, inspect/open actions.
3. **Draft content card:** editable generated narrative/structured data, save draft, propose canon changes; explicitly marked noncanonical.
4. **Proposal card:** summary of operations, before/after diff, conflicts, validation status, open review flow.
5. **Timeline card:** bounded chronological sequence with filterable events and link to full timeline view.
6. **Relationship card:** small focused network around selected entities; expand into full explorer.
7. **Story scene card:** scene narration, player choices, freeform action field, characters/location and provisional session state.
8. **Source inspector:** stable fact/source reference, world/timeline/revision, provenance and navigation to wiki/history.

Each result should use shared design tokens and clear empty/loading/error states. Content status is not inferred from colour alone: display explicit text such as **Published canon**, **Draft**, **Proposal**, **Hypothetical**, **Session state**.

## 6. Dedicated secondary screens

### Explore / Wiki

- Search and browse by entity/event type, world and timeline.
- Entity page: readable prose first, structured attributes and relationship/history sections second.
- Linked names navigate to canonical pages; page header exposes status, revision and source provenance.
- Graph and timeline visualisations are scoped by a selected entity/time range; avoid rendering an unreadable whole-world hairball.
- “Ask AI about this” opens the main workspace with the entity attached as context.

### Drafts & Proposals

- List filters for *My drafts*, *Needs review*, *Conflicts*, *Published*, *Rejected*.
- Review shows human-readable before/after, affected entities/events, deterministic rule violations, AI consistency concerns and links to evidence.
- Approve/reject/publish are separate actions according to world permissions; show who can act and why an action is disabled.
- Confirm publication on stale canon only after revalidation/rebase; API must enforce all authorisation and concurrency checks.
- Provide a clear return path to the originating conversation or story.

### Stories

- Library with **Start**, **Resume**, **Create story**.
- Story session is immersive: narration/scene at centre, options plus open text action, collapsible characters/inventory/location panel.
- Session's pinned canon revision and `Sandbox` vs `Canon-eligible` mode are accessible, without distracting from play.
- Narrative turn results may include proposed session-state changes, but only the validated story engine commits those.
- At checkpoints/end: **Review outcomes** turns significant events into draft canonical changes; user can edit or discard before formal submission.

### World Settings

- World guide and style guide editing with version history.
- Entity types, relationship definitions, timelines/calendars, roles, approval policies, inference preferences/cost guardrails (where permitted).
- Advanced configuration is progressively disclosed; it should not dominate ordinary writing flows.

## 7. Essential end-to-end flows

**Flow A — Ask about canon:** Open world → type question → AI selects authorised retrieval → sourced answer and uncertainty → click fact → inspect wiki/revision → follow-up in same conversation.

**Flow B — AI creates a new character:** Prompt → draft entity/relationships and prose → edit locally → validate against world rules → create change set → submit for review → reviewer publishes atomically → canonical wiki and derived search update.

**Flow C — Change established history:** Prompt “Aldric survived” → preview required operations → surface affected death, succession, burial and timeline facts → resolve contradictions/edit proposal → reviewer approves → server revalidates current revision → atomic publication or actionable stale-revision error.

**Flow D — Interactive story becomes canon:** Start story using pinned published revision → narrative turns with persistent provisional state → resume later → extract outcome proposal → compare against latest canon → review and publish through the normal change-set mechanism. Playing alone never changes canon.

**Flow E — Direct non-AI editing:** Explore → open entity → edit structured facts → save draft → validate → review/publish. The AI-first design must never prevent conventional authoring or read-only browsing.

## 8. Visual design direction

**Tone:** modern, calm, creative and literary, not a fantasy-game skin or a developer console. The content should provide the fantasy/sci-fi atmosphere, not hard-coded application chrome.

- Neutral surfaces with one restrained accent colour for primary actions; world artwork can enrich cards and stories.
- Generous whitespace, readable long-form typography, restrained visual hierarchy, comfortable chat density.
- Consistent role for typography: editorial display headings where appropriate; readable UI font for controls and body.
- Light/dark theme support using existing app theming, not duplicated CSS themes.
- Explicit status labels with accessible semantic colours; e.g. published, draft, proposal, conflict, hypothetical.
- Avoid permanent split-screen chat + wiki + graph + editor on small displays; show one focus at a time.
- Use existing component libraries where possible. Do not introduce several competing UI kits.

## 9. Frontend architecture and API integration

Reuse the current SPA framework, router, authentication/session handling and API client conventions where practical. Refactor into clear domains, for example `workspace`, `conversations`, `canon`, `explore`, `proposals`, `stories`, `settings` and shared `components`/`design-system`. These names are illustrative, not presumed existing paths.

- External Go API remains the authorised gateway for reads, draft operations, reviews, publication and story session persistence.
- Processor handles AI reasoning behind existing API orchestration; the frontend should not call inference providers directly.
- Define typed UI contracts for message role, intent, response block type, status, sources, action availability, job state, world ID, timeline ID and canon revision.
- Reuse canonical IDs and revisions across cards, wiki, review views and stories; never infer published status from model prose.
- Apply server-side filtering and RBAC; UI hides unavailable operations but never replaces API authorisation.
- Use route-based deep links for worlds, entities, proposals, stories and sessions, and preserve return-to-conversation context.
- Maintain accessibility for rich cards, streaming content, keyboard focus and review diffs.

### Suggested structured conversation result

```json
{
  "worldId": "world-001",
  "timelineId": "primary",
  "canonRevision": 128,
  "intent": "ask",
  "blocks": [
    {
      "type": "answer",
      "text": "King Aldric died at Ashfall in year 842.",
      "citations": [{"type": "fact", "id": "fact-456", "revision": 128}]
    },
    {"type": "entity", "entityId": "character-123", "status": "published"}
  ],
  "actions": ["open_sources", "show_relationships"]
}
```

This is a design example. Codex should reconcile actual API structures before implementing.

## 10. Incremental implementation plan

**UI-0: Audit existing UX.** Inspect routes, components, state, theme, API clients, auth, story/proposal capabilities and frontend test setup. Document reusable parts and design gaps before replacement.

**UI-1: Application shell.** World selector, collapsible navigation, AI workspace as default route, context inspector, mobile navigation, empty/loading/error states and theme tokens.

**UI-2: Conversation experience.** Composer, history, explicit world/timeline context, streaming/progress, prompt starters, typed response renderer and deep links. Stub missing endpoints with clearly labelled development fixtures only; never claim fake content is canon.

**UI-3: Canon exploration.** Wiki entity pages, search, source inspector, “Ask about this” contextual navigation and basic graph/timeline results.

**UI-4: Drafts and proposals.** Editable draft cards, human-readable diff, impact/conflict findings, reviewer permissions, review/publish flow; use real backend contracts.

**UI-5: Stories.** Story library, live player experience, persistent session history/state, freeform and suggested choices, canon-eligible outcome review.

**UI-6: Polish and hardening.** Accessibility, responsiveness, visual consistency, deep links, performance, integration tests and removal of superseded UI flows.

UI phases may overlap backend migration phases, but don't silently bypass missing APIs. Build functional vertical slices; include fixtures only for isolated demos/tests, not production paths.

## 11. Acceptance criteria

1. Opening a world lands on an AI-first workspace with immediately usable composer.
2. A user can ask, create, propose a change or start a story from the same primary surface.
3. Factual AI responses show resolvable sources and express uncertainty when evidence is missing.
4. Users can browse and edit world data without AI.
5. Draft, hypothetical, proposal, published canon and provisional story state are visually unmistakable.
6. AI-created entities and changes remain drafts until human-reviewed, atomic publication.
7. Users can inspect conflicts and related facts through human-readable review interfaces.
8. Story state persists and a session outcome can create a proposal, never immediate canon.
9. Navigation and composer work on desktop/mobile, by keyboard and with screen readers.
10. The frontend uses existing auth/network patterns, no direct LLM calls, and no cross-world data leaks.
11. UI tests cover core navigation and all five end-to-end flows above using mock and real API integration where available.
12. Active frontend documentation and screenshots/design notes reflect the new UI; superseded docs are archived or linked appropriately.

## 12. Codex redesign instructions

> Read this document, the Worldbuilder architecture and migration plan, and all applicable `AGENTS.md` instructions. Inspect the actual UI and supporting API code before modifying it. Redesign the application into the AI-first workspace defined above while preserving the existing stack, auth, build tools and useful components. Build in the UI-0 to UI-6 sequence, prioritising a functional end-to-end workspace over static mockups. Implement typed response cards, context-aware conversations, grounded source navigation, draft/proposal reviews and persistent interactive stories where backend contracts permit. Use explicitly labelled fixtures only when a backend capability is missing; record the gap and do not misrepresent sample data as canon. Add responsive layouts, accessibility support and tests. Update or archive contradictory frontend docs, record architectural choices and test results, and do not initiate cluster mutations or GitOps rollouts while working remotely. End with a summary of actual changes, completed/blocked flows, executed tests and LAN-dependent checks.
