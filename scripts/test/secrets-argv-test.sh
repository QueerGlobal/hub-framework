#!/usr/bin/env bash
# Secrets must not appear on child argv (bootstrap, rotation, curl helpers).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/test/lib/argv-logging-shims.sh
source "${ROOT}/scripts/test/lib/argv-logging-shims.sh"

WORKDIR="$(mktemp -d)"
SHIM_BIN="${WORKDIR}/bin"
ARGV_LOG="${WORKDIR}/argv.log"
trap 'rm -rf "${WORKDIR}"' EXIT

GITHUB_SECRET='github-secret-argv-test-token-xyzzy-16'
PASS_A='rotation-argv-test-passphrase-a16'
PASS_B='rotation-argv-test-passphrase-b16'
CF_TOKEN='cloudflare-bearer-argv-test-token-xyzzy'

export ARGV_SHIM_LOG="${ARGV_LOG}"

# shellcheck source=scripts/test/lib/cloudflare-preflight-mock.sh
source "${ROOT}/scripts/test/lib/cloudflare-preflight-mock.sh"
export CF_PREFLIGHT_FIXTURE_ROOT="${ROOT}/scripts/test/fixtures/cloudflare"
cloudflare_preflight_mock_install "${SHIM_BIN}"
argv_shim_install "${SHIM_BIN}" "${ARGV_LOG}" bash gh tofu
export PATH="${SHIM_BIN}:/usr/local/bin:/usr/bin:/bin"

: >"${ARGV_LOG}"
export CLOUDFLARE_API_TOKEN="${CF_TOKEN}"
export CLOUDFLARE_ACCOUNT_ID=00000000000000000000000000000001
export GITHUB_TOFU_RELEASE_ENV="${GITHUB_SECRET}"
export TF_VAR_bootstrap_state_encryption_passphrase="${PASS_A}"
export TF_VAR_state_encryption_passphrase="${PASS_B}"
export BOOTSTRAP_AUTO_CONFIRM=1
export BOOTSTRAP_DOPPLER_SECRET_DRY_RUN=1

if ! bash -c "source '${ROOT}/scripts/lib/preflight.sh'; preflight_cloudflare_bootstrap_api_token '${CF_TOKEN}' '00000000000000000000000000000001'" >/dev/null; then
  echo "FAIL: preflight helper should pass with mocks" >&2
  exit 1
fi

FIXTURE="${ROOT}/infra/opentofu/encryption-rotation-fixture"
ROTATE="${ROOT}/scripts/tofu-rotate-encryption-passphrase.sh"
mkdir -p "${WORKDIR}/stack"
cp "${FIXTURE}/variables.tf" "${FIXTURE}/main.tf" "${FIXTURE}/versions.tf" "${WORKDIR}/stack/"
export TOFU=tofu
export TF_VAR_passphrase="${PASS_A}"
( cd "${WORKDIR}/stack" && tofu init -input=false >/dev/null && tofu apply -input=false -auto-approve >/dev/null )

: >"${ARGV_LOG}"
TOFU_ROTATE_OLD_PASSPHRASE="${PASS_A}" TOFU_ROTATE_NEW_PASSPHRASE="${PASS_B}" \
  "${ROTATE}" "${WORKDIR}/stack" bootstrap

for secret in "${PASS_A}" "${PASS_B}" "${GITHUB_SECRET}" "${CF_TOKEN}" "Bearer ${CF_TOKEN}"; do
  if ! argv_shim_assert_no_secrets "${ARGV_LOG}" "${secret}"; then
    exit 1
  fi
done

# Bootstrap script (doppler run + bash -c tofu paths); use mock tofu after real rotation above.
export ARGV_SHIM_TOFU_TEST_MODE=1
argv_shim_install "${SHIM_BIN}" "${ARGV_LOG}" tofu

E2E_WRITE_LOG="${WORKDIR}/bootstrap-writes.log"
E2E_SECRET_DB="${WORKDIR}/bootstrap-secrets.tsv"
: >"${E2E_WRITE_LOG}"
: >"${E2E_SECRET_DB}"
# shellcheck source=scripts/test/lib/e2e-doppler-mock.sh
source "${ROOT}/scripts/test/lib/e2e-doppler-mock.sh"
e2e_doppler_mock_install "${SHIM_BIN}"
argv_shim_install "${SHIM_BIN}" "${ARGV_LOG}" bash gh tofu
export PATH="${SHIM_BIN}:/usr/local/bin:/usr/bin:/bin"
export E2E_DOPPLER_WRITE_LOG="${E2E_WRITE_LOG}"
export E2E_DOPPLER_SECRET_DB="${E2E_SECRET_DB}"
export BOOTSTRAP_DOPPLER_SECRET_DRY_RUN=0
export ARGV_SHIM_TOFU_MOCK_OUTPUT=1
export BOOTSTRAP_ALLOW_MISSING_LOCAL_STATE_BACKUP=1
export CF_MOCK_USER_VERIFY_OK=0
export CF_MOCK_ACCOUNT_VERIFY_OK=1
export TF_VAR_bootstrap_state_encryption_passphrase="${PASS_A}"
export TF_VAR_state_encryption_passphrase="${PASS_B}"
: >"${ARGV_LOG}"
CLONE_DIR="${WORKDIR}/repo-clone"
mkdir -p "${CLONE_DIR}"
git -C "${ROOT}" archive HEAD | tar -x -C "${CLONE_DIR}"
cp -a "${ROOT}/scripts/." "${CLONE_DIR}/scripts/"
cp -a "${ROOT}/infra/." "${CLONE_DIR}/infra/"
(
  cd "${CLONE_DIR}"
  ./scripts/bootstrap-release-infra.sh
)
for secret in "${PASS_A}" "${PASS_B}" "${GITHUB_SECRET}" "${CF_TOKEN}" "Bearer ${CF_TOKEN}"; do
  if ! argv_shim_assert_no_secrets "${ARGV_LOG}" "${secret}"; then
    echo "FAIL: bootstrap path leaked secret on argv" >&2
    exit 1
  fi
done

# Mutation: leak one secret via curl argv and expect failure.
MUT_LOG="${WORKDIR}/mut.log"
: >"${MUT_LOG}"
export ARGV_SHIM_LOG="${MUT_LOG}"
if bash -c "curl -fsS -H \"Authorization: Bearer ${CF_TOKEN}\" https://example.test/" >/dev/null 2>&1; then
  :
fi
if argv_shim_assert_no_secrets "${MUT_LOG}" "${CF_TOKEN}"; then
  echo "FAIL: mutation with bearer on argv should fail assert" >&2
  exit 1
fi

echo "OK: secrets absent from child argv (rotation, preflight curl, mutation caught leak)"
