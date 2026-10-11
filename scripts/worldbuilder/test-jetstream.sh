#!/usr/bin/env bash
# Three disposable loopback processes; never reads operator endpoints or secrets.
set -euo pipefail
[[ $(uname -s) == Linux ]] || { echo 'This process-failure drill requires Linux'; exit 1; }
repo_dir=$(cd "$(dirname "$0")/../.." && pwd)
test_tools=$(mktemp -d)
trap 'rm -rf "$test_tools"' EXIT
# Pin the server independently of application dependencies; the public Go
# checksum database verifies module content. No production module file changes.
GOBIN="$test_tools" GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org \
  GONOSUMDB='' GOPRIVATE='' go install github.com/nats-io/nats-server/v2@v2.11.9
cd "$repo_dir/apps/orchestrator"
WORLDBUILDER_TEST_NATS_SERVER_BIN="$test_tools/nats-server" \
  go test -race -count=1 -timeout=5m -v -run '^TestGenerationThreeNodeRecovery$' .
