# Dependency validation — 2026-10-10

The migration's UI and processor retain their existing React/Vite and Node/LangGraph frameworks. This increment updates vulnerable dependencies and adds `npm audit --audit-level=high` after build/tests in both publishing workflows. High/critical reports now prevent image/chart publication; moderate reports remain visible.

| Application | Changes | Verified checks | Remaining audit result |
|---|---|---|---|
| UI | Vite minimum 7.3.7, Vitest 4.1.11, compatible transitive lockfile updates | Production build; 33 tests under Vitest 4.1.11 | 0 advisories |
| Processor | Compatible transitive updates, including XML tooling and development tooling; MinIO SDK 8 retained | TypeScript build; 30 tests | 4 moderate, 0 high/critical |

The Vitest major update is justified by its maintainer's [redirect-mock advisory](https://github.com/vitest-dev/vitest/security/advisories/GHSA-82fw-gwwq-j7x9): the fix is available in 4.1.11 and older major versions are not maintained for that issue. Vite's [Windows path advisory](https://github.com/vitejs/vite/security/advisories/GHSA-fx2h-pf6j-xcff) also supports updating the existing Vite 7 line. The repo's development server binds all interfaces; these are relevant development dependencies even though production serves built static assets.

Commands run from each application directory, using separate `/tmp/worldbuilder-npm-cache[-ui]` caches:

```bash
# UI
npm install --save-dev 'vitest@^4.1.11' 'vite@^7.3.7' --ignore-scripts --cache /tmp/worldbuilder-npm-cache-ui
npm audit fix --ignore-scripts --cache /tmp/worldbuilder-npm-cache-ui
npm run build && npm test
npm audit --json --cache /tmp/worldbuilder-npm-cache-ui

# Processor
npm audit fix --ignore-scripts --cache /tmp/worldbuilder-npm-cache
npm run build && npm test
npm audit --json --cache /tmp/worldbuilder-npm-cache
```

Processor's remaining findings involve `minio -> query-string -> decode-uri-component` and `minio -> stream-json`. npm proposes a forced MinIO downgrade to 7.1.3. That downgrade was not performed. Registry/source inspection shows the patched decoder 0.5.0 and stream-json 3.7.0 use ES modules, while the installed MinIO/query-string code expects CommonJS callable exports and legacy parser paths. Blind overrides would change those APIs. No compatible patch was fabricated or vendored.

Current processor source (`src/storage.ts`) calls MinIO `getObject`; it does not call bucket notifications or SelectObjectContent. This narrows the observed use, without claiming comprehensive exploitability analysis. Track an upstream-compatible SDK fix or a separately tested adapter. Actual external MinIO behaviour remains part of LAN acceptance. Retain the existing private host and authenticated delivery boundary.

Audit counts are observations at this checkpoint and can change as advisories are published. Local JSON diagnostics are in `/tmp/worldbuilder-{ui,processor}-audit-final.json` and may disappear; this document records the durable results.

Increment 15 adds exact development dependency `@playwright/test` 1.64.0 for real Chromium/API/database acceptance, without changing production UI runtime dependencies. UI audit remains zero; build and 41 component tests pass. Browser/system libraries are isolated test prerequisites, installed on disposable CI runners. The eight scripted processor evaluation cases bring its suite to 44 tests; these are structural checks, not provider-quality claims.
