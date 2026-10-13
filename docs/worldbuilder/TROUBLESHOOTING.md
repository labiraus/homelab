# Worldbuilder migration troubleshooting

## AI workspace navigation and generation recovery (2026-10-12)

A disappeared conversation after reload or a world/account switch is expected: world-chat persistence is not supported by the API. Saved proposals and private stories remain available from their libraries. A stopped/failed synchronous request may have consumed an allowance; retry explicitly. Resume the original story or generate afresh when a candidate reports an earlier/different turn. Resolve stale proposals through the comparison flow, preserving the original base revision.

The redesign fixes authentication loading that previously reset deep hash links before auth status resolved. It also preserves submitted scope on save-triggered refresh, uses snapshot-pinned source links, and reuses request IDs after uncertain manual saves. Do not replace a generation’s revision/session version to bypass a conflict. Historical source questions retain their source pin until **Use displayed world context** is selected. Response cards stay mounted across local conversation switches so editable content, retry IDs and already-saved state are not reset.

Browser test navigation must wait for the world navigation container to be attached before deciding whether to open the mobile drawer. An immediate visibility check after world selection falsely treated a loading desktop as mobile and waited forever for a hidden toggle. Actual Chromium traces established this race; the helper now waits for the DOM container’s attachment (hidden links are absent from role queries). Native selects use accessible combobox roles, since label text can include option text. Keep fixture responses sensitive to the current request, not accidentally to retained follow-up context.


## Deployed UI verification and browser login (2026-10-12)

Testing initial deployed UI `38303543096` found the desktop Workspace link beneath the sticky header after scrolling. The existing journeys passed because Playwright could scroll again before later clicks; `isVisible` did not detect the obstruction. A new `elementFromPoint` regression fails against that release. Header offsets alone fixed the navigation but a tall inspector was still pushed upward by its grid boundary at page end. The fix measures header and footer heights and reserves workspace/layout spacing when bounding both sticky panels. All three local workspace journeys now pass at desktop/mobile sizes, including the scrolled 1280×720 and 1120×600 hit-target checks. Keep the original failure evidence separate from acceptance of the replacement release.

Successful native MCP reads do not supply a browser Google-proxy session. A public `/` request redirecting to sign-in is expected without that session. Authenticated deployed browser acceptance needs a legitimate existing browser session; keep exported Playwright state in ignored/private local storage, never in Git or chat. Do not inject identity headers, mint proxy cookies or substitute native MCP tokens to make a browser test pass. Fixture identities remain confined to disposable loopback tests.

Verify the exact UI source revision in `flux-system/ui` OCI artifact metadata, the Ready HelmRelease in that same namespace, and the image/digest of the available `homelab` Pod. The HelmRelease is not in the workload namespace; a NotFound response from `homelab` was a namespace lookup mistake, not a missing deployment. Check compiled assets from the actual replacement Pod as well as public TLS/authentication behavior. None of these checks substitutes for authenticated user journeys.

Use [implementation status](IMPLEMENTATION-STATUS.md) for exact check results and [platform troubleshooting](../../runbooks/troubleshooting.md) for existing operations; do not duplicate legacy incident logs here.

- Missing engine/infra or Cassandra: audit actual paths before following illustrative target names. Do not create replacement services or remove storage based on assumptions.
- Git branch creation failed in the restricted sandbox because `.git` is read-only. Tool-approved local `git switch -c feat/worldbuilder-offline-foundation` succeeded. No commits or pushes were made.
- Do not run `scripts/pg-update.sh`: audit found an unfinished migra placeholder and advisory lock on a short-lived connection. Worldbuilder migrations use the new explicit transactional migration files; existing schemas are untouched.
- The initial remote readiness checkpoint was **BLOCKED_CLUSTER_ACCESS**. API access is verified again during the 2026-10-12 UI release; see the current [acceptance matrix](ACCEPTANCE-MATRIX.md). Passing local checks alone never establishes deployed state.

- Go cache paths `/home/vscode/.cache/go-build` and `/go/pkg/mod` are read-only in the sandbox. Use `GOCACHE=/tmp/worldbuilder-go-cache`, `GOMODCACHE=/tmp/worldbuilder-go-mod`; tools installed with `go run ...@version` also need `GOPATH=/tmp/worldbuilder-gopath` for checksum cache access. Do not disable checksum verification.
- There is no root Go workspace/module: repository-root `go test ./...` fails. Use `(cd apps/external && go test ./...)` and the equivalent module directories. CI and `apps/AGENTS.md` now encode this rule.
- Docker runs outside the devcontainer. A PostgreSQL port bound to host loopback may not be reachable as container `127.0.0.1`; the disposable harness probes and uses `host.docker.internal` when needed. Only the disposable generated port/password is used; no operator environment is sourced.
- Default Helm render placeholders do not cover the Flux bootstrap registry secret lookup. Offline checks supply an empty `{"auths":{}}` fixture via `--set-file imagePullSecret.dockerconfigjson=...`; `--set-string` with JSON braces is parsed as Helm list syntax and fails. Never pass these placeholders for actual deployment.
- npm ci reported pre-existing advisories: processor 7 (1 low, 5 moderate, 1 high), UI 13 (2 low, 2 moderate, 7 high, 2 critical). Lockfiles/dependency versions were not changed by this migration. Assess and remediate these in a tested dependency update; no audit-clean claim is made.
- `gh auth status` has no configured CLI login, while Git's credential path supports an authenticated dry-run push. Do not print/export Git credential-helper output to work around CLI status.

- actionlint rejected old setup-go/setup-node/cache action runtimes. Updated the app CI actions to verified upstream major tags, kept the app toolchain versions unchanged, and reran actionlint successfully.
- Canon AI candidates must retain the revision used for generation. Do not use a freshly loaded UI revision when persisting an older candidate; the API/React regression boundary now preserves `baseRevision` and refuses stale proposal generation.

- A final repeat on a Docker-assigned high port (32771) was unreachable from this Docker-outside-Docker devcontainer, even though PostgreSQL was healthy inside its disposable container. The stalled test was interrupted and only that test container was removed. The harness now preflights the actual connection, sets `connect_timeout=5`, bounds Go tests to 120 seconds and accepts `WORLDBUILDER_TEST_PORT=25433`. The explicit low-port probe succeeded. CI can retain automatic host ports where localhost reaches Docker directly.

The final database repeat succeeded on explicit port 25434 after adding bounded TCP readiness retries across localhost and host.docker.internal. Container socket readiness alone can precede final TCP/host forwarding readiness; the harness now checks the actual connection before starting Go tests.

- First main external CI (`38061475231`) passed unit tests but failed the disposable database script at exit 126. The Windows-backed workspace allowed local execution while Git had stored the new shell script as mode `100644`. Corrected tracked mode to `100755`, explicitly invoked Bash in CI, and added the rule to root AGENTS.md. External artifact publication was blocked by its test dependency; processor publication succeeded independently.

The corrective external run `38061652923` completed successfully, including PostgreSQL checks and image/chart publication. All five application pipelines now pass; the executable-mode failure is resolved, not an outstanding deployment blocker.

## Stale proposal or story outcome

Open “Compare proposal with current canon” or “Reconcile story outcome for review”. Inspect original/current/proposed records, choose a resolution for each conflict and supply a reason. Save the fresh draft and obtain new independent approval. A 409 after preview means canon or the story changed again: refresh the comparison and inspect it again. Cross-record 422 findings (for example a deleted subject or changed schema) require omitting the invalid change or preparing a corrected proposal in the structured editor. Keeping current canon for every record produces no changes and cannot create an empty draft.

A staged manual draft retains its original revision when canon is refreshed. Reload that revision to continue staging, or clear the staged changes before editing another revision. Do not replace its base revision to suppress conflicts. History restore also produces a reviewed draft; published revisions are never rewritten.

## Lifecycle or identity migration conflict

A 409 administration response means its expected administration version changed; reload world metadata and review the action. An archived world rejects canon/story/inference writes until its owner reopens it. Owners can still revoke members while archived. Transfer leaves the former owner as admin, so new owner-only operations from that account correctly return 403.

Migration 002 aborts if immutable history reuses an ID for different record kinds. Inventory the affected world/ID/revisions and design an explicit reviewed migration; never rewrite old snapshots to bypass the check. Current external requires schema 2 and dedicated grants. Apply migrations only through the authorised LAN sequence; a v1 app rollback requires the feature disabled.

## Disposable Docker PostgreSQL is healthy but unreachable

Docker-outside-Docker can keep its API available while published host-port forwarding stops working, even on previously successful fixed ports. The harness now falls back to a loopback socat listener relaying binary traffic through `docker exec` into its own generated `worldbuilder-test-*` container. To skip unreliable host-port probing, run `WORLDBUILDER_TEST_TRANSPORT=exec bash scripts/worldbuilder/test-postgres.sh`. This requires local `socat` and container Bash; no LAN endpoint or production credentials are used. The harness removes the listener and its own container on exit. Do not interpret a transport preflight failure as a passing integration test.

## Empty trusted-principal list passed Helm required

Delegation negative rendering found that `required` accepted `[]`, emitting an empty `notPrincipals` list. Added explicit `empty` rejection and wildcard checks; all negative/positive cases now pass in `check-manifests.py`. Validate security-sensitive collection inputs explicitly, rather than inferring string-style required semantics.

## Generation remains queued, failed or uncertain

Inspect the private job in Worlds and refresh; completed parts remain inspectable. Queueing reserves no future budget, so later parts can stop at allowance or stale-story checks. `uncertain` means a reserved part lost its checkpoint/lease: do not automatically repeat it or refund its allowance. Check dispatch/consumer state and per-part inference attribution using the LAN runbook. Broker retention can require controlled metadata replay; never reset completed steps or reserved inference. A provider call currently holds the world transaction, so cancellation and other same-world operations can wait for its bounded timeout. This serialisation is a measured-load acceptance gate.

## Correction browser gate and viewport checks (2026-10-13)

Keep `test:workspace` (loopback HTTP fixtures) separate from `test:browser` (actual disposable API/SQL governance). The first correction harness timed out while old selectors expected technical labels and administration outside its new menu. Update visible wording/disclosures while retaining exact quorum, privacy, stale-revision and source-pin assertions; inspect retained traces before blaming database transport. Verify owned fixture cleanup before retrying. The software-keyboard acceptance remains a real-device check, independent of the automated viewport matrix.

## Offline Chromium or Helm schema export fails before application checks

A minimal devcontainer may lack Chromium's NSPR/NSS/X11/DBus libraries and fonts. The first local browser launches failed on `libnspr4.so`, then missing Fontconfig configuration; temporary Debian packages/fonts under `/tmp`, an explicit `LD_LIBRARY_PATH` and `FONTCONFIG_FILE` resolved startup. CI uses Playwright's dependency installer on the disposable runner. Keep fixture identities in `_test.go` and restrict browser requests to the fixture origin.

`helm show crds` returned empty files for upstream charts that render CRDs from templates. Use `helm template --include-crds` and fail validation when any rendered custom-resource kind lacks a schema. Set `HELM_CACHE_HOME` to a writable temporary directory; setting only `HELM_REPOSITORY_CACHE` did not redirect the initial repository-index download in the locally installed Helm. No cluster discovery is needed for these exports.
