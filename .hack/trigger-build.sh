#!/usr/bin/env bash
set -euo pipefail

GITEA_URL="${GITEA_URL:-http://localhost:3000}"
GITEA_USER="${GITEA_USER:-gitea_admin}"
GITEA_PASS="${GITEA_PASS:-admin1234}"

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <org/repo> [message]"
  echo ""
  echo "Examples:"
  echo "  $0 platform/tower"
  echo "  $0 team-alpha/sample-app"
  echo "  $0 platform/ria 'fix: update templates'"
  exit 1
fi

REPO="$1"
MESSAGE="${2:-trigger: rebuild $(date +%Y-%m-%dT%H:%M:%S)}"

TMP=$(mktemp -d)
trap "rm -rf ${TMP}" EXIT

echo "==> Cloning ${REPO}..."
git clone -q "http://${GITEA_USER}:${GITEA_PASS}@localhost:3000/${REPO}.git" "${TMP}/repo"

cd "${TMP}/repo"
echo "# build $(date +%s)" >> README.md

git -c user.name="obs-platform" -c user.email="trigger@obs-platform.local" add -A
git -c user.name="obs-platform" -c user.email="trigger@obs-platform.local" commit -q -m "${MESSAGE}"
git push -q

echo "==> Pushed to ${REPO}. Pipeline should trigger."
echo "    Watch: kubectl get pipelineruns -A --watch"
