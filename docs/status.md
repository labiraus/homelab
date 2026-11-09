# Status and evidence

Documentation reconciled against repository source on **26 October 2026**; live
recovery observations added on **27 October 2026**, with user confirmation of browser
sign-in on **28 October 2026**. The documentation cleanup itself did not perform
live checks, publication or deployment.
Dates below are observation dates; none establishes present production health.
Source implementation, local fixtures, hosted publication and deployment are
separate evidence levels. Git history retains superseded checkpoint narratives.

Local dependency and fixture validation was performed on **6 November 2026**
against the uncommitted working tree based on `e9b0292`. This did not publish
artifacts, deploy workloads or revalidate production health.

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
- SeaweedFS 4.51 is the private target pin. Source defaults retain MinIO until final
  frozen reference/copy/map/restore/cutover gates pass. Source data is retained.
- Application modules and CI use Go 1.27.3; Node images use 26.12.1. The inference
  JSON Schema v6 and optional Pub/Sub v2 migrations have consistent module graphs.

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
| 9 Nov CI failure diagnosis and source repair | The 8 November `db1d93` runs for all six apps, Helm and standalone platform acceptance failed the CNPG backup gate on the same stale conflicting-bootstrap message; hosted logs show CNPG 1.30.1 correctly rejected `spec.bootstrap`. The source repair accepts the old and current field-specific conflict while retaining the unavailable-webhook refusal and recovery checks. In an owned kind fixture, the real 1.30.1 webhook, external SeaweedFS 4.51 and PostgreSQL 16.15 passed base backup, WAL, source removal, invalid-credential refusal, latest and named-PITR restore; both recovered all 22 canon tables and auth users with security, immutable-ledger and sequence checks. The final, stricter predicate was separately exercised against that live webhook; teardown and fixture ownership checks passed. This is local source evidence, not a hosted rerun of the uncommitted fix. The separate [8 November public run](https://github.com/labiraus/homelab/actions/runs/42487982695) failed on a connection reset for the forged `/message` probe. A [9 November hosted public-only run](https://github.com/labiraus/homelab/actions/runs/42589839106) passed on its first complete attempt (`attempts[0].ok: true`); authenticated functionality and Flux reconciliation were not tested. Normal DNS and the observed WAN matched `81.151.43.97`; the private Gateway also passed hostname-verified TLS. Local public TLS presented a leaf expiring 22 November while the hosted run presented one expiring 9 January 2027, so the different termination paths remain unexplained. |
| 6 Nov dependency source acceptance | All application Go modules passed tests with `-mod=readonly`; external/MCP/orchestrator built, and inference/PubSub race tests passed. UI: 100 component tests, 23 workspace journeys, 12 PWA cases and the real PostgreSQL/API/browser gate passed. Processor: 63 tests passed with 2 endpoint-dependent cases skipped in the unit run; its actual S3 client passed separately. Node images built and runtime smoke checks passed. The complete feature-enabled devcontainer built and its tools ran as `vscode`; explicit `GOTOOLCHAIN=auto` selected verified Go 1.27.3 from the base's 1.27.2 compiler. Kubernetes 1.37 accepted 524 renders against 79 CRDs, including refusal checks; Helm packaging and inference renders passed. Inference accounting/restore and Worldbuilder migrations 001-010/all-table restore passed. SeaweedFS 4.51 actual-client, historical-version and 5 GiB + 17 byte migration/re-export gates passed with owned cleanup. UI audit has 4 moderate/2 low advisories, none high/critical; processor audit is clean. All 17 referenced CI action tags exist. Hosted CI, controller/physical-backup fixtures for the upgraded pins, and deployment remain unverified. |
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

## Minecraft switch observation

On **1 November 2026**, `nidavellir` switched from `ftb_skies_2_aero` to
`just_create_smp_2`, pinned to CurseForge client file **8718376** (Just Create SMP
Season 2 **1.0.3**), Minecraft **1.21.1**, NeoForge **21.1.233**, Java 21.
The new profile has a fresh world; all prior profile data remains intact.

Aero was flushed and stopped for a complete cold `data` archive. The VM archive
`/srv/minecraft/archives/ftb-skies-2-aero-before-jcsmp2-20261101.tar.gz` and the
independent operator copy under ignored `.local/minecraft-20261101/` matched
SHA-256 `86fcf00ffa0c00f12873e6954a9f6eed49be93812ccc233072696e28f6e977f2`;
tar listing passed. The archive is 3,954,823,084 bytes. Existing backups were retained.
No copy to `svartalfheim` was made in this session.

Ansible syntax and repository template rendering passed. Deployment used the
rendered profile and switch helper over Windows SSH, retaining secrets on the VM;
the full remote Ansible playbook was not run because devcontainer SSH host-key
verification failed. No SSH trust was changed. After the final restart, startup, authenticated RCON, shared server-property
verification and workstation LAN TCP 25565 passed. At **15:35 UTC**, logs showed
a successful player login and RCON listed one online player. Public DNS was not
independently probed, and voice-chat UDP reachability was not tested.
The VM had approximately **4.3 GiB free**, so capacity expansion or separately
reviewed backup offloading remains necessary for comfortable world growth.

## Minecraft storage offload attempt

On **5 November 2026**, live Minecraft remained at **20 TPS**, with sampled
average tick times of **5.5–7.3 ms** against the 50 ms budget. The workstation's
public-address connection used its WireGuard interface; TCP connection probes
took **279–361 ms**, versus **4–16 ms** to `192.168.8.126:25565`. These are TCP
connection measurements, not an in-game ping trace. The VM disk was **98% full**:
about **64 GiB** in historical archives, **30 GiB** in four inactive profiles,
**1.3 GiB** in the active Just Create profile and **15 GiB** in Docker overlay data.

The user authorized moving old worlds to private S3. Svartalfheim's external
disk was mounted with **5.2 TiB free**; its wired link negotiated **100 Mb/s**.
The enabled private S3 socket had failed while SeaweedFS was active. Starting
the existing socket restored authenticated S3 access and anonymous HTTP 403.
No SSH trust changed: the valid certificate already in `.devcontainer/ssh/`
worked, whereas the workstation's default certificate was expired.

The attempt created `minecraft-backups/nidavellir/offload-20261105/`. Two archives
passed full S3 readback SHA-256/size checks. Only the readable **41,429,084-byte**
`ftb_skies_2_aero-world-20260821T165800Z.tar.gz` was removed locally. The
**83,623,936-byte** `atm11-20260424T192419Z.tar.gz` failed tar validation and was
retained locally despite its verified byte-for-byte S3 copy.

During the third upload, svartalfheim stopped answering SSH/S3. The transfer was
terminated; all remaining archives and all inactive profiles stayed on the VM.
After SSH recovered, kernel logs showed **SeaweedFS OOM-killed at 20:06:48 UTC**
and repeated undervoltage events. The host has **905 MiB RAM** and about **904 MiB
swap**; SeaweedFS restarted automatically. This is an **incomplete offload**, not
disk-capacity remediation. Resolve the storage host's power/memory constraints
before resuming bulk writes. Private scripts, checksums and logs are under
`.local/minecraft-offload-20261105/`; the S3 manifest records per-object results.
Both completed objects passed another full readback after SeaweedFS restarted;
the attempt's unfinished multipart upload was aborted and the manifest explicitly
marked incomplete. Temporary credential copies from that attempt were removed
from both hosts.

The user subsequently requested continued retries despite the known power
constraint. At **20:31 UTC**, a serial retry began using independent **8 MiB S3
objects**, per-part full readback checks, archive reconstruction manifests and
10–60 second backoff after service/transport failures. By **21:41 UTC**, five
archives totaling **8,706,062,671 bytes** had passed all part readbacks and tar
validation and been removed locally, in addition to the earlier small unchunked
archive. Free VM space increased to approximately **12 GiB**. The next ATM11
archive was uploading. The full scope inventory records **15 original archives
totaling 67,743,612,723 bytes and four inactive profiles**; not all are moved yet.
This retry remains in progress, not completed cleanup. Its private runner/log and
checkpoint are under `/home/ubuntu/minecraft-offload-20261105/` on `nidavellir`,
with source under ignored `.local/minecraft-offload-20261105/`; its S3 subprefix is
`nidavellir/offload-20261105/chunked/`. The credential copy is restricted to the
private task directory and is removed on completion. The active world is excluded.

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

The 9 November local backup evidence is under `/tmp/homelab-ci-backup-fixed-20261109-U3ZaU2/`; the failed reproduction and hosted logs are also session-local under `/tmp/homelab-ci-*`. They contain no production data and are not committed evidence.

The 6 November source-check logs are session-local `/tmp/homelab-finish-*.log`;
platform schemas/renders are under `/tmp/homelab-finish-platform/`. They are not
committed artifacts or evidence of publication/deployment.

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
