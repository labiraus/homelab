{{- define "commonapi.networkPolicy" -}}
{{- $root := . -}}
{{- $security := (.Values.security | default dict) -}}
{{- $networkPolicy := (get $security "networkPolicy" | default dict) -}}
{{- if (get $networkPolicy "enabled" | default false) }}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ include "commonapi.fullname" $root }}
  namespace: {{ $root.Values.namespace }}
spec:
  podSelector:
    matchLabels:
      {{- include "commonapi.selectorLabels" $root | nindent 6 }}
  policyTypes:
    {{- if (get $networkPolicy "defaultDenyIngress" | default true) }}
    - Ingress
    {{- end }}
    {{- if (get $networkPolicy "defaultDenyEgress" | default true) }}
    - Egress
    {{- end }}
  {{- with (get $networkPolicy "ingress" | default list) }}
  ingress:
{{- toYaml . | nindent 4 }}
  {{- end }}
  {{- with (get $networkPolicy "egress" | default list) }}
  egress:
{{- toYaml . | nindent 4 }}
  {{- end }}
{{ "\n" }}---{{ "\n" }}
{{- end }}
{{- end -}}
