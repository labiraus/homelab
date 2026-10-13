# Frontend correction acceptance — 13 October 2026

This source-only correction follows the [correction brief](WORLDBUILDER-UI-CORRECTION-BRIEF.md), [original specification](05-AI-FIRST-UI.md) and [signed-in deployed audit](WORLDBUILDER-DEPLOYED-UI-AUDIT.md). Historical release checks in UI-IMPLEMENTATION.md describe the earlier deployment, not acceptance of this correction.

## Implementation map

1. `styles.css`, `WorldWorkspace.jsx`, `Worldbuilder.jsx`: constrain intrinsic widths, measure available viewport space, reserve composer height, scroll messages and supporting tools independently.
2. `App.jsx`, `Worldbuilder.jsx`, `WorldWorkspace.jsx`: reuse the existing header with world/search slots, move conversation controls to the rail/drawer and retain mounted conversations and exact source pins.
3. `WorldResponse.jsx`, `WorldHistory.jsx`, `Worldbuilder.jsx`: readable record labels/attributes, progressively disclosed filters/policy/JSON and primary three-way comparison.
4. `WorldStorySetup.jsx`, `Worldbuilder.jsx`: pinned canon entity picker and scene-first saved sessions; player directions use the existing `/reason` `story_turn` contract, manual narration remains authoring.
5. World configuration and overlay focus: expose supported settings through reviewed drafts; disclose guide/style API gaps, group administration separately, use shared tokens and accessible drawers/sheets.

All tests use local HTTP fixtures or the established owned disposable PostgreSQL/API harness. No production canon, deployment, cluster resources or GitOps state is changed.

## Contract limits

- World conversations remain in memory; no world-chat persistence or streaming API exists.
- Header search uses the selected world's existing canon/search routes. No universal cross-world canon search/command endpoint is exposed; document search remains available through Document tools.
- The canonical Record contract has no dedicated world-guide/style-guide fields or record kinds. Generic string attributes are supported, but do not establish a named guide/style contract or model consumption. Dedicated guide/style editors must wait for that contract.
- Story definitions provide private premise/objective/style/participants. `story_turn` processes player directions at the session's inspected version and returns candidates; explicit save persists private state, not canon. No Sandbox mode, structured location or inventory contract is exposed. Never infer these from prose.
- Browser cancellation stops waiting; it cannot prove provider cancellation or a refund.

## Validation

| Check | Result |
|---|---|
| `npm test` | 84 tests pass across 16 files, including original-pin configuration, named cast selection, lookup refusal, generated direction/version and archived scene regressions. |
| `npm run build` | Production Vite build passes. Generated `dist/` remains local and ignored. |
| `npm run test:workspace` | Eight Chromium journeys pass: existing conversation/draft/story paths; viewport matrix; modal focus/return and context continuity; archived three-way comparison; saved scene/picker; long reply/multiline scrolling. |
| Disposable actual API browser gate | `external/worldbuilder` race suite passes with all three actual Chromium governance/story/branch journeys. Independent approvals, privacy, stale pins and reconciled outcomes remain asserted. |
| Full disposable PostgreSQL harness | Passes: migrations 001–008, populated upgrade/conflict gates, role isolation, canon/lifecycle/story/outbox integration, native OAuth restore refusal and all-table restore checksum. Owned fixture cleanup verified. |
| `git diff --check` | Passes. |

The full `scripts/worldbuilder/test-postgres.sh` run passes. It used `WORLDBUILDER_TEST_TRANSPORT=exec`, `WORLDBUILDER_BROWSER_TEST=1` and loopback fixture credentials; it never sourced an operator DSN. All labelled disposable PostgreSQL containers are gone after completion. The initial two harness runs failed/timed out on obsolete presentation selectors and an obscuring creation menu; neither is counted as success. Menu closing is fixed, readable-label/disclosure expectations updated, and actual API tests now pass. Fixture and API Playwright suites are selected separately to preserve the bounded Go gate.

At widths **320, 390, 768, 1024 and 1440px**, rendered document width stays within the viewport on Workspace, Explore, Drafts & Proposals, Stories and Settings. The full composer is visible at **1366×768, 1440×900 and 390×844**, with additional 320/768/1024 checks. Long replies scroll independently, multiline input grows to its bound and then scrolls, and narrow comparisons stack with Original / Current canon / Proposed labels. Archived tests include an unusually long world name. Keyboard checks cover drawer/sheet focus containment, Escape and return; desktop tests retain real hit-target checks after scroll/resize.

## Screenshots and retained evidence

These screenshots contain **local synthetic HTTP-fixture content**, not production canon. Artifacts are retained for this session in `/tmp/worldbuilder-correction-evidence/`:

- [Desktop, 1440×900](/tmp/worldbuilder-correction-evidence/desktop-1440x900.png) and [1366×768](/tmp/worldbuilder-correction-evidence/desktop-1366x768.png).
- [Mobile, 390×844](/tmp/worldbuilder-correction-evidence/mobile-390x844.png) and [320px](/tmp/worldbuilder-correction-evidence/mobile-320.png).
- [Saved scene on mobile](/tmp/worldbuilder-correction-evidence/story-mobile.png).
- [Proposal comparison on mobile](/tmp/worldbuilder-correction-evidence/proposal-mobile.png) and [desktop](/tmp/worldbuilder-correction-evidence/proposal-desktop.png).
- [Metadata, viewport list and compiled-asset hashes](/tmp/worldbuilder-correction-evidence/metadata.json); component/browser logs are in the same directory. These hashes identify local artifacts, not signatures or deployment evidence.

## Unverified flows

- A real mobile software keyboard, browser-chrome/safe-area interaction, assistive screen reader and systematic contrast audit. Dynamic visual-viewport sizing, safe-area padding, focus and existing themes are implemented, but resized desktop Chromium is not real-device evidence.
- Actual provider quality/latency, live player directions, generated turns/proposal consistency and durable provider cancellation. Player-direction contract and version-bound explicit saves have local fixture coverage; the actual API story journey uses human-authored narration.
- Production authenticated active-world generation, creation/publication, conflicts and outcome review after this correction. The supplied deployed audit covers archived reads; no world was reopened or production canon changed for validation.
- Production acceptance of these source changes. No commit, push, publication, deployment, cluster mutation or GitOps rollout was initiated. The user's pre-existing devcontainer-lock change and supplied brief/audit files were preserved.
