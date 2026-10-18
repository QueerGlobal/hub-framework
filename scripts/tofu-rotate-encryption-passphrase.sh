#!/usr/bin/env bash
# Two-hop passphrase rotation via TF_ENCRYPTION (no tracked .tf changes).
# Reads the current passphrase from Doppler, stages a generated one as _NEXT, rotates,
# verifies the state was re-keyed, backs the bootstrap state up, then promotes _NEXT.
# A run that stops part way is resumed by running the script again: it reuses the staged _NEXT.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lib/secret-shell-guard.sh
source "${ROOT}/scripts/lib/secret-shell-guard.sh"
secret_shell_refuse_xtrace
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
if ! OLD_PASS="$(doppler_secret_read "${TOFU_ROTATE_SECRET_NAME}" \
  "${TOFU_ROTATE_DOPPLER_PROJECT}" "${TOFU_ROTATE_DOPPLER_CONFIG}")"; then
  echo "Doppler error while reading ${TOFU_ROTATE_SECRET_NAME} from ${TOFU_ROTATE_DOPPLER_PROJECT}/${TOFU_ROTATE_DOPPLER_CONFIG}; refusing to rotate." >&2
  exit 1
fi
if [[ -z "${OLD_PASS}" ]]; then
  echo "Missing current passphrase ${TOFU_ROTATE_SECRET_NAME} in Doppler ${TOFU_ROTATE_DOPPLER_PROJECT}/${TOFU_ROTATE_DOPPLER_CONFIG}." >&2
  exit 1
fi
if [[ ${#OLD_PASS} -lt 16 ]]; then
  echo "Current passphrase in Doppler is too short." >&2
  exit 1
fi

if ! NEW_PASS="$(doppler_secret_read "${TOFU_ROTATE_NEXT_SECRET_NAME}" \
  "${TOFU_ROTATE_DOPPLER_PROJECT}" "${TOFU_ROTATE_DOPPLER_CONFIG}")"; then
  echo "Doppler error while reading ${TOFU_ROTATE_NEXT_SECRET_NAME} from ${TOFU_ROTATE_DOPPLER_PROJECT}/${TOFU_ROTATE_DOPPLER_CONFIG}; refusing to rotate." >&2
  exit 1
fi
if [[ -n "${NEW_PASS}" ]]; then
  if [[ "${NEW_PASS}" == "${OLD_PASS}" ]]; then
    # A run that stopped after promoting: only the leftover _NEXT remains.
    tofu_rotate_delete_next
    echo "${TOFU_ROTATE_SECRET_NAME} was already promoted; removed the leftover ${TOFU_ROTATE_NEXT_SECRET_NAME}."
    exit 0
  fi
  echo "Resuming the rotation staged in ${TOFU_ROTATE_NEXT_SECRET_NAME}."
else
  NEW_PASS="$(secret_shell_generate_passphrase)" || exit 1
  doppler_secret_set_stdin "${TOFU_ROTATE_NEXT_SECRET_NAME}" "${NEW_PASS}" \
    "${TOFU_ROTATE_DOPPLER_PROJECT}" "${TOFU_ROTATE_DOPPLER_CONFIG}"
fi

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

cd "${STACK_DIR}"

if tofu_rotate_committed_config_decrypts "${STACK_DIR}" "${PROVIDER_NAME}" "${NEW_PASS}" "${TOFU}"; then
  echo "State already decrypts with ${TOFU_ROTATE_NEXT_SECRET_NAME}; skipping the two hops."
else
  export TF_ENCRYPTION
  tofu_rotate_export_passphrase_vars "${PROVIDER_NAME}" "${OLD_PASS}"
  TF_ENCRYPTION="$(hop1_tf_encryption)"
  "${TOFU}" apply -refresh-only -auto-approve -input=false

  tofu_rotate_export_passphrase_vars "${PROVIDER_NAME}" "${NEW_PASS}"
  TF_ENCRYPTION="$(hop2_tf_encryption)"
  "${TOFU}" apply -refresh-only -auto-approve -input=false

  unset TF_ENCRYPTION
fi

tofu_rotate_verify_rekeyed "${STACK_DIR}" "${PROVIDER_NAME}" "${OLD_PASS}" "${NEW_PASS}" "${TOFU}"
tofu_rotate_backup_state "${STACK_DIR}" "${PROVIDER_NAME}"

doppler_secret_set_stdin "${TOFU_ROTATE_SECRET_NAME}" "${NEW_PASS}" \
  "${TOFU_ROTATE_DOPPLER_PROJECT}" "${TOFU_ROTATE_DOPPLER_CONFIG}"
tofu_rotate_delete_next

echo "Passphrase rotation complete for ${PROVIDER_NAME} (Doppler ${TOFU_ROTATE_SECRET_NAME} updated)."
