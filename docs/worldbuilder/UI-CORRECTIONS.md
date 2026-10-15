# Frontend correction acceptance

This correction follows the [correction brief](WORLDBUILDER-UI-CORRECTION-BRIEF.md), [original specification](05-AI-FIRST-UI.md) and [signed-in deployed audit](WORLDBUILDER-DEPLOYED-UI-AUDIT.md). The latest source review and last verified deployment checkpoint are separate below; historical success does not establish present deployment health.

## World library and menu review — 15 October 2026

Source-only follow-up to the reported inert-looking Worlds destination and disappearing World menu. The old `#/worlds` destination rendered an introduction and header selector without a world library. Opening the old menu applied fixed positioning to the entire `details` element, moving its summary out of the header into the panel.

The Worlds button now leads to a named library with searchable loaded world names, access and archive labels, workspace links, pagination and explicit loading/empty/error states. It remains reachable at mobile widths. Cards use the existing neutral surfaces, accent and literary headings; long titles truncate visually while retaining the full accessible text and title.

World menu now has a bordered, stationary button with a chevron/expanded state and a separate popup. Browse all worlds, Create a world and Account administration have clear rows; the creation form opens on demand. Escape restores trigger focus, while outside clicks, focus leaving the panel and route changes dismiss it. Successful creation/selection closes it, and creation errors retain the name beside an enabled retry action. Ordinary accounts receive an administration-access explanation rather than an empty disclosure.

| Check | Result for this source revision |
|---|---|
| `npm test` | **86 tests pass**, including list failure/retry and rejected creation state. On this Windows Node 26 host, `NODE_OPTIONS=--no-experimental-webstorage` is required so jsdom supplies browser storage. |
| `npm run test:workspace` | **13 Chromium journeys pass** against local synthetic HTTP fixtures. Four new journeys cover menu geometry/dismissal, library routing/search/back navigation, both themes at 320/390/768/1024/1440px with long archived names, and empty-library creation/closure. The nine existing composer, source, review and story journeys also pass. |
| `npm run build` | Passes with the locked dependencies installed for Windows. |
| Visual review | Desktop library/menu and 320/390px light/dark library/menu screenshots inspected. Narrow header controls stay on one row; expanded menu bounds remain inside the viewport. |
| Actual API/PostgreSQL browser gate | **Not rerun in this session.** The Windows host lacks `psql` and a usable Python installation for the Linux fixture harness; its running devcontainer belongs to another project. The actual API browser selector now explicitly opens Create a world. This gate remains required before publication. |
| Deployment and live acceptance | No deployment, production canon changes or new production authenticated evidence. The 13 October release evidence below is retained as a historical checkpoint. |

Local fixture screenshots are retained under `.local/ui-review-20261015-evidence/`: [desktop library](../../.local/ui-review-20261015-evidence/worlds-desktop.png), [desktop menu](../../.local/ui-review-20261015-evidence/world-menu-desktop.png), [320px library](../../.local/ui-review-20261015-evidence/worlds-320-light.png), [390px menu](../../.local/ui-review-20261015-evidence/world-menu-390-light.png), and [dark menu](../../.local/ui-review-20261015-evidence/world-menu-390-dark.png). These ignored local artifacts contain synthetic data only; they are not shipped assets or production screenshots.

World-chat persistence/streaming, dedicated guide/style APIs, structured story inventory/location and Sandbox mode remain contract gaps. Real software-keyboard, screen-reader, systematic contrast and production authenticated journeys remain unverified. This iteration preserves the existing conversation, canon-review and story contracts.

## Deployment and repeated design review — 13 October 2026

### Verified release checkpoint

Follow-up source `ee1feba65f1a815398cb7766ed6a812557970f88` passes complete [UI CI/CD 38405146042](https://github.com/labiraus/homelab/actions/runs/38405146042), including the new responsive gate, existing actual API/database browser acceptance, all eight platform gates and image/chart publication. Automatic Flux reconciliation installs Ready Helm revision **60** at 13:39:42 UTC. The exact source is recorded in OCI metadata; chart digest is `sha256:a85e955c97bb514d4576fcf8292977714981ad99c962ee1f7140fbb85e5563fb`, and the UI image digest is `sha256:2d80e57973dcc004d38452cf1d6df0e8c5b678c8bc90d168c6acb7cf062c8817`. UI and mesh sidecar are Ready with zero restarts; the Service has exactly one Ready endpoint pointing to the replacement Pod.

All **nine** workspace journeys pass again against that Pod's compiled assets, and all **ten** served files match the verified build byte-for-byte. Final public TLS/discovery and anonymous/forged-header refusal checks pass independently. The owned loopback forward is closed and its listener is gone. Kubernetes readyz and all five application Deployment availability checks pass; deployed external's source `ceaeb29` has no differences from the tested current external/shared Go code, and its workload image matches its chart's app version. These checks do not establish production Google/model/active-world acceptance.

Retained evidence: [checksum manifest](/tmp/worldbuilder-reevaluation-final-evidence.json), [release/Pod/endpoint identity](/tmp/worldbuilder-reevaluation-final-release.json), [served-asset equality](/tmp/worldbuilder-reevaluation-final-assets.json), [public checks](/tmp/worldbuilder-reevaluation-public-final.json), [terminal CI jobs](/tmp/worldbuilder-reevaluation-final-ci-jobs.json), and screenshots under `/tmp/worldbuilder-reevaluation-deployed-final/`. All browser screenshots use explicit synthetic API fixtures; they contain no production canon.

### Initial rollout and source failure

Initial pushed source `094db4f4fc30428cee48d269938c236654007259` passes complete [UI CI/CD 38402588759](https://github.com/labiraus/homelab/actions/runs/38402588759), including the UI/database browser checks, all eight platform gates, image smoke and chart publication. Automatic Flux reconciliation installs UI Helm revision 59, Ready at 13:21:13 UTC. Its OCI metadata identifies that exact source, chart digest `sha256:0020350a4cb7c2ff5be3817b2c0956b9c21776b61f0ebe04fe97bf027b8e004d` and workload image digest `sha256:66dc0a0dab4732c6ea71fb54d31263ee68c07cfff7462a9d7f1968bbffd2b7ab`; the UI Pod is Ready with zero restarts. Served JS/CSS match the retained correction-build hashes byte-for-byte.

All nine workspace journeys pass against the actual Pod's compiled assets through an owned HTTP loopback forward, with explicit synthetic API fixtures. Required viewport containment, composer bounds, navigation/source focus and continuity, readable archived comparisons, scene-first story/pinned cast and multiline scrolling pass. Desktop/mobile screenshots were inspected. This proves shipped frontend behavior, not authenticated production API/model integration. Public TLS, native discovery and anonymous/forged-header refusal pass independently in `/tmp/worldbuilder-reevaluation-public-before.json`.

Repeated source testing exposes a reduced-height focus failure at 390×500 in Vite, although the same check passes against the deployed production build. The follow-up explicitly brings a focused composer above the visible bottom on resize and updates its height on document scroll. Source checks pass all 84 component tests, the build and all nine workspace journeys. CI now requires that responsive suite before image publication, separately from real API/database journeys; failed traces/screenshots are retained. The workspace config accepts an owned loopback `WORLDBUILDER_BROWSER_URL` for served-asset checks and rejects external fixture hosts. The accepted follow-up above supersedes this initial source failure.

### Current design assessment

| Design requirement | Repeated assessment |
|---|---|
| Default conversation, compact world/search header and rail history | Passes against deployed assets; section/world isolation remains asserted |
| Immediately visible composer, no page overflow, optional inspector | Passes six viewport sizes from 320 to 1440px, multiline replies and focus/return checks; reduced-height source behavior is hardened separately |
| Plain-language exploration and three-way proposal review | Passes readable blue/green comparison and progressive disclosures; API governance is separately covered by hosted disposable acceptance |
| Saved story scene and named cast picker | Passes scene-first resume and pinned entity search; actual live model/player quality remains unverified |
| Supported settings through reviewed drafts | Source/component checks preserve the original revision; dedicated guide/style APIs remain unavailable |
| Production authentication and safety | Public refusal/discovery checks pass; no legitimate Google-proxy browser session is available for active-world production journeys |
| Broader product/device capabilities | World-chat persistence/streaming, cross-world search, dedicated guides/style, structured inventory/location and Sandbox mode remain backend gaps; real keyboard/screen-reader/contrast acceptance remains open |

The user confirms the Google audience has not been changed. Proxy wildcard domains and no login `hd` filter are already verified; External audience and registered outside-domain sign-in remain pending. No authenticated Google Cloud console/CLI access is available in this session to make that audience change. No production canon is modified or archived world reopened for this review.

## Implementation map

1. `styles.css`, `WorldWorkspace.jsx`, `Worldbuilder.jsx`: constrain intrinsic widths, measure available viewport space, reserve composer height, scroll messages and supporting tools independently.
2. `App.jsx`, `Worldbuilder.jsx`, `WorldWorkspace.jsx`: reuse the existing header with world/search slots, move conversation controls to the rail/drawer and retain mounted conversations and exact source pins.
3. `WorldResponse.jsx`, `WorldHistory.jsx`, `Worldbuilder.jsx`: readable record labels/attributes, progressively disclosed filters/policy/JSON and primary three-way comparison.
4. `WorldStorySetup.jsx`, `Worldbuilder.jsx`: pinned canon entity picker and scene-first saved sessions; player directions use the existing `/reason` `story_turn` contract, manual narration remains authoring.
5. World configuration and overlay focus: expose supported settings through reviewed drafts; disclose guide/style API gaps, group administration separately, use shared tokens and accessible drawers/sheets.

The original source-only implementation used local HTTP fixtures and the established owned disposable PostgreSQL/API harness. Its validation changed no production canon, deployment, cluster resources or GitOps state; the later authorized GitOps releases are recorded above.

## Contract limits

- World conversations remain in memory; no world-chat persistence or streaming API exists.
- Header search uses the selected world's existing canon/search routes. No universal cross-world canon search/command endpoint is exposed; document search remains available through Document tools.
- The canonical Record contract has no dedicated world-guide/style-guide fields or record kinds. Generic string attributes are supported, but do not establish a named guide/style contract or model consumption. Dedicated guide/style editors must wait for that contract.
- Story definitions provide private premise/objective/style/participants. `story_turn` processes player directions at the session's inspected version and returns candidates; explicit save persists private state, not canon. No Sandbox mode, structured location or inventory contract is exposed. Never infer these from prose.
- Browser cancellation stops waiting; it cannot prove provider cancellation or a refund.

## Original source validation

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
