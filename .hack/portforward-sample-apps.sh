#!/usr/bin/env bash
set -euo pipefail

ENV="${1:-stage}"
PID_FILE="/tmp/obs-platform-portforward-sample-apps.pids"

ALL_APPS="sample-app-alpha sample-app-beta"

get_spec() {
  local app="$1" env="$2"
  case "${app}" in
    sample-app-alpha) echo "${app}-${env} svc/sample-app-alpha 9001 8000" ;;
    sample-app-beta)  echo "${app}-${env} svc/sample-app-beta 9002 8080" ;;
    *)                return 1 ;;
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
  local app="$1"
  local spec
  if ! spec=$(get_spec "${app}" "${ENV}"); then
    echo "    WARN: Unknown app '${app}'. Available: ${ALL_APPS}"
    return
  fi

  local ns resource local_port remote_port
  read -r ns resource local_port remote_port <<< "${spec}"

  if ! kubectl get ns "${ns}" &>/dev/null; then
    echo "    SKIP: ${app} (namespace '${ns}' not found)"
    return
  fi

  echo "    ${app} (${ENV}) -> localhost:${local_port}"
  kubectl port-forward -n "${ns}" "${resource}" "${local_port}:${remote_port}" &>/dev/null &
  echo $! >> "${PID_FILE}"
}

if [[ "${1:-}" == "stop" ]]; then
  stop_all
  echo "==> All sample-app port-forwards stopped."
  exit 0
fi

stop_all

echo "==> Starting sample-app port-forwards (env: ${ENV})..."
touch "${PID_FILE}"

shift 2>/dev/null || true

if [[ $# -eq 0 ]]; then
  for app in ${ALL_APPS}; do
    start_forward "${app}"
  done
else
  for app in "$@"; do
    start_forward "${app}"
  done
fi

echo ""
echo "==> Sample-app port-forwards active. PIDs saved to ${PID_FILE}"
echo "    Stop with: $0 stop"
echo ""
echo "  Service              URL                       Environment"
echo "  ───────              ───                       ───────────"
echo "  sample-app-alpha     http://localhost:9001      ${ENV}"
echo "  sample-app-beta      http://localhost:9002      ${ENV}"
echo ""
