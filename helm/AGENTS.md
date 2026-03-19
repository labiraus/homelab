## Scope

This file applies to everything under `helm/`.

## Purpose

`helm/` contains installable charts, bootstrap charts, shared libraries, and environment-specific values overlays used by Flux and local chart workflows.

Keep chart behavior predictable and interfaces consistent across chart families.

## Working Rules

- Preserve the current top-level split: `apps/`, `bootstrap/`, `data/`, `infra/`, `libraries/`, `observability/`, `workloads/`.
- Prefer reusable templates and consistent values structure over copy-pasted chart logic.
- When changing chart inputs or outputs, update values files, templates, and operator docs together.
- Treat Flux-facing bootstrap charts as part of the delivery contract; avoid one-off behavior that breaks repository or secret conventions.
- Keep secret handling indirect where possible: reference existing secrets or generated values rather than committing literal credentials.
- Treat git push as the deployment trigger for charts in normal operation. Chart build and publication are performed by `.github/workflows/helm-all.yml`, and Flux deploys from the published OCI artifacts rather than from local chart files.
- For app-backed charts, remember that container publication and chart publication move together. The app workflows publish a new container image and then publish the corresponding chart with the matching app version, so image and chart changes should stay aligned.

## Build And Deployment Model

- Installable charts are published as OCI artifacts by the Helm CI pipeline in `.github/workflows/helm-all.yml`.
- The normal GHCR path is: commit to git -> GitHub Actions builds/publishes chart -> Flux `OCIRepository` sees the new tag -> Flux `HelmRelease` reconciles the deployment.
- App workflows under `.github/workflows/app-*.yml` also publish the corresponding chart after building the container image, passing the new image tag as `app-version`.
- Runtime deployment is wholly Flux-driven. Do not assume charts are applied manually unless the task explicitly says so.

## Naming And Values Conventions

- Chart names should stay aligned with their directory names and the Flux release names that reference them.
- Use `helm/workloads/` for miscellaneous or optional workloads that are not part of the core platform/system footprint.
- OCI charts are published under the repo naming pattern `oci://ghcr.io/labiraus/homelab/charts/<chart-name>`.
- For GHCR-backed deployment, effective values are a combination of `values.yaml` and `values-ghcr.yaml`.
- For EKS-style deployment, effective values are a combination of `values.yaml` and `values-ecr.yaml`.
- `values.yaml` is the base/default layer. Environment-specific files such as `values-ghcr.yaml` and `values-ecr.yaml` provide the registry or platform-specific overrides.
- If a chart changes image registry assumptions, secret references, or environment-specific config values, check all values variants, not only `values.yaml`.

## Repo-Specific Notes

- `libraries/commonapi/` is the shared chart template library
- bootstrap charts define how Flux discovers and deploys workloads
- `workloads/` is for miscellaneous user workloads that are deployed by Flux but are not considered core platform services
- values files under `values/` complement this directory and should be checked when chart behavior changes
- Bootstrap Flux charts create `OCIRepository` and `HelmRelease` resources that point at published chart artifacts, typically using the `ghcr-creds` secret for authenticated GHCR access.
- The `helm-all.yml` workflow tests charts on non-main branches and publishes them to GHCR on `main` and `dev`.
- If a chart directory contains `values-ghcr.yaml`, the Helm CI pipeline uses that file for GHCR packaging; otherwise it falls back to `values.yaml`.

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
