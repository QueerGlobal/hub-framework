#!/usr/bin/env bash
# Two-hop passphrase rotation via TF_ENCRYPTION (no tracked .tf changes).
# Keeps state metadata on the original key_provider name (bootstrap / state).
# Requires OpenTofu 1.13.1+ (refresh-only re-encryption with TF_ENCRYPTION).
set -euo pipefail

if [[ -z "${TOFU:-}" ]]; then
  if command -v tofu-1.13.1 >/dev/null 2>&1; then
    TOFU=tofu-1.13.1
  else
    TOFU=tofu
  fi
fi
export TOFU

usage() {
  echo "Usage: TOFU_ROTATE_OLD_PASSPHRASE=... TOFU_ROTATE_NEW_PASSPHRASE=... $0 <stack_dir> <provider_name>" >&2
  echo "  provider_name: bootstrap (state-bootstrap) or state (release-environment)" >&2
  exit 1
}

[[ $# -eq 2 ]] || usage

STACK_DIR=$1
PROVIDER_NAME=$2
OLD_PASS=${TOFU_ROTATE_OLD_PASSPHRASE:?}
NEW_PASS=${TOFU_ROTATE_NEW_PASSPHRASE:?}

if [[ ${#OLD_PASS} -lt 16 || ${#NEW_PASS} -lt 16 ]]; then
  echo "Passphrases must be at least 16 characters." >&2
  exit 1
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
