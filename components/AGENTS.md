## Scope

This file applies to everything under `components/`.

## Purpose

`components/` contains Terraform entry points that assemble reusable modules, providers, variables, and environment overlays into runnable infrastructure plans.

Keep this layer focused on composition, not on burying business logic that should live in reusable modules.

Terraform in this repo is designed to be deployed through the `bin/tf` wrapper script, not by treating the component directory as a standalone manual workflow.

## Working Rules

- Prefer putting reusable infrastructure behavior in `modules/` and keeping `components/` as the wiring layer.
- Keep variable names, outputs, and provider configuration explicit; avoid hidden defaults that make plans harder to reason about.
- Preserve compatibility with the repo workflow driven by `bin/tf` and the `Makefile`.
- Assume `bin/tf` is the canonical execution path for plan/apply/destroy because it assembles tfvars layers, workspace naming, generated files, and automation settings expected by this repo.
- Treat generated files such as `cloud.auto.tf` as workflow artifacts, not hand-maintained source of truth.
- When changing input expectations, also check `etc/env/`, `etc/nodes/`, `README.md`, and `.codex/REPO_MAP.md`.

## Safety Notes

- Do not commit real credentials, kubeadm tokens, kubeconfig data, or local Terraform Cloud secrets.
- Be mindful that bootstrap or join data can leak into state if embedded incautiously.

## Self-Learning Loop

If a Terraform task exposes a repeated pitfall, record it here.

If the issue affects operator workflow, also update `README.md` or the relevant runbook.
