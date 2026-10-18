#!/usr/bin/env bash
# Absent and empty Doppler secrets count as missing; a Doppler error is never read as missing,
# so bootstrap never generates a passphrase over one it could not read.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "${WORKDIR}"' EXIT
mkdir -p "${WORKDIR}/bin"
WRITES="${WORKDIR}/writes.log"
: >"${WRITES}"
# Behaves like the real CLI: with --no-exit-on-missing-secret an absent secret prints an empty line
# and exits 0; an auth error exits 1 with text that mentions "not found".
cat >"${WORKDIR}/bin/doppler" <<'MOCK'
#!/usr/bin/bash
set -euo pipefail
if [[ "${1:-} ${2:-}" == "secrets set" ]]; then
  cat >/dev/null
  echo "set ${3:-}" >>"${MOCK_WRITES:?}"
  exit 0
fi
[[ " $* " == *" --no-exit-on-missing-secret "* ]] || { echo "Doppler Error: Could not find requested secret" >&2; exit 1; }
case "${MOCK_SECRET_STATE:?}" in
  present) echo 'stored-value-0123456789' ;;
  empty|absent) echo ;;
  error) echo "Doppler Error: Unable to authenticate: token not found (HTTP 404)" >&2; exit 1 ;;
esac
MOCK
chmod +x "${WORKDIR}/bin/doppler"
export PATH="${WORKDIR}/bin:${PATH}" MOCK_WRITES="${WRITES}"
# shellcheck source=scripts/lib/bootstrap-doppler-sync.sh
source "${ROOT}/scripts/lib/bootstrap-doppler-sync.sh"

expect_state() {
  local state=$1 want=$2 got rc=0
  got="$(MOCK_SECRET_STATE="${state}" doppler_secret_state NAME hub-framework infra 2>"${WORKDIR}/err")" || rc=$?
  if [[ "${want}" == error ]]; then
    if [[ ${rc} -eq 0 ]]; then
      echo "FAIL: Doppler secret state error was read as ${got}" >&2
      exit 1
    fi
    if ! grep -q 'Doppler error while reading NAME from hub-framework/infra' "${WORKDIR}/err"; then
      echo "FAIL: Doppler secret state error missing reason" >&2
      exit 1
    fi
  elif [[ ${rc} -ne 0 || "${got}" != "${want}" ]]; then
    echo "FAIL: Doppler secret state ${state} was treated as ${got:-an error}, expected ${want}" >&2
    exit 1
  fi
  echo "OK: Doppler secret state ${state} counts as ${want}"
}

expect_state absent missing
expect_state empty missing
expect_state present present
expect_state error error

no_confirm() { :; }

expect_generate() {
  local state=$1 want_writes=$2 rc=0
  : >"${WRITES}"
  MOCK_SECRET_STATE="${state}" bootstrap_ensure_generated_passphrase PASS_NAME hub-framework infra no_confirm \
    2>/dev/null || rc=$?
  if [[ "$(grep -c '^set PASS_NAME$' "${WRITES}")" != "${want_writes}" ]]; then
    echo "FAIL: passphrase generation with Doppler state ${state} did not make exactly ${want_writes} write(s)" >&2
    exit 1
  fi
  if [[ "${state}" == error && ${rc} -eq 0 ]]; then
    echo "FAIL: passphrase generation succeeded after a Doppler read error" >&2
    exit 1
  fi
  echo "OK: passphrase generation with Doppler state ${state} wrote ${want_writes}"
}

expect_generate absent 1
expect_generate empty 1
expect_generate present 0
expect_generate error 0
# The sync must stop on a Doppler error instead of reading the stored GitHub token as present or missing.
rc=0
: >"${WRITES}"
MOCK_SECRET_STATE=error GITHUB_TOFU_RELEASE_ENV='' CLOUDFLARE_ACCOUNT_ID='' \
  bootstrap_sync_env_secrets_to_doppler hub-framework infra bootstrap-admin no_confirm 2>"${WORKDIR}/err" || rc=$?
if [[ ${rc} -eq 0 || -s "${WRITES}" ]]; then
  echo "FAIL: bootstrap sync passed or wrote after a Doppler error reading GITHUB_TOFU_RELEASE_ENV" >&2
  exit 1
fi
if ! grep -q 'Doppler error while reading GITHUB_TOFU_RELEASE_ENV from hub-framework/bootstrap-admin' "${WORKDIR}/err"; then
  echo "FAIL: bootstrap sync stopped after a Doppler error without naming it" >&2
  exit 1
fi
echo "OK: bootstrap sync stops on a Doppler error"
echo "OK: bootstrap Doppler secret checks treat empty values as missing and errors as errors"
