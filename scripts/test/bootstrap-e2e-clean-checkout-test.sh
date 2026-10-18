#!/usr/bin/env bash
# Clean-checkout E2E: preflight before writes, config split, bootstrap test, release init.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLONE_DIR="$(mktemp -d)"
TESTDIR="$(mktemp -d)"
trap 'rm -rf "${CLONE_DIR}" "${TESTDIR}"' EXIT

git -C "${ROOT}" archive HEAD | tar -x -C "${CLONE_DIR}"
cp -a "${ROOT}/scripts/." "${CLONE_DIR}/scripts/"
cp -a "${ROOT}/infra/." "${CLONE_DIR}/infra/"
cp -a "${ROOT}/.github/." "${CLONE_DIR}/.github/"

FAKE_BIN="${TESTDIR}/bin"
mkdir -p "${FAKE_BIN}"
WRITE_LOG="${TESTDIR}/doppler-writes.log"
SECRET_DB="${TESTDIR}/doppler-secrets.tsv"
: >"${WRITE_LOG}"
: >"${SECRET_DB}"

# shellcheck source=scripts/test/lib/cloudflare-preflight-mock.sh
source "${ROOT}/scripts/test/lib/cloudflare-preflight-mock.sh"
export CF_PREFLIGHT_FIXTURE_ROOT="${ROOT}/scripts/test/fixtures/cloudflare"
cloudflare_preflight_mock_install "${FAKE_BIN}"

# shellcheck source=scripts/test/lib/e2e-doppler-mock.sh
source "${ROOT}/scripts/test/lib/e2e-doppler-mock.sh"
e2e_doppler_mock_install "${FAKE_BIN}"

export PATH="${FAKE_BIN}:${PATH}"
export E2E_DOPPLER_WRITE_LOG="${WRITE_LOG}"
export E2E_DOPPLER_SECRET_DB="${SECRET_DB}"
export CLOUDFLARE_API_TOKEN=e2e-mock-token
export CLOUDFLARE_ACCOUNT_ID=00000000000000000000000000000001
export GITHUB_TOFU_RELEASE_ENV=e2e-mock-github-token
export BOOTSTRAP_AUTO_CONFIRM=1
export ARGV_SHIM_TOFU_TEST_MODE=1
export ARGV_SHIM_TOFU_MOCK_OUTPUT=1
export BOOTSTRAP_ALLOW_MISSING_LOCAL_STATE_BACKUP=1
# shellcheck source=scripts/test/lib/argv-logging-shims.sh
source "${ROOT}/scripts/test/lib/argv-logging-shims.sh"
export ARGV_SHIM_LOG="${TESTDIR}/e2e-argv.log"
argv_shim_install "${FAKE_BIN}" "${ARGV_SHIM_LOG}" tofu

# Preset legacy env must not bypass the marker file gate.
export BOOTSTRAP_PREFLIGHT_COMPLETE=1
export BOOTSTRAP_PREFLIGHT_MARKER_PATH="${TESTDIR}/preset-env-not-a-real-marker"
if PATH="${FAKE_BIN}:${PATH}" E2E_DOPPLER_WRITE_LOG="${WRITE_LOG}" E2E_DOPPLER_SECRET_DB="${SECRET_DB}" \
  doppler projects create hub-framework --project hub-framework 2>/dev/null; then
  echo "FAIL: preset BOOTSTRAP_PREFLIGHT_COMPLETE must not satisfy Doppler write gate" >&2
  exit 1
fi
echo "OK: preset env var does not bypass preflight marker file gate"
export BOOTSTRAP_DOPPLER_SECRET_DRY_RUN=0
export CF_MOCK_USER_VERIFY_OK=0
export CF_MOCK_ACCOUNT_VERIFY_OK=1

(
  cd "${CLONE_DIR}"
  ./scripts/bootstrap-release-infra.sh
)

if ! grep -q 'secret set GITHUB_TOFU_RELEASE_ENV' "${WRITE_LOG}" \
  || ! grep -q 'bootstrap-admin' "${WRITE_LOG}"; then
  echo "FAIL: E2E did not write GITHUB_TOFU_RELEASE_ENV to bootstrap-admin" >&2
  exit 1
fi

if grep 'TOFU_STATE_BUCKET' "${WRITE_LOG}" | grep -q 'bootstrap-admin'; then
  echo "FAIL: R2 backend secrets must be written to infra, not bootstrap-admin" >&2
  exit 1
fi

echo "OK: clean-checkout E2E bootstrap path (preflight before writes, config split, init paths)"
