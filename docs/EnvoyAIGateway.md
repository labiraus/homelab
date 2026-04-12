# Envoy AI Gateway

Envoy AI Gateway is installed through `helm/infra/envoy-ai-gateway/` and reconciled by Flux through the `envoy-ai-gateway` release declared in `helm/bootstrap/flux-apps/values.yaml`.

The infra stack installs three upstream controller artifacts plus one upstream Git source:

- Envoy Gateway CRDs from the upstream `envoyproxy/gateway` Git repository
- Envoy Gateway into `envoy-gateway-system`
- Envoy AI Gateway CRDs into `envoy-ai-gateway-system`
- Envoy AI Gateway controller into `envoy-ai-gateway-system`

The Envoy Gateway Helm values include the upstream AI Gateway compatibility settings so Envoy Gateway exposes the Backend API and the xDS translation hooks required by Envoy AI Gateway.

Current scope:

- installs the control-plane components only
- does not yet create a default `GatewayClass`, `Gateway`, `AIGatewayRoute`, `AIServiceBackend`, or provider credentials
- leaves tenant- or app-specific AI routing resources to follow-up charts once concrete models, backends, and auth secrets are defined

Pinned upstream versions in this repo:

- Envoy Gateway `v1.7.0`
- Envoy Gateway CRDs `v1.7.0`
- Envoy AI Gateway CRDs `v0.5.0`
- Envoy AI Gateway controller `v0.5.0`
- Gateway API standard CRDs `v1.4.1` via `helm/bootstrap/flux-bootstrap/`

Current repo note:

- Gateway API is pinned to `v1.4.1` to stay aligned with the upstream Envoy Gateway `v1.7.x` compatibility matrix used by this repo
- clean installs split CRD ownership so Flux bootstrap owns Gateway API CRDs, a Flux `GitRepository` + `Kustomization` owns Envoy Gateway CRDs from `envoyproxy/gateway`, and the main Envoy Gateway chart installs with CRD management disabled
- operator access to Postgres no longer depends on a permanent Gateway API `TCPRoute`; the supported workflow is local `make postgres` with a temporary `kubectl port-forward`

Upstream references used for this integration:

- Envoy AI Gateway install guide: `https://aigateway.envoyproxy.io/docs/getting-started/installation/`
- Envoy AI Gateway base Envoy Gateway values: `https://raw.githubusercontent.com/envoyproxy/ai-gateway/v0.5.0/manifests/envoy-gateway-values.yaml`
