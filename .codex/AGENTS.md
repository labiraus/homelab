## Learning loop

When a problem is fixed, update `runbooks/troubleshooting.md` with:
- date
- symptom
- root cause
- fix
- how it was verified

If the same issue or class of issue happens more than once, add a permanent rule to this file or the nearest directory-level `AGENTS.md`.

If Codex makes an incorrect assumption about this repo, correct it in `AGENTS.md` so the fix persists for future sessions.

## Documentation upkeep

When Codex changes code, config, workflows, or repository structure, update the relevant human documentation in the same task.

At minimum, consider whether any of these need updates:

- `README.md`
- app-level READMEs under `apps/`
- setup or operator docs under `docs/`
- `.codex/REPO_MAP.md` if the change affects how future agents should navigate the repo
