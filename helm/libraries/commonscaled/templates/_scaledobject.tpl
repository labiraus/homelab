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
    - type: nats-jetstream
      metadata:
        natsServerMonitoringEndpoint: {{ .Values.keda.natsJetStream.natsServerMonitoringEndpoint | quote }}
        account: {{ .Values.keda.natsJetStream.account | quote }}
        stream: {{ .Values.keda.natsJetStream.stream | quote }}
        consumer: {{ .Values.keda.natsJetStream.consumer | quote }}
        lagThreshold: {{ .Values.keda.natsJetStream.lagThreshold | quote }}
---{{ "\n" }}
{{- end }}
{{- end -}}
