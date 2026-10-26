# Inference

The source catalogue and procedures below are versioned repository contracts.
Publication, activation and live model acceptance are tracked in [status](status.md).

The independent Go service in `apps/inference` is the policy boundary for Worldbuilder and operator experiments. It uses the existing Envoy AI Gateway controllers for translation and routing. Local is the default; there is no paid fallback. Source activation is opt-in; the latest retained evidence does not establish publication or deployment of the independent proxy. AWS account access, allowance eligibility and live model acceptance remain activation gates.

## Routes and contracts

| Alias | Profile revision | Upstream | Policy |
| --- | --- | --- | --- |
| `local-default` | `local-qwen-3b-r1` | Qwen/Qwen2.5-3B-Instruct-AWQ on Helheim | No monetary/daily quota; two concurrent calls |
| `bedrock-nova` | `nova-2-lite-r1` | `eu.amazon.nova-2-lite-v1:0` | Explicit selection; shared $100 UTC-month ceiling |
| `bedrock-sonnet` | `sonnet-4-5-r1` | `eu.anthropic.claude-sonnet-4-5-20250929-v1:0` | Same ceiling across all clients |

`helm/infra/inference/values.yaml` is the versioned catalogue. Change a profile revision whenever model weights, relevant runtime settings or response behavior change. The initial contract supports non-streaming text, function tools and `response_format: {type: "json_schema", json_schema: ...}`. Cloud structured responses use a required schema-constrained tool call, then JSON Schema validation and normalization. Worldbuilder independently validates semantic/citation boundaries. Streaming, images, audio, extended thinking, provider caching, arbitrary provider parameters and unknown fields are refused. Embeddings retain their existing route and settings.

`GET /v1/models` and `POST /v1/chat/completions` require a bearer credential. Requests are bounded to 256 KiB, 128 messages, 32 tools and a 60-second upstream timeout. The local context limit is 4,096 tokens (including output, enforced by vLLM), output at most 768; cloud output is limited to 4,096. A UUID `X-Request-ID` correlates each attempt. A cloud ID can never dispatch twice, including after process restart. A retryable capacity rejection returns 429 and `Retry-After`; an uncertain dispatched request must never be replayed automatically with a fresh ID.

Responses carry `inference` provenance: profile/revision, provider, model, proxy request ID, accounting status and integer USD micros. Costs are estimates, not AWS invoices. Operator-only `GET /v1/usage` exposes settled estimates, held reservations and remaining budget. `POST /admin/local` with `{"drain":true}` rejects new application calls and lets active calls finish; operator evaluations remain permitted. `GET /admin/local` returns drain state and active count. Health and metrics endpoints are private network surfaces.

## Secrets and local access

Create separate randomly generated credentials for `worldbuilder` and `operator`; each must contain at least 32 characters. Store `clients.json` in Secret `inference-clients` in `homelab`:

```json
[{"id":"worldbuilder","token":"<random secret>","cloud":false},
 {"id":"operator","token":"<different random secret>","cloud":false}]
```

The example placeholders are not usable credentials. Give the processor its own Secret `inference-worldbuilder` with key `INFERENCE_TOKEN` matching the Worldbuilder client. Only enable a client's `cloud` flag after its paid access is intended. No application receives AWS credentials. Changing Secret contents requires a proxy rollout because credentials are loaded at startup.

Keep the endpoint private. Use an authenticated Kubernetes localhost forward, then the independent operator token:

```sh
kubectl -n homelab port-forward --address 127.0.0.1 service/inference 18080:8080
# In a separate shell, load INFERENCE_OPERATOR_TOKEN from your secret store.
python3 scripts/inference/experiment.py status
python3 scripts/inference/experiment.py evaluate --revision local-qwen-3b-r1
```

The proxy accepts processor traffic through NetworkPolicy. Its separate upstream Gateway only accepts proxy pods; strict vLLM policy admits this Gateway and removes processor/direct legacy chat access. Kubernetes administrators retain administrative port-forward/exec authority; these policies are not intended to constrain cluster administrators. Do not grant application identities pod creation, label modification or port-forward rights. The legacy Gateway remains for embeddings.

## Accounting and database

Provision an independent `inference` database on the existing PostgreSQL service, a migration owner, and a non-owner `inference_runtime` login with no inherited roles, superuser, role creation or RLS bypass. Apply `sql/inference/001_up.sql` as the owner and `grants.sql` explicitly. Runtime must have no Worldbuilder or legacy app-schema privileges. Do not reuse Worldbuilder's DSN. The optional `helm/bootstrap/flux-data/values-inference.yaml` overlay adds CNPG-managed owner/runtime roles and narrowly scoped database ingress; provision its password Secrets in `data` first. Open `inferenceRoles.migrationLogin` only during explicit migrations. As database administrator create the independent database owned by `inference_owner`, revoke PUBLIC database access, grant CONNECT to owner/runtime and close the owner login after migration. Secret named by `databaseSecret` supplies `INFERENCE_DATABASE_URL`; require TLS for the cluster database connection. Local operation can start and continue without a database.

One database advisory transaction lock serializes reservations across clients, models and proxy instances. Before any cloud dispatch it reserves maximum billable model input (a deliberately conservative 1,000,000 tokens) plus requested allowed output, using integer USD micros and ceiling rounding. The fixed tariff version, rates and reservation are immutable. Response usage settles once, at the captured rate, only within the original upper bound. Reservations are retained for missing usage, transport errors, cancellation or failed settlement. A 120-second capacity lease expires independently of the financial hold. Unknown outcomes are not refunds. The configured ceiling can be reduced but cannot exceed $100 in this implementation.

Usage belongs to the UTC month of reservation; a late response settles that original month. Old unresolved reservations remain retained. Missing/expired pricing, disabled models, unavailable accounting or exhausted budget refuses cloud requests. This conservative approach can exhaust available capacity before AWS actually bills $100. It never claims to enforce the entire AWS account bill, taxes, or non-proxy traffic.

Configure `monitoring.enabled` and the existing Prometheus label selectors to expose metrics, budget alerts at 50%, 80%, 95%, and accounting-unavailable alerts. The Grafana dashboard is optional. Verify the actual scrape, rule evaluation and notification delivery before paid activation. AWS Budgets is a delayed secondary check, configured by Terraform with credits/refunds excluded and optional notification emails.

Back up the independent database and roles with the existing CNPG backup infrastructure. Restore into an isolated database with cloud disabled, compare all attempt rows and grants, then reconnect using the same ledger. Never start a fresh ledger against an already-used month's budget. Never discard unresolved reservations during restore or rollback. The standalone disposable test exercises populated ledger dump/restore and denial of runtime deletion; Worldbuilder's full physical fixture covers its separate historical/pinned audit records.

## Local model experiments

The pinned baseline is vLLM 0.20.2, Qwen2.5 3B AWQ revision `3559b226e8ce77211e2c1bd7ddfb7686fec4d6dd`, Helheim's single GPU, eager execution and 0.85 memory utilization. The activation overlay adds an 80 GiB Longhorn PVC with retention. One Recreate Deployment owns one GPU model. Midgard and alternate runtimes are deferred.

Use `helm/infra/vllm/profiles/helheim-qwen3b.yaml` as the baseline experiment profile. A new profile must pin an immutable model revision and compatible tool parser/context limits. Keep the corresponding inference catalogue and Worldbuilder revision pins consistent. Do not switch a model in response to an individual request.

1. Record the currently published image/chart, values and PVC identity. Publish `config.local_paused: true` through the inference Flux values and wait for reconciliation. This persists maintenance across proxy replacement. Run `experiment.py drain` and require `active: 0`.
2. Publish the selected vLLM profile and the matching proxy catalogue revision through the normal CI/Flux path. The retained cache survives replacement; expect the first weight download to take longer. Wait for vLLM readiness and inspect GPU allocation, events and logs.
3. While application access remains paused, evaluate with the operator credential. `experiment.py evaluate` reports first/warm latency, two-call throughput and token usage; optional `--gpu-pod` records NVIDIA memory samples. Retain cold startup duration separately from pod events because the first API call alone does not measure model loading.
4. Run the existing bounded Worldbuilder eleven-case quality suite through the proxy. Confirm structured responses, scope/citations and no unintended canon writes. Verify cache PVC identity/retained model files after a restart, then repeat the warm suite.
5. On failure, restore the recorded baseline profile and catalogue, keep the same PVC, wait for readiness and repeat validation. On success, set the selected Worldbuilder profile revision, publish `local_paused: false`, then resume. Queued work with an unavailable old pin refuses execution until its profile is restored; it never silently changes provider.

## AWS preparation and activation

Use ignored `.local/aws/config` and `.local/aws/credentials` for an operator profile named `homelab-bedrock` in `eu-west-1`, populated privately when needed. Do not assume a prior session's local profile exists. It is operator provisioning access; workload access uses Roles Anywhere certificates, never these static credentials. To select it:

```sh
export AWS_CONFIG_FILE="$PWD/.local/aws/config"
export AWS_SHARED_CREDENTIALS_FILE="$PWD/.local/aws/credentials"
export AWS_PROFILE=homelab-bedrock
aws sts get-caller-identity
```

Use the repository `bin/tf` workflow with component `inference-bedrock` and node layer `inference`; supply the **public** dedicated CA certificate via `TF_VAR_inference_ca_certificate_pem`. Keep the CA private key offline, outside this repository and cluster. Review the plan before applying. The module creates a CN-bound role, Roles Anywhere anchor/profile, exact profile/model IAM permissions and AWS budget. The role allows non-streaming InvokeModel only, scoped to the two EU inference profiles and their foundation models in Frankfurt, Stockholm, Milan, Spain, Ireland and Paris. Global profiles and unrelated models are absent from both IAM and proxy allowlists. Reverify destination resources with `bedrock get-inference-profile` before enabling because availability can change.

Issue a short-lived dedicated workload certificate directly from that CA with subject CN `inference-bedrock`, a valid chain, digital-signature key usage and client-auth EKU. Store only its leaf certificate and private key in `inference-bedrock-certificate` in **envoy-gateway-system**, where Envoy creates its data-plane pods. Never put the CA private key in this Secret. Set the three Terraform output ARNs and immutable published helper image in the cloud chart values.

The pinned helper 1.8.5 is injected next to the credential-consuming gateway process. `serve` exposes an IMDSv2 endpoint on `127.0.0.1:9911` with hop limit 1; GatewayConfig sets the default credential chain's endpoint and disables IMDSv1. The helper refreshes temporary credentials automatically. No localhost credential port is exposed by a Service. Image downloads verify architecture-specific checksums. An optional BuildKit `build_ca` secret permits builds behind a trusted TLS inspection CA without disabling certificate verification or baking that CA into runtime images.

Renew certificates before expiry: issue from the offline CA, verify subject/chain/validity, update the workload Secret, restart the gateway pod through the configured rollout workflow, and prove a bounded authenticated paid request obtains fresh credentials. Retain the previous valid certificate privately until renewal is verified. To revoke, disable cloud in the proxy first, revoke the certificate through an imported Roles Anywhere CRL (or disable its trust anchor/profile for immediate issuance denial), and invalidate existing role sessions when required. Revocation/expiry stops new credentials; already-issued sessions can remain usable until expiry. The proxy enable flag provides immediate refusal independently of AWS credential expiry. Test real renewal, expiry and revocation with the account before production cloud activation; the offline fixture uses synthetic temporary credentials and does not prove AWS PKI behavior.

Verify the $145 allowance includes Bedrock, model access/EULA/account requirements, EU profile destinations, current standard EU input/output tariffs and any long-context premiums. Populate each rate in USD micros per million tokens, a traceable `price_version`, and bounded RFC3339 `price_valid_until`. Set conservative maximum rates that cover the entire permitted input range. Shipped rates are zero and intentionally cannot activate paid routing. Price expiry refuses paid requests rather than silently using an old tariff.

Activate in stages:

1. Before publishing the updated external/orchestrator binaries, back up and apply Worldbuilder migration 010 and grants: enabled binaries require schema 10 even while proxy activation is off. Drain current generation work before that coordinated upgrade. Provision client Secrets and publish the inference image/chart through App Inference CI/CD. Merge the explicit `helm/bootstrap/flux-infra/values-inference.yaml` and `helm/bootstrap/flux-apps/values-inference.yaml` settings into the repository's effective Flux values. Default values alone leave activation off. Local cutover removes the old chat bypass; embeddings stay unchanged. Drain existing generation/inference before upgrading; legacy queued jobs retain legacy pins and require explicit handling.
2. Verify local authenticated requests, direct bypass denial, Worldbuilder quality/generation recovery, retained cache and dashboards before enabling any cloud route.
3. Enable cloud with verified pricing, dedicated ledger, certificate/helper ARNs and **operator-only** cloud permission. Run `experiment.py evaluate --model bedrock-nova --paid` and then `bedrock-sonnet`; run the same Worldbuilder eleven-case suite for each through the proxy. These calls consume the shared budget.
4. Only after both model-specific acceptance checks, permit Worldbuilder cloud access and explicitly configure `worldbuilder.aiBudget.profile` and `profileRevision` in deployment values. Keep `local-default` otherwise. Queued jobs retain their original profile/revision; their provider never follows a later configuration change.

Rollback disables `cloud.enabled`, returns Worldbuilder to the tested local profile and preserves both ledgers and model cache. Do not downgrade populated schema 010 or erase historical allocations.

## Verification and references

Offline commands: `go test -race ./...` in `apps/inference`, `bash scripts/inference/test-postgres.sh`, `python3 scripts/inference/test-manifests.py`, the existing Worldbuilder PostgreSQL/platform/backup gates, and `python3 scripts/inference/test-gateway.py OUTPUT SCHEMAS` after fetching pinned platform schemas. Gateway acceptance uses actual Envoy/Cilium controllers with synthetic providers and temporary credentials; no AWS account or production cluster is used. See [acceptance status](status.md) for the boundary between source tests and live deployment.

After building the processor with `npm --prefix apps/processor run build`, run the bounded Worldbuilder suite through the authenticated localhost forward:

```sh
AI_GATEWAY_BASE_URL=http://127.0.0.1:18080/v1 AI_CHAT_MODEL=local-default \
INFERENCE_PROFILE=local-default INFERENCE_PROFILE_REVISION=local-qwen-3b-r1 \
INFERENCE_TOKEN="$INFERENCE_OPERATOR_TOKEN" node scripts/worldbuilder/check-live-ai.mjs
```

For an approved paid evaluation, select the cloud alias and its matching revision in these variables and explicitly add `INFERENCE_ALLOW_PAID=true`. The suite does not bypass proxy accounting. Never substitute a direct Bedrock endpoint.

Authoritative activation references: [Nova 2 Lite](https://docs.aws.amazon.com/bedrock/latest/userguide/model-card-amazon-nova-2-lite.html), [Sonnet 4.5](https://docs.aws.amazon.com/bedrock/latest/userguide/model-card-anthropic-claude-sonnet-4-5.html), [Bedrock pricing](https://aws.amazon.com/bedrock/pricing/), [credential helper](https://docs.aws.amazon.com/rolesanywhere/latest/userguide/credential-helper.html), [AWS Budgets](https://docs.aws.amazon.com/cost-management/latest/userguide/budgets-managing-costs.html). Account-specific eligibility and current tariffs must be verified before paid activation.

## Legacy gateway and embeddings

Envoy AI Gateway is installed through `helm/infra/envoy-ai-gateway/` and reconciled by Flux through the `envoy-ai-gateway` component block and template in `helm/bootstrap/flux-infra/`.

The infra stack installs two upstream controller artifacts plus two parent-owned CRD sources:

- Envoy Gateway CRDs from the upstream `envoyproxy/gateway` Git repository
- Envoy Gateway into `envoy-gateway-system`
- Envoy AI Gateway CRDs from the upstream `envoyproxy/ai-gateway-crds-helm` OCI artifact via a dedicated Flux `HelmRelease`
- Envoy AI Gateway controller into `envoy-ai-gateway-system`

The Envoy Gateway Helm values include the upstream AI Gateway compatibility settings so Envoy Gateway exposes the Backend API and the xDS translation hooks required by Envoy AI Gateway.

Current scope:

- installs the control-plane components only
- app-specific AI routing resources now live with the infra chart that owns the model runtime
- `helm/infra/vllm` creates a local vLLM service plus Envoy Gateway `Backend`, Envoy AI Gateway `AIServiceBackend`, `AIGatewayRoute`, and an internal `GatewayClass`/`Gateway` for the first OpenAI-compatible local model path
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

- chart: `helm/infra/vllm`
- default image: `vllm/vllm-openai:v0.20.2`
- default model: `Qwen/Qwen2.5-3B-Instruct-AWQ`
- default node path: `helheim` with `node-llm=gpu` and `nvidia.com/gpu: 1`
- runtime flags enable OpenAI-compatible serving, tool-call parsing, automatic tool choice, prefix caching, and eager-mode execution for faster startup on the current consumer GPU path; quantization can be enabled by setting `model.quantization`
- the startup probe allows roughly one hour for first-load model download, CUDA setup, and compilation on the lab GPU path before Kubernetes restarts the container
- the processor service uses `AI_GATEWAY_BASE_URL=http://homelab-vllm-ai-gateway.envoy-gateway-system.svc.cluster.local/v1`; Envoy AI Gateway routes that OpenAI-compatible traffic to the in-cluster vLLM backend
- OpenSearch ML Commons models used by RAG indexing/search should be configured with connectors that call Envoy AI Gateway, so embedding providers can move from local/OpenSearch-backed inference to Bedrock without app code changes
- current gap: the repo creates the OpenSearch ingest/search pipelines but does not yet automate ML Commons model and connector registration; operators must register `OPENSEARCH_RAG_MODEL_ID` against Envoy AI Gateway before processor indexing succeeds
- `helm/infra/embeddings` supplies a separate private CPU embedding runtime and model-specific gateway route. Owned real inference passes and bootstrap enables its deployment; live gateway, registration and ingestion checks remain required. See [document search recovery](documents.md).

Near-term validation focus:

- confirm the vLLM pod schedules onto `helheim` with the expected `node-llm=gpu` label and one `nvidia.com/gpu`
- confirm the model cache and startup probe settings are sufficient for first-load startup without repeated restarts
- confirm the Envoy AI Gateway service accepts OpenAI-compatible chat requests from `processor`
- keep the default model conservative until startup time, memory pressure, answer quality, and tool-call compatibility are measured

## Runtime Smoke Test

Use the repo target after the `vllm`, `processor`, and Envoy AI Gateway releases have reconciled:

```bash
make vllm-gateway-smoke
```

The target runs `scripts/vllm-gateway-smoke.sh`. It:

- checks nodes labeled `node-llm=gpu` and reports allocatable `nvidia.com/gpu`
- waits for `deployment/homelab-vllm` in namespace `homelab`
- prints current vLLM pod placement so `helheim` scheduling is visible
- checks the internal Envoy AI Gateway service `envoy-gateway-system/homelab-vllm-ai-gateway`
- creates a temporary in-cluster curl `Job` in namespace `homelab` with the same app labels used by the processor network-policy path
- sends an OpenAI-compatible `POST /v1/chat/completions` request to `AI_GATEWAY_BASE_URL` or the legacy `LLM_BASE_URL`
- requires a response containing `choices`

Default smoke-test inputs:

- `AI_GATEWAY_BASE_URL=http://homelab-vllm-ai-gateway.envoy-gateway-system.svc.cluster.local/v1`
- `AI_CHAT_MODEL=Qwen/Qwen2.5-3B-Instruct-AWQ`
- `VLLM_ROLLOUT_TIMEOUT=70m`
- `VLLM_SMOKE_TIMEOUT=5m`

Useful overrides:

```bash
AI_CHAT_MODEL=Qwen/Qwen2.5-3B-Instruct-AWQ make vllm-gateway-smoke
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
- `curl` timeouts from the smoke job usually point to the Gateway service, Envoy proxy readiness, or network-policy labels on the processor-to-gateway path
- HTTP 4xx/5xx responses with vLLM logs usually mean the served model name, OpenAI-compatible path, or Envoy AI Gateway route no longer matches the processor configuration

Keep the default model small and unquantized until this smoke test is reliable and the startup time, memory pressure, response quality, and tool-call behavior are measured on the live `helheim` GPU path.

Upstream references used for this integration:

- Envoy AI Gateway install guide: `https://aigateway.envoyproxy.io/docs/getting-started/installation/`
- Envoy AI Gateway base Envoy Gateway values: `https://raw.githubusercontent.com/envoyproxy/ai-gateway/v1.0.0/manifests/envoy-gateway-values.yaml`
