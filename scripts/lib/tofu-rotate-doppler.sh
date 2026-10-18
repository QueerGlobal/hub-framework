# shellcheck shell=bash
# Doppler helpers for passphrase rotation (values never on argv).

# shellcheck source=scripts/lib/doppler-configs.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/doppler-configs.sh"
# shellcheck source=scripts/lib/doppler-secrets.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/doppler-secrets.sh"

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
}

tofu_rotate_doppler_get_plain() {
  local name=$1 project=$2 config=$3
  local err_file out rc=0
  err_file="$(mktemp)"
  # shellcheck disable=SC2064
  trap "rm -f '${err_file}'" RETURN
  out="$(doppler secrets get "${name}" --plain -p "${project}" -c "${config}" 2>"${err_file}")" || rc=$?
  if [[ ${rc} -eq 0 ]]; then
    printf '%s' "${out}"
    return 0
  fi
  if grep -Eiq 'could not find|not find|not found|does not exist|404' "${err_file}"; then
    return 2
  fi
  cat "${err_file}" >&2
  return 1
}

tofu_rotate_doppler_secret_empty_or_missing() {
  local name=$1 project=$2 config=$3
  local value rc=0
  value="$(tofu_rotate_doppler_get_plain "${name}" "${project}" "${config}")" || rc=$?
  if [[ ${rc} -eq 2 ]]; then
    return 0
  fi
  if [[ ${rc} -ne 0 ]]; then
    return "${rc}"
  fi
  [[ -z "${value}" ]]
}

tofu_rotate_generate_passphrase() {
  local pass
  pass="$(openssl rand -base64 32)"
  if [[ ${#pass} -lt 16 ]]; then
    echo "Generated passphrase was too short." >&2
    return 1
  fi
  printf '%s' "${pass}"
}

tofu_rotate_assert_old_passphrase_fails() {
  local stack_dir=$1 provider_name=$2 old_pass=$3 tofu_bin=$4
  if [[ -n "${TOFU_ROTATE_ASSERT_RAN:-}" ]]; then
    : >"${TOFU_ROTATE_ASSERT_RAN}"
  fi
  (
    cd "${stack_dir}"
    unset TF_ENCRYPTION
    case "${provider_name}" in
      bootstrap)
        export TF_VAR_passphrase="${old_pass}"
        export TF_VAR_bootstrap_state_encryption_passphrase="${old_pass}"
        ;;
      state)
        export TF_VAR_state_encryption_passphrase="${old_pass}"
        ;;
    esac
    if "${tofu_bin}" plan -input=false >/dev/null 2>&1; then
      echo "Old passphrase must not decrypt state after rotation." >&2
      exit 1
    fi
  )
}
