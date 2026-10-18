#!/usr/bin/env bash
set -euo pipefail

umask 077

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lib/secret-shell-guard.sh
source "${ROOT}/scripts/lib/secret-shell-guard.sh"
secret_shell_refuse_xtrace
export DOPPLER_PROJECT="${DOPPLER_PROJECT:-hub-framework}"
export DOPPLER_CONFIG="${DOPPLER_CONFIG:-infra}"
# shellcheck source=scripts/lib/doppler-secrets.sh
source "${ROOT}/scripts/lib/doppler-secrets.sh"
# shellcheck source=scripts/lib/doppler-configs.sh
source "${ROOT}/scripts/lib/doppler-configs.sh"
# shellcheck source=scripts/lib/preflight.sh
source "${ROOT}/scripts/lib/preflight.sh"
# shellcheck source=scripts/lib/plan-files.sh
source "${ROOT}/scripts/lib/plan-files.sh"
# shellcheck source=scripts/lib/bootstrap-backend-mode.sh
source "${ROOT}/scripts/lib/bootstrap-backend-mode.sh"
# shellcheck source=scripts/lib/doppler-ci-token.sh
source "${ROOT}/scripts/lib/doppler-ci-token.sh"
# shellcheck source=scripts/lib/bootstrap-doppler-sync.sh
source "${ROOT}/scripts/lib/bootstrap-doppler-sync.sh"

BOOTSTRAP_DIR="${ROOT}/infra/opentofu/state-bootstrap"
trap cleanup_plan_files EXIT

RELEASE_DIR="${ROOT}/infra/opentofu/release-environment"
STATE_BUCKET_DEFAULT="queerglobal-hub-framework-tofu-state"
RELEASE_STATE_KEY="hub-framework/release-environment/terraform.tfstate"
BOOTSTRAP_PLAN="${BOOTSTRAP_DIR}/bootstrap.tfplan"
RELEASE_PLAN="${RELEASE_DIR}/release.tfplan"

register_plan_file "${BOOTSTRAP_PLAN}"
register_plan_file "${RELEASE_PLAN}"

confirm() {
  local prompt=$1
  read -r -p "${prompt} [y/N] " reply
  case "${reply}" in
    y|Y|yes|YES) return 0 ;;
    *) echo "Aborted." >&2; exit 1 ;;
  esac
}

require_cmd() {
  preflight_require_commands "$@"
}

require_cmd tofu doppler curl jq openssl

export CLOUDFLARE_API_TOKEN="${CLOUDFLARE_API_TOKEN:-}"

ensure_doppler_bootstrap_admin_config() {
  if ! doppler configs get "${DOPPLER_BOOTSTRAP_ADMIN_CONFIG}" --project "${DOPPLER_PROJECT_NAME}" >/dev/null 2>&1; then
    echo "Creating Doppler config ${DOPPLER_PROJECT_NAME}/${DOPPLER_BOOTSTRAP_ADMIN_CONFIG} (bootstrap-admin, not used by CI)..."
    doppler configs create "${DOPPLER_BOOTSTRAP_ADMIN_CONFIG}" --project "${DOPPLER_PROJECT_NAME}"
  fi
}

ACCOUNT_ID="$(preflight_bootstrap_inputs_readonly "${DOPPLER_PROJECT_NAME}" "${DOPPLER_INFRA_CONFIG}" \
  "${DOPPLER_BOOTSTRAP_ADMIN_CONFIG}")" || exit 1

preflight_ensure_doppler_project_and_config "${DOPPLER_PROJECT_NAME}" "${DOPPLER_INFRA_CONFIG}"
ensure_doppler_bootstrap_admin_config
bootstrap_sync_env_secrets_to_doppler "${DOPPLER_PROJECT_NAME}" "${DOPPLER_INFRA_CONFIG}" \
  "${DOPPLER_BOOTSTRAP_ADMIN_CONFIG}" confirm || exit 1

if ! doppler_ci_infra_service_token_exists "${DOPPLER_PROJECT_NAME}" "${DOPPLER_INFRA_CONFIG}" \
  "${DOPPLER_CI_INFRA_SERVICE_TOKEN_NAME}"; then
  confirm "Create read-only Doppler service token ${DOPPLER_CI_INFRA_SERVICE_TOKEN_NAME} for CI (${DOPPLER_INFRA_CONFIG} only)?"
  CI_DOPPLER_TOKEN="$(ensure_doppler_ci_infra_service_token "${DOPPLER_PROJECT_NAME}" "${DOPPLER_INFRA_CONFIG}")"
  confirm "Store ${DOPPLER_CI_INFRA_SERVICE_TOKEN_NAME} token in Doppler (${DOPPLER_INFRA_CONFIG}) for GitHub Actions secret sync?"
  doppler_secret_set_stdin DOPPLER_CI_INFRA_SERVICE_TOKEN "${CI_DOPPLER_TOKEN}"
fi

export TF_VAR_cloudflare_account_id="${ACCOUNT_ID}"
export TF_VAR_state_bucket_name="${STATE_BUCKET_DEFAULT}"

bootstrap_ensure_generated_passphrase TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE \
  "${DOPPLER_PROJECT_NAME}" "${DOPPLER_BOOTSTRAP_ADMIN_CONFIG}" confirm || exit 1

export TF_VAR_bootstrap_state_encryption_passphrase
TF_VAR_bootstrap_state_encryption_passphrase="$(doppler secrets get TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE --plain \
  -p "${DOPPLER_PROJECT_NAME}" -c "${DOPPLER_BOOTSTRAP_ADMIN_CONFIG}")"

bootstrap_ensure_generated_passphrase TOFU_STATE_ENCRYPTION_PASSPHRASE \
  "${DOPPLER_PROJECT_NAME}" "${DOPPLER_INFRA_CONFIG}" confirm || exit 1

echo "Bootstrap stack (R2 bucket + release scoped credentials; bootstrap state stays local)"
confirm "Run tofu init/plan/apply for state-bootstrap (creates R2 bucket and release-scoped API token)?"

cd "${BOOTSTRAP_DIR}"
tofu init -input=false
tofu plan -input=false -out="${BOOTSTRAP_PLAN}"
confirm "Apply bootstrap plan?"
tofu apply -input=false "${BOOTSTRAP_PLAN}"

confirm "Back up encrypted local bootstrap state to Doppler (${DOPPLER_BOOTSTRAP_ADMIN_CONFIG})?"
bootstrap_backup_local_state_to_doppler "${BOOTSTRAP_DIR}/terraform.tfstate" \
  "${DOPPLER_PROJECT_NAME}" "${DOPPLER_BOOTSTRAP_ADMIN_CONFIG}"

BUCKET="$(tofu output -raw state_bucket_name)"
if [[ "${BUCKET}" != "${STATE_BUCKET_DEFAULT}" ]]; then
  echo "Unexpected bucket name ${BUCKET}; expected ${STATE_BUCKET_DEFAULT}." >&2
  exit 1
fi
ENDPOINT="$(tofu output -raw r2_s3_endpoint)"
ACCESS_KEY="$(tofu output -raw tofu_state_access_key_id)"
SECRET_KEY="$(tofu output -raw tofu_state_secret_access_key)"

confirm "Write release-environment R2 backend settings to Doppler (${DOPPLER_INFRA_CONFIG})?"
doppler_secret_set_stdin TOFU_STATE_BUCKET "${BUCKET}"
doppler_secret_set_stdin TOFU_STATE_ENDPOINT "${ENDPOINT}"
doppler_secret_set_stdin TOFU_STATE_REGION "auto"
doppler_secret_set_stdin TOFU_STATE_KEY "${RELEASE_STATE_KEY}"
doppler_secret_set_stdin AWS_ACCESS_KEY_ID "${ACCESS_KEY}"
doppler_secret_set_stdin AWS_SECRET_ACCESS_KEY "${SECRET_KEY}"

BACKEND_CONFIG="${RELEASE_DIR}/backend-config.hcl"
confirm "Write ${BACKEND_CONFIG} from Doppler (gitignored)?"
export BACKEND_CONFIG
doppler run -p "${DOPPLER_PROJECT_NAME}" -c "${DOPPLER_INFRA_CONFIG}" -- bash -c '
set -euo pipefail
cat > "${BACKEND_CONFIG}" <<EOF
bucket                      = "${TOFU_STATE_BUCKET}"
key                         = "${TOFU_STATE_KEY}"
region                      = "${TOFU_STATE_REGION}"
endpoint                    = "${TOFU_STATE_ENDPOINT}"
encrypt                     = true
use_lockfile                = true
skip_credentials_validation = true
skip_region_validation      = true
skip_requesting_account_id  = true
EOF
'

echo "Release environment stack"
confirm "Run tofu init/plan/apply for release-environment (GitHub environment)?"

cd "${RELEASE_DIR}"
export TF_VAR_state_encryption_passphrase
TF_VAR_state_encryption_passphrase="$(doppler secrets get TOFU_STATE_ENCRYPTION_PASSPHRASE --plain -p "${DOPPLER_PROJECT_NAME}" -c "${DOPPLER_INFRA_CONFIG}")"

export GITHUB_TOKEN
GITHUB_TOKEN="$(preflight_github_tofu_release_env_token "${DOPPLER_PROJECT_NAME}" "${DOPPLER_BOOTSTRAP_ADMIN_CONFIG}")"
export RELEASE_PLAN

doppler run -p "${DOPPLER_PROJECT_NAME}" -c "${DOPPLER_INFRA_CONFIG}" -- bash -c '
  set -euo pipefail
  tofu init -input=false -backend-config=backend-config.hcl -reconfigure
  tofu plan -input=false -out="${RELEASE_PLAN}"
'

confirm "Apply release-environment plan?"
doppler run -p "${DOPPLER_PROJECT_NAME}" -c "${DOPPLER_INFRA_CONFIG}" -- bash -c '
  set -euo pipefail
  tofu apply -input=false "${RELEASE_PLAN}"
'

echo "Done. Release environment is ready; bootstrap state remains local with backup in ${DOPPLER_BOOTSTRAP_ADMIN_CONFIG}."
