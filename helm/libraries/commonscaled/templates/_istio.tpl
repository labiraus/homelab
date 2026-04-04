{{- define "commonscaled.istio" -}}
{{- $root := . -}}
{{- $security := (.Values.security | default dict) -}}
{{- $istio := (get $security "istio" | default dict) -}}
{{- $peer := (get $istio "peerAuthentication" | default dict) -}}
{{- if (get $peer "enabled" | default false) }}
apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: {{ include "commonscaled.fullname" $root }}
  namespace: {{ $root.Values.namespace }}
spec:
  selector:
    matchLabels:
      {{- include "commonscaled.selectorLabels" $root | nindent 6 }}
  mtls:
    mode: {{ get $peer "mode" | default "STRICT" }}
---
{{- end }}
{{- $authz := (get $istio "authorizationPolicy" | default dict) -}}
{{- if (get $authz "enabled" | default false) }}
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: {{ include "commonscaled.fullname" $root }}
  namespace: {{ $root.Values.namespace }}
spec:
  selector:
    matchLabels:
      {{- include "commonscaled.selectorLabels" $root | nindent 6 }}
  action: {{ get $authz "action" | default "ALLOW" }}
  rules:
{{- toYaml (get $authz "rules" | default list) | nindent 4 }}
---
{{- end }}
{{- end -}}
