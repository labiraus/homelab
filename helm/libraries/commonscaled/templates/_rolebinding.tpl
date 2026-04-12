{{ define "commonscaled.rolebinding" }}
{{- if .Values.roles -}}
{{- $root := . -}}
{{- range $role := .Values.roles }}
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: "rolebinding-{{ include "commonscaled.serviceAccountName" $root }}-{{ $role }}"
  namespace: {{ $root.Values.namespace }}
subjects:
- kind: ServiceAccount
  name: {{ include "commonscaled.serviceAccountName" $root }}
  namespace: {{ $root.Values.namespace }}
roleRef:
  kind: Role
  name: {{ $role | quote }}
  apiGroup: rbac.authorization.k8s.io
---{{ "\n" }}
{{- end }}
{{- end }}
{{- end -}}
