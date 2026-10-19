# PWA local acceptance — 19 October 2026

This report applies to the working-tree source and locally built artifacts. No image,
chart, migration or website was published or deployed as part of this task.

## Completed source and fixture checks

| Check | Result | Evidence scope |
| --- | --- | --- |
| UI component suite | 95 passed / 17 files | Local Vitest, including durable queue, ordering, quota, exact pins and account separation |
| Production PWA suite | 12 passed | Real Chromium worker, isolated HTTP fixture, offline restart, explicit save/replay, pending-work update, HTTP 401 preservation, cross-tab sign-out |
| Responsive workspace suite | 13 passed | HTTP fixture journeys, focus/drawers, scoped references, scene resume and composer containment |
| Axe accessibility and screenshots | 10 layouts passed | Both themes, 320/390/768/1024/1440px, WCAG 2 A/AA and 2.1 AA automated checks |
| Processor build/test | 62 passed, 2 environment-dependent skips | Prompt/contract regressions; skipped database/embedding integrations are not live-model evidence |
| External Go modules | Passed | Ordinary tests; endpoint-dependent cases require the separate disposable run |
| Orchestrator Go module | Passed | Ordinary tests; not deployed dispatch evidence |
| Disposable PostgreSQL/API harness | Passed, including 3 actual-API browser journeys | Migrations 001–009 rerun/empty rollback/populated upgrade, roles/policies, established ledger/supersession/history, exact dates, shared conversations, private stories, outbox rollback, runtime grants and all-22-table plus identity-registry restore |
| Native MCP authorization/restore | Passed | Disposable OAuth database revocation and restored-consent checks; not deployed browser login |

## Production artifact identity

- `index-CSCQUVxP.js`: SHA-256 `7637b8ef8a230c4f0cde1e2ccf496727ce8263a7fc0678683f1fc2fa7de63b18`
- `workbox-window.prod.es5-BBnX5xw4.js`: SHA-256 `cadd21be4a88598a4dec91a26a025df0a5790ff6baec54a884e401c3f5c95557`
- `sw.js`: SHA-256 `e3b4c04eea5a4a1fb60af8dc7ed78b230f7d6874a023e2b077e87d71efea735c`

[Responsive screenshots](screenshots/pwa-2026-10-19/README.md) are retained in the repository.
CI retains production PWA screenshots and traces through the UI workflow.

## Reproduction

From `apps/ui`, run `npm ci`, `npm run build`, `npm test`,
`npm run test:workspace` and `npm run test:pwa`. Install the lockfile's Playwright
Chromium and its OS dependencies first. `WORLDBUILDER_BROWSER_ARTIFACTS` controls
retained screenshot/trace output. Finish the build before starting browsers; keep
`dist` unchanged throughout the run.

From the repository root, run:

```sh
WORLDBUILDER_TEST_TRANSPORT=exec WORLDBUILDER_BROWSER_TEST=1 bash scripts/worldbuilder/test-postgres.sh
```

The completed disposable run ended with:

```text
ok external/worldbuilder 57.529s
ok orchestrator 1.535s
ok external/mcpauth 1.428s
ok external 1.184s
PASS: native MCP authorization database restore requires fresh consent and retains client identities
PASS: migrations 001–009 rerun/rollback, populated upgrade/conflict gates, role isolation, canon/lifecycle/story/outbox integration and all-table restore checksum
```

The application-worker suite subsequently passed all 12 cases on the final bundle.
PostgreSQL/API and workspace
checks used the preceding bundle; the final UI change only added the device metadata
store to local clearing, covered by the repeated eight offline tests and worker suite.
No Go, SQL or API contract changed after the passing disposable run.

## Release gates not established by this report

Real desktop/Android installation and standalone launch, Android software keyboard
and dictation, screen-reader interaction, production Google authentication, live model
semantic quality and deployed synchronization require device/deployment acceptance.
Desktop reduced-height tests and Axe are separate evidence. iOS follows independently.

The new database contract requires migration 009 and coordinated schema-9 services;
follow [operator procedure](PWA-IMPLEMENTATION.md#operator-procedure). Preserve all
published history and ledger tables during activation or rollback.
