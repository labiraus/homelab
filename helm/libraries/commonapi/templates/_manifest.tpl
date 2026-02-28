{{- define "commonapi.manifest" -}}
{{- include "commonapi.deployment" . }}
{{- include "commonapi.serviceAccount" . }}
{{- include "commonapi.rolebinding" . }}
{{- include "commonapi.service" . }}
{{- include "commonapi.secret" . }}
{{- include "commonapi.configmap" . }}
{{- include "commonapi.poddisruptionbudget" . }}
{{- include "commonapi.hpa" . }}
{{- include "commonapi.httproute" .}}
{{- include "commonapi.canary" .}}
{{- end}}
