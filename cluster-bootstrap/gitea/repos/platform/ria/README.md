# RIA — Reconciler for Infrastructure Automation

RIA is the platform orchestrator for obs-platform. It watches the service-catalog for component registrations and reconciles the infrastructure state to match.

## What RIA does

When a component is registered in Tower (written to `platform/service-catalog`), RIA:

1. Generates K8s manifests in `platform/gitops-deploy/components/<name>/`
2. Creates ArgoCD ApplicationSets in `platform/gitops-deploy/appsets/base/<name>/`
3. Updates appset overlays for stage, prod, and CI environments
4. Ensures `.tekton/push.yaml` exists in the component's source repo
5. Creates Gitea webhooks for Pipelines-as-Code

## Architecture

RIA is a level-triggered reconciler — it reads the full desired state from the catalog and converges the actual state to match. The webhook from Gitea is just a signal to run the reconciliation loop, not an instruction about what changed.

- **Persistent shallow clones** for service-catalog, gitops-deploy, and component repos
- **Atomic commits** via go-git — all gitops-deploy changes in one commit per reconciliation
- **Gitea API** used only for webhook management
