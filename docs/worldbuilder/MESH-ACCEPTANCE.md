# Disposable network and identity acceptance

`scripts/worldbuilder/test-mesh.py` exercises the source-rendered external/MCP NetworkPolicies, STRICT PeerAuthentication, browser/delegation/worker/metrics AuthorizationPolicies, native-auth boundaries, Gateway and HTTPRoutes. It uses a fresh, owned Kubernetes 1.35.0 kind cluster with Cilium 1.20.2 and Istio 1.30.5. Production kubeconfig, credentials, source data and network endpoints are never used.

The shared fixture disables kind's default CNI only for this test, installs Cilium before waiting for node/DNS readiness, and retains the existing ownership-checked exec transport and loopback-only API. Other controller/backup tests keep their normal networking. The bootstrap follows the [Cilium kind installation](https://docs.cilium.io/en/stable/installation/kind/) and [Istio Helm installation](https://istio.io/latest/docs/setup/install/helm/) contracts. Public registry trust can be supplied through the existing bounded `WORLDBUILDER_TEST_CA_BUNDLE` input; TLS verification remains enabled.

Run after preparing the source schema artifacts with `scripts/worldbuilder/test-platform.sh`:

```bash
python3 scripts/worldbuilder/test-docker-exec.py
python3 scripts/worldbuilder/test-mesh.py /tmp/worldbuilder-mesh /tmp/platform-evidence/schemas
```

Use a fresh output directory. Docker, Python/PyYAML, Helm and OpenSSL are required. The fixture creates its own kubeconfig and deletes only its owned kind cluster. Its private directory contains ephemeral cluster and test TLS keys and is removed on successful cleanup. Retain public render hashes, certificate SAN identities, observed image IDs, synthetic request results, diagnostic logs and `mesh-result.json`; a failed assertion or failed cleanup cannot report success. Cilium startup allows fifteen minutes and synthetic workload readiness ten minutes for cold downloads; expiry still fails acceptance.

The acceptance matrix requires:

- Positive GET and POST requests from the exact browser, MCP and generation-worker identities, with cross-role requests denied. Matching Pod labels and forged forwarded identity/certificate headers do not replace the service-account identity.
- Actual certificate SAN inspection, STRICT plaintext refusal, namespace/Pod selector refusal and source egress scope/port checks. An unrestricted client first proves every tested synthetic destination/port is reachable.
- A distinct metrics reader allowed only on GET `/metrics`, with forged scraper identity, application routes and other methods denied.
- The real generated Istio Gateway serving a source-configured TLS hostname with certificate verification and explicit unrelated-CA refusal. All four source-configured MCP transport paths, including actual MCP/message POST methods, require the source CUSTOM authorization call; missing/invalid credentials and forged user headers are refused, and accepted identity headers come from the authority.
- No public route to private worker/delegation/authority operations. Direct browser-to-MCP transport is blocked by source policy.
- Unavailable authorization backend refusing previously accepted callers, followed by recovery through a replacement backend and real Istio/Cilium/Gateway Pod replacement with positive and negative requests repeated.

`mesh-fixture.py` supplies synthetic HTTP services and a fixed synthetic credential authority only inside the disposable cluster. It does not replace the separate actual-application tests of OAuth, database membership, signed assertions or world authorization. The Gateway uses a ClusterIP because the fixture has no external load balancer; the listeners, route matching and TLS/authentication policies remain source-derived. This gate cannot establish installed policy/CRD parity, real OAuth registration, physical cross-node routing, certificate rotation, production capacity or LAN outage recovery.

Source `51dbe03` passes the complete initial matrix and teardown in the [hosted mesh job](https://github.com/labiraus/homelab/actions/runs/38188329408/job/114615955719), also repeated successfully by the external/orchestrator workflows. This includes workload SANs, network/role/metrics/refusal checks, Gateway trust/authentication, authority outage and all three controller replacements. The final explicit POST extension and complete publication results remain pending; a bootstrap alone is not completed acceptance.

The shared fixture's default-networking regression passes locally in `/tmp/homelab-mesh-default-controller-v2`: actual admission and trust refusal, source ServiceMonitor selection/metric filtering, removal/restoration, operator and Prometheus replacement, and owned teardown. The standalone hosted observability job encountered a registry HTTP 502 before executing its test; equivalent reusable jobs pass on the same source. Keep that failed startup distinct from mesh acceptance and require the final complete workflow to pass.

The first complete local bootstrap passed Cilium but exposed CoreDNS's existing progress deadline expiring while the node waited for its CNI. The custom-CNI fixture extends that Deployment deadline before installation, then still requires actual node and DNS readiness. Default-networking fixtures retain their original deadline. The failed run reported `FAIL` and removed its owned cluster; it is not policy acceptance evidence.

Hosted run `38187833959` passes actual certificate SANs, positive routes, STRICT plaintext refusal, namespace/label/egress isolation and browser/MCP/worker/metrics boundaries. It then correctly failed an over-specific test assumption: unknown SNI is reset before the Gateway presents a certificate. The corrected trust-negative check keeps the valid SNI and supplies an unrelated CA, requiring `SSLCertVerificationError`. Istio startup and replacement also require a valid server-side admission request after the validator has a CA and `failurePolicy: Fail`; a Ready Pod or initial `Ignore` webhook alone is insufficient. Full Gateway/outage/replacement acceptance remains pending.

The corrected client trust check passes an independent loopback TLS server test: configured CA accepted, unrelated CA rejected with the exact verification error, then the configured CA accepted again. Hosted Gateway verification must still pass with the actual source listener.
