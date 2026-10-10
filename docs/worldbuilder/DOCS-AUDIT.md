# Documentation audit — 2026-10-10

Disposition describes intended reconciliation; implementation status records checks. No documents are deletion candidates. Legacy endpoint/resource names remain valid and must not be mass-renamed.

| File | Purpose | Disposition | Reason / replacement |
|---|---|---|---|
| `README.md` | Active product/platform guidance | update | Worldbuilder scope and verified implementation; [architecture](01-WORLDBUILDER-ARCHITECTURE.md), [status](IMPLEMENTATION-STATUS.md); prior plan preserved in archive |
| `AGENTS.md` | Active product/platform guidance | update | Worldbuilder scope and verified implementation; [architecture](01-WORLDBUILDER-ARCHITECTURE.md), [status](IMPLEMENTATION-STATUS.md); prior plan preserved in archive |
| `.codex/REPO_PLAN.md` | Active product/platform guidance | supersede | Worldbuilder scope and verified implementation; [architecture](01-WORLDBUILDER-ARCHITECTURE.md), [status](IMPLEMENTATION-STATUS.md); prior plan preserved in archive |
| `.codex/REPO_MAP.md` | Active product/platform guidance | update | Worldbuilder scope and verified implementation; [architecture](01-WORLDBUILDER-ARCHITECTURE.md), [status](IMPLEMENTATION-STATUS.md); prior plan preserved in archive |
| `apps/AGENTS.md` | Active product/platform guidance | update | Worldbuilder scope and verified implementation; [architecture](01-WORLDBUILDER-ARCHITECTURE.md), [status](IMPLEMENTATION-STATUS.md); prior plan preserved in archive |
| `apps/external/README.md` | Active product/platform guidance | update | Worldbuilder scope and verified implementation; [architecture](01-WORLDBUILDER-ARCHITECTURE.md), [status](IMPLEMENTATION-STATUS.md); prior plan preserved in archive |
| `apps/processor/README.md` | Active product/platform guidance | update | Worldbuilder scope and verified implementation; [architecture](01-WORLDBUILDER-ARCHITECTURE.md), [status](IMPLEMENTATION-STATUS.md); prior plan preserved in archive |
| `apps/orchestrator/README.md` | Active product/platform guidance | update | Worldbuilder scope and verified implementation; [architecture](01-WORLDBUILDER-ARCHITECTURE.md), [status](IMPLEMENTATION-STATUS.md); prior plan preserved in archive |
| `apps/ui/README.md` | Active product/platform guidance | update | Worldbuilder scope and verified implementation; [architecture](01-WORLDBUILDER-ARCHITECTURE.md), [status](IMPLEMENTATION-STATUS.md); prior plan preserved in archive |
| `docs/RAG.md` | Active product/platform guidance | update | Worldbuilder scope and verified implementation; [architecture](01-WORLDBUILDER-ARCHITECTURE.md), [status](IMPLEMENTATION-STATUS.md); prior plan preserved in archive |
| `docs/async-ingestion.md` | Active product/platform guidance | update | Worldbuilder scope and verified implementation; [architecture](01-WORLDBUILDER-ARCHITECTURE.md), [status](IMPLEMENTATION-STATUS.md); prior plan preserved in archive |
| `docs/Setup.md` | Active product/platform guidance | update | Worldbuilder scope and verified implementation; [architecture](01-WORLDBUILDER-ARCHITECTURE.md), [status](IMPLEMENTATION-STATUS.md); prior plan preserved in archive |
| `docs/RetentionAndRecovery.md` | Active product/platform guidance | update | Worldbuilder scope and verified implementation; [architecture](01-WORLDBUILDER-ARCHITECTURE.md), [status](IMPLEMENTATION-STATUS.md); prior plan preserved in archive |
| `runbooks/troubleshooting.md` | Active product/platform guidance | update | Worldbuilder scope and verified implementation; [architecture](01-WORLDBUILDER-ARCHITECTURE.md), [status](IMPLEMENTATION-STATUS.md); prior plan preserved in archive |
| `apps/mcp/AGENTS.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `apps/mcp/README.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `helm/AGENTS.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `sql/AGENTS.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/Auth.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/OAuth2ProxyGoogle.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/GoogleOIDCSetup.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/ManualClusterSecrets.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/Secrets.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/EnvoyAIGateway.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/MCPPrompts.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/FileTypeExpansion.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `evals/ragas/README.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/StorageBootstrap.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/WorkerRedeploy.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/NodeClassification.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/Names.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/notes.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/MinecraftVM.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `ansible/AGENTS.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `components/AGENTS.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `.devcontainer/AGENTS.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `.codex/PI_NOTES.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |

Target docs `00`–`04` are retained requirements; reconcile concrete deployment assumptions in `01` and `03`. New API/schema docs are maintained beside their source. No OpenAPI/spec generator, ADR tree or generated app diagram was found. The stale standalone-assistant Mermaid diagram in `docs/RAG.md` is handwritten and will be replaced in its source. Terraform-generated docs are unrelated and retained. Helm values and CI contracts are checked directly.

Existing product history is preserved at [pre-pivot plan](archive/PRE-WORLDBUILDER-PLAN.md). New decisions live in [DECISIONS.md](DECISIONS.md).

## Reconciliation performed

The active `.codex/REPO_PLAN.md` now points to Worldbuilder priorities instead of the old 33-phase document product plan; the old plan is preserved with a superseded notice. README, root/app agent rules, RAG diagram, app READMEs, setup and recovery guidance were revised in place. Retained document route names, storage and auth conventions remain accurate. The target architecture's Fastify/Cassandra/Postgres-add assumptions were reconciled against source. Remote guidance records the subsequent main-branch publication authorisation and keeps LAN activation separate.

| Additional source | Disposition | Documentation linkage |
|---|---|---|
| `.github/workflows/app-{external,orchestrator,mcp,processor,ui}.yml` | update | Main tests gate publication; [decisions](DECISIONS.md) |
| `helm/apps/{external,orchestrator,processor}/` | update | Default-disabled flags, private secret references, mesh boundary; [LAN runbook](LAN-RUNBOOK.md) |
| `sql/worldbuilder/` | retain (new) | Versioned migrations, grants, [schema notes](../../sql/worldbuilder/README.md) |
| `scripts/worldbuilder/` | retain (new) | Disposable database, local manifests and Flux schema validation; [status](IMPLEMENTATION-STATUS.md) |
| `docs/worldbuilder/API.md` | retain (new) | Source-maintained route/domain/event contract, no generator exists |
| `Kustomization-gateway-api.yaml` | retain (historical diagnostic export) | Contains prior live status/UIDs; not a local app Kustomize overlay or deployment input |

No documents deleted. Relative links in the changed product docs and new execution records were checked locally. No generated OpenAPI files were edited; no application spec generator was found.
