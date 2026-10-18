#!/usr/bin/env bash
# Clean-checkout E2E: preflight before writes, config split, bootstrap apply and backup, release init and plan.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLONE_DIR="$(mktemp -d)"
TESTDIR="$(mktemp -d)"
trap 'rm -rf "${CLONE_DIR}" "${TESTDIR}"' EXIT

git -C "${ROOT}" archive HEAD | tar -x -C "${CLONE_DIR}"
cp -a "${ROOT}/scripts/." "${CLONE_DIR}/scripts/"
cp -a "${ROOT}/infra/." "${CLONE_DIR}/infra/"
cp -a "${ROOT}/.github/." "${CLONE_DIR}/.github/"
find "${CLONE_DIR}/infra" -name .terraform -type d -prune -exec rm -rf {} +

FAKE_BIN="${TESTDIR}/bin"
mkdir -p "${FAKE_BIN}"
WRITE_LOG="${TESTDIR}/doppler-writes.log"
SECRET_DB="${TESTDIR}/doppler-secrets.tsv"
CURL_EVENTS="${TESTDIR}/curl-events.log"
RUN_LOG="${TESTDIR}/bootstrap-run.log"
: >"${WRITE_LOG}"
: >"${SECRET_DB}"
: >"${CURL_EVENTS}"

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
export E2E_CURL_EVENT_LOG="${CURL_EVENTS}"
export CLOUDFLARE_API_TOKEN=e2e-mock-token
export CLOUDFLARE_ACCOUNT_ID=00000000000000000000000000000001
export GITHUB_TOFU_RELEASE_ENV=e2e-mock-github-token
export ARGV_SHIM_TOFU_TEST_MODE=1
export ARGV_SHIM_TOFU_MOCK_OUTPUT=1
export CF_MOCK_ACCOUNT_VERIFY_OK=1
# shellcheck source=scripts/test/lib/argv-logging-shims.sh
source "${ROOT}/scripts/test/lib/argv-logging-shims.sh"
export ARGV_SHIM_LOG="${TESTDIR}/e2e-argv.log"
argv_shim_install "${FAKE_BIN}" "${ARGV_SHIM_LOG}" tofu

if doppler projects create hub-framework 2>/dev/null; then
  echo "FAIL: Doppler write accepted before any Cloudflare preflight call" >&2
  exit 1
fi
echo "OK: Doppler writes are refused before preflight"

(
  cd "${CLONE_DIR}"
  { yes || true; } | ./scripts/bootstrap-release-infra.sh
) >"${RUN_LOG}" 2>&1 || {
  echo "FAIL: bootstrap exited non-zero" >&2
  cat "${RUN_LOG}" >&2
  exit 1
}

if grep -q 'Error:' "${RUN_LOG}"; then
  echo "FAIL: bootstrap printed an OpenTofu error but still exited zero" >&2
  cat "${RUN_LOG}" >&2
  exit 1
fi

if ! grep -q 'secret set GITHUB_TOFU_RELEASE_ENV hub-framework/bootstrap-admin' "${WRITE_LOG}"; then
  echo "FAIL: E2E did not write GITHUB_TOFU_RELEASE_ENV to bootstrap-admin" >&2
  exit 1
fi

if ! grep -q 'secret set TOFU_BOOTSTRAP_STATE_LOCAL_BACKUP hub-framework/bootstrap-admin' "${WRITE_LOG}"; then
  echo "FAIL: E2E did not back up bootstrap state to bootstrap-admin" >&2
  exit 1
fi

if ! grep -q 'secret set TOFU_STATE_BUCKET hub-framework/infra' "${WRITE_LOG}"; then
  echo "FAIL: R2 backend secrets must be written to infra" >&2
  exit 1
fi

BACKEND_CONFIG="${CLONE_DIR}/infra/opentofu/release-environment/backend-config.hcl"
if ! grep -q '^bucket *= "queerglobal-hub-framework-tofu-state"$' "${BACKEND_CONFIG}" \
  || ! grep -q '^key *= "hub-framework/release-environment/terraform.tfstate"$' "${BACKEND_CONFIG}"; then
  echo "FAIL: backend-config.hcl was not filled from Doppler infra" >&2
  cat "${BACKEND_CONFIG}" >&2
  exit 1
fi

echo "OK: clean-checkout E2E bootstrap path (preflight before writes, config split, backup, release init and plan)"
