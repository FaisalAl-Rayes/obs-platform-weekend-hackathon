#!/usr/bin/env bash
set -euo pipefail

ENV="${1:-stage}"
PID_FILE="/tmp/obs-platform-portforward-base.pids"

ALL_APPS="tower ria"

get_spec() {
  local app="$1" env="$2"
  case "${app}" in
    tower) echo "${app}-${env} svc/tower 8888 80" ;;
    ria)   echo "${app}-${env} svc/ria 8889 80" ;;
    *)     return 1 ;;
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
  echo "==> All base port-forwards stopped."
  exit 0
fi

stop_all

echo "==> Starting base port-forwards (env: ${ENV})..."
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
echo "==> Base port-forwards active. PIDs saved to ${PID_FILE}"
echo "    Stop with: $0 stop"
echo ""
echo "  Service   URL                      Environment"
echo "  ───────   ───                      ───────────"
echo "  Tower     http://localhost:8888     ${ENV}"
echo "  RIA       http://localhost:8889     ${ENV}"
echo ""
