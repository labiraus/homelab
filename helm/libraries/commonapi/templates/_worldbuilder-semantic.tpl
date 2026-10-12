{{- define "commonapi.worldbuilderSemanticConfig" -}}
{{- $s := .Values.worldbuilder.semantic }}
{{- if $s.enabled }}
{{- if not .Values.worldbuilder.enabled }}{{- fail "semantic search requires worldbuilder.enabled" }}{{- end }}
{{- $url := required "semantic embeddingURL is required" $s.embeddingURL }}
{{- if not (regexMatch "^https?://[a-z0-9-]+\\.[a-z0-9-]+\\.svc\\.cluster\\.local(:[0-9]+)?/v1/embeddings$" $url) }}{{- fail "semantic embeddings require a private cluster service URL" }}{{- end }}
{{- $model := required "semantic model is required" $s.model }}
{{- $revision := required "semantic immutable model revision is required" $s.revision }}
{{- if or (lt (int $s.dimensions) 2) (gt (int $s.dimensions) 2048) }}{{- fail "semantic dimensions must be 2..2048" }}{{- end }}
{{- $_ := set .Values "env" (merge (dict "WORLDBUILDER_SEMANTIC_ENABLED" "true" "WORLDBUILDER_EMBEDDING_URL" $url "WORLDBUILDER_EMBEDDING_MODEL" $model "WORLDBUILDER_EMBEDDING_REVISION" $revision "WORLDBUILDER_EMBEDDING_DIMENSIONS" (toString (int64 $s.dimensions)) "WORLDBUILDER_EMBEDDING_QUERY_PREFIX" $s.queryPrefix) (.Values.env | default dict)) }}
{{- if $s.tokenSecret }}{{- $_ := set .Values "envFromSecrets" (append .Values.envFromSecrets $s.tokenSecret) }}{{- end }}
{{- end }}
{{- end }}

{{- define "commonapi.worldbuilderSemanticNetwork" -}}
{{- if .Values.worldbuilder.semantic.enabled }}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ include "commonapi.fullname" . }}-canon-embeddings
  namespace: {{ .Values.namespace }}
spec:
  podSelector:
    matchLabels:
      {{- include "commonapi.selectorLabels" . | nindent 6 }}
  policyTypes: [Egress]
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: envoy-gateway-system
          podSelector:
            matchLabels:
              app.kubernetes.io/component: proxy
              app.kubernetes.io/name: envoy
              gateway.envoyproxy.io/owning-gateway-name: vllm-ai-gateway
              gateway.envoyproxy.io/owning-gateway-namespace: homelab
      ports:
        - protocol: TCP
          port: 80
        - protocol: TCP
          port: 10080
{{- end }}
{{- end }}
