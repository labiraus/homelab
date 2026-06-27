#!/usr/bin/env bash
set -euo pipefail

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Error: $1 is required." >&2
    exit 1
  fi
}

require_command kubectl

namespace="${VLLM_NAMESPACE:-homelab}"
deployment="${VLLM_DEPLOYMENT:-homelab-vllm}"
gateway_namespace="${VLLM_GATEWAY_NAMESPACE:-envoy-gateway-system}"
gateway_service="${VLLM_GATEWAY_SERVICE:-homelab-vllm-ai-gateway}"
gateway_url="${AI_GATEWAY_BASE_URL:-${LLM_BASE_URL:-http://homelab-vllm-ai-gateway.envoy-gateway-system.svc.cluster.local/v1}}"
model="${AI_CHAT_MODEL:-${LLM_MODEL:-Qwen/Qwen2.5-0.5B-Instruct}}"
image="${VLLM_SMOKE_IMAGE:-curlimages/curl:8.11.1}"
rollout_timeout="${VLLM_ROLLOUT_TIMEOUT:-70m}"
job_timeout="${VLLM_SMOKE_TIMEOUT:-5m}"
request_timeout="${VLLM_SMOKE_REQUEST_TIMEOUT:-120}"
job_name="${VLLM_SMOKE_JOB_NAME:-vllm-gateway-smoke-$(date +%s)}"

echo "Checking GPU node labels and allocatable devices..."
kubectl get nodes -l node-llm=gpu -o custom-columns=NAME:.metadata.name,LLM:.metadata.labels.node-llm,GPU:.status.allocatable.'nvidia\.com/gpu',READY:.status.conditions[-1].status --no-headers

echo "Checking vLLM deployment rollout in namespace ${namespace}..."
kubectl -n "$namespace" rollout status "deployment/${deployment}" --timeout="$rollout_timeout"

echo "Current vLLM pod placement:"
kubectl -n "$namespace" get pods -l app.kubernetes.io/instance="$deployment",app.kubernetes.io/name=vllm -o wide

echo "Checking Envoy AI Gateway service ${gateway_namespace}/${gateway_service}..."
kubectl -n "$gateway_namespace" get service "$gateway_service" -o wide

cleanup() {
  if [ "${VLLM_SMOKE_KEEP_JOB:-0}" != "1" ]; then
    kubectl -n "$namespace" delete job "$job_name" --ignore-not-found >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

echo "Creating in-cluster smoke job ${namespace}/${job_name}..."
cat <<EOF | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: ${job_name}
  namespace: ${namespace}
  labels:
    app.kubernetes.io/name: vllm-gateway-smoke
    app.kubernetes.io/part-of: homelab
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 300
  template:
    metadata:
      labels:
        app.kubernetes.io/name: assistant
        app.kubernetes.io/instance: homelab-assistant
        app.kubernetes.io/part-of: homelab
        labiraus.com/smoke-test: vllm-gateway
    spec:
      restartPolicy: Never
      automountServiceAccountToken: false
      containers:
        - name: curl
          image: ${image}
          imagePullPolicy: IfNotPresent
          env:
            - name: AI_GATEWAY_BASE_URL
              value: "${gateway_url}"
            - name: AI_CHAT_MODEL
              value: "${model}"
            - name: REQUEST_TIMEOUT
              value: "${request_timeout}"
          command:
            - sh
            - -c
          args:
            - |
              set -eu
              cat >/tmp/request.json <<JSON
              {"model":"\${AI_CHAT_MODEL}","messages":[{"role":"system","content":"You are a terse runtime smoke test."},{"role":"user","content":"Reply with exactly: vllm smoke ok"}],"temperature":0,"max_tokens":16}
              JSON
              curl -fsS --max-time "\${REQUEST_TIMEOUT}" \
                -H 'Accept: application/json' \
                -H 'Content-Type: application/json' \
                --data @/tmp/request.json \
                "\${AI_GATEWAY_BASE_URL%/}/chat/completions" \
                -o /tmp/response.json
              grep -q '"choices"' /tmp/response.json
              sed -n '1,80p' /tmp/response.json
EOF

if ! kubectl -n "$namespace" wait --for=condition=complete "job/${job_name}" --timeout="$job_timeout"; then
  echo "Smoke job did not complete successfully. Recent logs and pod details follow." >&2
  kubectl -n "$namespace" logs "job/${job_name}" --tail=200 >&2 || true
  pod_name="$(kubectl -n "$namespace" get pods -l job-name="$job_name" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  if [ -n "$pod_name" ]; then
    kubectl -n "$namespace" describe pod "$pod_name" >&2 || true
  fi
  exit 1
fi

echo "Smoke response:"
kubectl -n "$namespace" logs "job/${job_name}"
echo "vLLM Envoy AI Gateway smoke test passed."
