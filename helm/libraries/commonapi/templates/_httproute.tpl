{{ define "commonapi.httproute" }}
{{- if .Values.ingress.enabled -}}
{{- $root := . -}}
{{- $ingress := .Values.ingress | default dict -}}
{{- $httpsRedirect := (get $ingress "httpsRedirect" | default false) -}}
{{- $httpListenerName := (get $ingress "httpListenerName" | default "default") -}}
{{- $httpsListenerName := (get $ingress "httpsListenerName" | default "https") -}}
{{- $acmeChallengePath := (get $ingress "acmeChallengePath" | default "/.well-known/acme-challenge/") -}}
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
    {{- if $httpsRedirect }}
    sectionName: {{ $httpsListenerName }}
    {{- end }}
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
{{ "\n" }}---{{ "\n" }}
{{- if $httpsRedirect }}
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: {{ include "commonapi.fullname" $root }}-http-redirect
  namespace: {{ $root.Values.namespace }}
spec:
  hostnames: [{{ $host.host }}]
  parentRefs:
  - name: gateway
    namespace: ingress
    sectionName: {{ $httpListenerName }}
  rules:
  {{- range $path := $host.paths }}
  {{- if and (eq $path.pathType "PathPrefix") (eq $path.path "/") }}
  - matches:
    - path:
        type: Exact
        value: /
    filters:
    - type: RequestRedirect
      requestRedirect:
        scheme: https
        statusCode: 301
  {{- else }}
  - matches:
    - path:
        type: {{ $path.pathType }}
        value: {{ $path.path }}
    filters:
    - type: RequestRedirect
      requestRedirect:
        scheme: https
        statusCode: 301
  {{- end }}
  {{- end }}
{{ "\n" }}---{{ "\n" }}
{{- end }}
{{- end }}
{{- end }}
{{- end -}}
