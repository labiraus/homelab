{{- define "commonscaled.manifest" -}}
{{- include "commonscaled.deployment" . }}
{{- include "commonscaled.serviceAccount" . }}
{{- include "commonscaled.rolebinding" . }}
{{- include "commonscaled.rbac" . }}
{{- include "commonscaled.service" . }}
{{- include "commonscaled.secret" . }}
{{- include "commonscaled.generatedSecrets" . }}
{{- include "commonscaled.configmap" . }}
{{- include "commonscaled.poddisruptionbudget" . }}
{{- include "commonscaled.scaledObject" . }}
{{- include "commonscaled.istio" . }}
{{- include "commonscaled.networkPolicy" . }}
{{- end}}
