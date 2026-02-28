{{ define "commonapi.serviceAccount" }}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "commonapi.serviceAccountName" . }}
  namespace: {{ .Values.namespace }}
  labels:
    {{- include "commonapi.labels" . | nindent 4 }}
  {{- with .Values.serviceAccount.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
automountServiceAccountToken: {{ .Values.serviceAccount.automount }}
{{- if and ( .Values.secret ) ( .Values.secret.enabled ) }}
secrets:
  - name: {{ .Release.Name }}-secret
{{- end }}
imagePullSecrets:
{{- if .Values.imagePullSecrets }}
  {{- range .Values.imagePullSecrets }}
  - name: {{ .name }}
  {{- end }}
{{- end }}
---
{{- end }}
