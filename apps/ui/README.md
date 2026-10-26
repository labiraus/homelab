# Worldbuilder frontend

React/Vite PWA for Worldbuilder and retained Labiraus document tools. Read the
[experience and journeys](../../docs/worldbuilder/experience.md) before UX changes,
[API](../../docs/worldbuilder/api.md) for authority/persistence, and
[status](../../docs/status.md) for dated evidence and release gaps.

## Local development

Run from `apps/ui`:

```bash
npm ci
npm run dev
```

The development server listens on `0.0.0.0:3000`. Set `VITE_API_BASE_URL` when
using a separately configured API origin. Production build is `npm run build`;
`dist/` is generated and must not be committed.

## Tests and image

```bash
npm test
npm run build
npm run test:workspace
npm run test:pwa
```

Install the pinned Playwright Chromium and OS dependencies first. Build once before
browser gates and keep `dist` unchanged until they finish. Workspace tests use HTTP
fixtures; PWA tests use the real production worker with fixture APIs. For real
SQL/API Chromium journeys run, from repository root:

```bash
WORLDBUILDER_TEST_TRANSPORT=exec WORLDBUILDER_BROWSER_TEST=1 bash scripts/worldbuilder/test-postgres.sh
```

`test:browser` selects those actual API journeys and requires that launcher.
Artifacts use `WORLDBUILDER_BROWSER_ARTIFACTS` or `/tmp/worldbuilder-browser-artifacts`.
An owned loopback forward with `WORLDBUILDER_BROWSER_URL=http://127.0.0.1:19473`
lets workspace fixtures inspect deployed assets; it does not authenticate production
API/model journeys. See [testing](../../docs/testing.md).

`dockerfile` builds locked dependencies with `npm ci` and serves static output via
Nginx. Docker context excludes local dependencies/output/env. CI gates publication
on components, workspace and real API/browser checks, then validates actual index/
compiled assets with `browser/image-smoke.py` inside the image with networking off.

## Code and routes

| Area | Source |
| --- | --- |
| World shell, routing and review handoffs | `src/Worldbuilder.jsx` |
| Threads, pins, composer and result continuity | `src/WorldWorkspace.jsx`, `src/worldWorkspace.js` |
| Answers, sources and editable candidates | `src/WorldResponse.jsx`, `src/WorldDraftArtifact.jsx` |
| Structured records, governance and reconciliation | `src/WorldRecordEditor.jsx`, `src/WorldAdministration.jsx`, `src/WorldHistory.jsx` |
| Scoped graph/chronology and branch origin | `src/WorldViews.jsx`, `src/WorldTimelineBranch.jsx` |
| Private setup and scenes | `src/WorldStorySetup.jsx`, `src/WorldStoryScene.jsx` |
| Device writing and immutable queue | `src/worldOffline.js`, `src/useLocalWriting.js` |
| Shared styles | `src/styles.css`, `src/editorial.css` |

World links use `#/worlds/{world}/{workspace|explore|drafts|stories|settings}`.
Source links carry `record`, `revision`, `timeline`, optional exact `at`; proposals
use `proposal` and stories use `session`. Preserve hash links through authentication
loading. Exact fictional ticks require JSON source-token support and decimal strings
outside JavaScript's safe range; unsupported browsers report update-required.

Shared messages/final results persist when the API advertises support; private Play
stays private. Unsaved candidate edits survive mounted section/thread navigation
and polling but not reload/world/account exit. Explicit save receipts distinguish
device queueing from server persistence. Never repin or replay inference implicitly.

The account menu retains online document tools: inventory, scan, search/context,
history, guarded text edits/reprocess, object browser/upload and assistant/audit
routes. `/api/documents/events` supplies best-effort notifications. Static legal
pages remain at `/privacy-policy.html` and `/terms-of-service.html`.

Inference provenance distinguishes free local, estimated cloud, reserved/uncertain
and historical allowances. Profile selection is deployment configuration; there is
no browser paid-provider toggle.
