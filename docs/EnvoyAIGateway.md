# Envoy AI Gateway

Envoy AI Gateway is installed through `helm/infra/envoy-ai-gateway/` and reconciled by Flux through the `envoy-ai-gateway` component block and template in `helm/bootstrap/flux-infra/`.

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
- the Flux bootstrap ownership for the `vllm` chart now lives in `helm/bootstrap/flux-infra/` alongside `envoy-ai-gateway`, rather than under `flux-apps`

Pinned upstream versions in this repo:

- Envoy Gateway `v1.7.0`
- Envoy Gateway CRDs `v1.7.0`
- Envoy AI Gateway CRDs `v1.0.0`
- Envoy AI Gateway controller `v1.0.0`
- Gateway API standard CRDs `v1.4.1` via `helm/bootstrap/flux-bootstrap/`

Current repo note:

- Gateway API is pinned to `v1.4.1` to stay aligned with the upstream Envoy Gateway `v1.7.x` compatibility matrix used by this repo
- clean installs split CRD ownership so Flux bootstrap owns Gateway API CRDs, a Flux `GitRepository` + `Kustomization` owns Envoy Gateway CRDs from `envoyproxy/gateway`, a dedicated Flux `HelmRelease` owns Envoy AI Gateway CRDs from the upstream CRD chart, and the controller HelmReleases install with CRD management disabled
- the top-level `envoy-ai-gateway` Flux component in `flux-infra` is configured with wait disabled because it bootstraps child Flux objects; readiness is tracked on the child `HelmRelease` and `Kustomization` resources instead of on the wrapper chart itself
- operator access to Postgres no longer depends on a permanent Gateway API `TCPRoute`; the supported workflow is local `make postgres` with a temporary `kubectl port-forward`

Local vLLM integration:

- chart: `helm/apps/vllm`
- default image: `vllm/vllm-openai:v0.20.2`
- default model: `Qwen/Qwen2.5-0.5B-Instruct`
- default node path: `helheim` with `node-llm=gpu` and `nvidia.com/gpu: 1`
- runtime flags enable OpenAI-compatible serving, tool-call parsing, automatic tool choice, prefix caching, and eager-mode execution for faster startup on the current consumer GPU path; quantization can be enabled by setting `model.quantization`
- the startup probe allows roughly one hour for first-load model download, CUDA setup, and compilation on the lab GPU path before Kubernetes restarts the container
- the assistant service uses `AI_GATEWAY_BASE_URL=http://homelab-vllm-ai-gateway.envoy-gateway-system.svc.cluster.local/v1`; Envoy AI Gateway routes that OpenAI-compatible traffic to the in-cluster vLLM backend
- OpenSearch ML Commons models used by RAG indexing/search should be configured with connectors that call Envoy AI Gateway, so embedding providers can move from local/OpenSearch-backed inference to Bedrock without app code changes
- current gap: the repo creates the OpenSearch ingest/search pipelines but does not yet automate ML Commons model and connector registration; operators must register `OPENSEARCH_RAG_MODEL_ID` against Envoy AI Gateway before processor indexing succeeds

Near-term validation focus:

- confirm the vLLM pod schedules onto `helheim` with the expected `node-llm=gpu` label and one `nvidia.com/gpu`
- confirm the model cache and startup probe settings are sufficient for first-load startup without repeated restarts
- confirm the Envoy AI Gateway service accepts OpenAI-compatible chat requests from `assistant`
- keep the default model conservative until startup time, memory pressure, answer quality, and tool-call compatibility are measured

## Runtime Smoke Test

Use the repo target after the `vllm`, `assistant`, and Envoy AI Gateway releases have reconciled:

```bash
make vllm-gateway-smoke
```

The target runs `scripts/vllm-gateway-smoke.sh`. It:

- checks nodes labeled `node-llm=gpu` and reports allocatable `nvidia.com/gpu`
- waits for `deployment/homelab-vllm` in namespace `homelab`
- prints current vLLM pod placement so `helheim` scheduling is visible
- checks the internal Envoy AI Gateway service `envoy-gateway-system/homelab-vllm-ai-gateway`
- creates a temporary in-cluster curl `Job` in namespace `homelab` with the same app labels used by the assistant network-policy path
- sends an OpenAI-compatible `POST /v1/chat/completions` request to `AI_GATEWAY_BASE_URL` or the legacy `LLM_BASE_URL`
- requires a response containing `choices`

Default smoke-test inputs:

- `AI_GATEWAY_BASE_URL=http://homelab-vllm-ai-gateway.envoy-gateway-system.svc.cluster.local/v1`
- `AI_CHAT_MODEL=Qwen/Qwen2.5-0.5B-Instruct`
- `VLLM_ROLLOUT_TIMEOUT=70m`
- `VLLM_SMOKE_TIMEOUT=5m`

Useful overrides:

```bash
AI_CHAT_MODEL=Qwen/Qwen2.5-0.5B-Instruct make vllm-gateway-smoke
VLLM_SMOKE_KEEP_JOB=1 make vllm-gateway-smoke
VLLM_ROLLOUT_TIMEOUT=10m VLLM_SMOKE_TIMEOUT=2m make vllm-gateway-smoke
```

## Runtime Checks

Start with scheduling and GPU visibility:

```bash
kubectl get nodes -l node-llm=gpu -o wide
kubectl describe node helheim | grep -A8 -E 'Labels:|Allocatable:'
kubectl -n homelab get deploy,pod -l app.kubernetes.io/name=vllm -o wide
kubectl -n homelab describe pod -l app.kubernetes.io/name=vllm
```

Then inspect startup and cache behavior:

```bash
kubectl -n homelab logs deploy/homelab-vllm --tail=200
kubectl -n homelab get events --sort-by=.lastTimestamp | grep -E 'homelab-vllm|Failed|BackOff|Unhealthy'
kubectl -n homelab get deploy homelab-vllm -o jsonpath='{.spec.template.spec.volumes[?(@.name=="model-cache")].emptyDir.sizeLimit}{"\n"}'
```

Check the Gateway resources and generated proxy service:

```bash
kubectl -n homelab get gatewayclass,gateway,backend,aiservicebackend,aigatewayroute
kubectl -n envoy-gateway-system get svc,pod -l gateway.envoyproxy.io/owning-gateway-name=vllm-ai-gateway -o wide
kubectl -n envoy-gateway-system logs -l gateway.envoyproxy.io/owning-gateway-name=vllm-ai-gateway --tail=120
```

If `make vllm-gateway-smoke` fails:

- a pending vLLM pod usually means `helheim` is missing the `node-llm=gpu` label, the GPU device plugin is not advertising `nvidia.com/gpu`, or the node cannot satisfy CPU/memory requests
- repeated startup probe failures usually mean first-load model download or CUDA setup exceeded the current probe window, the model cache is too small, or the image cannot initialize the local GPU runtime
- `curl` timeouts from the smoke job usually point to the Gateway service, Envoy proxy readiness, or network-policy labels on the assistant-to-gateway path
- HTTP 4xx/5xx responses with vLLM logs usually mean the served model name, OpenAI-compatible path, or Envoy AI Gateway route no longer matches the assistant configuration

Keep the default model small and unquantized until this smoke test is reliable and the startup time, memory pressure, response quality, and tool-call behavior are measured on the live `helheim` GPU path.

Upstream references used for this integration:

- Envoy AI Gateway install guide: `https://aigateway.envoyproxy.io/docs/getting-started/installation/`
- Envoy AI Gateway base Envoy Gateway values: `https://raw.githubusercontent.com/envoyproxy/ai-gateway/v1.0.0/manifests/envoy-gateway-values.yaml`
