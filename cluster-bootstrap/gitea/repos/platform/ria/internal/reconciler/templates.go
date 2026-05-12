package reconciler

import (
	"bytes"
	"text/template"
)

// ---------------------------------------------------------------------------
// Template data structs
// ---------------------------------------------------------------------------

// ComponentData carries the values that most per-component templates need.
type ComponentData struct {
	Name        string
	Team        string
	Port        int
	Stack       string
	Owner       string // Gitea org/user that owns the source repo
	Repo        string // Gitea repo name (may differ from component name)
	HasPostgres bool   // true when the component requests Postgres infrastructure
	HasSLO      bool   // true when the component declares SLO configuration
}

// PrometheusRuleData carries the values for the PrometheusRule template.
type PrometheusRuleData struct {
	Name          string
	NameSafe      string // Name with hyphens replaced by underscores (for recording rule names)
	Target        string // availability target as ratio string, e.g. "0.999"
	TargetPercent string // pre-computed percentage string, e.g. "99.9"
	Window        string // e.g. "30d"
	CustomSLOs    []CustomSLOData
}

// CustomSLOData carries the values for a single custom SLO rule group.
type CustomSLOData struct {
	Name          string
	Query         string
	Target        string // target as ratio string
	TargetPercent string // pre-computed percentage string
	Window        string
}

// PostgresData carries the values for the CloudNativePG Cluster template.
type PostgresData struct {
	Name     string
	Size     string
	Replicas int
}

// AppsetOverlayData carries the values needed to regenerate the appsets
// overlay kustomization files.
type AppsetOverlayData struct {
	Components []string // sorted component names
}

// TektonPushData carries the values for the .tekton/push.yaml template.
type TektonPushData struct {
	Name     string
	TaskFile string // e.g. "python-build.yaml"
	TaskName string // e.g. "python-build"
}

// ---------------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------------

// Each template produces a single YAML file. They are kept as raw strings so
// that the binary has zero external file dependencies.

var tmplNamespace = mustParse("namespace", `apiVersion: v1
kind: Namespace
metadata:
  name: {{ .Name }}-stage
  labels:
    obs-platform/component: "{{ .Name }}"
    obs-platform/team: "{{ .Team }}"
`)

var tmplDeployment = mustParse("deployment", `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Name }}
  labels:
    app: {{ .Name }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .Name }}
  template:
    metadata:
      labels:
        app: {{ .Name }}
    spec:
      containers:
        - name: {{ .Name }}
          # localhost:5000 is the node-level containerd mirror; images are
          # pushed there by the in-cluster registry so the kubelet can pull
          # without TLS.
          image: localhost:5000/{{ .Name }}:latest
          # Always pull so that ":latest" picks up new pushes immediately.
          imagePullPolicy: Always
          ports:
            - containerPort: {{ .Port }}
{{- if .HasPostgres }}
          env:
          - name: DATABASE_HOST
            valueFrom:
              secretKeyRef:
                name: {{ .Name }}-pg-app
                key: host
          - name: DATABASE_PORT
            valueFrom:
              secretKeyRef:
                name: {{ .Name }}-pg-app
                key: port
          - name: DATABASE_NAME
            valueFrom:
              secretKeyRef:
                name: {{ .Name }}-pg-app
                key: dbname
          - name: DATABASE_USER
            valueFrom:
              secretKeyRef:
                name: {{ .Name }}-pg-app
                key: username
          - name: DATABASE_PASSWORD
            valueFrom:
              secretKeyRef:
                name: {{ .Name }}-pg-app
                key: password
          - name: DATABASE_URL
            valueFrom:
              secretKeyRef:
                name: {{ .Name }}-pg-app
                key: uri
{{- end }}
`)

var tmplService = mustParse("service", `apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}
  labels:
    app: {{ .Name }}
spec:
  selector:
    app: {{ .Name }}
  ports:
    - name: http
      port: {{ .Port }}
      targetPort: {{ .Port }}
`)

var tmplServiceMonitor = mustParse("servicemonitor", `apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: {{ .Name }}
  labels:
    app: {{ .Name }}
spec:
  selector:
    matchLabels:
      app: {{ .Name }}
  endpoints:
    - port: http
      path: /metrics
      interval: 15s
`)

var tmplPrometheusRule = mustParse("prometheus-rule", `apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: {{ .Name }}-slo
  labels:
    app: {{ .Name }}
spec:
  groups:
    - name: {{ .Name }}.slo.availability
      rules:
        - record: slo:{{ .NameSafe }}:availability:ratio
          expr: avg_over_time(up{job="{{ .Name }}", namespace="{{ .Name }}-prod"}[{{ .Window }}])
          labels:
            component: "{{ .Name }}"
            slo_target: "{{ .Target }}"
        - alert: SLOAvailabilityBreach
          expr: avg_over_time(up{job="{{ .Name }}", namespace="{{ .Name }}-prod"}[{{ .Window }}]) < {{ .Target }}
          for: 5m
          labels:
            severity: critical
            component: "{{ .Name }}"
          annotations:
            summary: "{{ .Name }} availability below {{ .TargetPercent }}% SLO target"
{{- range .CustomSLOs }}
    - name: {{ $.Name }}.slo.{{ .Name }}
      rules:
        - record: slo:{{ $.NameSafe }}:{{ .Name }}:ratio
          expr: {{ .Query }}
          labels:
            component: "{{ $.Name }}"
            slo_target: "{{ .Target }}"
        - alert: SLOCustomBreach{{ .Name }}
          expr: ({{ .Query }}) < {{ .Target }}
          for: 5m
          labels:
            severity: warning
            component: "{{ $.Name }}"
          annotations:
            summary: "{{ $.Name }} {{ .Name }} SLO below target"
{{- end }}
`)

var tmplBaseKustomization = mustParse("base-kustomization", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - namespace.yaml
  - deployment.yaml
  - service.yaml
  - servicemonitor.yaml
`)

// --- Overlays: stage / prod ------------------------------------------------

var tmplOverlayStageKustomization = mustParse("overlay-stage-kustomization", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
{{- if .HasPostgres }}
  - postgres.yaml
{{- end }}
namespace: {{ .Name }}-stage
`)

var tmplOverlayProdKustomization = mustParse("overlay-prod-kustomization", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
{{- if .HasPostgres }}
  - postgres.yaml
{{- end }}
{{- if .HasSLO }}
  - prometheusrule.yaml
{{- end }}
namespace: {{ .Name }}-prod
`)

// --- CloudNativePG Cluster -------------------------------------------------

var tmplPostgresCluster = mustParse("postgres-cluster", `apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: {{ .Name }}-pg
spec:
  instances: {{ .Replicas }}
  storage:
    size: {{ .Size }}
  postgresql:
    parameters:
      shared_buffers: "64MB"
      max_connections: "100"
`)

// --- Overlay: ci -----------------------------------------------------------

var tmplCINamespace = mustParse("ci-namespace", `apiVersion: v1
kind: Namespace
metadata:
  name: {{ .Name }}-ci
  labels:
    obs-platform/component: "{{ .Name }}"
    obs-platform/team: "{{ .Team }}"
    obs-platform/pac-enabled: "true"
`)

var tmplPACRepository = mustParse("pac-repository", `apiVersion: pipelinesascode.tekton.dev/v1alpha1
kind: Repository
metadata:
  name: {{ .Name }}
  namespace: {{ .Name }}-ci
spec:
  # localhost:3000 is used so that PAC's incoming-webhook URL matches the
  # event payload origin. The gitea-http.gitea.svc address is only for
  # API calls from inside the cluster.
  url: "http://localhost:3000/{{ .Owner }}/{{ .Repo }}"
  git_provider:
    url: "http://gitea-http.gitea.svc:3000"
    secret:
      name: gitea-pac-token
      key: token
    user: gitea_admin
`)

var tmplCIKustomization = mustParse("ci-kustomization", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - namespace.yaml
  - pac-repository.yaml
`)

// --- ApplicationSet --------------------------------------------------------

var tmplAppSet = mustParse("appset", `apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: {{ .Name }}-set
  namespace: argocd
spec:
  generators:
    - list:
        elements:
          - name: in-cluster
            nameNormalized: {{ .Name }}
            repoURL: http://gitea-http.gitea.svc:3000/platform/gitops-deploy.git
            path: components/{{ .Name }}/overlays/stage
            namespace: {{ .Name }}-stage
            environment: stage
  template:
    metadata:
      name: '{{"{{"}}nameNormalized{{"}}"}}-{{"{{"}}environment{{"}}"}}'
    spec:
      project: default
      source:
        repoURL: '{{"{{"}}repoURL{{"}}"}}'
        targetRevision: main
        path: '{{"{{"}}path{{"}}"}}'
      destination:
        server: https://kubernetes.default.svc
        namespace: '{{"{{"}}namespace{{"}}"}}'
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
`)

var tmplAppSetKustomization = mustParse("appset-kustomization", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - {{ .Name }}-appset.yaml
`)

// --- Appsets overlay kustomization files -----------------------------------

// Platform components (tower, ria) are always present in overlays.
// RIA only manages catalog-registered components but must preserve these.

var tmplAppsetsOverlayStage = mustParse("appsets-overlay-stage", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: argocd
namePrefix: stage-
resources:
  - ../../base/tower
  - ../../base/ria
{{- range .Components }}
  - ../../base/{{ . }}
{{- end }}
`)

var tmplAppsetsOverlayProd = mustParse("appsets-overlay-prod", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: argocd
namePrefix: prod-
resources:
  - ../../base/tower
  - ../../base/ria
  - ../../base/platform
{{- range .Components }}
  - ../../base/{{ . }}
{{- end }}
patches:
  - path: patches/prod-tower-patch.yaml
  - path: patches/prod-ria-patch.yaml
{{- range .Components }}
  - target:
      kind: ApplicationSet
      name: {{ . }}-set
    patch: |
      - op: replace
        path: /spec/generators/0/list/elements/0/path
        value: components/{{ . }}/overlays/prod
      - op: replace
        path: /spec/generators/0/list/elements/0/namespace
        value: {{ . }}-prod
      - op: replace
        path: /spec/generators/0/list/elements/0/environment
        value: prod
{{- end }}
`)

var tmplAppsetsOverlayCI = mustParse("appsets-overlay-ci", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: argocd
namePrefix: ci-
resources:
  - ../../base/tower
  - ../../base/ria
{{- range .Components }}
  - ../../base/{{ . }}
{{- end }}
patches:
  - path: patches/ci-tower-patch.yaml
  - path: patches/ci-ria-patch.yaml
{{- range .Components }}
  - target:
      kind: ApplicationSet
      name: {{ . }}-set
    patch: |
      - op: replace
        path: /spec/generators/0/list/elements/0/path
        value: components/{{ . }}/overlays/ci
      - op: replace
        path: /spec/generators/0/list/elements/0/namespace
        value: {{ . }}-ci
      - op: replace
        path: /spec/generators/0/list/elements/0/environment
        value: ci
{{- end }}
`)

// --- Tekton push pipeline --------------------------------------------------

var tmplTektonPush = mustParse("tekton-push", `apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  name: {{ .Name }}-push
  annotations:
    pipelinesascode.tekton.dev/on-event: "[push]"
    pipelinesascode.tekton.dev/on-target-branch: "[main]"
    pipelinesascode.tekton.dev/task: "http://gitea-http.gitea.svc:3000/platform/pipelines/raw/branch/main/tasks/{{ .TaskFile }}"
spec:
  pipelineSpec:
    tasks:
      - name: build
        taskRef:
          name: {{ .TaskName }}
        params:
          # In-cluster Gitea address so the task can clone without leaving the cluster network.
          - name: repo_url
            value: "http://gitea-http.gitea.svc:3000/{{ "{{" }} repo_owner {{ "}}" }}/{{ "{{" }} repo_name {{ "}}" }}"
          - name: revision
            value: "{{ "{{" }} revision {{ "}}" }}"
          # registry.kube-system.svc.cluster.local is the in-cluster registry
          # that kaniko pushes to; the node-level containerd mirror (localhost:5000)
          # then makes it available for kubelet pulls.
          - name: image
            value: "registry.kube-system.svc.cluster.local/{{ .Name }}:{{ "{{" }} revision {{ "}}" }}"
          - name: image_latest
            value: "registry.kube-system.svc.cluster.local/{{ .Name }}:latest"
        workspaces:
          - name: source
            workspace: shared
    workspaces:
      - name: shared
  workspaces:
    - name: shared
      volumeClaimTemplate:
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 1Gi
`)

// ---------------------------------------------------------------------------
// Render helpers
// ---------------------------------------------------------------------------

func mustParse(name, text string) *template.Template {
	return template.Must(template.New(name).Parse(text))
}

// render executes a template with the given data and returns the result.
func render(t *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
