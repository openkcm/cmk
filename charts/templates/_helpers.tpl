{{/*
Expand the name of the chart.
*/}}
{{- define "cmk.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Namespace for all resources to be installed into
If not defined in values file then the helm release namespace is used
By default this is not set so the helm release namespace will be used

This gets around an problem within helm discussed here
https://github.com/helm/helm/issues/5358
*/}}
{{- define "cmk.namespace" -}}
    {{ .Values.namespace | default .Release.Namespace }}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "cmk.fullname" -}}
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
{{- define "cmk.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "cmk.labels" -}}
helm.sh/chart: {{ include "cmk.chart" . }}
{{ include "cmk.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Task Scheduler labels
*/}}
{{- define "cmk.task-scheduler.labels" -}}
helm.sh/chart: {{ include "cmk.chart" . }}
{{ include "cmk.task-scheduler.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}


{{/*
DB Migrator labels
*/}}
{{- define "cmk.db-migrator.labels" -}}
helm.sh/chart: {{ include "cmk.chart" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Task Worker labels
*/}}
{{- define "cmk.task-worker.labels" -}}
helm.sh/chart: {{ include "cmk.chart" . }}
{{ include "cmk.task-worker.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Task CLI labels
*/}}
{{- define "cmk.task-cli.labels" -}}
helm.sh/chart: {{ include "cmk.chart" . }}
{{ include "cmk.task-cli.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}


{{/*
Tenant Manager labels
*/}}
{{- define "cmk.tenant-manager.labels" -}}
helm.sh/chart: {{ include "cmk.chart" . }}
{{ include "cmk.tenant-manager.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Tenant Manager CLI labels
*/}}
{{- define "cmk.tenant-manager-cli.labels" -}}
helm.sh/chart: {{ include "cmk.chart" . }}
{{ include "cmk.tenant-manager-cli.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "cmk.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cmk.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: {{ .Chart.Name }}
{{- end }}

{{/*
task-scheduler Selector Labels
*/}}
{{- define "cmk.task-scheduler.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cmk.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}-task-scheduler
app.kubernetes.io/component: {{ .Chart.Name }}-task-scheduler
{{- end }}

{{/*
task-worker Selector Labels
*/}}
{{- define "cmk.task-worker.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cmk.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}-task-worker
app.kubernetes.io/component: {{ .Chart.Name }}-task-worker
{{- end }}

{{/*
task-cli Selector Labels
*/}}
{{- define "cmk.task-cli.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cmk.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}-task-cli
app.kubernetes.io/component: {{ .Chart.Name }}-task-cli
{{- end }}

{{/*
Tenant Manager Selector labels
*/}}
{{- define "cmk.tenant-manager.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cmk.name" . }}-tenant-manager
app.kubernetes.io/instance: {{ .Release.Name }}-tenant-manager
app.kubernetes.io/component: {{ .Chart.Name }}-tenant-manager
{{- end }}

{{/*
Tenant Manager CLI Selector labels
*/}}
{{- define "cmk.tenant-manager-cli.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cmk.name" . }}-tenant-manager-cli
app.kubernetes.io/instance: {{ .Release.Name }}-tenant-manager-cli
app.kubernetes.io/component: {{ .Chart.Name }}-tenant-manager-cli
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "cmk.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "cmk.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "cmk.postgresqlName" -}}
{{- printf "%s-postgresql" .Release.Name -}}
{{- end }}

{{/*
Util function for generating the image URL based on the provided options.
*/}}
{{- define "cmk.image" -}}
{{- $defaultTag := index . 1 -}}
{{- with index . 0 -}}
{{- if .registry -}}{{ printf "%s/%s" .registry .repository }}{{- else -}}{{- .repository -}}{{- end -}}
{{- if .digest -}}{{ printf "@%s" .digest }}{{- else -}}{{ printf ":%s" (default $defaultTag .tag) }}{{- end -}}
{{- end }}
{{- end }}

{{/*
Active plugins
*/}}
{{- define "cmk.plugins" -}}
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
