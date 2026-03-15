{{- define "commonapi.manifest" -}}
{{- include "commonapi.deployment" . }}
{{- include "commonapi.serviceAccount" . }}
{{- include "commonapi.rolebinding" . }}
{{- include "commonapi.rbac" . }}
{{- include "commonapi.service" . }}
{{- include "commonapi.secret" . }}
{{- include "commonapi.generatedSecrets" . }}
{{- include "commonapi.configmap" . }}
{{- include "commonapi.poddisruptionbudget" . }}
{{- include "commonapi.hpa" . }}
{{- include "commonapi.httproute" .}}
{{- include "commonapi.istio" . }}
{{- include "commonapi.networkPolicy" . }}
{{- end}}
