{{- define "commonscaled.scaledObject" -}}
{{- if .Values.keda.enabled }}
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: {{ include "commonscaled.fullname" . }}
  namespace: {{ .Values.namespace }}
spec:
  scaleTargetRef:
    name: {{ include "commonscaled.fullname" . }}
  pollingInterval: {{ .Values.keda.pollingInterval }}
  cooldownPeriod: {{ .Values.keda.cooldownPeriod }}
  minReplicaCount: {{ .Values.keda.minReplicaCount }}
  maxReplicaCount: {{ .Values.keda.maxReplicaCount }}
  triggers:
    - type: kafka
      metadata:
        bootstrapServers: {{ .Values.keda.kafka.bootstrapServers | quote }}
        consumerGroup: {{ .Values.keda.kafka.consumerGroup | quote }}
        topic: {{ .Values.keda.kafka.topic | quote }}
        lagThreshold: {{ .Values.keda.kafka.lagThreshold | quote }}
---{{ "\n" }}
{{- end }}
{{- end -}}
