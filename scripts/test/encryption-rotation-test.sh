#!/usr/bin/env bash
# Real encrypted state: two-hop TF_ENCRYPTION rotation; metadata stays on original provider name.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FIXTURE="${ROOT}/infra/opentofu/encryption-rotation-fixture"
ROTATE="${ROOT}/scripts/tofu-rotate-encryption-passphrase.sh"
if command -v tofu-1.13.1 >/dev/null 2>&1; then
  export TOFU=tofu-1.13.1
else
  export TOFU=tofu
fi
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

PASS_A='rotation-test-passphrase-a16'
PASS_B='rotation-test-passphrase-b16'
PASS_C='rotation-test-passphrase-c16'

cp "${FIXTURE}/variables.tf" "${FIXTURE}/main.tf" "${FIXTURE}/versions.tf" "${WORKDIR}/"

(
  cd "${WORKDIR}"
  export TF_VAR_passphrase="${PASS_A}"
  "${TOFU}" init -input=false >/dev/null
  "${TOFU}" apply -input=false -auto-approve >/dev/null
)

"${ROTATE}" "${WORKDIR}" bootstrap "${PASS_A}" "${PASS_B}"

(
  cd "${WORKDIR}"
  export TF_VAR_passphrase="${PASS_B}"
  "${TOFU}" init -input=false >/dev/null
  if ! "${TOFU}" plan -input=false >/dev/null; then
    echo "FAIL: plan with new passphrase after rotation should succeed" >&2
    exit 1
  fi
)

(
  cd "${WORKDIR}"
  export TF_VAR_passphrase="${PASS_A}"
  if "${TOFU}" plan -input=false >/dev/null 2>&1; then
    echo "FAIL: plan with old passphrase after rotation should fail" >&2
    exit 1
  fi
)

# shellcheck source=scripts/lib/tofu-encryption-metadata.sh
source "${ROOT}/scripts/lib/tofu-encryption-metadata.sh"
if ! tofu_assert_state_method_metadata "${WORKDIR}/terraform.tfstate" bootstrap; then
  exit 1
fi
echo "OK: state metadata on key_provider.pbkdf2.bootstrap after first rotation"

"${ROTATE}" "${WORKDIR}" bootstrap "${PASS_B}" "${PASS_C}"

(
  cd "${WORKDIR}"
  export TF_VAR_passphrase="${PASS_C}"
  "${TOFU}" init -input=false >/dev/null
  "${TOFU}" plan -input=false >/dev/null
)

(
  cd "${WORKDIR}"
  export TF_VAR_passphrase="${PASS_B}"
  if "${TOFU}" plan -input=false >/dev/null 2>&1; then
    echo "FAIL: plan with superseded passphrase should fail after second rotation" >&2
    exit 1
  fi
)

if ! tofu_assert_state_method_metadata "${WORKDIR}/terraform.tfstate" bootstrap; then
  exit 1
fi

echo "OK: TF_ENCRYPTION two-hop rotation; plan new succeeds, old fails; metadata stays on bootstrap"
