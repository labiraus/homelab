# Terraform component rules

Apply the root policy and these directory-specific constraints.

## Working rules

Use `bin/tf` as the canonical execution path; components compose modules. Never commit credentials or bootstrap secrets, including through Terraform state.

- Prefer putting reusable infrastructure behavior in `modules/` and keeping `components/` as the wiring layer.
- Keep variable names, outputs, and provider configuration explicit; avoid hidden defaults that make plans harder to reason about.
- Preserve compatibility with the repo workflow driven by `bin/tf` and the `Makefile`.
- Assume `bin/tf` is the canonical execution path for plan/apply/destroy because it assembles tfvars layers, workspace naming, generated files, and automation settings expected by this repo.
- Treat generated files such as `cloud.auto.tf` as workflow artifacts, not hand-maintained source of truth.
- When changing input expectations, also check `etc/env/`, `etc/nodes/`, `README.md`, and `docs/README.md`.
