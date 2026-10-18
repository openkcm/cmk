{{/*
Expand the name of the chart.
*/}}
{{- define "cmk.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Namespace for all resources. Defaults to the release namespace.
See https://github.com/helm/helm/issues/5358
*/}}
{{- define "cmk.namespace" -}}
{{- .Values.namespace | default .Release.Namespace }}
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
Version of the deployed application: the image tag if set, else the chart's appVersion.
*/}}
{{- define "cmk.version" -}}
{{- (.Values.image.tag | default .Chart.AppVersion) | toString | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Labels shared by all resources. Each component adds its selector labels.
*/}}
{{- define "cmk.commonLabels" -}}
helm.sh/chart: {{ include "cmk.chart" . }}
app.kubernetes.io/part-of: {{ include "cmk.name" . }}
{{- with include "cmk.version" . }}
app.kubernetes.io/version: {{ . | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels of each component. They differ between components for historical reasons.
Do not change them: a Deployment's selector cannot be changed on upgrade.
*/}}
{{- define "cmk.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cmk.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: {{ .Chart.Name }}
{{- end }}

{{- define "cmk.tenant-manager.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cmk.name" . }}-tenant-manager
app.kubernetes.io/instance: {{ .Release.Name }}-tenant-manager
app.kubernetes.io/component: {{ .Chart.Name }}-tenant-manager
{{- end }}

{{- define "cmk.event-reconciler.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cmk.name" . }}-event-reconciler
app.kubernetes.io/instance: {{ .Release.Name }}-event-reconciler
app.kubernetes.io/component: {{ .Chart.Name }}-event-reconciler
{{- end }}

{{- define "cmk.task-scheduler.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cmk.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}-task-scheduler
app.kubernetes.io/component: {{ .Chart.Name }}-task-scheduler
{{- end }}

{{- define "cmk.task-worker.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cmk.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}-task-worker
app.kubernetes.io/component: {{ .Chart.Name }}-task-worker
{{- end }}

{{- define "cmk.cmkctl.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cmk.name" . }}-cmkctl
app.kubernetes.io/instance: {{ .Release.Name }}-cmkctl
app.kubernetes.io/component: {{ .Chart.Name }}-cmkctl
{{- end }}

{{- define "cmk.db-migrator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cmk.name" . }}-db-migrator
app.kubernetes.io/instance: {{ .Release.Name }}-db-migrator
app.kubernetes.io/component: {{ .Chart.Name }}-db-migrator
{{- end }}

{{/*
Labels of each component: the common labels and its selector labels.
*/}}
{{- define "cmk.labels" -}}
{{ include "cmk.commonLabels" . }}
{{ include "cmk.selectorLabels" . }}
{{- end }}

{{- define "cmk.tenant-manager.labels" -}}
{{ include "cmk.commonLabels" . }}
{{ include "cmk.tenant-manager.selectorLabels" . }}
{{- end }}

{{- define "cmk.event-reconciler.labels" -}}
{{ include "cmk.commonLabels" . }}
{{ include "cmk.event-reconciler.selectorLabels" . }}
{{- end }}

{{- define "cmk.task-scheduler.labels" -}}
{{ include "cmk.commonLabels" . }}
{{ include "cmk.task-scheduler.selectorLabels" . }}
{{- end }}

{{- define "cmk.task-worker.labels" -}}
{{ include "cmk.commonLabels" . }}
{{ include "cmk.task-worker.selectorLabels" . }}
{{- end }}

{{- define "cmk.cmkctl.labels" -}}
{{ include "cmk.commonLabels" . }}
{{ include "cmk.cmkctl.selectorLabels" . }}
{{- end }}

{{- define "cmk.db-migrator.labels" -}}
{{ include "cmk.commonLabels" . }}
{{ include "cmk.db-migrator.selectorLabels" . }}
{{- end }}

{{/*
Base name of the ServiceAccounts. With serviceAccount.create, the API server uses this name
and every other component "<name>-<component>". Without creation, all components use
serviceAccount.name, or "default".
*/}}
{{- define "cmk.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "cmk.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Image reference. Either image.tag or image.digest must be set; there is no fallback to appVersion.
A digest takes precedence over a tag.
*/}}
{{- define "cmk.image" -}}
{{- with index . 0 -}}
{{- if not (or .digest .tag) -}}{{ fail "Either image.tag or image.digest must be set" }}{{- end -}}
{{- if .registry -}}{{ printf "%s/%s" .registry .repository }}{{- else -}}{{- .repository -}}{{- end -}}
{{- if .digest -}}{{ printf "@%s" (toString .digest) }}{{- else -}}{{ printf ":%s" (toString .tag) }}{{- end -}}
{{- end }}
{{- end }}

{{/*
Annotations with every value rendered as a string. Values given with --set can be numbers
or booleans, which Kubernetes rejects in annotations.
*/}}
{{- define "cmk.annotations" -}}
{{- range $key, $value := . }}
{{ $key }}: {{ $value | toString | quote }}
{{- end }}
{{- end }}
