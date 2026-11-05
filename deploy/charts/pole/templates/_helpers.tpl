{{- define "pole.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "pole.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := include "pole.name" . }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "pole.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | quote }}
app.kubernetes.io/name: {{ include "pole.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: pole
{{- end }}

{{- define "pole.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pole.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "pole.componentLabels" -}}
{{ include "pole.selectorLabels" .root }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{- define "pole.image" -}}
{{- $image := . -}}
{{- if $image.digest -}}
{{ printf "%s@%s" $image.repository $image.digest }}
{{- else -}}
{{ printf "%s:%s" $image.repository (required "image tag is required when digest is empty" $image.tag) }}
{{- end -}}
{{- end }}

{{- define "pole.appImage" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag -}}
{{- if .Values.image.digest -}}
{{ printf "%s@%s" .Values.image.repository .Values.image.digest }}
{{- else -}}
{{ printf "%s:%s" .Values.image.repository $tag }}
{{- end -}}
{{- end }}

{{- define "pole.secretName" -}}
{{- if .Values.secrets.create -}}
{{ include "pole.fullname" . }}-runtime
{{- else -}}
{{ required "secrets.existingSecret is required when secrets.create=false" .Values.secrets.existingSecret }}
{{- end -}}
{{- end }}

{{- define "pole.mysqlName" -}}{{ include "pole.fullname" . }}-mysql{{- end }}
{{- define "pole.greptimeName" -}}{{ include "pole.fullname" . }}-greptimedb{{- end }}
{{- define "pole.otelName" -}}{{ include "pole.fullname" . }}-otel-collector{{- end }}
{{- define "pole.controlPlaneName" -}}{{ include "pole.fullname" . }}-control-plane{{- end }}
{{- define "pole.consoleName" -}}{{ include "pole.fullname" . }}-console{{- end }}
{{- define "pole.limiterName" -}}{{ include "pole.fullname" . }}-limiter{{- end }}
