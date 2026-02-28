{{ define "commonapi.secret" }}
{{- if and ( .Values.secret ) ( .Values.secret.enabled ) }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ .Release.Name }}-secret
  namespace: {{ .Values.namespace }}
type: {{ .Values.secret.type }}
data:
  {{- range $key, $value := .Values.secret.data }}
  {{ $key }}: {{ $value | b64enc | quote }}
  {{- end }}
---
{{- end }}
{{- end -}}