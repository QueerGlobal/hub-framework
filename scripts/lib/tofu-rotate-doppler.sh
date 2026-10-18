# shellcheck shell=bash
# Helpers for passphrase rotation. Values stay in variables and on stdin, never on argv.

_TOFU_ROTATE_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib/doppler-configs.sh
source "${_TOFU_ROTATE_LIB_DIR}/doppler-configs.sh"
# shellcheck source=scripts/lib/doppler-secrets.sh
source "${_TOFU_ROTATE_LIB_DIR}/doppler-secrets.sh"
# shellcheck source=scripts/lib/doppler-secret-read.sh
source "${_TOFU_ROTATE_LIB_DIR}/doppler-secret-read.sh"
# shellcheck source=scripts/lib/bootstrap-backend-mode.sh
source "${_TOFU_ROTATE_LIB_DIR}/bootstrap-backend-mode.sh"

tofu_rotate_doppler_names() {
  local provider_name=$1
  case "${provider_name}" in
    bootstrap)
      TOFU_ROTATE_SECRET_NAME=TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE
      TOFU_ROTATE_NEXT_SECRET_NAME=TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE_NEXT
      TOFU_ROTATE_DOPPLER_PROJECT="${DOPPLER_PROJECT_NAME}"
      TOFU_ROTATE_DOPPLER_CONFIG="${DOPPLER_BOOTSTRAP_ADMIN_CONFIG}"
      ;;
    state)
      TOFU_ROTATE_SECRET_NAME=TOFU_STATE_ENCRYPTION_PASSPHRASE
      TOFU_ROTATE_NEXT_SECRET_NAME=TOFU_STATE_ENCRYPTION_PASSPHRASE_NEXT
      TOFU_ROTATE_DOPPLER_PROJECT="${DOPPLER_PROJECT_NAME}"
      TOFU_ROTATE_DOPPLER_CONFIG="${DOPPLER_INFRA_CONFIG}"
      ;;
    *)
      echo "Unknown provider_name: ${provider_name}" >&2
      return 1
      ;;
  esac
  export TOFU_ROTATE_SECRET_NAME TOFU_ROTATE_NEXT_SECRET_NAME \
    TOFU_ROTATE_DOPPLER_PROJECT TOFU_ROTATE_DOPPLER_CONFIG
}

# Exports the TF_VAR_* passphrase variables the committed config reads for this provider.
tofu_rotate_export_passphrase_vars() {
  local provider_name=$1
  local pass=$2
  case "${provider_name}" in
    bootstrap)
      export TF_VAR_passphrase="${pass}"
      export TF_VAR_bootstrap_state_encryption_passphrase="${pass}"
      ;;
    state)
      export TF_VAR_state_encryption_passphrase="${pass}"
      ;;
    *)
      echo "Unknown provider_name: ${provider_name}" >&2
      return 1
      ;;
  esac
}

# Read-only plan with the committed encryption config (no TF_ENCRYPTION) and one passphrase.
# Writes OpenTofu's stderr to err_file and returns the plan exit code.
tofu_rotate_plan_with() {
  local stack_dir=$1
  local provider_name=$2
  local pass=$3
  local tofu_bin=$4
  local err_file=$5
  (
    cd "${stack_dir}" || exit 1
    unset TF_ENCRYPTION
    tofu_rotate_export_passphrase_vars "${provider_name}" "${pass}" || exit 1
    "${tofu_bin}" plan -input=false -lock=false -refresh=false -no-color >/dev/null 2>"${err_file}"
  )
}

# Returns 0 when the committed config decrypts state with this passphrase.
tofu_rotate_committed_config_decrypts() {
  local err_file rc=0
  err_file="$(mktemp)"
  tofu_rotate_plan_with "$1" "$2" "$3" "$4" "${err_file}" || rc=$?
  rm -f "${err_file}"
  return "${rc}"
}

# Proves the rotation took before anything is promoted: the committed config decrypts state
# with the new passphrase, and the old passphrase now fails with a decryption error.
tofu_rotate_verify_rekeyed() {
  local stack_dir=$1
  local provider_name=$2
  local old_pass=$3
  local new_pass=$4
  local tofu_bin=$5
  local err_file rc=0
  if ! tofu_rotate_committed_config_decrypts "${stack_dir}" "${provider_name}" "${new_pass}" "${tofu_bin}"; then
    echo "The committed config cannot decrypt state with the new passphrase; not promoting." >&2
    return 1
  fi
  err_file="$(mktemp)"
  tofu_rotate_plan_with "${stack_dir}" "${provider_name}" "${old_pass}" "${tofu_bin}" "${err_file}" || rc=$?
  if [[ ${rc} -eq 0 ]]; then
    rm -f "${err_file}"
    echo "Old passphrase must not decrypt state after rotation; not promoting." >&2
    return 1
  fi
  if ! grep -q 'decryption failed' "${err_file}"; then
    rm -f "${err_file}"
    echo "The old passphrase failed for a reason other than decryption; not promoting." >&2
    return 1
  fi
  rm -f "${err_file}"
}

# The bootstrap stack keeps local state with an encrypted copy in Doppler. That copy must be
# re-encrypted before the old passphrase is retired, or it could never be decrypted again.
tofu_rotate_backup_state() {
  local stack_dir=$1
  local provider_name=$2
  if [[ "${provider_name}" != bootstrap ]]; then
    return 0
  fi
  bootstrap_backup_local_state_to_doppler "${stack_dir}/terraform.tfstate" \
    "${TOFU_ROTATE_DOPPLER_PROJECT}" "${TOFU_ROTATE_DOPPLER_CONFIG}"
}

tofu_rotate_delete_next() {
  doppler secrets delete "${TOFU_ROTATE_NEXT_SECRET_NAME}" \
    -p "${TOFU_ROTATE_DOPPLER_PROJECT}" -c "${TOFU_ROTATE_DOPPLER_CONFIG}" --silent --no-interactive
}
