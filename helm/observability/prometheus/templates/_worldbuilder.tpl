{{- define "prometheus.worldbuilder.validate" -}}
{{- if .Values.worldbuilderObservability.enabled -}}
{{- if not (regexMatch "^[0-9]+\\.[0-9]+\\.[0-9]+$" .Values.stack.chartVersion) -}}
{{- fail "Worldbuilder provisioning requires an explicitly verified exact stack.chartVersion" -}}
{{- end -}}
{{- if not (regexMatch "^[A-Za-z0-9_-]{1,40}$" .Values.worldbuilderObservability.datasourceUID) -}}
{{- fail "Worldbuilder provisioning requires an explicit Grafana datasourceUID (1-40 letters, digits, _ or -)" -}}
{{- end -}}
{{- range $namespace := list .Values.stack.targetNamespace .Values.worldbuilderObservability.externalNamespace -}}
{{- if or (gt (len $namespace) 63) (not (regexMatch "^[a-z0-9]([a-z0-9-]*[a-z0-9])?$" $namespace)) -}}
{{- fail "Worldbuilder provisioning requires exact targetNamespace and externalNamespace" -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- end -}}
