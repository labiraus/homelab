{{- define "commonapi.istio" -}}
{{- $root := . -}}
{{- $security := (.Values.security | default dict) -}}
{{- $istio := (get $security "istio" | default dict) -}}
{{- $peer := (get $istio "peerAuthentication" | default dict) -}}
{{- if (get $peer "enabled" | default false) }}
apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: {{ include "commonapi.fullname" $root }}
  namespace: {{ $root.Values.namespace }}
spec:
  selector:
    matchLabels:
      {{- include "commonapi.selectorLabels" $root | nindent 6 }}
  mtls:
    mode: {{ get $peer "mode" | default "STRICT" }}
---{{ "\n" }}
{{- end }}
{{- $authz := (get $istio "authorizationPolicy" | default dict) -}}
{{- $authzList := (get $istio "authorizationPolicies" | default list) -}}
{{- if gt (len $authzList) 0 }}
{{- range $index, $policy := $authzList }}
{{- $nameSuffix := (get $policy "nameSuffix" | default (printf "%d" $index)) -}}
{{- $provider := (get $policy "provider" | default dict) -}}
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: {{ include "commonapi.fullname" $root }}-{{ $nameSuffix }}
  namespace: {{ $root.Values.namespace }}
spec:
  {{- if (get $policy "targetRefs") }}
  targetRefs:
{{ toYaml (get $policy "targetRefs") | nindent 4 }}
  {{- else }}
  selector:
    matchLabels:
      {{- include "commonapi.selectorLabels" $root | nindent 6 }}
  {{- end }}
  action: {{ get $policy "action" | default "ALLOW" }}
  {{- if eq (get $policy "action" | default "ALLOW") "CUSTOM" }}
  provider:
    name: {{ get $provider "name" | quote }}
  {{- end }}
  rules:
{{- toYaml (get $policy "rules" | default list) | nindent 4 }}
---{{ "\n" }}
{{- end }}
{{- else if (get $authz "enabled" | default false) }}
{{- $provider := (get $authz "provider" | default dict) -}}
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: {{ include "commonapi.fullname" $root }}
  namespace: {{ $root.Values.namespace }}
spec:
  selector:
    matchLabels:
      {{- include "commonapi.selectorLabels" $root | nindent 6 }}
  action: {{ get $authz "action" | default "ALLOW" }}
  {{- if eq (get $authz "action" | default "ALLOW") "CUSTOM" }}
  provider:
    name: {{ get $provider "name" | quote }}
  {{- end }}
  rules:
{{- toYaml (get $authz "rules" | default list) | nindent 4 }}
---{{ "\n" }}
{{- end }}
{{- end -}}
