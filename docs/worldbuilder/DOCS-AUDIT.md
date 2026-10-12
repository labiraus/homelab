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
| `apps/mcp/README.md` | Platform or retained document capability | update | Active integration/evaluation/schema guidance reconciled; see [status](IMPLEMENTATION-STATUS.md); no claim of Worldbuilder deployment |
| `helm/AGENTS.md` | Platform or retained document capability | update | Active integration/evaluation/schema guidance reconciled; see [status](IMPLEMENTATION-STATUS.md); no claim of Worldbuilder deployment |
| `sql/AGENTS.md` | Platform or retained document capability | update | Active integration/evaluation/schema guidance reconciled; see [status](IMPLEMENTATION-STATUS.md); no claim of Worldbuilder deployment |
| `docs/Auth.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/OAuth2ProxyGoogle.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/GoogleOIDCSetup.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/ManualClusterSecrets.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/Secrets.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/EnvoyAIGateway.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/MCPPrompts.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `docs/FileTypeExpansion.md` | Platform or retained document capability | retain | Accurate capability/operations guidance remains; no claim of Worldbuilder deployment |
| `evals/ragas/README.md` | Platform or retained document capability | update | Active integration/evaluation/schema guidance reconciled; see [status](IMPLEMENTATION-STATUS.md); no claim of Worldbuilder deployment |
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

The 2026-10-12 acceptance audit reconciles the current matrix and offline summary with retained live schema/identity/AI/generation/storage evidence. Historical increment logs retain their original observations. The migration approach now names the actual monorepo; its gates are requirements, not assertions that already verified production installation never happened. Current health remains unverified during the Pi outage, and final cutover plus multi-user/story-outcome acceptance remain open.

The active index, remote deployment guide, Codex execution prompt, concrete LAN runbook and evaluation README now use the same checkpoint distinction. They preserve established activation/managed identities and standing main/LAN recovery authorization, scope initial provisioning to missing/new environments, and retain separate destructive-retirement and new-feature gates. Multi-step mutating shell examples stop on errors. No deployment or permission expansion is implied by this documentation repair.

The active `.codex/REPO_PLAN.md` now points to Worldbuilder priorities instead of the old 33-phase document product plan; the old plan is preserved with a superseded notice. README, root/app agent rules, RAG diagram, app READMEs, setup and recovery guidance were revised in place. Retained document route names, storage and auth conventions remain accurate. The target architecture's Fastify/Cassandra/Postgres-add assumptions were reconciled against source. Remote guidance records the subsequent main-branch publication authorisation and keeps LAN activation separate.

| Additional source | Disposition | Documentation linkage |
|---|---|---|
| `.github/workflows/app-{external,orchestrator,mcp,processor,ui}.yml` | update | Main tests gate publication; [decisions](DECISIONS.md) |
| `helm/apps/{external,orchestrator,processor}/` | update | Default-disabled flags, private secret references, mesh boundary; [LAN runbook](LAN-RUNBOOK.md) |
| `sql/worldbuilder/` | retain (new) | Versioned migrations, grants, [schema notes](../../sql/worldbuilder/README.md) |
| `scripts/worldbuilder/` | retain (new) | Disposable database, local manifests and Flux schema validation; [status](IMPLEMENTATION-STATUS.md) |
| `docs/worldbuilder/DEPENDENCIES.md` | retain (new) | Version/security rationale, exact checks, remaining MinIO dependency compatibility findings |
| `docs/worldbuilder/API.md` | retain (new) | Source-maintained route/domain/event contract, no generator exists |
| `docs/worldbuilder/ACCEPTANCE-MATRIX.md` | retain (new) | Requirement-level remaining work, linked from index/status; prevents treating an initial slice as full migration completion |
| `Kustomization-gateway-api.yaml` | retain (historical diagnostic export) | Contains prior live status/UIDs; not a local app Kustomize overlay or deployment input |

Search/evidence contracts and coverage limitations were reconciled in API, external/processor/UI READMEs, decisions, acceptance matrix and LAN validation gates in increment 7.

MCP delegation contracts, chart inputs and activation gates were reconciled in increment 8. The initial deferral decision is retained with an explicit superseding decision. Helm and MCP agent guidance records the verified boundaries.

No documents deleted. Relative links in the changed product docs and new execution records were checked locally. No generated OpenAPI files were edited; no application spec generator was found.

Schema-v3 budget/provenance contracts supersede v2 startup instructions in the active API, SQL, external README and SQL agent guidance. Historical increment logs retain their original schema/version evidence.

Schema-v4 private definitions/memory replace the initial title-only story guidance in the API, app READMEs, acceptance matrix, SQL guidance and LAN runbook. Existing story pins and immutable narration remain preserved.

Increment 11 updates active SQL/runtime contracts to schema 5, app READMEs and API with private checkpointed jobs, and the LAN/troubleshooting runbooks with separate worker activation and uncertain-work recovery. Historical increment counts/schema observations remain historical. The acceptance matrix and repository plan track durable jobs as locally verified while retaining LAN quorum/security/provider gates.

Increment 13: `evals/ragas/README.md`, `docs/RAG.md` and the Make target are updated from implicit CNPG/pgvector evaluation to authenticated external search by default. Legacy comparison code is retained explicitly; no source data is deleted. `evals/worldbuilder/README.md` documents synthetic canon fixtures and separate LAN semantic evaluation. Evaluation adapter CI is read-only and does not publish infrastructure.

Increment 15 adds reproducible Chromium/API/database acceptance, bounded domain load and complete custom-resource schema coverage to `OFFLINE-ACCEPTANCE.md`; app and Helm guidance record actual fixture/dependency/schema lessons. The API documents scoped connection traversal. The acceptance matrix now separates completed remote requirements from actual LAN identity/provider/admission/load/cutover gates, optional visualisation and upstream dependency follow-ups.

Final handover reconciles the execution prompt and remote checklist with the explicit main-branch publication override; direct cluster mutation/activation restrictions remain. Original wording is preserved in Git history.
