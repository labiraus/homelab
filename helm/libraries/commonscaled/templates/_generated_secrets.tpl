{{- define "commonscaled.generatedSecrets" -}}
{{- $root := . -}}
{{- range $secret := (.Values.generatedSecrets | default list) }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ $secret.name }}
  namespace: {{ $root.Values.namespace }}
  labels:
    {{- include "commonscaled.labels" $root | nindent 4 }}
type: {{ $secret.type | default "Opaque" }}
stringData:
  {{- range $key, $value := ($secret.stringData | default dict) }}
  {{ $key }}: {{ include "commonscaled.generatedSecretValue" (dict "root" $root "spec" $value) | quote }}
  {{- end }}
{{ "\n" }}---{{ "\n" }}
{{- end }}
{{- end -}}
