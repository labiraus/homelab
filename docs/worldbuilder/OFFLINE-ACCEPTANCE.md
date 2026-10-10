# Reproducible offline acceptance

No command here connects to or mutates Kubernetes. Docker fixtures create/remove only their own disposable services; SQL never loads operator `.env` files. Public schema/browser downloads need internet access, not LAN credentials.

## Real browser and canon database

Build the UI, install its pinned Playwright Chromium, then invoke the database harness:

```bash
(cd apps/ui && npm ci && npm run build && npx playwright install --with-deps chromium)
WORLDBUILDER_BROWSER_TEST=1 bash scripts/worldbuilder/test-postgres.sh
```

The test-only Go server binds loopback, serves the production UI build and actual canon handler against disposable PostgreSQL, with two explicit fixture identities in separate browser contexts. The fixture auth route exists only in `_test.go`, never a deployed binary. Chromium drives typed drafting, independent review/publication, wiki discovery and a private pinned story with refresh/resume and cross-owner denial. This proves browser/API/database integration; OAuth/header sanitisation, mesh/network policy and real user registration still need LAN acceptance. CI UI publication is gated on this test. Traces default to `/tmp/worldbuilder-browser-artifacts` and contain only fixture data.

A minimal devcontainer may lack desktop libraries/fonts. Prefer the approved Playwright dependency installation in a disposable development environment; temporary extracted libraries may alternatively be supplied with `LD_LIBRARY_PATH` and a private Fontconfig configuration. Do not weaken production authentication or add test identity switches to runtime code to make the test pass.

## Custom-resource schemas

```bash
python3 -m venv /tmp/worldbuilder-checks
/tmp/worldbuilder-checks/bin/pip install -r scripts/worldbuilder/requirements.txt
python3 scripts/worldbuilder/check-manifests.py
bash scripts/worldbuilder/export-crd-schemas.sh /tmp/worldbuilder-schema-check
/tmp/worldbuilder-checks/bin/python scripts/worldbuilder/validate-custom-resources.py /tmp/worldbuilder-schema-check /tmp/worldbuilder-manifests-REPLACE_WITH_PRINTED_DIRECTORY
```

The exporter runs only `helm template`, public HTTP downloads and `flux install --export`. It writes version/checksum metadata beside the schemas. Gateway v1.7.0 and AI Gateway v1.0.0 match repository pins; CNPG chart 0.23.2 and KEDA 2.16.1 satisfy repository ranges. Istio 1.30.5 is an explicit comparison snapshot because the repo's upstream chart is unconstrained. Flux uses the local CLI export (2.9.6 in the recorded check). Verify installed versions/CRDs read-only on LAN and rerun against those exact schemas before activation.

Every custom resource in base/overlay and feature-activation renders must have a schema; missing schemas fail. Structural types/required fields/enums/bounds are checked, including Kubernetes IntOrString/nullable conversion. This does not execute admission webhooks, Kubernetes CEL, unknown-field pruning or built-in resource schemas; Helm/Kustomize checks cover render structure, with server-side admission reserved for controlled LAN validation. Never label this as a live dry run.

## Bounded domain load

```bash
(cd apps/external && go test ./worldbuilder -run '^$' -bench BenchmarkBoundedCanon5000 -benchtime=10x -count=1)
```

The benchmark validates/applies an update and queries a 5,000-record world, reporting time and allocations. It is a local domain microbenchmark, not concurrent database/provider/network throughput. Use the LAN runbook for realistic latency, world-lock contention, cancellation and provider limits before raising record or concurrency caps.
