{{- define "replica-control.name" -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- define "replica-control.labels" -}}
app.kubernetes.io/name: replica-control
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
{{- define "replica-control.selectorLabels" -}}
app.kubernetes.io/name: replica-control
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
