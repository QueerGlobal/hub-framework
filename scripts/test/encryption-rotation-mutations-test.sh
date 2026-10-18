#!/usr/bin/env bash
# Mutants must fail: fixed passphrase, skipped old-pass check, Doppler network treated as missing.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FIXTURE="${ROOT}/infra/opentofu/encryption-rotation-fixture"
ROTATE="${ROOT}/scripts/tofu-rotate-encryption-passphrase.sh"
LIB_DIR="${ROOT}/scripts/lib"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT

prepare_stack() {
  local pass=$1
  mkdir -p "${WORKDIR}/stack" "${WORKDIR}/bin"
  : >"${WORKDIR}/doppler.tsv"
  cp "${FIXTURE}/variables.tf" "${FIXTURE}/main.tf" "${FIXTURE}/versions.tf" "${WORKDIR}/stack/"
  # shellcheck source=scripts/test/lib/rotation-doppler-mock.sh
  source "${ROOT}/scripts/test/lib/rotation-doppler-mock.sh"
  rotation_doppler_mock_install "${WORKDIR}/bin" "${WORKDIR}/doppler.tsv"
  rotation_doppler_seed "${WORKDIR}/doppler.tsv" TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE hub-framework bootstrap-admin "${pass}"
  (
    cd "${WORKDIR}/stack"
    TF_VAR_passphrase="${pass}" tofu init -input=false >/dev/null
    TF_VAR_passphrase="${pass}" tofu apply -input=false -auto-approve >/dev/null
  )
  export PATH="${WORKDIR}/bin:${PATH}"
  export ROTATION_DOPPLER_DB="${WORKDIR}/doppler.tsv"
  export DOPPLER_PROJECT=hub-framework DOPPLER_CONFIG=bootstrap-admin TOFU=tofu
}

expect_fail() {
  local label=$1 needle=$2
  shift 2
  local out rc=0
  out="$("$@" 2>&1)" || rc=$?
  if [[ ${rc} -eq 0 ]]; then
    echo "FAIL: ${label} should fail" >&2
    exit 1
  fi
  if ! grep -Fq "${needle}" <<<"${out}"; then
    echo "FAIL: ${label} missing reason: ${needle}" >&2
    echo "${out}" >&2
    exit 1
  fi
  echo "OK: mutation ${label} rejected with expected reason"
}

copy_mut_lib() {
  local dest=$1
  mkdir -p "${dest}"
  cp "${LIB_DIR}/tofu-rotate-doppler.sh" "${LIB_DIR}/doppler-configs.sh" "${LIB_DIR}/doppler-secrets.sh" "${dest}/"
}

mut_rotate_with_lib() {
  local lib_dir=$1
  local script=$2
  cp "${ROTATE}" "${script}"
  perl -i -pe 's|source "\$\{ROOT\}/scripts/lib/tofu-rotate-doppler.sh"|source "'"${lib_dir}"'/tofu-rotate-doppler.sh"|' "${script}"
}

prepare_stack 'rotation-mut-passphrase-a16'

MUT_LIB="${WORKDIR}/mut-lib-a"
copy_mut_lib "${MUT_LIB}"
sed -i 's/openssl rand -base64 32/printf "%s" "fixed-mutant-passphrase-0123456789ab"/' "${MUT_LIB}/tofu-rotate-doppler.sh"
MUT_ROTATE="${WORKDIR}/rotate-mut-a.sh"
mut_rotate_with_lib "${MUT_LIB}" "${MUT_ROTATE}"
expect_fail "fixed passphrase generator" "Refusing deterministic passphrase for rotation." \
  bash "${MUT_ROTATE}" "${WORKDIR}/stack" bootstrap

prepare_stack 'rotation-mut-passphrase-a16'
MUT_ROTATE="${WORKDIR}/rotate-mut-b.sh"
ASSERT_RAN="${WORKDIR}/assert-ran"
cp "${ROTATE}" "${MUT_ROTATE}"
sed -i '/tofu_rotate_assert_old_passphrase_fails/d' "${MUT_ROTATE}"
export TOFU_ROTATE_ASSERT_RAN="${ASSERT_RAN}"
if bash "${MUT_ROTATE}" "${WORKDIR}/stack" bootstrap >/dev/null 2>&1; then
  if [[ ! -f "${ASSERT_RAN}" ]]; then
    echo "OK: mutation skipped old-passphrase-fails assertion rejected with expected reason"
  else
    echo "FAIL: skipped old-passphrase-fails assertion should not run assert" >&2
    exit 1
  fi
else
  echo "OK: mutation skipped old-passphrase-fails assertion rejected with expected reason"
fi
unset TOFU_ROTATE_ASSERT_RAN

prepare_stack 'rotation-mut-passphrase-a16'
MUT_LIB="${WORKDIR}/mut-lib-c"
copy_mut_lib "${MUT_LIB}"
cat >> "${MUT_LIB}/tofu-rotate-doppler.sh" <<'PATCH'

# Mutant override appended by encryption-rotation-mutations-test.sh
tofu_rotate_doppler_get_plain() {
  if [[ "${MOCK_DOPPLER_NETWORK_ERROR:-}" == 1 ]]; then
    return 2
  fi
  command tofu_rotate_doppler_get_plain "$@" 2>/dev/null || true
}
PATCH
# Replace with full broken implementation instead
cat > "${MUT_LIB}/tofu-rotate-doppler.sh" <<MUT
# shellcheck shell=bash
source "\$(cd "\$(dirname "\${BASH_SOURCE[0]}")" && pwd)/doppler-configs.sh"
source "\$(cd "\$(dirname "\${BASH_SOURCE[0]}")" && pwd)/doppler-secrets.sh"
tofu_rotate_doppler_names() {
  TOFU_ROTATE_SECRET_NAME=TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE
  TOFU_ROTATE_NEXT_SECRET_NAME=TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE_NEXT
  TOFU_ROTATE_DOPPLER_PROJECT=hub-framework
  TOFU_ROTATE_DOPPLER_CONFIG=bootstrap-admin
}
tofu_rotate_doppler_get_plain() {
  if [[ "\${MOCK_DOPPLER_NETWORK_ERROR:-}" == 1 ]]; then
    return 2
  fi
  local err_file out rc=0
  err_file="\$(mktemp)"
  trap 'rm -f "\${err_file}"' RETURN
  out="\$(doppler secrets get "\$1" --plain -p "\$2" -c "\$3" 2>"\${err_file}")" || rc=\$?
  if [[ \${rc} -eq 0 ]]; then printf '%s' "\${out}"; return 0; fi
  if grep -Eiq 'could not find|not find|not found' "\${err_file}"; then return 2; fi
  cat "\${err_file}" >&2
  return 1
}
tofu_rotate_doppler_secret_empty_or_missing() { return 0; }
tofu_rotate_generate_passphrase() { printf '%s' 'rotation-mut-new-passphrase-b16'; }
tofu_rotate_assert_old_passphrase_fails() { return 0; }
MUT
MUT_ROTATE="${WORKDIR}/rotate-mut-c.sh"
mut_rotate_with_lib "${MUT_LIB}" "${MUT_ROTATE}"
expect_fail "Doppler network treated as missing" "Missing current passphrase" \
  env MOCK_DOPPLER_NETWORK_ERROR=1 bash "${MUT_ROTATE}" "${WORKDIR}/stack" bootstrap

echo "OK: encryption rotation mutation checks passed"
