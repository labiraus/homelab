{{- define "flux-utils.allReleases" -}}
{{- $all := list -}}
{{- range $r := .Values.releases }}
{{- $all = append $all $r -}}
{{- with $r.dependencies }}
{{- range $dep := . }}
{{- $all = append $all $dep -}}
{{- end }}
{{- end }}
{{- end }}
{{- toYaml $all -}}
{{- end -}}
