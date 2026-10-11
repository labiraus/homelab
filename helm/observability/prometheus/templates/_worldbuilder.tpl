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

{{- define "prometheus.storage.validate" -}}
{{- if not (kindIs "bool" .Values.prometheusStorage.enabled) -}}
{{- fail "prometheusStorage.enabled must be a boolean" -}}
{{- end -}}
{{- if .Values.prometheusStorage.enabled -}}
{{- if not (regexMatch "^[0-9]+\\.[0-9]+\\.[0-9]+$" .Values.stack.chartVersion) -}}
{{- fail "Persistent Prometheus requires an explicitly verified exact stack.chartVersion" -}}
{{- end -}}
{{- if or (gt (len .Values.stack.targetNamespace) 63) (not (regexMatch "^[a-z0-9]([a-z0-9-]*[a-z0-9])?$" .Values.stack.targetNamespace)) -}}
{{- fail "Persistent Prometheus requires an exact targetNamespace" -}}
{{- end -}}
{{- if or (gt (len .Values.prometheusStorage.storageClassName) 253) (not (regexMatch "^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$" .Values.prometheusStorage.storageClassName)) -}}
{{- fail "Persistent Prometheus requires an explicit storageClassName" -}}
{{- end -}}
{{- if not (regexMatch "^[1-9][0-9]*(Mi|Gi|Ti)$" .Values.prometheusStorage.size) -}}
{{- fail "Persistent Prometheus size must be a positive whole Mi, Gi or Ti quantity" -}}
{{- end -}}
{{- if not (regexMatch "^[1-9][0-9]*(h|d|w)$" .Values.prometheusStorage.retention) -}}
{{- fail "Persistent Prometheus retention must be a positive whole h, d or w duration" -}}
{{- end -}}
{{- end -}}
{{- end -}}
