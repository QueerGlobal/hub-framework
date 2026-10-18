#!/usr/bin/env bash
# Secrets must not appear on child argv (preflight curl, rotation, full bootstrap).
set -euo pipefail

unset ARGV_SHIM_TOFU_TEST_MODE ARGV_SHIM_TOFU_MOCK_OUTPUT

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/test/lib/argv-logging-shims.sh
source "${ROOT}/scripts/test/lib/argv-logging-shims.sh"
# shellcheck source=scripts/test/lib/cloudflare-preflight-mock.sh
source "${ROOT}/scripts/test/lib/cloudflare-preflight-mock.sh"
# shellcheck source=scripts/test/lib/e2e-doppler-mock.sh
source "${ROOT}/scripts/test/lib/e2e-doppler-mock.sh"

WORKDIR="$(mktemp -d)"
SHIM_BIN="${WORKDIR}/bin"
ARGV_LOG="${WORKDIR}/argv.log"
trap 'rm -rf "${WORKDIR}"' EXIT

GITHUB_SECRET='github-secret-argv-test-token-xyzzy-16'
PASS_A='rotation-argv-test-passphrase-a16'
PASS_B='rotation-argv-test-passphrase-b16'
CF_TOKEN='cloudflare-bearer-argv-test-token-xyzzy'
SECRETS=("${PASS_A}" "${PASS_B}" "${GITHUB_SECRET}" "${CF_TOKEN}")
ACCOUNT=00000000000000000000000000000001

export ARGV_SHIM_LOG="${ARGV_LOG}"
export CF_PREFLIGHT_FIXTURE_ROOT="${ROOT}/scripts/test/fixtures/cloudflare"
export E2E_DOPPLER_WRITE_LOG="${WORKDIR}/bootstrap-writes.log"
export E2E_DOPPLER_SECRET_DB="${WORKDIR}/bootstrap-secrets.tsv"
export E2E_CURL_EVENT_LOG="${WORKDIR}/curl-events.log"
: >"${E2E_DOPPLER_WRITE_LOG}"
: >"${E2E_DOPPLER_SECRET_DB}"
: >"${E2E_CURL_EVENT_LOG}"
cloudflare_preflight_mock_install "${SHIM_BIN}"
e2e_doppler_mock_install "${SHIM_BIN}"
argv_shim_install "${SHIM_BIN}" "${ARGV_LOG}" bash gh tofu jq
export PATH="${SHIM_BIN}:/usr/local/bin:/usr/bin:/bin"

export CLOUDFLARE_API_TOKEN="${CF_TOKEN}"
export CLOUDFLARE_ACCOUNT_ID="${ACCOUNT}"
export GITHUB_TOFU_RELEASE_ENV="${GITHUB_SECRET}"
export CF_MOCK_ACCOUNT_VERIFY_OK=1

# Runs Cloudflare preflight from a copy of scripts/lib, so a mutant can be swapped in.
run_preflight_from() {
  local lib_dir=$1
  : >"${ARGV_LOG}"
  bash -c 'source "$1/preflight.sh"; preflight_cloudflare_bootstrap_api_token "${CLOUDFLARE_API_TOKEN}" "${CLOUDFLARE_ACCOUNT_ID}"' \
    _ "${lib_dir}" >/dev/null
}

# Segment 1: preflight curl calls (baseline for the mutation below).
run_preflight_from "${ROOT}/scripts/lib"
argv_shim_assert_no_secrets "${ARGV_LOG}" "preflight" "${SECRETS[@]}"
echo "OK: preflight keeps the Cloudflare token off argv"

# Mutation: put the bearer header back on curl argv in a copy of the library; the check must name curl.
MUTANT_LIB="${WORKDIR}/mutant-lib"
cp -a "${ROOT}/scripts/lib" "${MUTANT_LIB}"
sed -i 's|printf '"'"'Authorization: Bearer %s\\n'"'"' "${token}" \| curl -fsS -H @- "$@"|curl -fsS -H "Authorization: Bearer ${token}" "$@"|' \
  "${MUTANT_LIB}/curl-bearer-auth.sh"
if cmp -s "${ROOT}/scripts/lib/curl-bearer-auth.sh" "${MUTANT_LIB}/curl-bearer-auth.sh"; then
  echo "FAIL: argv mutant was not applied" >&2
  exit 1
fi
run_preflight_from "${MUTANT_LIB}"
leak_tool="$(argv_shim_find_leak "${ARGV_LOG}" "${CF_TOKEN}" || true)"
if [[ "${leak_tool}" != "curl" ]]; then
  echo "FAIL: argv mutant was not caught on curl (got '${leak_tool}')" >&2
  exit 1
fi
echo "OK: argv mutant caught: bearer token on curl argv"

MUTANT_PREFLIGHT="${WORKDIR}/mutant-preflight-lib"
cp -a "${ROOT}/scripts/lib" "${MUTANT_PREFLIGHT}"
sed -i 's|curl_with_bearer "${token}"|curl -fsS -H "Authorization: Bearer ${token}"|g' \
  "${MUTANT_PREFLIGHT}/preflight.sh"
run_preflight_from "${MUTANT_PREFLIGHT}"
leak_tool="$(argv_shim_find_leak "${ARGV_LOG}" "${CF_TOKEN}" || true)"
if [[ "${leak_tool}" != "curl" ]]; then
  echo "FAIL: preflight direct-curl mutant was not caught on curl (got '${leak_tool}')" >&2
  exit 1
fi
msg="$(argv_shim_assert_no_secrets "${ARGV_LOG}" "preflight" "${CF_TOKEN}" 2>&1)" || true
if ! grep -Fq 'preflight: a secret reached the argv of curl' <<<"${msg}"; then
  echo "FAIL: preflight direct-curl mutant missing expected reason" >&2
  echo "${msg}" >&2
  exit 1
fi
echo "OK: argv mutant caught: direct bearer in preflight_cloudflare_bootstrap_api_token"

# Segment 2: passphrase rotation with real OpenTofu and mock Doppler.
FIXTURE="${ROOT}/infra/opentofu/encryption-rotation-fixture"
ROT_DB="${WORKDIR}/rotation-doppler.tsv"
: >"${ROT_DB}"
# shellcheck source=scripts/test/lib/rotation-doppler-mock.sh
source "${ROOT}/scripts/test/lib/rotation-doppler-mock.sh"
rotation_doppler_mock_install "${SHIM_BIN}" "${ROT_DB}"
rotation_doppler_seed "${ROT_DB}" TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE hub-framework bootstrap-admin "${PASS_A}"
export ROTATION_DOPPLER_DB="${ROT_DB}" DOPPLER_PROJECT=hub-framework DOPPLER_CONFIG=bootstrap-admin
mkdir -p "${WORKDIR}/stack"
cp "${FIXTURE}/variables.tf" "${FIXTURE}/main.tf" "${FIXTURE}/versions.tf" "${WORKDIR}/stack/"
REAL_TOFU="$(PATH="/usr/local/bin:/usr/bin:/bin" command -v tofu)"
export TOFU="${REAL_TOFU}"
( cd "${WORKDIR}/stack" && TF_VAR_passphrase="${PASS_A}" "${REAL_TOFU}" init -input=false >/dev/null \
  && TF_VAR_passphrase="${PASS_A}" "${REAL_TOFU}" apply -input=false -auto-approve >/dev/null )
: >"${ARGV_LOG}"
"${ROOT}/scripts/tofu-rotate-encryption-passphrase.sh" "${WORKDIR}/stack" bootstrap >/dev/null
argv_shim_assert_no_secrets "${ARGV_LOG}" "rotation" "${SECRETS[@]}"
echo "OK: rotation keeps passphrases off argv"

# Segment 3: full bootstrap (doppler run bodies, tofu, curl) with test-mode tofu.
e2e_doppler_mock_install "${SHIM_BIN}"
export DOPPLER_PROJECT=hub-framework DOPPLER_CONFIG=infra
export ARGV_SHIM_TOFU_TEST_MODE=1
export ARGV_SHIM_TOFU_MOCK_OUTPUT=1
CLONE_DIR="${WORKDIR}/repo-clone"
mkdir -p "${CLONE_DIR}"
git -C "${ROOT}" archive HEAD | tar -x -C "${CLONE_DIR}"
cp -a "${ROOT}/scripts/." "${CLONE_DIR}/scripts/"
cp -a "${ROOT}/infra/." "${CLONE_DIR}/infra/"
find "${CLONE_DIR}/infra" -name .terraform -type d -prune -exec rm -rf {} +
printf '%s\t%s\t%s\t%s\t%s\n' 0 TOFU_BOOTSTRAP_STATE_ENCRYPTION_PASSPHRASE hub-framework bootstrap-admin "${PASS_A}" \
  >>"${E2E_DOPPLER_SECRET_DB}"
printf '%s\t%s\t%s\t%s\t%s\n' 0 TOFU_STATE_ENCRYPTION_PASSPHRASE hub-framework infra "${PASS_B}" \
  >>"${E2E_DOPPLER_SECRET_DB}"
: >"${ARGV_LOG}"
( cd "${CLONE_DIR}" && { yes || true; } | ./scripts/bootstrap-release-infra.sh >/dev/null )
argv_shim_assert_no_secrets "${ARGV_LOG}" "bootstrap" "${SECRETS[@]}"
for tool in bash curl doppler jq tofu; do
  if ! grep -q "^SHIM:${tool} " "${ARGV_LOG}"; then
    echo "FAIL: bootstrap segment never ran ${tool}, so its argv was not checked" >&2
    exit 1
  fi
done

MUTANT_CLONE="${WORKDIR}/mutant-clone"
rm -rf "${MUTANT_CLONE}"
mkdir -p "${MUTANT_CLONE}"
git -C "${ROOT}" archive HEAD | tar -x -C "${MUTANT_CLONE}"
cp -a "${ROOT}/scripts/." "${MUTANT_CLONE}/scripts/"
cp -a "${ROOT}/infra/." "${MUTANT_CLONE}/infra/"
find "${MUTANT_CLONE}/infra" -name .terraform -type d -prune -exec rm -rf {} +
sed -i '/confirm "Apply release-environment plan?"/,/tofu apply -input=false "\${RELEASE_PLAN}"/{
  s|^doppler run -p "\${DOPPLER_PROJECT_NAME}" -c "\${DOPPLER_INFRA_CONFIG}" -- bash|doppler run -p "${DOPPLER_PROJECT_NAME}" -c "${DOPPLER_INFRA_CONFIG}" "${TF_VAR_state_encryption_passphrase}" -- bash|
}' "${MUTANT_CLONE}/scripts/bootstrap-release-infra.sh"
: >"${ARGV_LOG}"
( cd "${MUTANT_CLONE}" && { yes || true; } | ./scripts/bootstrap-release-infra.sh >/dev/null 2>&1 ) || true
msg="$(argv_shim_assert_no_secrets "${ARGV_LOG}" "bootstrap" "${PASS_B}" 2>&1)" || true
if ! grep -Fq 'bootstrap: a secret reached the argv of doppler' <<<"${msg}"; then
  echo "FAIL: apply-body passphrase mutant was not caught on doppler" >&2
  echo "${msg}" >&2
  exit 1
fi
echo "OK: argv mutant caught: passphrase spliced into doppler apply argv"

echo "OK: secrets absent from child argv (preflight, rotation, bootstrap; curl mutant caught)"
