# Public endpoint validation — 2026-10-10

Read-only checks from the development container completed at approximately 19:54 UTC. **Public application functionality remains unverified: this network's Zscaler filter prevented a verified connection to the application.** No credentials were sent, application data changed, cluster commands executed, or feature flags activated.

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

Initial checker validation: six unit tests passed, covering unrelated/filter responses, TLS refusal before HTTP, identity rejection and redacted redirects. A local verified probe failed before HTTP, as expected from the current connection problem. Hosted-runner results will determine the next justified configuration change; no speculative gateway or DNS mutation is included.
