#!/usr/bin/env bash
# Mutation-proves GitHub token preflight before bootstrap writes.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/test/lib/cloudflare-preflight-mock.sh
source "${ROOT}/scripts/test/lib/cloudflare-preflight-mock.sh"

assert_fails_with() {
  local label=$1 needle=$2
  shift 2
  local out rc=0
  out="$(github_preflight_run "${ROOT}" "$@" 2>&1)" || rc=$?
  [[ $rc -ne 0 ]] || { echo "FAIL: ${label} should fail" >&2; exit 1; }
  grep -Fq "${needle}" <<<"${out}" || {
    echo "FAIL: ${label} missing reason: ${needle}" >&2
    echo "${out}" >&2
    exit 1
  }
  echo "OK: mutation ${label} rejected with expected reason"
}

assert_passes() {
  local label=$1
  shift
  local out rc=0
  out="$(github_preflight_run "${ROOT}" "$@" 2>&1)" || rc=$?
  [[ $rc -eq 0 ]] || {
    echo "FAIL: control case ${label} should pass" >&2
    echo "${out}" >&2
    exit 1
  }
  echo "OK: control case ${label} passes"
}

github_preflight_run() {
  local root=$1
  shift
  local -a env_exports=("$@")
  local testdir fake_bin rc=0 out
  testdir="$(mktemp -d)"
  fake_bin="${testdir}/bin"
  export CF_PREFLIGHT_FIXTURE_ROOT="${root}/scripts/test/fixtures/cloudflare"
  cloudflare_preflight_mock_install "${fake_bin}"
  out="$(
    (
      export PATH="${fake_bin}:${PATH}"
      for kv in "${env_exports[@]}"; do
        export "${kv?}"
      done
      # shellcheck source=scripts/lib/preflight.sh
      source "${root}/scripts/lib/preflight.sh"
      preflight_github_tofu_release_env hub-framework bootstrap-admin
    ) 2>&1
  )" || rc=$?
  rm -rf "${testdir}"
  printf '%s' "${out}"
  return "${rc}"
}

BASE=(
  GITHUB_TOFU_RELEASE_ENV=mock-github-pat-for-preflight-test
  GITHUB_MOCK_USER_OK=1
)

assert_passes "github user API" "${BASE[@]}"

assert_fails_with "invalid github token" \
  "not a valid GitHub token (user API returned no login)" \
  "${BASE[@]}" GITHUB_MOCK_USER_OK=0

echo "OK: GitHub token preflight mutation checks passed"
