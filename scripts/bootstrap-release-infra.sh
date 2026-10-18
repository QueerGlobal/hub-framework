#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BOOTSTRAP_DIR="${ROOT}/infra/opentofu/state-bootstrap"
RELEASE_DIR="${ROOT}/infra/opentofu/release-environment"
DOPPLER_PROJECT="${DOPPLER_PROJECT:-hub-framework}"
DOPPLER_CONFIG="${DOPPLER_CONFIG:-infra}"
STATE_BUCKET_DEFAULT="queerglobal-hub-framework-tofu-state"
RELEASE_STATE_KEY="hub-framework/release-environment/terraform.tfstate"

confirm() {
  local prompt=$1
  read -r -p "${prompt} [y/N] " reply
  case "${reply}" in
    y|Y|yes|YES) return 0 ;;
    *) echo "Aborted." >&2; exit 1 ;;
  esac
}

require_cmd() {
  for c in "$@"; do
    command -v "${c}" >/dev/null 2>&1 || { echo "Missing required command: ${c}" >&2; exit 1; }
  done
}

require_cmd tofu doppler jq

if [[ -z "${CLOUDFLARE_API_TOKEN:-}" ]]; then
  echo "Set CLOUDFLARE_API_TOKEN (R2 Admin Read & Write bootstrap token)." >&2
  exit 1
fi

export CLOUDFLARE_API_TOKEN

ACCOUNT_ID="${CLOUDFLARE_ACCOUNT_ID:-$(doppler secrets get CLOUDFLARE_ACCOUNT_ID --plain -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}" 2>/dev/null || true)}"
if [[ -z "${ACCOUNT_ID}" ]]; then
  echo "Set CLOUDFLARE_ACCOUNT_ID or store it in Doppler (hub-framework/infra)." >&2
  exit 1
fi

export TF_VAR_cloudflare_account_id="${ACCOUNT_ID}"

if ! doppler secrets get TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE --plain -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}" >/dev/null 2>&1; then
  BOOTSTRAP_PASS="$(openssl rand -base64 24)"
  confirm "Create TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE in Doppler?"
  doppler secrets set "TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE=${BOOTSTRAP_PASS}" -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}"
fi
export TF_VAR_bootstrap_state_encryption_passphrase
TF_VAR_bootstrap_state_encryption_passphrase="$(doppler secrets get TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE --plain -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}")"

if ! doppler secrets get TOFU_STATE_ENCRYPTION_PASSPHRASE --plain -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}" >/dev/null 2>&1; then
  RELEASE_PASS="$(openssl rand -base64 24)"
  confirm "Create TOFU_STATE_ENCRYPTION_PASSPHRASE in Doppler?"
  doppler secrets set "TOFU_STATE_ENCRYPTION_PASSPHRASE=${RELEASE_PASS}" -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}"
fi

echo "==> Bootstrap stack (R2 bucket + scoped state credentials)"
confirm "Run tofu init/plan/apply for state-bootstrap (creates R2 bucket and API token)?"

cd "${BOOTSTRAP_DIR}"
tofu init -input=false
tofu plan -input=false -out=bootstrap.tfplan
confirm "Apply bootstrap plan?"
tofu apply -input=false bootstrap.tfplan
rm -f bootstrap.tfplan

BUCKET="$(tofu output -raw state_bucket_name)"
ENDPOINT="$(tofu output -raw r2_s3_endpoint)"
ACCESS_KEY="$(tofu output -raw tofu_state_access_key_id)"
SECRET_KEY="$(tofu output -raw tofu_state_secret_access_key)"

confirm "Write R2 backend settings to Doppler (${DOPPLER_PROJECT}/${DOPPLER_CONFIG})?"
doppler secrets set \
  "TOFU_STATE_BUCKET=${BUCKET}" \
  "TOFU_STATE_ENDPOINT=${ENDPOINT}" \
  "TOFU_STATE_REGION=auto" \
  "TOFU_STATE_KEY=${RELEASE_STATE_KEY}" \
  "AWS_ACCESS_KEY_ID=${ACCESS_KEY}" \
  "AWS_SECRET_ACCESS_KEY=${SECRET_KEY}" \
  -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}"

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

doppler run -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}" -- bash -c '
  tofu init -input=false -backend-config=backend-config.hcl -reconfigure
  tofu plan -input=false -out=release.tfplan
'
confirm "Apply release-environment plan?"
doppler run -p "${DOPPLER_PROJECT}" -c "${DOPPLER_CONFIG}" -- bash -c '
  tofu apply -input=false release.tfplan
  rm -f release.tfplan
'

echo "Done. Release environment is ready; OpenTofu-managed reviewers are configured."
