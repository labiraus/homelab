{{ define "commonscaled.deployment" }}
{{- $root := . -}}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "commonscaled.fullname" . }}
  namespace: {{ .Values.namespace }}
  labels:
    app: {{ include "commonscaled.fullname" . }}
    {{- include "commonscaled.labels" . | nindent 4 }}
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}
  {{- with .Values.strategy }}
  strategy:
{{ toYaml . | nindent 4 }}
  {{- end }}
  minReadySeconds: {{ .Values.minReadySeconds | default 5 }}
  selector:
    matchLabels:
      {{- include "commonscaled.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/path: "/metrics"
        prometheus.io/port: "{{ .Values.service.port }}"
      {{- with .Values.podAnnotations }}
        {{- toYaml . | nindent 8 }}
      {{- end }}
      labels:
        {{- include "commonscaled.selectorLabels" . | nindent 8 }}
    spec:
      {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      serviceAccountName: {{ include "commonapi.serviceAccountName" . }}
      serviceAccountName: {{ include "commonscaled.serviceAccountName" . }}
      securityContext:
        {{- toYaml .Values.podSecurityContext | nindent 8 }}
      terminationGracePeriodSeconds: {{ .Values.terminationGracePeriodSeconds | default 30 }}
      containers:
        - name: {{ .Chart.Name }}
          securityContext:
            {{- toYaml .Values.securityContext | nindent 12 }}
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - name: http
              containerPort: {{ .Values.service.port }}
              protocol: TCP
          {{- with .Values.startupProbe }}
          startupProbe:
{{ toYaml . | nindent 12 }}
          {{- end }}
          livenessProbe:
{{ toYaml .Values.livenessProbe | nindent 12 }}
          readinessProbe:
{{ toYaml .Values.readinessProbe | nindent 12 }}
          {{- with .Values.lifecycle }}
          lifecycle:
{{ toYaml . | nindent 12 }}
          {{- end }}
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
          env:
            - name: configValue
              value: {{ default "Config Default" .Values.env.configValue }}
            - name: namespace
              value: {{ .Values.namespace }}
          {{- with .Values.envFromSecrets }}
          envFrom:
            {{- range . }}
            - secretRef:
                name: {{ . }}
            {{- end }}
          {{- end }}
      {{- if and ( .Values.secret ) ( .Values.secret.enabled ) }}
            {{- range $key, $value := .Values.secret.data }}
            - name: {{ $key | upper }}
              valueFrom:
                secretKeyRef:
                  name: {{ $root.Release.Name }}-secret
                  key: {{ $key }}
            {{- end }}
          volumeMounts:
          - name: secret-volume
            mountPath: /etc/secret-volume
            readOnly: true
      volumes:
      - name: secret-volume
        secret:
          secretName: {{ .Release.Name }}-secret
      {{- end }}  
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
{{ "\n" }}---{{ "\n" }}
{{- end }}
