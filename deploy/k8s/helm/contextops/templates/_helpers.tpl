{{/*
Expand the name of the chart.
*/}}
{{- define "contextops.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "contextops.fullname" -}}
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
{{- define "contextops.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "contextops.labels" -}}
helm.sh/chart: {{ include "contextops.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: contextops
{{- end }}

{{/*
Selector labels for a component
*/}}
{{- define "contextops.selectorLabels" -}}
app.kubernetes.io/name: {{ include "contextops.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Database URL
*/}}
{{- define "contextops.databaseURL" -}}
{{- printf "postgres://%s:%s@%s-postgresql:5432/%s?sslmode=disable" .Values.postgresql.auth.username .Values.postgresql.auth.password (include "contextops.fullname" .) .Values.postgresql.auth.database }}
{{- end }}

{{/*
Redis URL
*/}}
{{- define "contextops.redisURL" -}}
{{- printf "redis://%s-redis-master:6379/0" (include "contextops.fullname" .) }}
{{- end }}
