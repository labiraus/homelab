# Worldbuilder AI — deployed UI correction brief

**Status:** Implementation brief based on the signed-in deployment audit of 13 October 2026  
**Target application:** https://mcp.labiraus.com  
**Original specification:** `WORLDBUILDER-AI-FIRST-UI.md`  
**Audit:** `WORLDBUILDER-DEPLOYED-UI-AUDIT.md`

This brief supplements the original specification. It defines the corrective work required by the inspected deployment. Preserve the original AI-first interaction model and canon safeguards. Where this brief makes layout and acceptance requirements more specific, use those requirements.

## 1. Outcome and scope

Make the deployed application feel like a creative workspace as soon as a world opens. The user must be able to see and use the composer immediately on desktop and mobile, inspect sources without losing conversational context, review changes in plain language, and resume a story directly into its scene.

Preserve working features: the five world sections, workspace default route, existing authentication, world/timeline/revision context, source inspector, removable entity context, proposal filters and comparisons, approval history, saved story state and provisional/canon separation.

This is a frontend correction project. Inspect actual routes, components and API contracts before changing them. Do not replace working infrastructure, introduce a second component library, silently bypass missing APIs, populate production views with fabricated canon, or alter published records as part of UI verification. Backend gaps must be recorded separately from presentation gaps.

The audit inspected archived synthetic worlds. Disabled creation/publication controls in those worlds are expected and must not be interpreted as missing features. New generation, publication, conflict enforcement and story turns remain unverified.

## 2. Priority 1 — viewport layout and mobile overflow

### Desktop

- Use a compact global header, a collapsible left rail, a main conversation pane and an optional closeable inspector.
- The main workspace fills the available viewport height. Its conversation/history area scrolls independently above the composer.
- Keep the complete composer and Send action visible when a world opens. The user must not scroll past headers, a hero section or starter actions to reach them.
- Keep the composer at the bottom of the conversation pane using a layout that reserves its height. Do not overlay messages or controls with a fixed composer.
- Allow the composer itself to expand for multiline text up to a bounded height; then scroll its input content.
- Treat the empty-state title and starter actions as compact content inside the conversation area. They must give way to the conversation once messages exist.
- When an inspector opens, let the conversation use the remaining width. Collapse the inspector to an on-demand sheet/detail view before either pane becomes too narrow.

### Mobile and tablet

- Use one column, a navigation drawer and an on-demand context sheet/detail route.
- Eliminate document-level horizontal scrolling. Constrain flex/grid children, inputs, selectors, long world names and administration controls to their available width. Long content should wrap or scroll inside an explicitly bounded content component.
- Keep the composer visible at the bottom of the workspace. Account for browser chrome, safe-area insets and the software keyboard using appropriate viewport sizing.
- The message area must remain scrollable while composing. The keyboard must not cover the input or Send action.
- Keep Ask, Create, Change and Play reachable on mobile. They may wrap or use a compact action menu, but must not disappear solely because of screen width.
- Render proposal comparisons as labelled stacked sections on narrow screens, rather than a wide multi-column table.

**Acceptance:** At widths 320, 390, 768, 1024 and 1440px, document scroll width does not exceed viewport width on the inspected routes. At desktop sizes 1366×768 and 1440×900, and mobile size 390×844, the empty workspace exposes the full composer and Send action without scrolling. Verify mobile keyboard behaviour manually on a real device or equivalent device environment; a resized desktop browser is insufficient evidence for that check.

## 3. Priority 2 — compact shell and conversation continuity

- Move the world switcher into the global header. Retain a persistent, understandable world name; truncate long names accessibly where necessary.
- Integrate universal search into the header. Present timeline/revision as compact context controls near the workspace, rather than repeated full-width administrative rows.
- Place the five primary sections in the left rail. Include New conversation and recent conversations there, with the equivalent arrangement inside the mobile drawer.
- Move account administration and world creation into account/world-switcher menus. Keep world administration inside World Settings.
- Review the existing Worlds, Overview and Document tools destinations. Preserve useful routes and capabilities, while moving supporting tools into an appropriate menu so they do not compete with primary world navigation.
- Preserve selected conversation, entity references and timeline/revision context across section navigation. Do not start a new conversation merely because a user opens a source or returns from Explore.
- Retain clear paths back from source, proposal and story details to their originating conversation when that relationship is available.
- Make inspector context deliberate: selecting a new result updates it; unrelated navigation must not accidentally present an old source as the current story/result context. Either close it or clearly show it as pinned context.

**Acceptance:** World selection, search, primary navigation and account controls remain usable by keyboard and on mobile. No duplicate world-selection row remains above the workspace. Opening and closing details preserves the active conversation and its context.

## 4. Priority 3 — plain-language exploration and review

### Explore

- Present readable titles and descriptions before structured attributes.
- Translate internal record types into human labels, such as “World settings” instead of `world_settings`.
- Replace implementation explanations such as interval boundary semantics with short user-facing explanations; keep detailed semantics in advanced help where useful.
- Preserve search, type, timeline and historical context controls, with less prominent filters progressively disclosed on mobile.
- Preserve source links, Published canon labels, revision context, relationship exploration and Ask AI about this.
- Keep raw records available in advanced detail rather than as the default entity/source experience.

### Drafts & Proposals

- Lead each proposal with its title, explicit status, author, affected records and a readable summary of the proposed change.
- Make the human-readable comparison the primary review action. Preserve the existing Original / Current canon / Proposed comparison, including stale-revision context where relevant.
- Present eligibility in concise language, for example “0 of 1 required approvals”. Put the full policy explanation behind “How approval works”.
- Keep raw operation JSON behind an advanced-details disclosure.
- Show conflicts, deterministic validation results and AI concerns as distinct findings with evidence when supplied by the backend. Do not imply that an unperformed check passed.
- Explain unavailable actions next to the action. For archived worlds, show one clear banner: “This world is archived. You can browse its history; creating or publishing changes is paused.” Provide an owner-only route to the existing reopen control.
- Preserve separate review, approval/rejection and publication actions according to actual permissions and contracts. Retain stale-revision revalidation and independent-review requirements.

**Acceptance:** A nontechnical reviewer can understand the inspected “City gate: blue → green” proposal without opening JSON. Disabled actions have understandable reasons. Mobile comparisons remain readable without document-level horizontal scrolling.

## 5. Priority 4 — story library and scene-first player

- The Stories landing screen leads with resumable sessions and story definitions. Keep Create story available, with setup opened on demand.
- Replace “Cast entity IDs (comma-separated)” with a searchable character/entity picker using the existing canon API. Display names and types; keep IDs internal.
- When a session is resumed, make its current scene the main canvas. Collapse setup rather than leaving the full form above the narration.
- Display narration, any supported choices, a freeform player-action field and a clear continuation action. Collapsible characters, location, inventory and provisional-state details support the scene.
- Use the existing story engine/API to process actions. Do not relabel a field that accepts authored narration as a player-action field unless the underlying behaviour actually processes player actions.
- If only manual narration editing is supported, label that capability honestly and place it in authoring controls. Record the missing player-action contract; do not simulate interactive turns with fixtures in production.
- Keep pinned revision, timeline and Sandbox / Canon-eligible mode accessible through concise session details when the API exposes them. Never infer mode from generated prose.
- Preserve private/provisional state labels. Provide Review outcomes at supported checkpoints, using the existing proposal workflow. Playing must not directly publish canon.

**Acceptance:** Opening the existing saved session immediately exposes its scene, turn and provisional status without scrolling past setup. Existing sessions and definitions still load. Any new interaction controls map to supported backend behaviour, and missing capabilities are documented explicitly.

## 6. Priority 5 — approachable World Settings

- Lead with World guide, Style guide and world configuration where supported; group members/roles and approval policy separately from advanced timeline and administration controls.
- Provide focused editing views for guide/style records if those records are supported. Saving canonical configuration must use the existing draft/review path.
- Keep ownership transfer, membership changes and archive/reopen operations in a clearly labelled administration section. Do not expose them on the creative landing surface.
- Preserve configuration history and published/draft distinctions where supplied by the backend.

**Acceptance:** Ordinary guide/style editing does not require interpreting raw record types or identifiers. Unsupported guide/style contracts are recorded as gaps, rather than presented as functioning editors.

## 7. Shared interaction and visual requirements

- Reuse the app's existing theme tokens and components. Preserve the neutral surfaces, restrained accent and literary display typography.
- Reduce repeated borders, fieldsets and oversized header spacing. Establish consistent cards, headings, buttons, status chips and empty/loading/error states.
- Keep explicit text labels for Published canon, Draft, Proposal, Hypothetical and Session state wherever applicable.
- Keep intent detection on Auto by default. Optional intent selection must not become a required wizard.
- Preserve grounded source navigation and typed rich responses. Do not replace structured results with generic Markdown during the refactor.
- Preserve supported progress, retry/cancel and error behaviour; expose understandable states rather than raw backend errors.
- Ensure visible focus, accessible names, sufficient contrast, logical focus order and reduced-motion support. Drawers and sheets must manage focus and return it to the invoking control.
- Use existing light/dark theme support; avoid parallel theme implementations.

## 8. Implementation sequence and verification

1. Inspect the current frontend, applicable repository instructions, API contracts and existing tests. Identify reusable components and make a short implementation map against this brief.
2. Fix layout containment, viewport sizing and composer placement. Verify the required sizes before proceeding.
3. Consolidate the shell and history, preserving routes and conversation context.
4. Improve Explore and proposal review presentation without changing review policy.
5. Build the scene-first story view and entity picker around supported contracts.
6. Improve settings, shared styling and accessibility.
7. Run the existing relevant tests and add meaningful coverage for the changed behaviour: responsive containment, composer visibility, navigation/context retention, source detail, proposal comparison and saved-story opening. Do not claim layout correctness from DOM-only tests; include rendered browser checks.

Verify creation and publication flows only in an authorized disposable active test world or the project's established integration environment. Do not reopen archived worlds, publish proposals or modify production canon solely to complete this UI audit. If the environment cannot support a flow, report it as unverified with the exact dependency.

Deliver the implemented changes, desktop/mobile screenshots, checks actually executed, remaining backend gaps and unverified flows. Treat the attached implementation prompt as authorization to prepare and test changes in the relevant repository, not to deploy them. Do not initiate production deployments, cluster mutations or GitOps rollouts unless separately requested.

## 9. Implementation prompt

> Correct the Worldbuilder frontend using `WORLDBUILDER-UI-CORRECTION-BRIEF.md`, the original `WORLDBUILDER-AI-FIRST-UI.md` and the deployed audit `WORLDBUILDER-DEPLOYED-UI-AUDIT.md`. Follow applicable repository instructions and inspect the actual frontend/API contracts before editing. Implement the correction brief in priority order: first eliminate mobile horizontal overflow and make the composer visible at the bottom of the workspace; then consolidate the shell and conversation history, simplify Explore/review language, build a scene-first saved-story experience and supported character picker, and improve settings and accessibility. Preserve the working routes, authentication, context/source navigation, proposal comparison, independent review and provisional story state. Distinguish presentation gaps from backend capabilities that remain unverified. Use real supported contracts; never fabricate production canon or silently simulate missing features. Validate the specified viewport sizes, context continuity and read-only detail flows, run relevant existing tests and add meaningful regression coverage for the changed behaviour. Finish with actual changes, desktop/mobile screenshots, test results, unresolved API gaps and unverified flows. Do not modify production canon or initiate deployments, cluster mutations or GitOps rollouts.
