#!/usr/bin/env bash
# Proves passphrase rotation: fallback keeps original key_provider name; primary uses new passphrase.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FIXTURE="${ROOT}/infra/opentofu/encryption-rotation-fixture"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

LEGACY='rotation-test-legacy-passphrase-16'
PRIMARY='rotation-test-primary-passphrase-16'

cp "${FIXTURE}/variables.tf" "${FIXTURE}/main.tf" "${WORKDIR}/"
cp "${FIXTURE}/versions.initial.tf" "${WORKDIR}/versions.tf"

(
  cd "${WORKDIR}"
  tofu init -input=false >/dev/null
  export TF_VAR_passphrase_legacy="${LEGACY}"
  export TF_VAR_passphrase_primary="${PRIMARY}"
  tofu apply -input=false -auto-approve >/dev/null
)

cp "${FIXTURE}/versions.rotated.tf" "${WORKDIR}/versions.tf"

(
  cd "${WORKDIR}"
  export TF_VAR_passphrase_legacy="${LEGACY}"
  export TF_VAR_passphrase_primary="${PRIMARY}"
  tofu init -input=false >/dev/null
  tofu apply -input=false -refresh-only -auto-approve >/dev/null
  tofu state list >/dev/null
)

cp "${FIXTURE}/versions.initial.tf" "${WORKDIR}/versions.tf"
legacy_only_reads_state=false
if (
  cd "${WORKDIR}"
  export TF_VAR_passphrase_legacy="${LEGACY}"
  export TF_VAR_passphrase_primary="${PRIMARY}"
  tofu init -input=false >/dev/null
  tofu state list >/dev/null 2>&1
); then
  legacy_only_reads_state=true
fi
if [[ "${legacy_only_reads_state}" == true ]]; then
  echo "FAIL: legacy-only encryption config still reads state after rotation to bootstrap_primary" >&2
  exit 1
fi

echo "OK: encryption rotation reads state with new primary and rejects wrong primary after re-encrypt"
