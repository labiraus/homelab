{{ define "commonscaled.service" }}
apiVersion: v1
kind: Service
metadata:
  name: {{ include "commonscaled.fullname" . }}
  namespace: {{ .Values.namespace }}
  labels:
    {{- include "commonscaled.labels" . | nindent 4 }}
spec:
  type: {{ .Values.service.type }}
  ports:
    - port: 80
      targetPort: {{ .Values.service.port }}
      protocol: TCP
      name: http
  selector:
    {{- include "commonscaled.selectorLabels" . | nindent 4 }}
---{{ "\n" }}
{{- end }}
