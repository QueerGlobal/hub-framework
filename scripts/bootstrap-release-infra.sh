#!/usr/bin/env bash
set -euo pipefail

umask 077

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lib/doppler-secrets.sh
source "${ROOT}/scripts/lib/doppler-secrets.sh"
# shellcheck source=scripts/lib/preflight.sh
source "${ROOT}/scripts/lib/preflight.sh"
# shellcheck source=scripts/lib/plan-files.sh
source "${ROOT}/scripts/lib/plan-files.sh"
# shellcheck source=scripts/lib/bootstrap-backend-mode.sh
source "${ROOT}/scripts/lib/bootstrap-backend-mode.sh"

install_plan_file_trap

BOOTSTRAP_DIR="${ROOT}/infra/opentofu/state-bootstrap"
RELEASE_DIR="${ROOT}/infra/opentofu/release-environment"
DOPPLER_PROJECT="${DOPPLER_PROJECT:-hub-framework}"
DOPPLER_CONFIG="${DOPPLER_CONFIG:-infra}"
STATE_BUCKET_DEFAULT="queerglobal-hub-framework-tofu-state"
BOOTSTRAP_STATE_KEY="hub-framework/state-bootstrap/terraform.tfstate"
RELEASE_STATE_KEY="hub-framework/release-environment/terraform.tfstate"
BOOTSTRAP_PLAN="${BOOTSTRAP_DIR}/bootstrap.tfplan"
RELEASE_PLAN="${RELEASE_DIR}/release.tfplan"
BOOTSTRAP_BACKEND_CONFIG="${BOOTSTRAP_DIR}/bootstrap-backend-config.hcl"

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

preflight_doppler_logged_in
preflight_ensure_doppler_project_and_config "${DOPPLER_PROJECT}" "${DOPPLER_CONFIG}"

if [[ -z "${CLOUDFLARE_API_TOKEN:-}" ]]; then
  echo "Set CLOUDFLARE_API_TOKEN (see infra/opentofu/state-bootstrap/README.md for required permissions)." >&2
  exit 1
fi

export CLOUDFLARE_API_TOKEN

ACCOUNT_ID="${CLOUDFLARE_ACCOUNT_ID:-$(doppler secrets get CLOUDFLARE_ACCOUNT_ID --plain -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}" 2>/dev/null || true)}"
preflight_cloudflare_account_id "${ACCOUNT_ID}"

preflight_cloudflare_api_token "${CLOUDFLARE_API_TOKEN}" "${ACCOUNT_ID}"

preflight_doppler_secret_exists GITHUB_TOFU_RELEASE_ENV "${DOPPLER_PROJECT}" "${DOPPLER_CONFIG}"

export TF_VAR_cloudflare_account_id="${ACCOUNT_ID}"
export TF_VAR_state_bucket_name="${STATE_BUCKET_DEFAULT}"

if ! doppler secrets get TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE --plain -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}" >/dev/null 2>&1; then
  BOOTSTRAP_PASS="$(openssl rand -base64 24)"
  confirm "Create TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE in Doppler?"
  doppler_secret_set_stdin TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE "${BOOTSTRAP_PASS}"
fi
export TF_VAR_bootstrap_state_encryption_passphrase
TF_VAR_bootstrap_state_encryption_passphrase="$(doppler secrets get TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE --plain -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}")"

if ! doppler secrets get TOFU_STATE_ENCRYPTION_PASSPHRASE --plain -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}" >/dev/null 2>&1; then
  RELEASE_PASS="$(openssl rand -base64 24)"
  confirm "Create TOFU_STATE_ENCRYPTION_PASSPHRASE in Doppler?"
  doppler_secret_set_stdin TOFU_STATE_ENCRYPTION_PASSPHRASE "${RELEASE_PASS}"
fi

write_bootstrap_backend_config_from_env() {
  cat > "${BOOTSTRAP_BACKEND_CONFIG}" <<EOF
bucket                      = "${TOFU_STATE_BUCKET}"
key                         = "${TOFU_BOOTSTRAP_STATE_KEY}"
region                      = "${TOFU_STATE_REGION:-auto}"
endpoint                    = "${TOFU_STATE_ENDPOINT}"
encrypt                     = true
use_lockfile                = true
skip_credentials_validation = true
skip_region_validation      = true
skip_requesting_account_id  = true
EOF
}

bootstrap_backend_is_remote() {
  [[ -f "${BOOTSTRAP_DIR}/.terraform/terraform.tfstate" ]] \
    && grep -q '"type"[[:space:]]*:[[:space:]]*"s3"' "${BOOTSTRAP_DIR}/.terraform/terraform.tfstate"
}

bootstrap_init() {
  cd "${BOOTSTRAP_DIR}"
  if doppler secrets get TOFU_BOOTSTRAP_STATE_KEY --plain -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}" >/dev/null 2>&1 \
    && doppler secrets get TOFU_STATE_BUCKET --plain -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}" >/dev/null 2>&1; then
    bootstrap_versions_use_s3 "${BOOTSTRAP_DIR}"
    doppler run -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}" -- bash -c "
      write_bootstrap_backend_config_from_env() {
        cat > \"${BOOTSTRAP_BACKEND_CONFIG}\" <<EOF
bucket                      = \"\${TOFU_STATE_BUCKET}\"
key                         = \"\${TOFU_BOOTSTRAP_STATE_KEY}\"
region                      = \"\${TOFU_STATE_REGION:-auto}\"
endpoint                    = \"\${TOFU_STATE_ENDPOINT}\"
encrypt                     = true
use_lockfile                = true
skip_credentials_validation = true
skip_region_validation      = true
skip_requesting_account_id  = true
EOF
      }
      write_bootstrap_backend_config_from_env
      tofu init -input=false -backend-config=\"${BOOTSTRAP_BACKEND_CONFIG}\" -reconfigure
    "
  else
    bootstrap_versions_use_local "${BOOTSTRAP_DIR}"
    tofu init -input=false
  fi
}

migrate_bootstrap_state_to_r2() {
  local bucket=$1 endpoint=$2 access_key=$3 secret_key=$4
  cd "${BOOTSTRAP_DIR}"
  if bootstrap_backend_is_remote; then
    echo "Bootstrap state already uses the R2 backend."
    return 0
  fi
  if [[ ! -f "${BOOTSTRAP_DIR}/terraform.tfstate" ]]; then
    echo "No local bootstrap state to migrate." >&2
    return 1
  fi

  confirm "Migrate bootstrap OpenTofu state to R2 (${BOOTSTRAP_STATE_KEY})?"

  doppler_secret_set_stdin TOFU_BOOTSTRAP_STATE_KEY "${BOOTSTRAP_STATE_KEY}"
  doppler_secret_set_stdin TOFU_STATE_BUCKET "${bucket}"
  doppler_secret_set_stdin TOFU_STATE_ENDPOINT "${endpoint}"
  doppler_secret_set_stdin TOFU_STATE_REGION "auto"
  doppler_secret_set_stdin AWS_ACCESS_KEY_ID "${access_key}"
  doppler_secret_set_stdin AWS_SECRET_ACCESS_KEY "${secret_key}"

  export TOFU_STATE_BUCKET="${bucket}"
  export TOFU_BOOTSTRAP_STATE_KEY="${BOOTSTRAP_STATE_KEY}"
  export TOFU_STATE_ENDPOINT="${endpoint}"
  export TOFU_STATE_REGION="auto"
  export AWS_ACCESS_KEY_ID="${access_key}"
  export AWS_SECRET_ACCESS_KEY="${secret_key}"
  write_bootstrap_backend_config_from_env

  export TF_VAR_bootstrap_state_encryption_passphrase
  TF_VAR_bootstrap_state_encryption_passphrase="$(doppler secrets get TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE --plain -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}")"

  bootstrap_versions_use_s3 "${BOOTSTRAP_DIR}"
  tofu init -input=false -migrate-state -backend-config="${BOOTSTRAP_BACKEND_CONFIG}" -reconfigure
  echo "Bootstrap state migrated to R2."
}

echo "==> Bootstrap stack (R2 bucket + scoped state credentials)"
confirm "Run tofu init/plan/apply for state-bootstrap (creates R2 bucket and API token)?"

bootstrap_init

tofu plan -input=false -out="${BOOTSTRAP_PLAN}"
confirm "Apply bootstrap plan?"
tofu apply -input=false "${BOOTSTRAP_PLAN}"

BUCKET="$(tofu output -raw state_bucket_name)"
if [[ "${BUCKET}" != "${STATE_BUCKET_DEFAULT}" ]]; then
  echo "Unexpected bucket name ${BUCKET}; expected ${STATE_BUCKET_DEFAULT}." >&2
  exit 1
fi
ENDPOINT="$(tofu output -raw r2_s3_endpoint)"
ACCESS_KEY="$(tofu output -raw tofu_state_access_key_id)"
SECRET_KEY="$(tofu output -raw tofu_state_secret_access_key)"

migrate_bootstrap_state_to_r2 "${BUCKET}" "${ENDPOINT}" "${ACCESS_KEY}" "${SECRET_KEY}"

confirm "Write R2 backend settings for release-environment to Doppler (${DOPPLER_PROJECT}/${DOPPLER_CONFIG})?"
doppler_secret_set_stdin TOFU_STATE_BUCKET "${BUCKET}"
doppler_secret_set_stdin TOFU_STATE_ENDPOINT "${ENDPOINT}"
doppler_secret_set_stdin TOFU_STATE_REGION "auto"
doppler_secret_set_stdin TOFU_STATE_KEY "${RELEASE_STATE_KEY}"
doppler_secret_set_stdin AWS_ACCESS_KEY_ID "${ACCESS_KEY}"
doppler_secret_set_stdin AWS_SECRET_ACCESS_KEY "${SECRET_KEY}"

BACKEND_CONFIG="${RELEASE_DIR}/backend-config.hcl"
confirm "Write ${BACKEND_CONFIG} from Doppler (gitignored)?"
doppler run -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}" -- bash -c "cat > \"${BACKEND_CONFIG}\" <<EOF
bucket                      = \"\${TOFU_STATE_BUCKET}\"
key                         = \"\${TOFU_STATE_KEY}\"
region                      = \"\${TOFU_STATE_REGION}\"
endpoint                    = \"\${TOFU_STATE_ENDPOINT}\"
encrypt                     = true
use_lockfile                = true
skip_credentials_validation = true
skip_region_validation      = true
skip_requesting_account_id  = true
EOF"

echo "==> Release environment stack"
confirm "Run tofu init/plan/apply for release-environment (GitHub environment)?"

cd "${RELEASE_DIR}"
export TF_VAR_state_encryption_passphrase
TF_VAR_state_encryption_passphrase="$(doppler secrets get TOFU_STATE_ENCRYPTION_PASSPHRASE --plain -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}")"
export GITHUB_TOKEN
GITHUB_TOKEN="$(doppler secrets get GITHUB_TOFU_RELEASE_ENV --plain -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}")"

doppler run -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}" -- bash -c "
  tofu init -input=false -backend-config=backend-config.hcl -reconfigure
  tofu plan -input=false -out=\"${RELEASE_PLAN}\"
"
confirm "Apply release-environment plan?"
doppler run -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}" -- bash -c "
  tofu apply -input=false \"${RELEASE_PLAN}\"
"

echo "Done. Release environment is ready; OpenTofu-managed reviewers are configured."
