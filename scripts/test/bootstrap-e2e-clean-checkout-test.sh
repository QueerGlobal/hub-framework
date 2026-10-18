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

# The passphrases bootstrap generated must never have reached any argv.
GENERATED=()
for name in TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE TOFU_STATE_ENCRYPTION_PASSPHRASE; do
  value="$(awk -F '\t' -v n="${name}" '$2 == n { v = $5 } END { print v }' "${SECRET_DB}")"
  if [[ ${#value} -lt 32 ]]; then
    echo "FAIL: bootstrap did not generate ${name}" >&2
    exit 1
  fi
  GENERATED+=("${value}")
done
argv_shim_assert_no_secrets "${ARGV_SHIM_LOG}" "bootstrap generated passphrase" "${GENERATED[@]}"
echo "OK: generated passphrases stayed off argv"

# A Doppler read error on a stored passphrase must stop bootstrap, never regenerate it.
: >"${WRITE_LOG}"
rc=0
(
  cd "${CLONE_DIR}"
  { yes || true; } | E2E_DOPPLER_FAIL_GET=TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE ./scripts/bootstrap-release-infra.sh
) >"${RUN_LOG}" 2>&1 || rc=$?
if [[ ${rc} -eq 0 ]]; then
  echo "FAIL: bootstrap continued after a Doppler error reading the bootstrap passphrase" >&2
  exit 1
fi
if grep -q 'secret set TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE ' "${WRITE_LOG}"; then
  echo "FAIL: bootstrap overwrote the bootstrap passphrase after a Doppler read error" >&2
  exit 1
fi
if ! grep -q 'Doppler error while reading TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE' "${RUN_LOG}"; then
  echo "FAIL: bootstrap stopped after a Doppler read error without naming it" >&2
  cat "${RUN_LOG}" >&2
  exit 1
fi
echo "OK: a Doppler read error stops bootstrap without regenerating the passphrase"

echo "OK: clean-checkout E2E bootstrap path (preflight before writes, config split, backup, release init and plan)"
