Worldbuilder AI — User Experience Contract
Status: Proposed product-level clarification for docs/worldbuilder/.
Authority and compatibility
This document describes how using Worldbuilder should feel and behave. Read it with 05-AI-FIRST-UI.md and PWA-DESIGN.md. It does not supersede established permissions, offline constraints, immutable accounts, publication rules, story state, source provenance, or accessibility requirements. Where those requirements limit a proposed interaction, show an honest, comprehensible fallback instead of inventing capabilities. UI-IMPLEMENTATION.md records implementation evidence, not product intent.
Product promise
Worldbuilder is an AI-native collaborative writing studio centred on a shared, authoritative fictional world. The user collaborates with an assistant that can explain the world, propose additions, expose tensions in established lore, develop narrative drafts, and facilitate interactive stories. Users should rarely need to think in terms of database records, schemas, service calls, or governance plumbing.
The AI is an interface to the world, not its authority. The world and its approved history are authoritative. A generated suggestion is not published fact, and playing a story never silently changes canon.
The central workspace
A selected world opens directly into a creative workspace with an immediately available natural-language composer. The workspace is a living working surface, not a chatbot sidebar attached to a management dashboard.
The central column shows a coherent sequence of user intent, assistant responses, citations, and interactive artefacts. Artefacts should stay attached to the task that created them and support relevant next actions in place. The user should be able to shift between talking about a subject and manipulating a concrete proposed result without losing the conversational thread or world/timeline/revision context.
The supporting navigation, explorer, inspector, story library and review queue are escape hatches and focused workspaces, not hurdles required before asking the AI to do useful work. Opening details must preserve a clear return to the originating task.
First five seconds
On opening a world, the user can immediately answer: Which world am I in? Which timeline/revision am I using? What can I ask the AI? What have I been working on? What is draft versus established canon? The composer is the most prominent actionable element. Do not present a dense metrics dashboard, a mandatory intent wizard, or a blank technical chat tool.
Composition and continuity
As the user navigates, preserve the relevant task, selected entities and context. Separate three kinds of continuity:
1. Published knowledge: authoritative, cited, revision-scoped canon.
2. Creative working material: user text, suggestions, drafts and change sets that are not canon.
3. Conversation or story context: working thread and private session state, with explicit persistence guarantees.
Do not conflate local component state, durable server state and authoritative world state. Current APIs may not persist ordinary chat: show that honestly and preserve local work where supported; never label ephemeral chat as saved.
Interaction grammar
The user can Ask, Create, Change, Explore or Play using natural language. These are kinds of intent, not five separate chat products or a compulsory mode-selection sequence. The assistant may infer an intent when safe and provide a small, editable indication. Ambiguous mutation requests require an inline clarification. All canonical changes follow explicit proposal, review and publication gates.
Ask
Offer a direct, readable answer with resolvable citations, uncertainty when evidence is weak, contextual entities and relevant follow-up actions. Clicking a cited fact leads to its exact source scope, without silently changing the whole conversation context.
Create
Turn an idea into an editable creative artefact, not just a prose blob or a redirect to a form. Keep the generated description, structured properties and relationships accessible together. Refinement prompts should target that artefact; the user can inspect differences before saving or proposing canon changes. Clearly mark unapproved content as draft.
Change
Translate a request into a human-readable explanation of what would change and why. Show affected entities, conflicting facts, deterministic validation, advisory AI findings, original and proposed values, and consequences. Never silently overwrite or publish. Existing established accounts retain their immutable legal/chronicle meaning under the established-account contract.
Explore
Use whichever representation best answers the question: prose, focused entity cards, relationships, chronology, source detail or an expanded explorer. Prefer a small comprehensible view around the subject over rendering the complete world's data model. The user can return to the conversation with selected context intact.
Play
Enter an immersive but still legible narrative flow. Show scene prose, choices and freeform actions, relevant cast/location/session state, and visible private/provisional status. Session saves and turns follow the actual engine. A story outcome may become a reviewable proposal; it never directly becomes canon.
Rich artefact contract
A response may include a grounded answer, entity preview, editable draft, scoped timeline, relationship view, proposed operations, review comparison, or interactive story scene. The result must be usable, not merely decorated Markdown.
For each artefact show (where applicable): title and type, published/draft/proposal/private status, world/timeline/revision scope, source/provenance, primary next action, secondary inspect/edit actions, loading/error/retry/saved state, and the distinction between AI suggestions and deterministic validation. Avoid exposing raw JSON except under advanced disclosure.
Use focused views or contextual sheets for complexity. Do not force every operation through a dense inspector or a full-screen form. Do not hide permissions or revision conflicts behind generic failure toasts.
Visual hierarchy
Creative content leads. Controls are calm and restrained. The app's neutral editorial appearance is independent of the genre of the world. Use the typography, colors, accessibility and responsive constraints in PWA-DESIGN.md. On desktop the conversation owns most of the canvas; the world rail and contextual inspector are subordinate. On mobile there is one focused working surface, not a squeezed three-column desktop.
Non-negotiable anti-patterns
- A CRUD dashboard with a chat bubble added as a secondary feature.
- Five disconnected forms or chat modes for Ask/Create/Change/Explore/Play.
- A response that says "created" when only unapproved text was generated.
- Generated lore presented as published fact without provenance.
- A useful suggestion that can only be acted on by starting over in another screen.
- A generic markdown transcript when the result calls for an editable draft, comparison, scoped chronology or story controls.
- Raw IDs, schema fields, HTTP terminology or JSON as the everyday language of creative work.
- A fictional-world themed game HUD or developer console that overwhelms readable writing.
- UI-only approval, offline publication, or any shortcut around server authority.
- Pretending conversations, offline AI, streaming, inventory or arbitrary response blocks exist when the backend lacks them.
Decision test for Codex
Before implementing a UI feature, answer in the change summary:
1. What user intention does this support?
2. What is the direct path from natural-language intent to a useful, manipulable result?
3. What is authoritative canon here, and what is provisional?
4. What context survives navigation, failure and refresh, and what does not?
5. Does the interface feel like working with an intelligent creative partner or administering a database?
6. Which actual API/data contract supports each displayed claim?
7. How does the experience work on narrow screens and with keyboard-only input?
If answers 2, 3, 5 or 6 are weak, the feature is not finished even if it renders and tests pass.
Acceptance standard
Use UX-JOURNEYS.md as behaviour-level acceptance. For each journey verify the visible initial state, prompt, intermediate UI, provenance and status, inline follow-ups, permission/conflict handling, and return path. Test with fixture and actual API where available; label each evidence tier. Do not assert deployed or Android acceptance from a desktop fixture.