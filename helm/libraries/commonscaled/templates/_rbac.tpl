{{- define "commonscaled.rbac" -}}
{{- $root := . -}}
{{- $rbac := (.Values.rbac | default dict) -}}
{{- $rules := (get $rbac "rules" | default list) -}}
{{- if gt (len $rules) 0 }}
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ include "commonscaled.fullname" $root }}
  namespace: {{ $root.Values.namespace }}
rules:
{{- toYaml $rules | nindent 2 }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ include "commonscaled.fullname" $root }}
  namespace: {{ $root.Values.namespace }}
subjects:
- kind: ServiceAccount
  name: {{ include "commonscaled.serviceAccountName" $root }}
  namespace: {{ $root.Values.namespace }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ include "commonscaled.fullname" $root }}
---
{{- end }}
{{- $clusterRbac := (.Values.clusterRbac | default dict) -}}
{{- $clusterRules := (get $clusterRbac "rules" | default list) -}}
{{- if gt (len $clusterRules) 0 }}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ include "commonscaled.fullname" $root }}
rules:
{{- toYaml $clusterRules | nindent 2 }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ include "commonapi.fullname" $root }}
subjects:
- kind: ServiceAccount
  name: {{ include "commonscaled.serviceAccountName" $root }}
  namespace: {{ $root.Values.namespace }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ include "commonscaled.fullname" $root }}
---
{{- end }}
{{- end -}}
