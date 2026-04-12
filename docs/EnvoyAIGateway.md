# Envoy AI Gateway

Envoy AI Gateway is installed through `helm/infra/envoy-ai-gateway/` and reconciled by Flux through the `envoy-ai-gateway` release declared in `helm/bootstrap/flux-apps/values.yaml`.

The infra stack installs three upstream OCI charts:

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
- Envoy AI Gateway CRDs `v0.5.0`
- Envoy AI Gateway controller `v0.5.0`

Upstream references used for this integration:

- Envoy AI Gateway install guide: `https://aigateway.envoyproxy.io/docs/getting-started/installation/`
- Envoy AI Gateway base Envoy Gateway values: `https://raw.githubusercontent.com/envoyproxy/ai-gateway/v0.5.0/manifests/envoy-gateway-values.yaml`
