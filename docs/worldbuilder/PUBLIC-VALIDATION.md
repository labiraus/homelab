# Public endpoint validation — 2026-10-10

Read-only checks from the development container completed at approximately 19:54 UTC. **Public application functionality remains unverified: this network's Zscaler filter prevented a verified connection to the application.** No credentials were sent, application data changed, cluster commands executed, or feature flags activated.

**Update at 20:10 UTC:** independent GitHub-hosted checks also reject the public endpoint's certificate for a hostname mismatch. MCP image/chart publication succeeded, but the expected deployed version could not be observed. This is now independently confirmed public TLS failure, not merely a local filter diagnosis. Flux reconciliation remains unknown.

## Observations

| Check | Observed result |
|---|---|
| Configured Labiraus MCP | Both local Codex configurations point `homelab-mcp` at `https://mcp.labiraus.com/mcp`; no callable Labiraus connector was exposed in this session. HTTP diagnostics were used. |
| Local DNS for `mcp.labiraus.com` | `87.117.64.111`; TCP 443 and TLS 1.3 responded. This alone does not establish that the cluster gateway was reached. |
| Verified HTTPS GET | `/`, `/api/auth/status`, `/.well-known/mcp.json`, `/.well-known/oauth-protected-resource`, and `/api/worlds` all failed TLS hostname verification before an application response. |
| Certificate presented with SNI `mcp.labiraus.com` | Zscaler-issued leaf, subject CN `bath.bprj.co.uk`; SANs `bath.bprj.co.uk`, `home.bprj.co.uk`, `imap.bprj.co.uk`, `smtp.bprj.co.uk`. None covers the requested hostname. Leaf validity October 10–18, 2026. |
| Anonymous diagnostic GET with verification temporarily disabled | `/`, `/.well-known/mcp.json`, `/api/auth/status`, and `/mcp` returned HTTP 403, `Server: Zscaler/6.2`, HTML titled “Internet Security by Zscaler”. The block page reports “Access denied due to bad server certificate” and a Web Security policy block. This was solely to identify the responder; it is not an application/authentication test or an acceptable connection configuration. |
| Legacy `uat.labiraus.com` | Did not resolve through this environment's resolver. It is referenced by legacy ECR overlays, not the current configured MCP endpoint. |
| Apex `labiraus.com` | Resolved to GitHub Pages addresses; its presented certificate covered `labiraus.com` and was also Zscaler-issued. It is not the configured application host. |

No proxy environment variables were set, and a direct Python TLS socket still received the Zscaler-issued certificate. The evidence identifies network TLS interception, but cannot establish whether the underlying public origin also serves the wrong certificate. The 403 is from the network filter, not evidence that application authorization is working. No attempts were made to bypass the filter.

## Unresolved checks and next actions

1. Have the network administrator investigate the block, or repeat normal certificate-verified access from the approved LAN/network. Keep TLS verification enabled and do not submit application credentials until the hostname validates.
2. Compare public DNS and router/NAT forwarding with the actual ingress address. On LAN, inspect the Gateway listener and certificate reference, Certificate readiness and HTTPRoute attachment read-only. Source expects `mcp.labiraus.com` and secret `mcp-labiraus-com-tls`; installed state is unknown. Inspect only certificate metadata/public certificates, never private key or secret contents in logs.
3. Once HTTPS works, test discovery, browser OAuth redirect and authenticated `/api/auth/status`. With an approved identity, exercise MCP `initialize`, `notifications/initialized`, `tools/list`, `resources/list`, and a bounded document inventory read. Compare with repo-native database/log observations when available. Discovery alone cannot establish upstream database, MinIO or search health.
4. Verify actual deployed image/chart revisions and Worldbuilder flags using the [LAN runbook](LAN-RUNBOOK.md). Source defaults keep Worldbuilder disabled; these public checks do not prove installed flag or schema state. Canon, search, story, generation and delegation acceptance remain pending the existing controlled activation gates.

There is insufficient evidence for a source or gateway configuration fix. No application tests passed or failed in this probe: connectivity failed before a verified application response. Existing offline test and CI results remain separate evidence.

## CI-managed diagnosis and iteration

The user subsequently authorised repository fixes and CI/Flux deployment iterations. `.github/workflows/public-endpoints.yml` now runs credential-free public checks from a GitHub-hosted runner on checker changes, manual dispatch and successful app/Helm pipeline completion. It retries up to ten times, validates TLS normally, requires actual Labiraus discovery JSON and checks anonymous browser authentication behavior. Sanitized JSON artifacts retain DNS, certificate and response classifications without cookies, private bodies or redirect query parameters. Failure produces a failing CI check; it never disables TLS verification or invokes a cluster mutation.

The CI check provides public availability evidence, not proof that a particular revision reconciled. MCP's advertised version is recorded when discovery succeeds. Authenticated integration checks and Flux source/release status still require an approved identity and cluster connectivity. The local kubeconfig currently has no current context, so no reconciliation status is available here. Do not declare the cluster stopped reconciling merely because public connectivity fails.

Source audit found MCP's Docker build did not stamp `pkg/base.BuildVersion`, leaving discovery at `local`. MCP CI now supplies its immutable `0.0.0-uat.<run-id>` image tag as `BUILD_VERSION`; the Docker linker embeds it in the existing version field. Checks triggered by MCP publication require that exact advertised version and retry an older version as a deployment not yet observed. Other pipeline triggers remain availability checks only. This closes a verified observability defect; it is not claimed as a fix for the TLS/routing failure.

Initial checker validation: six unit tests passed, covering unrelated/filter responses, TLS refusal before HTTP, identity rejection and redacted redirects. A local verified probe failed before HTTP, as expected from the current connection problem. Hosted-runner results will determine the next justified configuration change; no speculative gateway or DNS mutation is included.

Version-aware validation: seven checker tests pass, including refusal to treat a healthy older release as the expected deployment. MCP's full Go suite passed with an injected build version, workflow actionlint passed, and the cert-issuers, gateway and flux-bootstrap charts passed Helm lint. Source certificate/HTTPS listener references consistently use `mcp.labiraus.com` and `ingress/mcp-labiraus-com-tls`; no public DNS-record or WAN port-forwarding management was found in the audited Terraform/Ansible/bootstrap paths.

First hosted run `38081898430` failed all ten attempts (19:58–20:03 UTC), resolving the same `87.117.64.111` but reaching no verified HTTP response. Its initial error classification was too broad: real `ssl.SSLError.reason` is a string, whereas `urllib.error.URLError.reason` wraps an exception. Corrected that distinction and extended the regression fixture; subsequent artifacts identify DNS/TCP/TLS stage, TLS verification code/message and exception type. A repeated local probe now correctly records TLS hostname mismatch, verification code 62. No TLS verification is disabled by the checker.

MCP CI run `38082111065` passed tests and published the version-stamped image and matching chart (`0.0.0-uat.38082111065`). This establishes publication only; the post-publication probe must still observe it. Probe concurrency is grouped by triggering workflow so newer checks supersede older checks of the same kind without dropping a pending MCP version check for an unrelated publication.

## Final observed CI results and continuation boundary

| Run | Result |
|---|---|
| [MCP publication 38082111065](https://github.com/labiraus/homelab/actions/runs/38082111065) | Tests and image/chart publication succeeded for source `4736490`, version `0.0.0-uat.38082111065`. |
| [Detailed public probe 38082334514](https://github.com/labiraus/homelab/actions/runs/38082334514) | Failed all ten attempts, 20:05:05–20:09:39 UTC: DNS `87.117.64.111`, stage `tls`, `SSLCertVerificationError`, verification code 62, hostname mismatch for `mcp.labiraus.com`. No HTTP response. |
| [Post-publication probe 38082259713](https://github.com/labiraus/homelab/actions/runs/38082259713) | Failed all ten attempts, 20:05:22–20:09:54 UTC, with the same TLS hostname mismatch. Expected MCP version `0.0.0-uat.38082111065` was never observed. |
| Earlier diagnostic push 38082111067 | Deliberately cancelled as superseded so the pending post-publication check could start. This was a CI cancellation, not a cluster operation. |
| Documentation-only MCP run 38082754514 | Cancellation requested after the final `apps/mcp/AGENTS.md` documentation update matched the app workflow's broad path trigger. Runtime source was unchanged from the successfully published and probed image; no additional rebuild was needed. |

Downloaded sanitized CI artifacts confirm these results. The revised checker's seven unit tests passed in hosted CI as well as locally. No application credentials were used by any probe. GitHub credentials were used only for authorized repository/CI operations, including reading diagnostic artifacts and cancelling the superseded check.

Read-only CI configuration inspection found no repository Actions secrets or `uat` environment secrets, and only `USE_ECR` among environment variable names; no approved cluster-observation credential was available there. The local kubeconfig has no active context. Consequently, neither successful Flux reconciliation nor stopped reconciliation can be asserted. Do not keep republishing unrelated changes to infer either state.

The next evidence required is the verified current WAN/ingress address and/or an approved read-only cluster connection showing Gateway listener, Certificate/CertificateRequest/ACME Challenge, HTTPRoute and Flux source/release status. The user was asked whether `87.117.64.111` is correct; no answer had arrived when recording this result. If DNS/NAT targets the wrong host, fix its actual configuration owner. If the correct Gateway is serving the wrong certificate, use the observed Certificate/route/controller failure to prepare a targeted chart change, test it, publish via CI and repeat the version-aware check. Current source already declares the matching hostname/certificate; changing it without installed-state evidence would be speculative. Authenticated Worldbuilder acceptance and controlled activation remain pending.
