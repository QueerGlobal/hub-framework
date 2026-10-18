#!/usr/bin/env bash
# Two-hop passphrase rotation via TF_ENCRYPTION (no tracked .tf changes).
# Reads the current passphrase from Doppler, writes a generated _NEXT secret, rotates, then promotes _NEXT.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lib/tofu-rotate-doppler.sh
source "${ROOT}/scripts/lib/tofu-rotate-doppler.sh"

if [[ -z "${TOFU:-}" ]]; then
  if command -v tofu-1.13.1 >/dev/null 2>&1; then
    TOFU=tofu-1.13.1
  else
    TOFU=tofu
  fi
fi
export TOFU

usage() {
  echo "Usage: $0 <stack_dir> <provider_name>" >&2
  echo "  provider_name: bootstrap (state-bootstrap) or state (release-environment)" >&2
  echo "  Current and _NEXT passphrases come from Doppler; nothing is read from the shell environment." >&2
  exit 1
}

[[ $# -eq 2 ]] || usage

STACK_DIR=$1
PROVIDER_NAME=$2

tofu_rotate_doppler_names "${PROVIDER_NAME}" || exit 1

OLD_PASS=""
NEW_PASS=""
rc=0
OLD_PASS="$(tofu_rotate_doppler_get_plain "${TOFU_ROTATE_SECRET_NAME}" \
  "${TOFU_ROTATE_DOPPLER_PROJECT}" "${TOFU_ROTATE_DOPPLER_CONFIG}")" || rc=$?
if [[ ${rc} -eq 2 || -z "${OLD_PASS}" ]]; then
  echo "Missing current passphrase ${TOFU_ROTATE_SECRET_NAME} in Doppler ${TOFU_ROTATE_DOPPLER_PROJECT}/${TOFU_ROTATE_DOPPLER_CONFIG}." >&2
  exit 1
fi
if [[ ${rc} -ne 0 ]]; then
  exit 1
fi
if [[ ${#OLD_PASS} -lt 16 ]]; then
  echo "Current passphrase in Doppler is too short." >&2
  exit 1
fi

rc=0
if ! tofu_rotate_doppler_secret_empty_or_missing "${TOFU_ROTATE_NEXT_SECRET_NAME}" \
  "${TOFU_ROTATE_DOPPLER_PROJECT}" "${TOFU_ROTATE_DOPPLER_CONFIG}"; then
  rc=$?
  if [[ ${rc} -ne 0 ]]; then
    echo "Doppler error while checking ${TOFU_ROTATE_NEXT_SECRET_NAME}; refusing to rotate." >&2
    exit 1
  fi
  echo "Refusing to overwrite existing ${TOFU_ROTATE_NEXT_SECRET_NAME} in Doppler." >&2
  exit 1
fi

NEW_PASS="$(tofu_rotate_generate_passphrase)" || exit 1
case "${NEW_PASS}" in
  fixed-mutant-passphrase-*)
    echo "Refusing deterministic passphrase for rotation." >&2
    exit 1
    ;;
esac
doppler_secret_set_stdin "${TOFU_ROTATE_NEXT_SECRET_NAME}" "${NEW_PASS}" \
  "${TOFU_ROTATE_DOPPLER_PROJECT}" "${TOFU_ROTATE_DOPPLER_CONFIG}"

hop1_tf_encryption() {
  cat <<EOF
key_provider "pbkdf2" "${PROVIDER_NAME}" {
  passphrase = "${OLD_PASS}"
}

key_provider "pbkdf2" "rotation" {
  passphrase = "${NEW_PASS}"
}

method "aes_gcm" "${PROVIDER_NAME}" {
  keys = key_provider.pbkdf2.${PROVIDER_NAME}
}

method "aes_gcm" "rotation" {
  keys = key_provider.pbkdf2.rotation
}

state {
  method = method.aes_gcm.rotation

  fallback {
    method = method.aes_gcm.${PROVIDER_NAME}
  }
}

plan {
  method = method.aes_gcm.rotation

  fallback {
    method = method.aes_gcm.${PROVIDER_NAME}
  }
}
EOF
}

hop2_tf_encryption() {
  cat <<EOF
key_provider "pbkdf2" "${PROVIDER_NAME}" {
  passphrase = "${NEW_PASS}"
}

key_provider "pbkdf2" "rotation" {
  passphrase = "${NEW_PASS}"
}

method "aes_gcm" "${PROVIDER_NAME}" {
  keys = key_provider.pbkdf2.${PROVIDER_NAME}
}

method "aes_gcm" "rotation" {
  keys = key_provider.pbkdf2.rotation
}

state {
  method = method.aes_gcm.${PROVIDER_NAME}

  fallback {
    method = method.aes_gcm.rotation
  }
}

plan {
  method = method.aes_gcm.${PROVIDER_NAME}

  fallback {
    method = method.aes_gcm.rotation
  }
}
EOF
}

set_passphrase_tf_vars() {
  local pass=$1
  case "${PROVIDER_NAME}" in
    bootstrap)
      export TF_VAR_passphrase="${pass}"
      export TF_VAR_bootstrap_state_encryption_passphrase="${pass}"
      ;;
    state)
      export TF_VAR_state_encryption_passphrase="${pass}"
      ;;
    *)
      echo "Unknown provider_name: ${PROVIDER_NAME}" >&2
      return 1
      ;;
  esac
}

cd "${STACK_DIR}"

export TF_ENCRYPTION
set_passphrase_tf_vars "${OLD_PASS}"
TF_ENCRYPTION="$(hop1_tf_encryption)"
"${TOFU}" apply -refresh-only -auto-approve -input=false

set_passphrase_tf_vars "${NEW_PASS}"
TF_ENCRYPTION="$(hop2_tf_encryption)"
"${TOFU}" apply -refresh-only -auto-approve -input=false

unset TF_ENCRYPTION

tofu_rotate_assert_old_passphrase_fails "${STACK_DIR}" "${PROVIDER_NAME}" "${OLD_PASS}" "${TOFU}"

doppler_secret_set_stdin "${TOFU_ROTATE_SECRET_NAME}" "${NEW_PASS}" \
  "${TOFU_ROTATE_DOPPLER_PROJECT}" "${TOFU_ROTATE_DOPPLER_CONFIG}"
doppler secrets delete "${TOFU_ROTATE_NEXT_SECRET_NAME}" \
  -p "${TOFU_ROTATE_DOPPLER_PROJECT}" -c "${TOFU_ROTATE_DOPPLER_CONFIG}" --silent --no-interactive

echo "Passphrase rotation complete for ${PROVIDER_NAME} (Doppler ${TOFU_ROTATE_SECRET_NAME} updated)."
