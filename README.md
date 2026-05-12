# obs-platform

A mini developer governance platform with observability, built in a 2-day hackathon using AI-assisted development (for the applications within the project since the focus was on the platform).

The platform demonstrates how a small platform team can support many engineering teams by turning infrastructure knowledge into standardized metadata and automation. Everything runs locally in minikube.

## What it does

1. **Register a component** in the Tower developer portal (name, stack, team, infrastructure needs, SLO targets)
2. **RIA automatically generates** all infrastructure: K8s manifests, CI pipelines, ArgoCD ApplicationSets, database clusters, Prometheus alerting rules, webhooks
3. **Push code** to the component's repo and Pipelines-as-Code builds and pushes the container image
4. **ArgoCD deploys** the component to stage and prod environments
5. **Prometheus scrapes** metrics, evaluates SLO alerts, and Tower displays availability and error budgets

Register once, deploy everywhere. No YAML by hand.

## Architecture

Everything runs inside a single minikube cluster.

```
┌──────────────────────────────── minikube ─────────────────────────────────┐
│                                                                           │
│  ┌──────────┐  writes   ┌──────────┐  webhook   ┌──────────┐              │
│  │  Tower   │ ────────> │  Gitea   │ ─────────> │   RIA    │              │
│  │ (Next.js)│  YAML     │  (repos) │            │   (Go)   │              │
│  └────┬─────┘           └────┬─────┘            └────┬─────┘              │
│       │                      │                       │                    │
│       │ queries              │ push                  │ generates          │
│       │                      v                       v                    │
│       │               ┌────────────┐        ┌─────────────────┐           │
│       │               │ Tekton+PAC │        │  gitops-deploy  │           │
│       │               │   (CI/CD)  │        │  (K8s manifests │           │
│       │               └──────┬─────┘        │   appsets, CRs) │           │
│       │                      │              └────────┬────────┘           │
│       │                      │ builds                │                    │
│       │                      │ image        ArgoCD watches & deploys      │
│       │                      v                       v                    │
│       │               ┌────────────┐        ┌─────────────────┐           │
│       │               │  Registry  │        │     ArgoCD      │           │
│       │               └────────────┘        └────────┬────────┘           │
│       │                                              │                    │
│       │                                    deploys to │                   │
│       │                      ┌───────────────────────┼────────────┐       │
│       │                      v                       v            v       │
│       │               ┌────────────┐        ┌────────────┐ ┌─────────┐    │
│       │               │   stage    │        │    prod    │ │   ci    │    │
│       │               │  ns + app  │        │  ns + app  │ │ ns+PAC  │    │
│       │               │ + postgres │        │ + postgres │ │         │    │
│       │               └────────────┘        └─────┬──────┘ └─────────┘    │
│       │                                           │                       │
│       │                                   ┌───────┴────────┐              │
│       │                                   │ PrometheusRule │              │
│       │                                   │  (SLO alerts)  │              │
│       │                                   └───────┬────────┘              │
│       │                                           │                       │
│       │                                  scrapes & evaluates              │
│       v                                           v                       │
│  ┌─────────────┐                         ┌──────────────┐                 │
│  │  SLO tab    │ <────── queries ──────  │  Prometheus  │                 │
│  │ (Tower UI)  │                         └──────────────┘                 │
│  └─────────────┘                         ┌──────────────┐                 │
│                                          │   Grafana    │                 │
│  ┌─────────────┐    ┌──────────┐         └──────────────┘                 │
│  │    Vault    │───>│   ESO    │───> K8s Secrets                          │
│  │  (secrets)  │    └──────────┘                                          │
│  └─────────────┘                                                          │
│                                                                           │
└───────────────────────────────────────────────────────────────────────────┘
```

## Components

| Component | Description |
|-----------|-------------|
| **Tower** | Developer portal (Next.js). Register components, view SLO dashboards, manage infrastructure. |
| **RIA** | Reconciler for Infrastructure Automation (Go). Watches the service catalog and generates all deployment artifacts. |
| **Gitea** | Git hosting. All repos, webhooks, and access control. |
| **ArgoCD** | GitOps deployments. Watches gitops-deploy and reconciles to the cluster. |
| **Tekton + Pipelines-as-Code** | CI/CD. Builds container images on push to main. |
| **Prometheus + Grafana** | Observability. Scrapes metrics, evaluates SLO alerts. |
| **Vault + External Secrets Operator** | Secrets management. Credentials flow from Vault into K8s via ESO. |
| **CloudNativePG** | Postgres operator. RIA generates Cluster CRs when components request a database. |

## Prerequisites

- [minikube](https://minikube.sigs.k8s.io/docs/start/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/)
- [helm](https://helm.sh/docs/intro/install/)
- [jq](https://jqlang.github.io/jq/download/)

## Quick start

```bash
# Start everything (may need to run twice on first setup for CRD registration)
bash setup.sh

# Port-forward platform services
.hack/portforward.sh

# Port-forward Tower and RIA
.hack/portforward-base.sh

# Open Tower UI
open http://localhost:8888
```

## Services & ports

| Service | URL | Credentials |
|---------|-----|-------------|
| Gitea | http://localhost:3000 | gitea_admin / admin1234 |
| ArgoCD | https://localhost:8080 | admin / (printed by setup.sh) |
| Tekton Dashboard | http://localhost:9097 | - |
| Vault | http://localhost:8200 | Token: obs-platform-root-token |
| Prometheus | http://localhost:9090 | - |
| Grafana | http://localhost:3001 | admin / admin1234 |
| Tower | http://localhost:8888 | - |
| RIA | http://localhost:8889 | - |
| Registry | http://localhost:5000 | - |

## How it works

### Registering a component

In Tower, you specify:
- **Name, description, stack** (Python/Go), **owner team**
- **Repository name** in Gitea
- **Infrastructure**: optional Postgres with per-environment sizing (stage/prod)
- **SLO**: availability target and window, plus optional custom PromQL-based SLOs

### What RIA generates

When a component is registered, RIA automatically creates:

```
gitops-deploy/
  components/<name>/
    base/
      namespace.yaml          # with labels for ESO, team, component
      deployment.yaml         # with DB env vars if Postgres requested
      service.yaml            # named port for ServiceMonitor
      servicemonitor.yaml     # Prometheus scrape config
      kustomization.yaml
    overlays/
      stage/
        kustomization.yaml
        postgres.yaml          # if requested (per-env sizing)
      prod/
        kustomization.yaml
        postgres.yaml          # if requested (per-env sizing)
        prometheusrule.yaml    # SLO recording rules + alerts (prod only)
      ci/
        namespace.yaml         # CI namespace with PAC label
        pac-repository.yaml    # Pipelines-as-Code Repository CR
        kustomization.yaml
  appsets/
    base/<name>/               # ArgoCD ApplicationSet
    overlays/{stage,prod,ci}/  # updated to include the new component
```

Plus:
- `.tekton/push.yaml` in the component's source repo (CI pipeline)
- Gitea webhook pointing to PAC controller

### The full loop

```
Tower registers component
    -> writes YAML to platform/service-catalog
        -> Gitea webhook fires
            -> RIA reconciles
                -> generates gitops-deploy artifacts (atomic commit)
                -> generates .tekton/push.yaml in source repo
                -> creates PAC webhook on source repo
                    -> ArgoCD syncs
                        -> namespaces, deployments, services created
                        -> Postgres cluster provisioned
                        -> ServiceMonitor + PrometheusRule deployed
                            -> developer pushes code
                                -> PAC triggers Tekton pipeline
                                    -> image built and pushed to registry
                                        -> ArgoCD deploys new image
                                            -> Prometheus scrapes /metrics
                                                -> SLO alerts evaluate
                                                    -> Tower shows SLO status
```

## Repo structure

```
obs-platform/
  setup.sh                    # one-command cluster setup
  teardown.sh                 # destroy everything
  .hack/
    portforward.sh            # platform services
    portforward-base.sh       # Tower + RIA
    portforward-sample-apps.sh # sample applications
  minikube/
    start.sh                  # minikube with addons + registry config
  cluster-bootstrap/
    argocd/                   # ArgoCD install + Gitea repo credentials
    tekton/                   # Tekton Pipelines + Dashboard
    gitea/                    # Gitea Helm chart + repo seeding
    vault/                    # Vault Helm chart + team policies
    cloudnative-pg/           # CloudNativePG operator
    external-secrets/         # ESO + ClusterSecretStore for Vault
    monitoring/               # kube-prometheus-stack (Prometheus + Grafana)
    rbac/                     # ArgoCD AppProjects per team
```

## Teardown

```bash
bash teardown.sh
```

## Known limitations

This is a hackathon project — it demonstrates the architecture and full loop, not production hardness. Some known shortcuts:

- **Image rollout**: Deployments use `imagePullPolicy: Always` with the `:latest` tag. After a pipeline builds a new image, the running pods don't automatically restart. You need to manually `kubectl rollout restart` the deployment or delete the pod. A production setup would use commit-SHA tags with the image reference updated in Git, triggering ArgoCD to roll out — but that introduces race conditions with RIA also writing to gitops-deploy.
- **Git race conditions**: RIA pushes directly to the main branch of gitops-deploy. If a developer or another RIA reconciliation pushes at the same time, one will fail. In production, main/master would be branch-protected and changes would go through PRs with rebase/merge. This was simplified for the hackathon scope.
- **Operator deployment via setup.sh**: Operators and Helm charts (ArgoCD, Tekton, Prometheus, etc.) are deployed by `setup.sh` using kustomize, not managed by ArgoCD itself. This was intentional — since Gitea runs inside the cluster, ArgoCD can't bootstrap from repos that don't exist yet. A production setup would use an external Git provider or a two-phase bootstrap.
- **Hardcoded credentials**: Gitea admin, Vault token, and Grafana passwords are hardcoded in the setup scripts and Helm values. In production these would be injected via a secrets manager or provisioned externally.
- **Single minikube node**: Everything runs on one node. There's no HA, no real network policies, and resource limits are minimal.
- **`setup.sh` may need two runs**: The monitoring stack (kube-prometheus-stack) has large CRDs that require a two-pass apply. The first run creates CRDs, the second creates the custom resources that depend on them.
- **Floating-point precision**: SLO targets like 99.9% produce `0.9990000000000001` in the PrometheusRule due to Go floating-point arithmetic. Functionally correct, cosmetically imperfect.
- **RIA overwrites custom pipelines**: RIA always regenerates `.tekton/push.yaml` from the global pipeline templates. If a team customizes their pipeline (e.g., referencing a task from their own `<team>/pipelines` repo), RIA will overwrite it on the next reconciliation. A more sophisticated RIA could detect custom `.tekton/` configurations and skip regeneration, or the component spec could include a field like `customPipeline: true` to opt out.

## Future improvements

Things that would be needed to make this production-ready but were out of scope for a 2-day hackathon:

- **ArgoCD RBAC + SSO**: ArgoCD AppProjects per team exist but all apps use `project: default`. In production, RIA would assign each component's ApplicationSet to the team's AppProject. Developer access would use Dex as an SSO provider (backed by Gitea OAuth) so devs log into ArgoCD and only see their team's applications.
- **Kubernetes RBAC**: No developer-facing kubectl access is configured. In production, each product namespace would have Roles/RoleBindings so developers can view logs, exec into pods, and debug — but not access other teams' namespaces. This could use OIDC or namespace-scoped ServiceAccounts.
- **Branch protection**: Main branches have no protection. In production, gitops-deploy and service-catalog would require PRs with approvals, and RIA would create PRs instead of pushing directly.
- **Image tagging**: Using `:latest` tags with `imagePullPolicy: Always` instead of commit-SHA tags with GitOps-driven rollouts.
- **Secrets rotation**: Credentials are static. In production, Vault would handle dynamic secrets with TTLs and automatic rotation.
- **Multi-cluster**: Everything runs in one minikube. In production, stage and prod would be separate clusters with ArgoCD managing both.

## License

MIT
