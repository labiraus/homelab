## Scope

This file applies to everything under `helm/`.

## Purpose

`helm/` contains installable charts, bootstrap charts, shared libraries, and environment-specific values overlays used by Flux and local chart workflows.

Keep chart behavior predictable and interfaces consistent across chart families.

## Working Rules

- Preserve the current top-level split: `apps/`, `bootstrap/`, `data/`, `infra/`, `libraries/`, `observability/`, `workloads/`.
- Put only cluster-fabric prerequisites in `bootstrap/`; utility controllers or shared services that other workloads consume but the cluster does not need in order to function belong in `infra/`.
- Prefer reusable templates and consistent values structure over copy-pasted chart logic.
- When changing chart inputs or outputs, update values files, templates, and operator docs together.
- Treat Flux-facing bootstrap charts as part of the delivery contract; avoid one-off behavior that breaks repository or secret conventions.
- SeaweedFS is the external Ansible-managed target on `svartalfheim:8333`; do not add an in-cluster object store. Until migration/restore gates pass, normal publication retains port-9000 source settings; target endpoints, Secrets and egress are prepared together in explicit `values-seaweedfs.yaml` overlays. Do not make routine app publication perform an unverified storage cutover. Follow `docs/SeaweedFSMigration.md`; application `MINIO_*` keys remain compatibility names for S3 clients.
- Keep secret handling indirect where possible: reference existing secrets or generated values rather than committing literal credentials.
- Treat git push as the deployment trigger for charts in normal operation. Chart build and publication are performed by `.github/workflows/helm-all.yml`, and Flux deploys from the published OCI artifacts rather than from local chart files.
- For app-backed charts, remember that container publication and chart publication move together. The app workflows publish a new container image and then publish the corresponding chart with the matching app version, so image and chart changes should stay aligned.

## Build And Deployment Model

- Installable charts are published as OCI artifacts by the Helm CI pipeline in `.github/workflows/helm-all.yml`.
- The normal GHCR path is: commit to git -> GitHub Actions builds/publishes chart -> Flux `OCIRepository` sees the new tag -> Flux `HelmRelease` reconciles the deployment.
- App workflows under `.github/workflows/app-*.yml` also publish the corresponding chart after building the container image, passing the new image tag as `app-version`.
- The repo uses repo-global GitHub Actions run IDs for non-semver prerelease tags so chart and app publishes share one monotonically increasing series across workflow files.
- `helm-all.yml` publishes non-app charts directly, but when only an app chart under `helm/apps/**` changes it should dispatch the matching app workflow so the app image and app chart are still published together from the same pipeline run.
- Runtime deployment is wholly Flux-driven. Do not assume charts are applied manually unless the task explicitly says so.

## Naming And Values Conventions

- Chart names should stay aligned with their directory names and the Flux release names that reference them.
- Use `helm/workloads/` for miscellaneous or optional workloads that are not part of the core platform/system footprint.
- OCI charts are published under the repo naming pattern `oci://ghcr.io/labiraus/homelab/charts/<chart-name>`.
- For GHCR-backed deployment, effective values are a combination of `values.yaml` and `values-ghcr.yaml`.
- For EKS-style deployment, effective values are a combination of `values.yaml` and `values-ecr.yaml`.
- `values.yaml` is the base/default layer. Environment-specific files such as `values-ghcr.yaml` and `values-ecr.yaml` provide the registry or platform-specific overrides.
- If a chart changes image registry assumptions, secret references, or environment-specific config values, check all values variants, not only `values.yaml`.
- For charts that use `generatedSecrets[*].stringData[*].fromSecretRef`, local/CI render checks should set `global.allowMissingGeneratedSecretRefs=true` so missing live Secrets render as placeholders. Do not set that flag for real deployment checks; normal renders should fail if source Secrets are unavailable.

## Repo-Specific Notes

- `libraries/commonapi/` is the shared chart template library
- Its Istio resources are in `libraries/commonapi/templates/_istio.tpl`; discover template paths with `rg --files` before reading them rather than deriving filenames from Kubernetes kinds.
- bootstrap charts define how Flux discovers and deploys workloads
- `workloads/` is for miscellaneous user workloads that are deployed by Flux but are not considered core platform services
- values files under `values/` complement this directory and should be checked when chart behavior changes
- Bootstrap Flux charts create `OCIRepository` and `HelmRelease` resources that point at published chart artifacts, typically using the `ghcr-creds` secret for authenticated GHCR access.
- The `helm-all.yml` workflow tests charts on non-main branches and publishes them to GHCR on `main` and `dev`.
- If a chart directory contains `values-ghcr.yaml`, the Helm CI pipeline uses that file for GHCR packaging; otherwise it falls back to `values.yaml`.
- The upstream NATS chart deployed here uses Helm release name `nats` in namespace `nats`, which yields service DNS like `nats-nats.nats.svc.cluster.local`; do not assume the broker service is `nats.nats.svc.cluster.local`.

## Safety Notes

- Do not commit live secret material in values or templates.
- Be careful with generated secret behavior and lookup-based templates so upgrades remain understandable and repeatable.

## Flux Troubleshooting

- Start by identifying the owning Flux objects: the `OCIRepository` that fetches the chart and the `HelmRelease` that applies it.
- If a deployment is stale, check whether the chart was actually published by GitHub Actions before assuming Flux is broken.
- If Flux cannot fetch the chart, inspect the `OCIRepository` first for bad repository URLs, wrong tags or semver filters, or auth failures against `ghcr-creds`.
- If Flux fetches the chart but the workload does not update, inspect the `HelmRelease` next for values/rendering errors or Kubernetes apply failures.
- When troubleshooting app deployments, verify that the app workflow published both the image and the chart with the expected matching version.
- When troubleshooting values issues, reason from base plus overlay: `values.yaml` + `values-ghcr.yaml` for GHCR, or `values.yaml` + `values-ecr.yaml` for EKS.
- Check the bootstrap charts under `helm/bootstrap/` when the problem looks systemic across many releases, since those charts define shared Flux repository and release behavior.
- Be cautious about assuming a local chart edit is live in-cluster. If the chart was not published and reconciled through Flux, the cluster may still be running the previous OCI artifact.

## Self-Learning Loop

If a charting convention or Flux integration rule proves durable, record it here.

If a lesson is specific to one chart family, add a more local `AGENTS.md` under that subtree later.

- Helm `required` does not reject an empty list in the locally verified Helm version. Security-sensitive principal lists must explicitly test `empty` and reject wildcard entries; include negative render tests.

- Offline CRD checks must fail on missing schemas. Some upstream charts render CRDs from templates, so `helm show crds` can be empty; use `helm template --include-crds`. Set `HELM_CACHE_HOME` for temporary writable caches. `scripts/worldbuilder/test-platform.sh` validates the Worldbuilder render matrix against a disposable pinned API server, including built-ins/CEL/strict fields. Keep source CRDs unchanged and operator kubeconfig unused; source validation does not establish installed versions, external webhooks or controller reconciliation. CI gates app and Helm publication on this check.

- Worldbuilder observability stays opt-in until scraper namespace/Pod labels/mTLS principal, monitor discovery and datasource UID are observed. Keep STRICT mTLS, exact metric-only identity boundaries and existing dashboard providers. Grafana file provisioning does not substitute manual-import `__inputs`: bind the UID explicitly and package one dashboard JSON source in the Prometheus chart. Compare selected upstream chart mounts/CRDs and execute the disposable metric/dashboard gate before publication.
- Public monitoring CRDs can contain a legal bare `=` enum. Use the repository's Kubernetes-aware safe structural loader; do not rewrite the source enum or bypass the API-server validation gate to accommodate a YAML parser error.
