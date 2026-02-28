{{ define "commonapi.httproute" }}
{{- if .Values.ingress.enabled -}}
{{- $root := . -}}
{{- range $host := .Values.ingress.hosts }}
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: {{ include "commonapi.fullname" $root }}
  namespace: {{ $root.Values.namespace }}
spec:
  hostnames: [{{ $host.host }}]
  parentRefs:
  - name: gateway
    namespace: ingress
  rules:
  {{- range $path := $host.paths }}
  - matches:
    - path:
        type: {{ $path.pathType }}
        value: {{ $path.path }}
    {{- if $path.rewrite }}
    filters:
      - type: URLRewrite
        urlRewrite:
{{ $path.rewrite | toYaml | indent 10 }}
    {{- end }}
    backendRefs:
    - name: {{ include "commonapi.fullname" $root }}
      kind: Service
      port: 80
  {{- end }}
---
{{- end }}
{{- end }}
{{- end -}}
