# Worldbuilder deployed UI comparison

Inspected 13 October 2026. Target: https://mcp.labiraus.com. Reference: `C:/Users/oliver.hathaway/Documents/WORLDBUILDER-AI-FIRST-UI.md`.

## Overall assessment

The deployed application implements a substantial portion of the specification's information architecture and canon-review UI. It opens a selected world in Workspace, provides all five primary sections, uses the specified composer placeholder, exposes revision/timeline context, supports an optional source inspector, and displays readable proposal comparisons. The main remaining gaps are the prominence of the conversation/composer, mobile layout, technical language, and the immersive story experience.

This is a signed-in inspection of deployed UI, not a source-code or backend-security audit. All 12 worlds available in the account's selector were labelled synthetic and archived. The principal inspected world was `Owned live AI acceptance (synthetic)`, revision 1, main timeline. Archived state prevents meaningful verification of creation, publication and new story turns. Disabled write controls in this world are not evidence that those features are missing.

No worlds, records, proposals or story turns were created or changed. The document's implementation instructions were treated as reference material, not authorization to redesign or deploy the application.

## Comparison

| Spec requirement | Observed deployment | Assessment |
|---|---|---|
| Opening a world lands in the AI workspace | Selecting a world navigated to its `/workspace` hash route | Matches |
| Workspace, Explore, Drafts & Proposals, Stories, World Settings | All five sections are present and navigable | Matches |
| Composer text and starter actions | Exact placeholder is present; Ask, Create, Change and Start adventure starters appear on desktop | Matches visible structure; writes untested |
| Sticky bottom composer, immediately usable | On the desktop view, the textarea begins at approximately y=989 in a viewport approximately 996px high. The composer is relatively positioned in ordinary page flow, with no sticky/fixed ancestor | Does not match |
| World selector and universal search in top bar | World selector is a separate large row below the global header. Search/change context is another row below the world heading. Worlds, Overview and Document tools remain global navigation | Partial; competing navigation and excessive vertical chrome |
| Conversation history on left, conversation takes precedence | Section navigation is on the left, while New conversation and Recent conversations are above the centre pane | Partial; history is not integrated into the left navigation |
| Optional closeable contextual inspector | Inspect City gate opens a right-hand inspector with Published canon, revision, timeline, description and a source link | Matches inspected case |
| Explicit removable entity context | Ask AI about City gate returns to Workspace and adds Remove context · City gate · r1 · main | Matches inspected case |
| Browse canon with search/type/time context | Explore has timeline, historical revision, fictional time, search and record-type controls; graph and chronology sections are present | Present; graph usefulness untested with the sparse world |
| Readable canon pages and nontechnical language | Record cards show useful descriptions, but raw type labels such as `world_settings` and explanatory database-style language remain prominent | Partial |
| Human-readable proposal review | Compare proposal with current canon shows Original, Current canon and Proposed; inspected example changes City gate from blue to green | Matches basic comparison |
| Review filters, status and approvals | All proposals, My drafts, Needs review, Conflicts, Published and Rejected filters appear. Draft/published status and approval counts/history are visible | Matches visible structure; publication enforcement untested |
| Technical details progressively disclosed | Raw JSON is exposed through Inspect operations, but the normal review surface also carries a long policy explanation and technical terminology | Partial; retain JSON as advanced detail and simplify default review |
| Story library and immersive player | A saved story opens with its narration, pinned revision, turn number and private provisional state. Setup form remains above the scene; Cast entity IDs (comma-separated) and Next narration/Save turn with staged changes are prominent | Partial; saved-state viewing works, player experience is still form-heavy |
| Story outcomes reviewed before becoming canon | UI states that canon-eligible outcomes require review and provisional state is private | Visible policy only; outcome extraction and publication untested |
| World/style guides with history and approachable settings | Settings directs users to edit published records through Explore; timeline branch and membership/administration controls dominate | Partial; no dedicated guide/style editing experience observed |
| Calm creative design | Neutral surfaces, restrained blue accent and a serif workspace heading align with the direction | Broadly matches; secondary screens rely heavily on fieldsets and generic forms |
| Mobile single column without overflow, bottom composer | At 390px width, document scroll width was 490px. Administration/world toolbar elements extend beyond the viewport; the composer remains far down the page | Does not match |
| Accessibility, themes, streaming, rich AI result types | Visible controls and some focus styling were observed, but no screen-reader, contrast, theme, streaming or full response-card audit was performed | Unverified |
| Existing auth/network patterns, no cross-world leaks or direct LLM calls | Google sign-in works for this inspection | Remaining architecture/security claims unverified |
| All five end-to-end flows and frontend test/documentation coverage | Read-only portions inspected; source repository, test results and frontend documentation were not available in this audit | Unverified |

## Recommended order of work

1. **Fix mobile overflow and composer placement.** Make the conversation pane fit the viewport, keep the composer visible at its bottom, and constrain the world/administration toolbar. Verify at 390px and narrower widths.
2. **Compress the shell.** Put the world selector into the top bar, consolidate context/search, integrate conversation history into the left navigation, and move administration behind account/settings controls. Reduce the vertical distance to the first prompt.
3. **Remove technical requirements from creative flows.** Replace Cast entity IDs with a searchable character picker; use friendly record-type names; put revision-policy detail behind an explanation affordance.
4. **Make resumed stories the main canvas.** Collapse setup once a session opens. Prioritize scene narration, choices/freeform action and contextual characters/location. Keep provisional-state editing available on demand.
5. **Finish the authoring/review presentation.** Make readable comparisons the obvious review entry point; keep raw operation JSON in advanced detail. Add focused world/style guide views while preserving reviewed publication.
6. **Validate active-world flows.** Use a suitable non-archived test world to verify cited answers, editable generated drafts, conflict handling, independent approval, stale-revision handling, persistence and outcome proposals. Then perform keyboard/screen-reader and theme checks.

## Evidence

- `deployed-desktop.png`: world-selection landing page.
- `deployed-workspace.png`: desktop workspace before opening details.
- `deployed-mobile.png`: 390px viewport capture showing 490px document width.
- `deployed-explore.png`: canon filters and record cards.
- `deployed-inspector.png`: source inspector.
- `deployed-drafts.png`: review queue and publication history.
- `deployed-proposal.png`: expanded raw operation details.
- `deployed-diff.png`: readable Original/Current/Proposed comparison.
- `deployed-stories.png`: story setup and saved-session entry.
- `deployed-story-session.png`: opened saved story and provisional state.
- `deployed-settings.png`: settings and world administration.

Screenshots taken after navigation or scrolling can show sticky header positioning or a retained source inspector. Those states should not be confused with the initial landing layout.
