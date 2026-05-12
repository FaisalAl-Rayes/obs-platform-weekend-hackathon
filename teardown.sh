#!/usr/bin/env bash
set -euo pipefail

PROFILE="obs-platform"
PID_FILE="/tmp/obs-platform-portforward.pids"

echo "============================================"
echo "  obs-platform Teardown"
echo "============================================"
echo ""

# --- Stop port-forwards ---
if [[ -f "${PID_FILE}" ]]; then
  echo "==> Stopping port-forwards..."
  while read -r pid; do
    kill "${pid}" 2>/dev/null || true
  done < "${PID_FILE}"
  rm -f "${PID_FILE}"
  echo "    Port-forwards stopped."
else
  echo "==> No active port-forwards found."
fi
echo ""

# --- Delete minikube cluster ---
echo "==> Deleting minikube cluster '${PROFILE}'..."
if minikube status -p "${PROFILE}" &>/dev/null; then
  minikube delete -p "${PROFILE}"
  echo "    Cluster deleted."
else
  echo "    Cluster '${PROFILE}' not found, nothing to delete."
fi
echo ""

echo "==> Teardown complete."
