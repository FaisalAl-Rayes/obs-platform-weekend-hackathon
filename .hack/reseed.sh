#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "${SCRIPT_DIR}")"

GITEA_URL="${GITEA_URL:-http://localhost:3000}"
VAULT_ADDR="${VAULT_ADDR:-http://localhost:8200}"

echo "==> Re-seeding repos..."
echo "    Gitea: ${GITEA_URL}"
echo "    Vault: ${VAULT_ADDR}"
echo ""

GITEA_URL="${GITEA_URL}" VAULT_ADDR="${VAULT_ADDR}" \
  bash "${ROOT_DIR}/cluster-bootstrap/gitea/setup-repos.sh"
