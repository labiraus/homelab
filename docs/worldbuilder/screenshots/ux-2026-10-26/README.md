# Experience correction evidence — 26 October 2026

These are real Chromium screenshots of local UI rendering with isolated HTTP
fixtures. Characters, answers and proposals are test data. They do not prove
production authentication, live-model quality, current deployment health or real
Android keyboard/installation/screen-reader behaviour.

Baseline: commit `97e0d1679108aeb9871ee4216962a1860a38a76f`, captured from a
preserved pre-change production bundle. After: uncommitted experience correction;
exact source/build/screenshot hashes are in [evidence.json](evidence.json).
The [audit](../../UX-AUDIT-2026-10-26.md) separates journey gaps and evidence tiers.

| Moment | Desktop | Mobile |
| --- | --- | --- |
| Before: existing Ask / private story | [1280×720, light](before-workspace-desktop.png) | [390×844, dark](before-workspace-mobile.png) |
| Ask → exact published source | [1440×900, light](ask-source-light-1440.png), [dark](ask-source-dark-1440.png) | [390×900, light](ask-source-light-390.png), [dark](ask-source-dark-390.png) |
| Create → inline edited draft | [1440×900, light](inline-draft-light-1440.png), [dark](inline-draft-dark-1440.png) | [390×900, light](inline-draft-light-390.png), [dark](inline-draft-dark-390.png) |
| Saved proposal → conversation return | [1440×900, light](review-return-light-1440.png), [dark](review-return-dark-1440.png) | [390×900, light](review-return-light-390.png), [dark](review-return-dark-390.png) |

Inline editing is also retained at
[320 light](inline-draft-light-320.png) / [dark](inline-draft-dark-320.png),
[768 light](inline-draft-light-768.png) / [dark](inline-draft-dark-768.png), and
[1024 light](inline-draft-light-1024.png) / [dark](inline-draft-dark-1024.png).

The journey assertions inspect exact source and reasoning scope, edited operation
payloads, shared-result refresh, retained local edits, detailed-editor application,
keyboard Name → Description order, source context, independent composer bounds,
explicit draft save, review submission and return. Fixture canon stays at revision
7; no publication request is sent. Refinement returns a labelled second candidate;
it does not silently replace the previous working draft.

Retained logs: [baseline](baseline.log), [build](build.log),
[100 component tests](components.log), [23 workspace tests](workspace.log),
[10 strengthened detailed-editor journeys](detailed-journeys.log),
[12 PWA tests](pwa.log). PWA uses the production service worker with fixture APIs;
its Axe/viewport checks cover the shell in both themes. These are separate from
the disposable Go/PostgreSQL/browser/restore harness recorded in the audit.

The [final-bundle API/PostgreSQL log](api-postgres.log) records the separate real
Go/SQL gate, including all three actual-API browser journeys, migration/security
and complete restore checks. [API canon-view capture](api-canon-views.png) comes
from that disposable authenticated fixture, not a production world. The final
owned database container was removed and absence verified. Its evidence does not
turn the synthetic AI answers above into live-model acceptance.

Only the transcript scrolls while the composer remains reserved. A screenshot
of a focused field may show a clipped earlier message at the transcript boundary;
that is distinct from document-width overflow or a hidden composer. This is still
a compact working area at 320px; real software-keyboard acceptance remains open.
