#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GITEA_URL="${GITEA_URL:-http://localhost:3000}"
ADMIN_USER="gitea_admin"
ADMIN_PASS="admin1234"
AUTH="-u ${ADMIN_USER}:${ADMIN_PASS}"

api() {
  local method="$1" path="$2"
  shift 2
  curl -sf ${AUTH} -H "Content-Type: application/json" \
    -X "${method}" "${GITEA_URL}/api/v1${path}" "$@"
}

api_ignore_conflict() {
  local method="$1" path="$2"
  shift 2
  local http_code
  http_code=$(curl -s ${AUTH} -H "Content-Type: application/json" \
    -X "${method}" "${GITEA_URL}/api/v1${path}" -o /dev/null -w "%{http_code}" "$@")
  if [[ "${http_code}" != "201" && "${http_code}" != "200" && "${http_code}" != "409" && "${http_code}" != "422" ]]; then
    echo "    WARN: ${method} ${path} returned ${http_code}"
    return 1
  fi
  return 0
}

echo "==> Waiting for Gitea API to be ready..."
until curl -sf "${GITEA_URL}/api/v1/version" &>/dev/null; do
  sleep 2
done
echo "    Gitea API is up."

# --- Create organizations ---
echo "==> Creating organizations..."
for org in platform team-alpha team-beta; do
  api_ignore_conflict POST "/orgs" -d "{\"username\": \"${org}\", \"visibility\": \"public\"}" && \
    echo "    Created org: ${org}" || echo "    Org '${org}' may already exist"
done

# --- Create platform repos ---
echo "==> Creating platform repos..."
PLATFORM_REPOS="service-catalog pipelines policies gitops-deploy tower ria"

for repo in ${PLATFORM_REPOS}; do
  api_ignore_conflict POST "/orgs/platform/repos" \
    -d "{\"name\": \"${repo}\", \"auto_init\": true, \"default_branch\": \"main\"}" && \
    echo "    Created repo: platform/${repo}" || echo "    Repo 'platform/${repo}' may already exist"
done

# --- Create team users ---
echo "==> Creating team users..."

create_user() {
  local user="$1" pass="$2"
  api_ignore_conflict POST "/admin/users" \
    -d "{
      \"username\": \"${user}\",
      \"password\": \"${pass}\",
      \"email\": \"${user}@obs-platform.local\",
      \"must_change_password\": false
    }" && \
    echo "    Created user: ${user}" || echo "    User '${user}' may already exist"
}

create_user "tower-bot" "tower-bot1234"
create_user "ria-bot" "ria-bot1234"
create_user "alpha-dev" "alpha1234"
create_user "beta-dev" "beta1234"

# --- Create team repos ---
echo "==> Setting up team repos..."
for team in team-alpha team-beta; do
  api_ignore_conflict POST "/orgs/${team}/repos" \
    -d "{\"name\": \"pipelines\", \"auto_init\": true, \"default_branch\": \"main\"}" && \
    echo "    Created repo: ${team}/pipelines" || true

  api_ignore_conflict POST "/orgs/${team}/repos" \
    -d "{\"name\": \"sample-app\", \"auto_init\": true, \"default_branch\": \"main\"}" && \
    echo "    Created repo: ${team}/sample-app" || true
done

# --- Configure team-based access control ---
echo "==> Configuring team-based access control..."

create_org_team() {
  local org="$1" team_name="$2" permission="$3"
  local team_id
  team_id=$(api POST "/orgs/${org}/teams" \
    -d "{
      \"name\": \"${team_name}\",
      \"permission\": \"${permission}\",
      \"units\": [\"repo.code\", \"repo.issues\", \"repo.pulls\"]
    }" 2>/dev/null | jq -r '.id // empty' 2>/dev/null || echo "")
  echo "${team_id}"
}

add_team_member() {
  local team_id="$1" username="$2"
  api_ignore_conflict PUT "/teams/${team_id}/members/${username}" 2>/dev/null || true
}

add_team_repo() {
  local team_id="$1" org="$2" repo="$3"
  api_ignore_conflict PUT "/teams/${team_id}/repos/${org}/${repo}" 2>/dev/null || true
}

alpha_devs_id=$(create_org_team "team-alpha" "developers" "write")
if [[ -n "${alpha_devs_id}" ]]; then
  add_team_member "${alpha_devs_id}" "alpha-dev"
  add_team_repo "${alpha_devs_id}" "team-alpha" "pipelines"
  add_team_repo "${alpha_devs_id}" "team-alpha" "sample-app"
  echo "    Configured alpha-dev write access to team-alpha/*"
fi

beta_devs_id=$(create_org_team "team-beta" "developers" "write")
if [[ -n "${beta_devs_id}" ]]; then
  add_team_member "${beta_devs_id}" "beta-dev"
  add_team_repo "${beta_devs_id}" "team-beta" "pipelines"
  add_team_repo "${beta_devs_id}" "team-beta" "sample-app"
  echo "    Configured beta-dev write access to team-beta/*"
fi

platform_readers_id=$(create_org_team "platform" "all-developers" "read")
if [[ -n "${platform_readers_id}" ]]; then
  add_team_member "${platform_readers_id}" "alpha-dev"
  add_team_member "${platform_readers_id}" "beta-dev"
  for repo in ${PLATFORM_REPOS}; do
    add_team_repo "${platform_readers_id}" "platform" "${repo}"
  done
  echo "    Configured read-only access to platform repos for all developers"
fi

# tower-bot: write access to service-catalog only
tower_bot_id=$(create_org_team "platform" "tower-bot" "write")
if [[ -n "${tower_bot_id}" ]]; then
  add_team_member "${tower_bot_id}" "tower-bot"
  add_team_repo "${tower_bot_id}" "platform" "service-catalog"
  echo "    Configured tower-bot write access to platform/service-catalog"
fi

# ria-bot: write access to platform repos + admin on all team repos
# RIA needs admin access to team repos to push .tekton/ and manage webhooks
ria_bot_id=$(create_org_team "platform" "ria-bot" "write")
if [[ -n "${ria_bot_id}" ]]; then
  add_team_member "${ria_bot_id}" "ria-bot"
  add_team_repo "${ria_bot_id}" "platform" "service-catalog"
  add_team_repo "${ria_bot_id}" "platform" "gitops-deploy"
  add_team_repo "${ria_bot_id}" "platform" "service-config"
  echo "    Configured ria-bot write access to platform/{service-catalog,gitops-deploy,service-config}"
fi

# ria-bot: admin access to all team repos (for .tekton/ push and webhook creation)
for team in team-alpha team-beta; do
  ria_team_id=$(create_org_team "${team}" "ria-bot" "admin")
  if [[ -n "${ria_team_id}" ]]; then
    add_team_member "${ria_team_id}" "ria-bot"
    add_team_repo "${ria_team_id}" "${team}" "pipelines"
    add_team_repo "${ria_team_id}" "${team}" "sample-app"
    echo "    Configured ria-bot admin access to ${team}/*"
  fi
done

# --- Seed repos from cluster-bootstrap/gitea/repos/ ---
echo "==> Seeding repos from source of truth..."
REPOS_DIR="${SCRIPT_DIR}/repos"

if [[ ! -d "${REPOS_DIR}" ]]; then
  echo "    WARN: ${REPOS_DIR} not found, skipping seeding."
else
  TMP_DIR=$(mktemp -d)
  trap "rm -rf ${TMP_DIR}" EXIT

  for org_dir in "${REPOS_DIR}"/*/; do
    org=$(basename "$org_dir")
    for repo_dir in "${org_dir}"*/; do
      repo=$(basename "$repo_dir")
      clone_dir="${TMP_DIR}/${org}/${repo}"

      echo "    Seeding ${org}/${repo}..."
      git clone -q "http://${ADMIN_USER}:${ADMIN_PASS}@localhost:3000/${org}/${repo}.git" "${clone_dir}" 2>/dev/null || {
        echo "    WARN: Could not clone ${org}/${repo}, skipping."
        continue
      }

      # Wipe all tracked content (except .git)
      find "${clone_dir}" -mindepth 1 -maxdepth 1 -not -name .git -exec rm -rf {} +

      # Copy seed content (regular files and dotfiles)
      cp -r "${repo_dir}"/* "${clone_dir}"/ 2>/dev/null || true
      cp -r "${repo_dir}"/.[!.]* "${clone_dir}"/ 2>/dev/null || true

      cd "${clone_dir}"
      git add -A
      if git diff --cached --quiet; then
        echo "      No changes, already in sync."
      else
        git -c user.name="obs-platform" -c user.email="bootstrap@obs-platform.local" \
          commit -q -m "seed: sync with cluster-bootstrap source of truth"
        git push --force -q
        echo "      Pushed."
      fi
    done
  done
fi

# --- Create PAC API token and store in Vault (idempotent) ---
# Strategy: always ensure the token exists in both Gitea and Vault.
# If the token exists in Gitea but not in Vault, we can't retrieve the value,
# so we delete and recreate it to guarantee it lands in Vault.
echo "==> Creating Gitea API token for Pipelines-as-Code..."

VAULT_ADDR="${VAULT_ADDR:-http://localhost:8200}"
VAULT_TOKEN="obs-platform-root-token"

VAULT_HAS_TOKEN=$(curl -sf "${VAULT_ADDR}/v1/secret/data/platform/pac" \
  -H "X-Vault-Token: ${VAULT_TOKEN}" 2>/dev/null | \
  jq -r 'if .data.data.token then "true" else "false" end' 2>/dev/null || echo "false")

if [[ "${VAULT_HAS_TOKEN}" == "true" ]]; then
  echo "    PAC token already in Vault, skipping."
else
  # Delete existing Gitea token if any (we can't read its value back)
  EXISTING_TOKEN_ID=$(curl -sf ${AUTH} "${GITEA_URL}/api/v1/users/${ADMIN_USER}/tokens" | \
    jq -r '.[] | select(.name=="pac-token") | .id // empty' 2>/dev/null || echo "")
  if [[ -n "${EXISTING_TOKEN_ID}" ]]; then
    curl -sf ${AUTH} -X DELETE "${GITEA_URL}/api/v1/users/${ADMIN_USER}/tokens/${EXISTING_TOKEN_ID}" || true
    echo "    Deleted stale Gitea token."
  fi

  # Create fresh token
  PAC_TOKEN=$(curl -sf ${AUTH} -H "Content-Type: application/json" \
    -X POST "${GITEA_URL}/api/v1/users/${ADMIN_USER}/tokens" \
    -d '{"name": "pac-token", "scopes": ["all"]}' | \
    jq -r '.sha1 // empty' 2>/dev/null || echo "")

  if [[ -n "${PAC_TOKEN}" ]]; then
    echo "    Created token 'pac-token'."
    echo "    Storing in Vault at secret/data/platform/pac..."
    curl -sf -X POST "${VAULT_ADDR}/v1/secret/data/platform/pac" \
      -H "X-Vault-Token: ${VAULT_TOKEN}" \
      -H "Content-Type: application/json" \
      -d "{\"data\": {\"token\": \"${PAC_TOKEN}\"}}" > /dev/null
    echo "    Stored in Vault."
  else
    echo "    WARN: Failed to create PAC token."
  fi
fi

# ExternalSecret for PAC token is managed via ClusterExternalSecret
# in gitops-deploy/components/platform/ — deployed by ArgoCD

# --- Store tower-bot credentials in Vault ---
echo "==> Storing tower-bot credentials in Vault..."
VAULT_HAS_TOWER_BOT=$(curl -sf "${VAULT_ADDR}/v1/secret/data/platform/tower" \
  -H "X-Vault-Token: ${VAULT_TOKEN}" 2>/dev/null | \
  jq -r 'if .data.data.username then "true" else "false" end' 2>/dev/null || echo "false")

if [[ "${VAULT_HAS_TOWER_BOT}" == "true" ]]; then
  echo "    tower-bot credentials already in Vault, skipping."
else
  curl -sf -X POST "${VAULT_ADDR}/v1/secret/data/platform/tower" \
    -H "X-Vault-Token: ${VAULT_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"data": {"username": "tower-bot", "password": "tower-bot1234"}}' > /dev/null
  echo "    Stored in Vault."
fi

# --- Store ria-bot credentials in Vault ---
echo "==> Storing ria-bot credentials in Vault..."
VAULT_HAS_RIA_BOT=$(curl -sf "${VAULT_ADDR}/v1/secret/data/platform/ria" \
  -H "X-Vault-Token: ${VAULT_TOKEN}" 2>/dev/null | \
  jq -r 'if .data.data.username then "true" else "false" end' 2>/dev/null || echo "false")

if [[ "${VAULT_HAS_RIA_BOT}" == "true" ]]; then
  echo "    ria-bot credentials already in Vault, skipping."
else
  curl -sf -X POST "${VAULT_ADDR}/v1/secret/data/platform/ria" \
    -H "X-Vault-Token: ${VAULT_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"data": {"username": "ria-bot", "password": "ria-bot1234"}}' > /dev/null
  echo "    Stored in Vault."
fi

# --- Create webhook for Tower repo ---
echo "==> Creating Gitea webhook for platform/tower..."
PAC_WEBHOOK_URL="http://pipelines-as-code-controller.pipelines-as-code.svc:8080"
HOOK_EXISTS=$(curl -sf ${AUTH} "${GITEA_URL}/api/v1/repos/platform/tower/hooks" | \
  jq -r "[.[] | select(.config.url==\"${PAC_WEBHOOK_URL}\")] | length > 0" 2>/dev/null || echo "false")

if [[ "${HOOK_EXISTS}" == "true" ]]; then
  echo "    Webhook already exists, skipping."
else
  api_ignore_conflict POST "/repos/platform/tower/hooks" \
    -d "{
      \"type\": \"gitea\",
      \"active\": true,
      \"events\": [\"push\", \"pull_request\", \"issue_comment\"],
      \"config\": {
        \"url\": \"${PAC_WEBHOOK_URL}\",
        \"content_type\": \"json\"
      }
    }" && \
    echo "    Webhook created for platform/tower." || \
    echo "    WARN: Failed to create webhook."
fi

# --- Create webhook for service-catalog repo (triggers RIA) ---
echo "==> Creating Gitea webhook for platform/service-catalog -> RIA..."
RIA_WEBHOOK_URL="http://ria.ria-stage.svc:80/webhook"
RIA_HOOK_EXISTS=$(curl -sf ${AUTH} "${GITEA_URL}/api/v1/repos/platform/service-catalog/hooks" | \
  jq -r "[.[] | select(.config.url==\"${RIA_WEBHOOK_URL}\")] | length > 0" 2>/dev/null || echo "false")

if [[ "${RIA_HOOK_EXISTS}" == "true" ]]; then
  echo "    Webhook already exists, skipping."
else
  api_ignore_conflict POST "/repos/platform/service-catalog/hooks" \
    -d "{
      \"type\": \"gitea\",
      \"active\": true,
      \"events\": [\"push\"],
      \"config\": {
        \"url\": \"${RIA_WEBHOOK_URL}\",
        \"content_type\": \"json\"
      }
    }" && \
    echo "    Webhook created for platform/service-catalog -> RIA." || \
    echo "    WARN: Failed to create webhook."
fi

# --- Create webhook for RIA repo (PAC) ---
echo "==> Creating Gitea webhook for platform/ria -> PAC..."
RIA_PAC_HOOK_EXISTS=$(curl -sf ${AUTH} "${GITEA_URL}/api/v1/repos/platform/ria/hooks" | \
  jq -r "[.[] | select(.config.url==\"${PAC_WEBHOOK_URL}\")] | length > 0" 2>/dev/null || echo "false")

if [[ "${RIA_PAC_HOOK_EXISTS}" == "true" ]]; then
  echo "    Webhook already exists, skipping."
else
  api_ignore_conflict POST "/repos/platform/ria/hooks" \
    -d "{
      \"type\": \"gitea\",
      \"active\": true,
      \"events\": [\"push\", \"pull_request\", \"issue_comment\"],
      \"config\": {
        \"url\": \"${PAC_WEBHOOK_URL}\",
        \"content_type\": \"json\"
      }
    }" && \
    echo "    Webhook created for platform/ria -> PAC." || \
    echo "    WARN: Failed to create webhook."
fi

# --- Apply app-of-appsets to ArgoCD ---
echo "==> Applying app-of-appsets to ArgoCD..."
GITOPS_DIR="${TMP_DIR}/platform/gitops-deploy"
if [[ -d "${GITOPS_DIR}/appsets/app-of-appsets/overlays/stage" ]]; then
  kubectl apply -k "${GITOPS_DIR}/appsets/app-of-appsets/overlays/stage/"
  echo "    Applied stage app-of-appsets."
  kubectl apply -k "${GITOPS_DIR}/appsets/app-of-appsets/overlays/prod/"
  echo "    Applied prod app-of-appsets."
  kubectl apply -k "${GITOPS_DIR}/appsets/app-of-appsets/overlays/ci/"
  echo "    Applied ci app-of-appsets."
else
  echo "    WARN: gitops-deploy not cloned, skipping app-of-appsets."
fi

echo ""
echo "==> Gitea setup complete."
echo "    Admin:      ${ADMIN_USER} / ${ADMIN_PASS}"
echo "    Alpha dev:  alpha-dev / alpha1234"
echo "    Beta dev:   beta-dev / beta1234"
echo ""
echo "    Orgs:       platform, team-alpha, team-beta"
echo "    Platform:   platform/{${PLATFORM_REPOS// /, }}"
echo "    Team Alpha: team-alpha/{pipelines, sample-app}"
echo "    Team Beta:  team-beta/{pipelines, sample-app}"
