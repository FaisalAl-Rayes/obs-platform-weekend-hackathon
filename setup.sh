#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BOOTSTRAP="${SCRIPT_DIR}/cluster-bootstrap"

echo "============================================"
echo "  obs-platform Infrastructure Setup"
echo "============================================"
echo ""

# --- Step 1: minikube ---
echo ">>> Step 1/9: Starting minikube..."
bash "${SCRIPT_DIR}/minikube/start.sh"
echo ""

# --- Step 2: Helm repos (needed for kustomize --enable-helm) ---
echo ">>> Step 2/9: Adding Helm repositories..."
helm repo add gitea-charts https://dl.gitea.com/charts/ 2>/dev/null || true
helm repo add hashicorp https://helm.releases.hashicorp.com 2>/dev/null || true
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts 2>/dev/null || true
helm repo add external-secrets https://charts.external-secrets.io 2>/dev/null || true
helm repo add cnpg https://cloudnative-pg.github.io/charts 2>/dev/null || true
helm repo update
echo ""

# --- Step 3: ArgoCD ---
echo ">>> Step 3/9: Deploying ArgoCD..."
kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -k "${BOOTSTRAP}/argocd/"
kubectl rollout status deployment/argocd-server -n argocd --timeout=120s
echo "    ArgoCD deployed."
echo ""

# --- Step 4: Tekton ---
echo ">>> Step 4/9: Deploying Tekton Pipelines + Dashboard + Pipelines-as-Code..."
kubectl apply -k "${BOOTSTRAP}/tekton/"
kubectl rollout status deployment/tekton-pipelines-controller -n tekton-pipelines --timeout=120s
kubectl rollout status deployment/tekton-dashboard -n tekton-pipelines --timeout=120s

echo "    Installing Pipelines-as-Code v0.41.1..."
kubectl apply -f "https://github.com/openshift-pipelines/pipelines-as-code/releases/download/v0.41.1/release.k8s.yaml"
kubectl label namespace pipelines-as-code obs-platform/pac-enabled=true --overwrite 2>/dev/null || true
kubectl rollout status deployment/pipelines-as-code-controller -n pipelines-as-code --timeout=120s
echo "    Tekton + Pipelines-as-Code deployed."
echo ""

# --- Step 5: Gitea ---
echo ">>> Step 5/9: Deploying Gitea..."
kubectl create namespace gitea --dry-run=client -o yaml | kubectl apply -f -
kustomize build --enable-helm "${BOOTSTRAP}/gitea/" | kubectl apply -f -
kubectl rollout status statefulset/gitea -n gitea --timeout=120s 2>/dev/null || \
  kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=gitea -n gitea --timeout=120s
echo "    Gitea deployed."
echo ""

# --- Step 6: Vault ---
echo ">>> Step 6/9: Deploying Vault..."
kubectl create namespace vault --dry-run=client -o yaml | kubectl apply -f -
kustomize build --enable-helm "${BOOTSTRAP}/vault/" | kubectl apply -f -
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=vault -n vault --timeout=120s
echo "    Vault deployed."
echo ""

# --- Step 7: CloudNativePG ---
echo ">>> Step 7/9: Deploying CloudNativePG operator..."
kubectl create namespace cnpg-system --dry-run=client -o yaml | kubectl apply -f -
kustomize build --enable-helm "${BOOTSTRAP}/cloudnative-pg/" | kubectl apply --server-side -f - 2>/dev/null || true
kubectl wait --for=condition=Established crd/clusters.postgresql.cnpg.io --timeout=60s
kustomize build --enable-helm "${BOOTSTRAP}/cloudnative-pg/" | kubectl apply --server-side -f -
kubectl rollout status deployment/cnpg-cloudnative-pg -n cnpg-system --timeout=120s
echo "    CloudNativePG operator deployed."
echo ""

# --- Step 8: External Secrets Operator ---
echo ">>> Step 8/9: Deploying External Secrets Operator..."
kubectl create namespace external-secrets --dry-run=client -o yaml | kubectl apply -f -
# ESO CRDs exceed 256KB — need server-side apply, same two-pass as monitoring
kustomize build --enable-helm "${BOOTSTRAP}/external-secrets/" | kubectl apply --server-side -f - 2>/dev/null || true
echo "    Waiting for ESO CRDs to register..."
kubectl wait --for=condition=Established crd/externalsecrets.external-secrets.io --timeout=60s
kubectl wait --for=condition=Established crd/clustersecretstores.external-secrets.io --timeout=60s
kubectl rollout status deployment/external-secrets -n external-secrets --timeout=120s
kubectl rollout status deployment/external-secrets-webhook -n external-secrets --timeout=120s
echo "    Applying CRs (second pass)..."
kustomize build --enable-helm "${BOOTSTRAP}/external-secrets/" | kubectl apply --server-side -f -
echo "    External Secrets Operator deployed."
echo ""

# --- Step 9: Monitoring ---
echo ">>> Step 9/9: Deploying kube-prometheus-stack..."
kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -
# First pass: creates CRDs + basic resources. CRs (Prometheus, ServiceMonitor, PrometheusRule)
# will fail because the CRDs aren't registered yet — that's expected. Second pass handles them.
kustomize build --enable-helm "${BOOTSTRAP}/monitoring/" | kubectl apply --server-side -f - 2>/dev/null || true
echo "    Waiting for CRDs to register..."
kubectl wait --for=condition=Established crd/prometheuses.monitoring.coreos.com --timeout=60s
kubectl wait --for=condition=Established crd/servicemonitors.monitoring.coreos.com --timeout=60s
echo "    Waiting for Prometheus operator to be ready..."
kubectl rollout status deployment/monitoring-kube-prometheus-operator -n monitoring --timeout=120s
echo "    Applying CRs (second pass)..."
kustomize build --enable-helm "${BOOTSTRAP}/monitoring/" | kubectl apply --server-side -f -
echo "    Waiting for Prometheus pods..."
sleep 15
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=prometheus -n monitoring --timeout=120s
kubectl rollout status deployment/monitoring-grafana -n monitoring --timeout=120s
echo "    Monitoring stack deployed."
echo ""


# --- Post-deploy: Gitea repos ---
echo ">>> Post-deploy: Setting up Gitea repos and users..."
GITEA_PF_STARTED=false
if curl -sf http://localhost:3000/api/v1/version &>/dev/null; then
  echo "    Gitea already reachable on :3000, skipping port-forward."
else
  kubectl port-forward -n gitea svc/gitea-http 3000:3000 &>/dev/null &
  PF_PID=$!
  sleep 5
  GITEA_PF_STARTED=true
fi

GITEA_URL="http://localhost:3000" bash "${BOOTSTRAP}/gitea/setup-repos.sh" || \
  echo "    WARN: Gitea setup had issues. Re-run after port-forwarding."

if [[ "${GITEA_PF_STARTED}" == "true" ]]; then
  kill "${PF_PID}" 2>/dev/null || true
fi
echo ""

# --- Post-deploy: Vault ---
echo ">>> Post-deploy: Configuring Vault..."
VAULT_PF_STARTED=false
if curl -sf http://localhost:8200/v1/sys/health &>/dev/null; then
  echo "    Vault already reachable on :8200, skipping port-forward."
else
  kubectl port-forward -n vault svc/vault 8200:8200 &>/dev/null &
  PF_PID=$!
  sleep 3
  VAULT_PF_STARTED=true
fi

VAULT_ADDR="http://localhost:8200" bash "${BOOTSTRAP}/vault/setup.sh" || \
  echo "    WARN: Vault setup had issues. Re-run after port-forwarding."

if [[ "${VAULT_PF_STARTED}" == "true" ]]; then
  kill "${PF_PID}" 2>/dev/null || true
fi
echo ""

# --- Summary ---
ARGOCD_PASS=$(kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' 2>/dev/null | base64 -d 2>/dev/null || echo "<not available>")

echo "============================================"
echo "  Infrastructure Setup Complete!"
echo "============================================"
echo ""
echo "  Run '.hack/portforward.sh' to access services."
echo ""
echo "  ArgoCD admin password: ${ARGOCD_PASS}"
echo "  ArgoCD CLI login:"
echo "    argocd login localhost:8080 --insecure --username admin --password '${ARGOCD_PASS}'"
echo ""
echo "  Teardown: bash teardown.sh"
echo ""
