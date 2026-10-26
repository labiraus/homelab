Worldbuilder AI — Experience Journeys and Acceptance Scenarios
Status: Proposed behavioural companion to UX-EXPERIENCE-CONTRACT.md.
These are product acceptance scenarios, not assertions about current API availability. All worlds, people and facts in these examples are illustrative test fixtures.
Journey 1 — Understand a world in conversation
Starting point: Reader opens The Shattered Empire, primary timeline, published revision 128.
User: "Why did House Veyr abandon Ashfall?"
Expected UI: The workspace answers in readable prose with linked/clickable sources and explicitly distinguishes witnessed accounts from inferred motives. A compact House Veyr card offers Open, Show relationships and Ask about this. The current timeline and revision remain visible. Following a citation opens a source inspector pinned to revision 128; returning preserves the question and answer.
Avoid: An unsourced answer; a full table of entities; a generic link to a separate wiki search with no retained context.
Acceptance: The sources resolve in the same world and scope; absent evidence produces honest uncertainty; readers cannot mutate canon.
Journey 2 — Co-create a character without losing the draft
User: "Create a young diplomat from House Veyr who secretly supports its enemies."
Expected UI: The assistant produces an editable draft with proposed name, description, motivations, key facts and relevant relationships. The user can say "Make her older and have her loyalty be to Ashfall rather than the rival house" and the same working artefact is updated or a clear candidate revision is presented for review. The draft remains conspicuously noncanonical. An author can save it and enter the existing change-set review process; a reader can suggest where permissions allow.
Avoid: A long AI paragraph followed only by "Copy" or "Open editor"; silently inserting canonical entities; losing edits when opening a related entity.
Acceptance: Generated claims are not shown as published, validation is distinct from AI commentary, and save/refresh guarantees match real persistence capabilities.
Journey 3 — Propose a consequential change
User: "Actually, King Aldric survived the Battle of Ashfall."
Expected UI: The assistant treats this as ambiguous until the user confirms they mean to propose a canon change rather than discuss an alternative scenario. The proposal explains affected death, succession, burial and chronology facts with human-readable original/proposed comparisons and cited dependencies. It explicitly separates hard rule violations from uncertain narrative implications. The user can adjust the proposed operations before review. Publication invokes the existing server-side approval/quorum, revision and atomicity rules.
Avoid: Flipping a deceased field directly; showing a green "success" state before publication; treating a historical established account as freely editable.
Acceptance: Stale revision or missing authority has a recoverable path and never bypasses publication safeguards.
Journey 4 — Explore relationships visually
User: "Show me which factions influence the Council of Ashfall."
Expected UI: A focused relationship view appears as an answer artefact, showing a legible subset with named edges and supporting source links. The user can select a faction to view its facts, expand the neighbourhood or switch to a dedicated Explore view; a Back to conversation action restores the originating task and selected entities.
Avoid: Unbounded node hairballs, unlabeled edges, or a graph disconnected from sources.
Acceptance: Relationships respect world, timeline, revision and permission scope.
Journey 5 — Play, resume, and propose an outcome
User: "Start an adventure as a courier entering Ashfall during the siege."
Expected UI: The story begins as an immersive narrative scene with choices and a freeform action, along with subtle cast/location/session context and explicit private/provisional status. A saved session can be resumed with its actual version and pinned canon context. At a meaningful outcome, the user can review candidate changes and submit them through ordinary proposal governance. Nothing played is canon by default.
Avoid: A standalone chat pretending its turns are durable; silently treating narrated events as official history; exposing private sessions in world activity.
Acceptance: Turn version conflicts and offline limitations are visible, with retries preserving user-authored text.
Journey 6 — Recover gracefully when the backend cannot do it
Scenario: The user refreshes a conversation or goes offline during drafting.
Expected UI: Previously committed story and proposal state is still available according to the API; ephemeral chat is not misrepresented as server-persistent. Local draft/offline status is explicitly labeled and governed by the PWA offline contract. The UI offers export/recovery when supported and does not automatically invoke a paid AI endpoint after reconnecting.
Avoid: Fake saved indicators, unconditional "Resume chat" claims, or silently replaying an approval/publication request.
Acceptance: Actual online/offline identity, pinned revisions, saved versus device-local state and reconciliation semantics are tested distinctly.
Cross-journey visual verification
For each journey, verify desktop at 1440px and responsive states at 1024px, 768px, 390px and 320px in both themes. Verify keyboard and focus order, visible status text, bounded scrolling, the composer remaining reachable, error recovery and accessible action names. Use real Android keyboard evidence before claiming Android acceptance.
Evidence template
For each test record: journey; source revision/commit; API versus fixture; viewport/device; screenshots before/action/after; successful assertions; known blocked behaviours; canonical state before/after; and whether the result is source-only, local integration, or deployed evidence.