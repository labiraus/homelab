{{ define "commonapi.canary" }}
{{- if and ( .Values.canary ) ( .Values.canary.enabled ) ( .Capabilities.APIVersions.Has "flagger.app/v1beta1/Canary" ) }}
apiVersion: flagger.app/v1beta1
kind: Canary
metadata:
  name: {{ include "commonapi.fullname" . }}
  namespace: {{ .Values.namespace }}
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{ include "commonapi.fullname" . }}
  {{- if .Values.autoscaling.enabled }}
  autoscalerRef:
    apiVersion: autoscaling/v2
    kind: HorizontalPodAutoscaler
    name: {{ include "commonapi.fullname" . }}
  {{- end }}
  service:
    port: 80
    appProtocol: TCP
    targetPort: {{ .Values.service.port }}
    portDiscovery: true
  analysis:
    interval: 1m
    threshold: 10
    maxWeight: 50
    stepWeight: 5
    metrics:
      - name: request-success-rate
        thresholdRange:
          min: 99
        interval: 1m
      - name: request-duration
        thresholdRange:
          max: 500
        interval: 1m
    webhooks:
      - name: load-test
        url: http://flagger-loadtester.test/
        metadata:
          cmd: "hey -z 1m -q 10 -c 2 http://{{ include "commonapi.fullname" . }}-canary.test/{{ .Values.canary.path }}"
---
{{- end }}
{{- end -}}
