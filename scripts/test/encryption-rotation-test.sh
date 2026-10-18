#!/usr/bin/env bash
# Real encrypted state: Doppler-driven rotation with generated _NEXT; metadata stays on original provider name.
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
FAKE_BIN="${WORKDIR}/bin"
DB="${WORKDIR}/doppler.tsv"
trap 'rm -rf "${WORKDIR}"' EXIT
mkdir -p "${FAKE_BIN}"
: >"${DB}"

# shellcheck source=scripts/test/lib/rotation-doppler-mock.sh
source "${ROOT}/scripts/test/lib/rotation-doppler-mock.sh"
rotation_doppler_mock_install "${FAKE_BIN}" "${DB}"
export PATH="${FAKE_BIN}:${PATH}"
export ROTATION_DOPPLER_DB="${DB}"
export DOPPLER_PROJECT=hub-framework
export DOPPLER_CONFIG=bootstrap-admin

PASS_A='rotation-test-passphrase-a16'

rotation_doppler_seed "${DB}" TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE hub-framework bootstrap-admin "${PASS_A}"

cp "${FIXTURE}/variables.tf" "${FIXTURE}/main.tf" "${FIXTURE}/versions.tf" "${WORKDIR}/"

(
  cd "${WORKDIR}"
  export TF_VAR_passphrase="${PASS_A}"
  "${TOFU}" init -input=false >/dev/null
  "${TOFU}" apply -input=false -auto-approve >/dev/null
)

"${ROTATE}" "${WORKDIR}" bootstrap

if ! grep -q $'\tTOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE\t' "${DB}"; then
  echo "FAIL: rotation did not promote passphrase in Doppler mock" >&2
  exit 1
fi
if grep -q $'\tTOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE_NEXT\t' "${DB}"; then
  echo "FAIL: _NEXT secret should be removed after promotion" >&2
  exit 1
fi

NEW_IN_DOPPLER="$(awk -F '\t' '$2 == "TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE" { v=$5 } END { print v }' "${DB}")"
if [[ "${NEW_IN_DOPPLER}" == "${PASS_A}" ]]; then
  echo "FAIL: Doppler secret was not updated after rotation" >&2
  exit 1
fi

(
  cd "${WORKDIR}"
  export TF_VAR_passphrase="${NEW_IN_DOPPLER}"
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

rotation_doppler_seed "${DB}" TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE hub-framework bootstrap-admin "${NEW_IN_DOPPLER}"

"${ROTATE}" "${WORKDIR}" bootstrap

FINAL="$(awk -F '\t' '$2 == "TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE" { v=$5 } END { print v }' "${DB}")"
if [[ "${FINAL}" == "${NEW_IN_DOPPLER}" ]]; then
  echo "FAIL: second rotation did not change Doppler secret" >&2
  exit 1
fi

(
  cd "${WORKDIR}"
  export TF_VAR_passphrase="${FINAL}"
  "${TOFU}" init -input=false >/dev/null
  "${TOFU}" plan -input=false >/dev/null
)

(
  cd "${WORKDIR}"
  export TF_VAR_passphrase="${NEW_IN_DOPPLER}"
  if "${TOFU}" plan -input=false >/dev/null 2>&1; then
    echo "FAIL: plan with superseded passphrase should fail after second rotation" >&2
    exit 1
  fi
)

if ! tofu_assert_state_method_metadata "${WORKDIR}/terraform.tfstate" bootstrap; then
  exit 1
fi

echo "OK: TF_ENCRYPTION two-hop rotation; plan new succeeds, old fails; metadata stays on bootstrap"
