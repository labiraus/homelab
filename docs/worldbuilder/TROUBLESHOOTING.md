# Worldbuilder migration troubleshooting

Use [implementation status](IMPLEMENTATION-STATUS.md) for exact check results and [platform troubleshooting](../../runbooks/troubleshooting.md) for existing operations; do not duplicate legacy incident logs here.

- Missing engine/infra or Cassandra: audit actual paths before following illustrative target names. Do not create replacement services or remove storage based on assumptions.
- Git branch creation failed in the restricted sandbox because `.git` is read-only. Tool-approved local `git switch -c feat/worldbuilder-offline-foundation` succeeded. No commits or pushes were made.
- Do not run `scripts/pg-update.sh`: audit found an unfinished migra placeholder and advisory lock on a short-lived connection. Worldbuilder migrations use the new explicit transactional migration files; existing schemas are untouched.
- Remote readiness is **BLOCKED_CLUSTER_ACCESS**. Passing local checks never establishes deployed state.

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
