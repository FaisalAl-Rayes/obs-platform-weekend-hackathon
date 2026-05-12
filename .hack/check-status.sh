#!/usr/bin/env bash
set -euo pipefail

echo "============================================"
echo "  obs-platform Status Check"
echo "============================================"
echo ""

PASS=0
FAIL=0

check() {
  local label="$1" cmd="$2"
  if (set +o pipefail; eval "${cmd}") &>/dev/null; then
    echo "  [OK]   ${label}"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] ${label}"
    FAIL=$((FAIL + 1))
  fi
}

echo "--- Cluster ---"
check "minikube running" "minikube status -p obs-platform"
echo ""

echo "--- Platform pods ---"
check "ArgoCD" "kubectl get pods -n argocd -l app.kubernetes.io/name=argocd-server --no-headers | grep -q Running"
check "Tekton Pipelines" "kubectl get pods -n tekton-pipelines -l app.kubernetes.io/part-of=tekton-pipelines --no-headers | grep -q Running"
check "Pipelines-as-Code" "kubectl get pods -n pipelines-as-code -l app.kubernetes.io/part-of=pipelines-as-code --no-headers | grep -q Running"
check "Gitea" "kubectl get pods -n gitea -l app.kubernetes.io/name=gitea --no-headers | grep -q Running"
check "Vault" "kubectl get pods -n vault -l app.kubernetes.io/name=vault --no-headers | grep -q Running"
check "Prometheus" "kubectl get pods -n monitoring -l app.kubernetes.io/name=prometheus --no-headers | grep -q Running"
check "Grafana" "kubectl get pods -n monitoring -l app.kubernetes.io/name=grafana --no-headers | grep -q Running"
check "External Secrets" "kubectl get pods -n external-secrets -l app.kubernetes.io/name=external-secrets --no-headers | grep -q Running"
check "CloudNativePG" "kubectl get pods -n cnpg-system --no-headers | grep -q Running"
echo ""

echo "--- ArgoCD applications ---"
APPS=$(kubectl get applications -n argocd --no-headers 2>/dev/null || echo "")
if [[ -n "${APPS}" ]]; then
  TOTAL=$(echo "${APPS}" | wc -l | tr -d ' ')
  HEALTHY=$(echo "${APPS}" | grep -c "Healthy" || echo 0)
  SYNCED=$(echo "${APPS}" | grep -c "Synced" || echo 0)
  echo "  ${TOTAL} apps: ${SYNCED} synced, ${HEALTHY} healthy"
  NOT_HEALTHY=$(echo "${APPS}" | grep -v "Healthy" || true)
  if [[ -n "${NOT_HEALTHY}" ]]; then
    echo "  Unhealthy:"
    echo "${NOT_HEALTHY}" | awk '{print "    " $1 " (" $2 "/" $3 ")"}'
  fi
else
  echo "  No applications found"
fi
echo ""

echo "--- Tower & RIA ---"
check "Tower (stage)" "kubectl get pods -n tower-stage -l app=tower --no-headers 2>/dev/null | grep -q Running"
check "RIA (stage)" "kubectl get pods -n ria-stage -l app=ria --no-headers 2>/dev/null | grep -q Running"
echo ""

echo "--- Prometheus targets ---"
TARGETS=$(curl -sf http://localhost:9090/api/v1/targets 2>/dev/null || echo "")
if [[ -n "${TARGETS}" ]]; then
  UP=$(echo "${TARGETS}" | jq '[.data.activeTargets[] | select(.health=="up")] | length' 2>/dev/null || echo "?")
  DOWN=$(echo "${TARGETS}" | jq '[.data.activeTargets[] | select(.health!="up")] | length' 2>/dev/null || echo "?")
  echo "  Targets: ${UP} up, ${DOWN} down"
else
  echo "  Prometheus not reachable (port-forward running?)"
fi
echo ""

echo "--- Gitea ---"
GITEA_OK=$(curl -sf http://localhost:3000/api/v1/version 2>/dev/null | jq -r '.version' 2>/dev/null || echo "")
if [[ -n "${GITEA_OK}" ]]; then
  echo "  Gitea v${GITEA_OK} reachable"
else
  echo "  Gitea not reachable (port-forward running?)"
fi
echo ""

echo "============================================"
echo "  ${PASS} passed, ${FAIL} failed"
echo "============================================"
