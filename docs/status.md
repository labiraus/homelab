# Status and evidence

Documentation reconciled against repository source on **26 October 2026**; live
recovery observations added on **27 October 2026**, with user confirmation of browser
sign-in on **28 October 2026**. The documentation cleanup itself did not perform
live checks, publication or deployment.
Dates below are observation dates; none establishes present production health.
Source implementation, local fixtures, hosted publication and deployment are
separate evidence levels. Git history retains superseded checkpoint narratives.

## Current source

- Worldbuilder uses schema **10**. Schema 9 introduced Authors/Readers, administrative
  publication policy, immutable established accounts, shared conversations and PWA
  writing; schema 10 adds inference profile pins. Older role/persistence descriptions
  are superseded for new writes, not retroactively rewritten in stored history.
- UI supports conversation-centered work, exact scoped sources, inline editable/
  refinable candidates, review return paths and opt-in account-bound offline writing.
  Unsaved candidate edits remain memory-only; see [experience](worldbuilder/experience.md).
- Independent inference proxy and explicit EU Bedrock profiles are implemented
  opt-in. Local has no monetary/daily quota; paid routing requires verified prices,
  credentials and the shared $100 UTC-month ledger. No paid fallback is permitted.
- Canon lexical discovery, reviewed branches, calendars, private scenes and durable
  jobs have source coverage. Optional canon semantic search needs its own deployment
  and real-corpus acceptance; document embeddings do not establish that activation.
- SeaweedFS 4.48 is the private target. Source defaults retain MinIO until final
  frozen reference/copy/map/restore/cutover gates pass. Source data is retained.

## Outstanding work

| Area | Required next evidence |
| --- | --- |
| Estate health and connectivity | The 27 October sweep restored intended workloads, all 40 HelmReleases and production schema 10 after a fresh backup and isolated rehearsal. The user confirmed successful browser sign-in on 28 October. Resolve the absent resource metrics API for Istiod autoscaling and explain the differing public/private TLS leaves. `midgard` remains intentionally cordoned/offline. See the recovery record below. |
| Current source release | UI revision 62, external revision 81 and orchestrator revision 84 reconciled on 27 October with schema 10. Exact source/signature provenance and real-user acceptance remain separate from these observed artifact versions; the optional inference proxy is not established by this recovery. |
| Identity and real-user experience | Production Google outside-domain login, registered/unregistered refusal, distinct actual Authors/Readers/Administrators, private-story isolation and governed outcomes. Single-operator or fixture acceptance is insufficient. |
| Product journeys | Real-model Ask/Create/consequence quality, focused graph answers, complete task return/selection continuity, durable candidate versions/task links and representative large-world performance. See [experience](worldbuilder/experience.md). |
| Devices | Real Android install/window, keyboard/dictation/TalkBack, update/offline/replay and account isolation. iOS follows later. |
| Inference | Local proxy/cache rollout and rollback, actual monitoring/alerts, AWS access/allowance eligibility, current EU tariffs, certificate renewal/revocation and both paid-model quality suites before activation. |
| Canon features / MCP | Live semantic profile/route/quality/index reconstruction, branch workflows and the newer MCP payload-preservation candidate rollout. Verify actual client/catalog and artifact independently. |
| Storage | Freeze all writers/expiry, fresh consistent reference export, complete all-version copy/map verification, historical application reads, coordinated Flux cutover, fresh target-backed restores and off-host SeaweedFS metadata/volume backups. The tool has no live incremental tail-sync implementation. |
| Operations and recovery | Production Velero coverage, recurring backups, Prometheus PVC/history/discovery, current mesh/identity/certificate checks, power-loss recovery and review/cleanup of owned restore clones/PVCs. Discover ownership before removal. |
| External inventory | Establish any external Cassandra consumers/data; absence locally does not authorize retirement. |

## Retained acceptance ledger

These entries summarize prior records; linked CI runs were not re-fetched during
this cleanup. Successful publication is claimed only where the retained record
explicitly includes the complete applicable publication.

| Observation | Evidence and boundary |
| --- | --- |
| 27 Oct cluster recovery and schema upgrade | Both Proxmox hosts returned; API/etcd and all intended Deployment/StatefulSet replicas recovered. All 40 HelmReleases Ready. Frozen backup `20261027T093222` restored in isolation: all 35 tables and 87 security/sequence entries matched. Migrations 009/010 and grants passed preservation, runtime denials and identical reruns before production upgrade to 10. Post-upgrade backup `20261027T094036` completed; temporary maintenance and the owned clone were removed. Public TLS/discovery/refusal passed; the user confirmed successful browser sign-in on 28 October. |
| 26 Oct UI/SQL source acceptance | [Committed evidence index](worldbuilder/screenshots/ux-2026-10-26/README.md): 100 component tests, 23 workspace journeys, 10 strengthened viewport/theme journeys, 12 production-worker PWA cases, and real Go/SQL migrations 001–010 with 3 API browser journeys and full logical restore. Exact source/build hashes are in the evidence JSON. Ask/Create answers are fixtures; no live-model/device/deployment claim. |
| 19 Oct PWA source | [Retained responsive screenshots](worldbuilder/screenshots/pwa-2026-10-19/README.md) cover both themes at five widths with fixture APIs. Schema-9 governance/device contracts are implemented; desktop screenshots do not establish Android acceptance. |
| 19 Oct CI repair | Source `59eaadbdff40d34e2fb8b1ec8b6c5463d8c647b7` failed [UI 39233407115](https://github.com/labiraus/homelab/actions/runs/39233407115) and [platform 39233407076](https://github.com/labiraus/homelab/actions/runs/39233407076) on an ambiguous browser locator and retired-role/19-table backup seed. The corrected exact selector and 22-table seed/restore assertions passed local Go/SQL/browser and physical latest/PITR recovery with source removal and cleanup. No passing hosted repair rerun is retained. |
| 19 Oct independent inference | Retained source record reports proxy race/accounting/restore checks, schema-10 upgrades, local Helheim eleven-case quality and synthetic Envoy/Cilium credential tests. Synthetic credentials do not prove AWS PKI; no proxy publication/deployment claim. |
| 13 Oct UI deployed checkpoint | Source `ee1feba`, complete [UI publication 38405146042](https://github.com/labiraus/homelab/actions/runs/38405146042), Ready Helm revision 60, ten served assets matched, nine deployed-asset fixture journeys and public refusal checks passed. Production Google/model/multi-human journeys remained unverified. |
| 12 Oct branches / story outcomes | `ceaeb29`: UI `38197879801` and external `38197879785` complete publication; `c342848`: UI `38198900178` includes real disposable story-to-reviewed-outcome browser acceptance. Actual deployed human workflows remain separate. |
| 12 Oct canon semantic | `ef1e9fc` passed complete app/platform source publication; `5419134` passed [real index platform 38191883197](https://github.com/labiraus/homelab/actions/runs/38191883197). A pinned CPU-model smoke ranked four expected paraphrase matches first. Synthetic-index and small real-model tests do not prove production quality or activation. |
| 11–12 Oct mesh | `721a312`: [platform 38190854271](https://github.com/labiraus/homelab/actions/runs/38190854271), external `38190854678`, orchestrator `38190854531` passed. Actual Cilium/Istio fixture and owned cleanup; physical estate parity remains separate. |
| 11 Oct storage tooling | `28cd70d` / `70dca55`: [38185792059](https://github.com/labiraus/homelab/actions/runs/38185792059) / [38186045068](https://github.com/labiraus/homelab/actions/runs/38186045068), actual Go/Node clients, explicit MinIO dialect and coverage cases. Ordering `626c905`: `38139117870`; multipart `904cc45`: `38138008247`; reference coverage `c0fda1b`: `38137124323`. Includes 5 GiB + 17 byte migration and uncertainty refusal; no production cutover. |
| 11 Oct platform recovery | CNPG `f59eb12` / `38147505119`, Velero `1076dea` / `38149569293`, Prometheus admission `020c4f0` / `38151131241`, TSDB `a67f7f3` / `38152998843`: complete recorded fixture/platform and applicable publication. Older fixtures used then-current schema inventories; current source requires 22 populated tables. |
| 11 Oct native MCP | Backup `20261011T203021`, external publication `38175617058` and MCP pin `38173624785`: actual client consent, initialize, 15 tools and a 42-byte document matching source passed. A later session separately verified the conversation connection; later tool count is below. Earlier Google DCR and stale-bridge failures were resolved in that record, not current blockers. |
| 11 Oct core application | Live schema 8, dedicated identities, native MCP consent/tools/document reads, operator publication/audit, AI and generation restart checkpoints were recorded. MCP `38179885575` exposed 20 tools on both transports. These historical self-review semantics do not describe schema-9 governance. |
| 11 Oct document recovery | Processor source `fd0ac3e`, deployed artifact `38182654891`: actual worker ownership-loss recovery and final read-only comparison recorded 115 processed texts, 2 unsupported images, 374 chunks and 133,440 source bytes. Original 2,199 lifecycle rows retained fingerprint `4905c033579662ce1e9ce401216ae3eb`. |
| 11 Oct pre-cutover storage/recovery | SeaweedFS snapshot and 33-object CNPG copy/restores passed isolated comparison; later source preflight read 119 versions and covered 116 restored references with `writersFrozen:false`, `migrationReady:false`. Schema-8 backup `20261011T225138` and isolated Prometheus recovery (45 series / 27,360 samples) were recorded. Production cutover/new backups and owned cleanup were not established. |

## Latest recovery observation

On **27 October 2026, 09:42 UTC**, following the user's reboot of both Proxmox
hosts, all five intended online Kubernetes nodes were Ready. API/etcd readyz
passed. `midgard` remained cordoned/offline in its documented intermittent state;
its Cilium, kube-proxy, GPU and retained engine-image Pods account for offline-node
readiness gaps. No node was force-fenced and no production Pod/PVC/replica was
force-deleted. The workstation SSH certificate expired on 1 August; no SSH trust
was replaced and Kubernetes recovery did not require a PKI reset.

The Helm controller's live-only hostname pin to `alfheim` had stranded it for
seven days. Removing that selector restored leadership on `niflheim` and resumed
published upgrades. All **40 HelmReleases** and discovered Flux sources were Ready
at the final check. All **47 Deployments/StatefulSets** met their configured replica
counts. External revision **81** uses artifact `40412803991`, orchestrator revision
**84** uses `40412804004`, processor revision **45** uses `39406630859`, and UI
revision **62** uses `40412804078`. These are observed deployment identities,
not signature or full source-provenance verification.

PostgreSQL reattached its original storage after `alfheim` returned. Local `psql`
confirmed two registered users, twelve worlds and schema 8. The browser's reported
`identity validation failed` matches the API's database-error path. The user approved
schema 10 after a fresh backup and isolated restore rehearsal. External, orchestrator
and processor were paused, including their releases and the processor scaler;
backup **`20261027T093222`** then completed against the unchanged MinIO source.

The backup was restored with PostgreSQL 16.13 into the owned, network-isolated
`recovery-upgrade-20261027` namespace. All **35 application tables** and **87
security/sequence entries** matched the frozen source exactly. Separate role,
membership and schema-owner comparison also passed. Migrations 009/010 and canon
grants ran as the independent owner; auth grants ran separately as `app`. Legacy
content, immutable history and sequences were preserved. Converted memberships
received new authority IDs through the existing trigger; historical reviews stayed
unchanged. New defaults, runtime permission denials and identical migration/grant
reruns passed. The clone had no application exposure or backup destination and
was deleted, including its namespace/PVC, after validation.

Production received the same hashed SQL, reached **schema 10 / 22 canon tables**,
and matched the rehearsed data/security/sequence state apart from independently
generated membership IDs and migration timestamps, which were checked separately.
Live external, relay, native-MCP and legacy-app permission checks passed. The canon
owner's login was closed again; original role attributes, memberships and schema
owners matched. All three writer releases and replicas resumed, KEDA's pause was
removed, and external/orchestrator started successfully. A fresh post-upgrade
backup **`20261027T094036`** completed; CNPG Ready, ContinuousArchiving and
LastBackupSucceeded were True. Existing backups and migration sources remain intact.

Prometheus, vLLM and both OpenSearch Pods were Ready. vLLM's live 80 GiB `emptyDir`
cache required a fresh unchanged-model download; it was allowed to finish. All
three NATS health checks passed; KEDA reported a valid zero backlog. OpenSearch
had both nodes, all 16 primary shards active and no pending tasks; yellow reflects
two unassigned replicas on its single data node. All attached Longhorn volumes
were healthy; an older detached restore volume was retained. Istiod's single
replica was healthy, but CPU autoscaling still lacks `metrics.k8s.io`; no metrics
server was present in the discovered desired configuration.

Normal DNS and observed WAN both returned `81.151.43.97`. Public reports at 09:12,
09:15 and **09:40** passed with `attempts[].ok: true`, including discovery and
anonymous/forged-header refusal. These are local normal-DNS observations, not
hosted-runner or authenticated acceptance. The public TLS leaf expires 8 November;
private Gateway `192.168.8.240` served the configured leaf expiring 9 January 2027.
Both verified for `mcp.labiraus.com`; differing termination paths still need
explanation. Public/private MCP discovery bytes matched. No Labiraus MCP connector
was available in this session. On **28 October 2026**, the user confirmed successful
browser sign-in after retrying, resolving the reported signed-out/identity-validation
incident. This is user-reported login acceptance; broader authenticated browser
journeys and native MCP flows were not revalidated by this recovery.

Private snapshots, backup records, full fingerprints, SQL hashes, rehearsal and
production acceptance, and public reports are under `.local/recovery-20261027/`.
No private recovery material was committed.

## Evidence custody

Committed screenshots, logs and hash manifests remain beside their evidence READMEs.
Private recovery records were documented under `.local/lan-recovery-20261011/`,
including `live-ai-acceptance.json`, `live-mcp-acceptance.json`, generation recovery,
document/worker recovery, source-history/reference inventories and
`prometheus-history-restore.json`. They are not committed evidence and their content
has not been revalidated by this documentation task.

The CI-repair record cited `/tmp/worldbuilder-ci-backup-retry/backup-result.json`
with SHA-256 `180609d72dd28a276b58202967d0493dc99779989c6486e879be59b888a2c2b2`;
public diagnostics cited `/tmp/public-endpoints-current.json` and LAN/WAN companions.
Those `/tmp` CI-repair/public-probe reports and `/tmp/inference-gateway-complete/acceptance.json` are unavailable in this checkout. The private recovery directory exists, but its contents were not revalidated. Never convert a missing report into fresh
verified evidence or publish its private kubeconfig/Secret subdirectories. Keep
full old chronology in Git history; add new evidence here only after observation.

The recorded canon-index CI artifact hash is
`9473f716dfaab3a427289d3ffaaf3977f74c47ddeaf9c8fbcb3f5df6171c1068`
(run `38191883197`); private retained copies were recorded under
`.local/lan-recovery-20261011/index-ci-evidence/`.

The 12 October semantic-index record identifies hosted job `114637957772` in run
`38195776765`, artifact `11723596632`, SHA-256
`94d6e26bb9b103fa36a452cd8d4cd571e99e2f46ee67359d6b623715f5684316`.
Its eight tests and cleanup passed; the private copy was recorded under
`.local/lan-recovery-20261011/canon-semantic-evidence/` and was not revalidated here.

The 11 October mesh record notes that ten old hosted artifacts containing twenty
fixture-generated keys were removed after sanitized, hash-verified copies were
retained under `.local/lan-recovery-20261011/mesh-ci-evidence/`. Sanitized bytes have
different hashes from execution originals. Revision `d7df943` introduced screening;
`220b8d4` passed the complete request matrix, and the later `721a312` publication is
in the ledger above. This is retained incident history, not a fresh remote listing.

Other 19 October inference report paths cited by the source record are `/tmp/inference-unit-complete.log`, `/tmp/inference-proxy-tariff-final.log`, `/tmp/inference-worldbuilder-postgres-corrected.log`, `/tmp/inference-physical-backup-retry.log`, `/tmp/inference-local-quality-final/quality.jsonl`, `/tmp/inference-local-quality-final/measurements.json`. These session-local reports are unavailable here.
