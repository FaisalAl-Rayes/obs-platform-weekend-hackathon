#!/usr/bin/env bash
set -euo pipefail

PID_FILE="/tmp/obs-platform-portforward.pids"

ALL_NAMES="gitea argocd tekton vault prometheus grafana registry"

get_spec() {
  case "$1" in
    gitea)      echo "gitea svc/gitea-http 3000 3000" ;;
    argocd)     echo "argocd svc/argocd-server 8080 443" ;;
    tekton)     echo "tekton-pipelines svc/tekton-dashboard 9097 9097" ;;
    vault)      echo "vault svc/vault 8200 8200" ;;
    prometheus) echo "monitoring svc/monitoring-kube-prometheus-prometheus 9090 9090" ;;
    grafana)    echo "monitoring svc/monitoring-grafana 3001 80" ;;
    registry)   echo "kube-system svc/registry 5000 80" ;;
    *)          return 1 ;;
  esac
}

stop_all() {
  if [[ -f "${PID_FILE}" ]]; then
    echo "==> Stopping existing port-forwards..."
    while read -r pid; do
      kill "${pid}" 2>/dev/null || true
    done < "${PID_FILE}"
    rm -f "${PID_FILE}"
  fi
}

start_forward() {
  local name="$1"
  local spec
  if ! spec=$(get_spec "${name}"); then
    echo "    WARN: Unknown service '${name}'. Available: ${ALL_NAMES}"
    return
  fi

  local ns resource local_port remote_port
  read -r ns resource local_port remote_port <<< "${spec}"

  echo "    ${name} -> localhost:${local_port}"
  kubectl port-forward -n "${ns}" "${resource}" "${local_port}:${remote_port}" &>/dev/null &
  echo $! >> "${PID_FILE}"
}

if [[ "${1:-}" == "stop" ]]; then
  stop_all
  echo "==> All port-forwards stopped."
  exit 0
fi

stop_all

echo "==> Starting port-forwards..."
touch "${PID_FILE}"

if [[ $# -eq 0 ]]; then
  for name in ${ALL_NAMES}; do
    start_forward "${name}"
  done
else
  for name in "$@"; do
    start_forward "${name}"
  done
fi

sleep 1
echo ""
echo "==> Port-forwards active. PIDs saved to ${PID_FILE}"
echo "    Stop with: $0 stop"
echo ""
echo "  ┌──────────────────────────────────────────────────────────────────────────────┐"
echo "  │  Service          URL                        Credentials                     │"
echo "  ├──────────────────────────────────────────────────────────────────────────────┤"
echo "  │  Gitea            http://localhost:3000       gitea_admin / admin1234        │"
echo "  │  ArgoCD           https://localhost:8080      admin / (see below)            │"
echo "  │  Tekton Dashboard http://localhost:9097       -                              │"
echo "  │  Vault            http://localhost:8200       Token: obs-platform-root-token │"
echo "  │  Prometheus       http://localhost:9090       -                              │"
echo "  │  Grafana          http://localhost:3001       admin / admin1234              │"
echo "  │  Registry         http://localhost:5000       -                              │"
echo "  └──────────────────────────────────────────────────────────────────────────────┘"
echo ""

ARGOCD_PASS=$(kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' 2>/dev/null | base64 -d 2>/dev/null || echo "<not available>")
echo "  ArgoCD admin password: ${ARGOCD_PASS}"
echo ""
echo "  ArgoCD CLI login:"
echo "    argocd login localhost:8080 --insecure --username admin --password '${ARGOCD_PASS}'"
echo ""

echo ""
echo "  Gitea users:"
echo "    Admin:      gitea_admin / admin1234"
echo "    Alpha dev:  alpha-dev / alpha1234"
echo "    Beta dev:   beta-dev / beta1234"
echo ""

