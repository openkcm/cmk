{{/*
Expand the name of the chart.
*/}}
{{- define "tenant-manager-cli.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Namespace for all resources to be installed into
If not defined in values file then the helm release namespace is used
By default this is not set so the helm release namespace will be used

This gets around an problem within helm discussed here
https://github.com/helm/helm/issues/5358
*/}}
{{- define "tenant-manager-cli.namespace" -}}
    {{ .Values.namespace | default .Release.Namespace }}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "tenant-manager-cli.fullname" -}}
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
{{- define "tenant-manager-cli.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "tenant-manager-cli.labels" -}}
helm.sh/chart: {{ include "tenant-manager-cli.chart" . }}
{{ include "tenant-manager-cli.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Task Scheduler labels
*/}}
{{- define "tenant-manager-cli.task-scheduler.labels" -}}
helm.sh/chart: {{ include "tenant-manager-cli.chart" . }}
{{ include "tenant-manager-cli.task-scheduler.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Task Worker labels
*/}}
{{- define "tenant-manager-cli.task-worker.labels" -}}
helm.sh/chart: {{ include "tenant-manager-cli.chart" . }}
{{ include "tenant-manager-cli.task-worker.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Tenant Manager labels
*/}}
{{- define "tenant-manager-cli.tenant-manager.labels" -}}
helm.sh/chart: {{ include "tenant-manager-cli.chart" . }}
{{ include "tenant-manager-cli.tenant-manager.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Tenant Manager CLI labels
*/}}
{{- define "tenant-manager-cli.tenant-manager-cli.labels" -}}
helm.sh/chart: {{ include "tenant-manager-cli.chart" . }}
{{ include "tenant-manager-cli.tenant-manager-cli.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "tenant-manager-cli.selectorLabels" -}}
app.kubernetes.io/name: {{ include "tenant-manager-cli.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: {{ .Chart.Name }}
{{- end }}

{{/*
task-scheduler Selector Labels
*/}}
{{- define "tenant-manager-cli.task-scheduler.selectorLabels" -}}
app.kubernetes.io/name: {{ include "tenant-manager-cli.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}-task-scheduler
app.kubernetes.io/component: {{ .Chart.Name }}-task-scheduler
{{- end }}

{{/*
task-worker Selector Labels
*/}}
{{- define "tenant-manager-cli.task-worker.selectorLabels" -}}
app.kubernetes.io/name: {{ include "tenant-manager-cli.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}-task-worker
app.kubernetes.io/component: {{ .Chart.Name }}-task-worker
{{- end }}

{{/*
Tenant Manager Selector labels
*/}}
{{- define "tenant-manager-cli.tenant-manager.selectorLabels" -}}
app.kubernetes.io/name: {{ include "tenant-manager-cli.name" . }}-tenant-manager
app.kubernetes.io/instance: {{ .Release.Name }}-tenant-manager
app.kubernetes.io/component: {{ .Chart.Name }}-tenant-manager
{{- end }}

{{/*
Tenant Manager CLI Selector labels
*/}}
{{- define "tenant-manager-cli.tenant-manager-cli.selectorLabels" -}}
app.kubernetes.io/name: {{ include "tenant-manager-cli.name" . }}-tenant-manager-cli
app.kubernetes.io/instance: {{ .Release.Name }}-tenant-manager-cli
app.kubernetes.io/component: {{ .Chart.Name }}-tenant-manager-cli
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "tenant-manager-cli.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "tenant-manager-cli.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "tenant-manager-cli.postgresqlName" -}}
{{- printf "%s-postgresql" .Release.Name -}}
{{- end }}

{{/*
Util function for generating the image URL based on the provided options.
*/}}
{{- define "tenant-manager-cli.image" -}}
{{- $defaultTag := index . 1 -}}
{{- with index . 0 -}}
{{- if .registry -}}{{ printf "%s/%s" .registry .repository }}{{- else -}}{{- .repository -}}{{- end -}}
{{- if .digest -}}{{ printf "@%s" .digest }}{{- else -}}{{ printf ":%s" (default $defaultTag .tag) }}{{- end -}}
{{- end }}
{{- end }}

{{/*
Active plugins
*/}}
{{- define "tenant-manager-cli.plugins" -}}
{{- $ := . }}
{{- $plugins := list -}}
{{- range .plugins -}}
{{- $plugin := . -}}
{{- range .tags -}}
{{- if has . $.activePlugins -}}

{{- $plugins = append $plugins $plugin -}}
{{- break -}}

{{- end -}}
{{- end -}}
{{- end -}}
{{- toYaml $plugins -}}
{{- end -}}
