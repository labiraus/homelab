{{/*
Expand the name of the chart.
*/}}
{{- define "commonscaled.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "commonscaled.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "commonscaled.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "commonscaled.labels" -}}
helm.sh/chart: {{ include "commonscaled.chart" . }}
{{ include "commonscaled.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "commonscaled.selectorLabels" -}}
app.kubernetes.io/name: {{ include "commonscaled.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "commonscaled.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "commonscaled.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Resolve a generated secret value from either a literal or an existing in-cluster Secret.
*/}}
{{- define "commonscaled.generatedSecretValue" -}}
{{- $root := .root -}}
{{- $spec := .spec -}}
{{- $allowMissing := false -}}
{{- if $root.Values.global -}}
{{- $allowMissing = default false $root.Values.global.allowMissingGeneratedSecretRefs -}}
{{- end -}}
{{- if hasKey $spec "value" -}}
{{- $spec.value -}}
{{- else if hasKey $spec "fromSecretRef" -}}
{{- $ref := $spec.fromSecretRef -}}
{{- $secret := lookup "v1" "Secret" $ref.namespace $ref.name -}}
{{- if not $secret -}}
{{- if $allowMissing -}}
{{- printf "missing-secret:%s/%s:%s" $ref.namespace $ref.name $ref.key -}}
{{- else -}}
{{- fail (printf "generated secret source Secret %s/%s not found" $ref.namespace $ref.name) -}}
{{- end -}}
{{- else -}}
{{- $value := index ($secret.data | default dict) $ref.key -}}
{{- if not $value -}}
{{- if $allowMissing -}}
{{- printf "missing-secret-key:%s/%s:%s" $ref.namespace $ref.name $ref.key -}}
{{- else -}}
{{- fail (printf "generated secret source key %s missing in Secret %s/%s" $ref.key $ref.namespace $ref.name) -}}
{{- end -}}
{{- else -}}
{{- $value | b64dec -}}
{{- end -}}
{{- end -}}
{{- else -}}
{{- fail "generated secret entries must define either value or fromSecretRef" -}}
{{- end -}}
{{- end -}}
