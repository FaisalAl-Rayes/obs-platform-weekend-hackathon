#!/usr/bin/env bash
set -euo pipefail

VAULT_ADDR="${VAULT_ADDR:-http://localhost:8200}"
VAULT_TOKEN="obs-platform-root-token"

export VAULT_ADDR VAULT_TOKEN

echo "==> Configuring Vault..."

if ! command -v vault &>/dev/null; then
  echo "    WARN: vault CLI not found. Skipping post-deploy configuration."
  echo "    Install vault CLI and re-run, or configure manually."
  exit 0
fi

echo "==> Enabling KV secrets engine..."
vault secrets enable -path=secret kv-v2 2>/dev/null || echo "    KV engine may already be enabled"

echo "==> Enabling Kubernetes auth..."
vault auth enable kubernetes 2>/dev/null || echo "    K8s auth may already be enabled"

echo "==> Creating team policies..."
for team in team-alpha team-beta; do
  vault policy write "${team}" - <<EOF
path "secret/data/${team}/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}
path "secret/metadata/${team}/*" {
  capabilities = ["list", "read"]
}
EOF
  echo "    Created policy: ${team}"
done

echo "==> Vault configured."
