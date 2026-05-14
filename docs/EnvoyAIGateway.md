# Envoy AI Gateway

Envoy AI Gateway is installed through `helm/infra/envoy-ai-gateway/` and reconciled by Flux through the `envoy-ai-gateway` release declared in `helm/bootstrap/flux-infra/values.yaml`.

The infra stack installs two upstream controller artifacts plus two parent-owned CRD sources:

- Envoy Gateway CRDs from the upstream `envoyproxy/gateway` Git repository
- Envoy Gateway into `envoy-gateway-system`
- Envoy AI Gateway CRDs from the upstream `envoyproxy/ai-gateway-crds-helm` OCI artifact via a dedicated Flux `HelmRelease`
- Envoy AI Gateway controller into `envoy-ai-gateway-system`

The Envoy Gateway Helm values include the upstream AI Gateway compatibility settings so Envoy Gateway exposes the Backend API and the xDS translation hooks required by Envoy AI Gateway.

Current scope:

- installs the control-plane components only
- app-specific AI routing resources now live with the app chart that owns the model runtime
- `helm/apps/vllm` creates a local vLLM service plus Envoy Gateway `Backend`, Envoy AI Gateway `AIServiceBackend`, `AIGatewayRoute`, and an internal `GatewayClass`/`Gateway` for the first OpenAI-compatible local model path
- provider credentials are still not managed by the infra chart; the local vLLM backend does not require an upstream provider API key

Pinned upstream versions in this repo:

- Envoy Gateway `v1.7.0`
- Envoy Gateway CRDs `v1.7.0`
- Envoy AI Gateway CRDs `v0.5.0`
- Envoy AI Gateway controller `v0.5.0`
- Gateway API standard CRDs `v1.4.1` via `helm/bootstrap/flux-bootstrap/`

Current repo note:

- Gateway API is pinned to `v1.4.1` to stay aligned with the upstream Envoy Gateway `v1.7.x` compatibility matrix used by this repo
- clean installs split CRD ownership so Flux bootstrap owns Gateway API CRDs, a Flux `GitRepository` + `Kustomization` owns Envoy Gateway CRDs from `envoyproxy/gateway`, a dedicated Flux `HelmRelease` owns Envoy AI Gateway CRDs from the upstream CRD chart, and the controller HelmReleases install with CRD management disabled
- the top-level `envoy-ai-gateway` wrapper release in `flux-infra` is configured with wait disabled because it bootstraps child Flux objects; readiness is tracked on the child `HelmRelease` and `Kustomization` resources instead of on the wrapper chart itself
- operator access to Postgres no longer depends on a permanent Gateway API `TCPRoute`; the supported workflow is local `make postgres` with a temporary `kubectl port-forward`

Local vLLM integration:

- chart: `helm/apps/vllm`
- default image: `vllm/vllm-openai:v0.20.2`
- default model: `Qwen/Qwen2.5-7B-Instruct-AWQ`
- default node path: `helheim` with `node-llm=gpu` and `nvidia.com/gpu: 1`
- runtime flags enable OpenAI-compatible serving, AWQ quantization, tool-call parsing, automatic tool choice, prefix caching, and eager-mode execution for faster startup on the current consumer GPU path
- the startup probe allows roughly one hour for first-load model download, CUDA setup, and compilation on the lab GPU path before Kubernetes restarts the container
- the assistant service uses `LLM_BASE_URL=http://homelab-vllm-ai-gateway.envoy-gateway-system.svc.cluster.local/v1`; Envoy AI Gateway routes that OpenAI-compatible traffic to the in-cluster vLLM backend

Upstream references used for this integration:

- Envoy AI Gateway install guide: `https://aigateway.envoyproxy.io/docs/getting-started/installation/`
- Envoy AI Gateway base Envoy Gateway values: `https://raw.githubusercontent.com/envoyproxy/ai-gateway/v0.5.0/manifests/envoy-gateway-values.yaml`
