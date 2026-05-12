#!/usr/bin/env bash
set -euo pipefail

PROFILE="obs-platform"
CPUS=4
MEMORY="8192"
DRIVER="docker"

echo "==> Starting minikube cluster '${PROFILE}'..."

if minikube status -p "${PROFILE}" &>/dev/null; then
  echo "    Cluster '${PROFILE}' already running, skipping start."
else
  minikube start \
    -p "${PROFILE}" \
    --cpus="${CPUS}" \
    --memory="${MEMORY}" \
    --driver="${DRIVER}" \
    --container-runtime=containerd \
    --insecure-registry="10.0.0.0/8" \
    --addons=registry,metrics-server
fi

echo "==> Enabling addons..."
minikube addons enable registry -p "${PROFILE}"
minikube addons enable metrics-server -p "${PROFILE}"

# Configure containerd to treat localhost:5000 as HTTP.
# The minikube registry addon proxies localhost:5000 on the node to the registry service.
# Containerd needs to know it's HTTP, not HTTPS.
echo "==> Configuring containerd for local registry..."
minikube -p "${PROFILE}" ssh -- "sudo mkdir -p /etc/containerd/certs.d/localhost:5000 && sudo tee /etc/containerd/certs.d/localhost:5000/hosts.toml > /dev/null <<EOF
server = \"http://localhost:5000\"

[host.\"http://localhost:5000\"]
  capabilities = [\"pull\", \"resolve\"]
  plain-http = true
EOF"

echo "==> Verifying cluster health..."
kubectl get nodes
echo ""
echo "==> minikube cluster '${PROFILE}' is ready."
echo "    Registry: registry.kube-system.svc.cluster.local (HTTP, in-cluster)"
