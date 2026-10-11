# Dependency validation — 2026-10-11

Container stages now consume the committed lockfiles with `npm ci`, including processor production-only dependencies. CI checks the actual processor image's lockfile hash/package tree and compiled imports, and serves the UI image's compiled assets, before chart publication. These image checks use disabled container networking; complete-build/publication evidence is recorded in [implementation status](IMPLEMENTATION-STATUS.md). A local Docker registry-issuer trust failure does not justify disabling TLS or reverting to unlocked installs.

Current UI and processor npm audits both report **zero vulnerabilities**. The processor now uses exact `@aws-sdk/client-s3` 3.1149.0 behind `DocumentStorage.readDocument`, removing the MinIO Node SDK and its four moderate transitive findings. Build and 50 unit/HTTP-contract tests pass; the integration-only test runs separately through the disposable storage harness. Processor CI now rejects moderate/high/critical advisories before publication, alongside its required real-client storage gate.

The [official GetObject API](https://docs.aws.amazon.com/AWSJavaScriptSDK/v3/latest/client/s3/command/GetObjectCommand/) provides the streaming byte conversion used by this adapter. Source audit found MinIO's only runtime operation was `getObject`; replacing this isolated adapter preserves existing `MINIO_*` configuration, explicit credentials, path-style private endpoints, byte content and S3 error codes. No Go SDK, MCP tool name, event subject or endpoint changes accompany it. This is the separately tested adapter option identified below, not an incompatible transitive override.

HTTP fixtures verify SigV4 credential/region scope, encoded object keys, binary/empty content, missing/access-denied errors, redirect refusal and truncated-stream failure. Endpoint origins must agree with `MINIO_USE_SSL`; credentials, paths and queries inside endpoints are rejected. Signing region defaults to `us-east-1`, with the existing `MINIO_REGION` override for other stores. Production endpoint/region acceptance remains a live gate.

Reproduce from `apps/processor` with `npm ci`, `npm run build`, `npm test` and `npm audit --audit-level=moderate` (use a writable `/tmp` npm cache if needed). Run the actual Go/Node fixture through [storage acceptance](../StorageVersionMigration.md). A clean audit is an observation, not a guarantee against future advisories. Source/checkpoint publication evidence belongs in [implementation status](IMPLEMENTATION-STATUS.md).

## Historical baseline and remediation

The following records the earlier baseline before the isolated adapter replacement above.

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

At that checkpoint processor source (`src/storage.ts`) called MinIO `getObject`, without bucket notifications or SelectObjectContent. That narrow use motivated the separately tested adapter above. Actual external storage behaviour remains part of LAN acceptance. Retain the existing private host and authenticated delivery boundary.

Audit counts are observations at this checkpoint and can change as advisories are published. Local JSON diagnostics are in `/tmp/worldbuilder-{ui,processor}-audit-final.json` and may disappear; this document records the durable results.

Increment 15 adds exact development dependency `@playwright/test` 1.64.0 for real Chromium/API/database acceptance, without changing production UI runtime dependencies. UI audit remains zero; build and 41 component tests pass. Browser/system libraries are isolated test prerequisites, installed on disposable CI runners. The eight scripted processor evaluation cases bring its suite to 44 tests; these are structural checks, not provider-quality claims.

Rechecked 2026-10-11 during offline storage acceptance: the npm registry still reports MinIO 8.0.7 as latest, and processor audit still reports four moderate findings with no high/critical findings. No incompatible override or SDK migration was applied. Build and 44 unit tests pass; a separate disposable SeaweedFS test now exercises the real storage adapter and passes with no skips. This establishes client compatibility for the tested operations, not remediation of the dependency advisories.
