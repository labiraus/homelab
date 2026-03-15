{{- define "commonapi.generatedSecrets" -}}
{{- $root := . -}}
{{- range $secret := (.Values.generatedSecrets | default list) }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ $secret.name }}
  namespace: {{ $root.Values.namespace }}
  labels:
    {{- include "commonapi.labels" $root | nindent 4 }}
type: {{ $secret.type | default "Opaque" }}
stringData:
  {{- range $key, $value := ($secret.stringData | default dict) }}
  {{ $key }}: {{ include "commonapi.generatedSecretValue" $value | quote }}
  {{- end }}
---
{{- end }}
{{- end -}}
