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

NEW_IN_DOPPLER="$(rotation_doppler_value "${DB}" TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE)"
if [[ "${NEW_IN_DOPPLER}" == "${PASS_A}" ]]; then
  echo "FAIL: Doppler secret was not updated after rotation" >&2
  exit 1
fi

# The Doppler copy of the local state must be the re-encrypted state, written before promotion.
assert_backup_before_promote() {
  local label=$1 got raw hash backup_line promote_line
  got="$(rotation_doppler_value "${DB}" TOFU_BOOTSTRAP_STATE_LOCAL_BACKUP)"
  raw="$(cat "${WORKDIR}/terraform.tfstate")"
  hash="sha256:$(sha256sum "${WORKDIR}/terraform.tfstate" | cut -d' ' -f1)"
  if [[ -z "${got}" || ( "${got}" != "${raw}" && "${got}" != "${hash}" ) ]]; then
    echo "FAIL: ${label}: the Doppler state backup is not the re-encrypted state" >&2
    exit 1
  fi
  backup_line="$(grep -n '^set TOFU_BOOTSTRAP_STATE_LOCAL_BACKUP ' "${DB}.writes" | tail -1 | cut -d: -f1)"
  promote_line="$(grep -n '^set TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE ' "${DB}.writes" | tail -1 | cut -d: -f1)"
  if [[ -z "${backup_line}" || -z "${promote_line}" || ${backup_line} -gt ${promote_line} ]]; then
    echo "FAIL: ${label}: the state backup was not written before the passphrase was promoted" >&2
    exit 1
  fi
  echo "OK: ${label}: state backed up to Doppler before promotion"
}
assert_backup_before_promote "first rotation"

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

"${ROTATE}" "${WORKDIR}" bootstrap

FINAL="$(rotation_doppler_value "${DB}" TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE)"
if [[ "${FINAL}" == "${NEW_IN_DOPPLER}" ]]; then
  echo "FAIL: second rotation did not change Doppler secret" >&2
  exit 1
fi
assert_backup_before_promote "second rotation"

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
