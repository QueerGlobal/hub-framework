#!/usr/bin/env bash
# Mutation-proves each Cloudflare preflight gate (break one check, expect failure).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/test/lib/cloudflare-preflight-mock.sh
source "${ROOT}/scripts/test/lib/cloudflare-preflight-mock.sh"

assert_fails() {
  local label=$1
  shift
  if cloudflare_preflight_run_preflight "${ROOT}" "$@"; then
    echo "FAIL: mutation ${label} should have failed preflight" >&2
    exit 1
  fi
  echo "OK: mutation ${label} rejected as expected"
}

assert_passes() {
  local label=$1
  shift
  if ! cloudflare_preflight_run_preflight "${ROOT}" "$@"; then
    echo "FAIL: control case ${label} should pass preflight" >&2
    exit 1
  fi
  echo "OK: control case ${label} passes"
}

BASE=(
  CF_MOCK_USER_VERIFY_OK=0
  CF_MOCK_ACCOUNT_VERIFY_OK=1
  CF_MOCK_ACCOUNT_DETAILS_OK=1
  CF_MOCK_DETAILS_FIXTURE=token-details-account-owned-allow.json
)

assert_passes "full account-owned token" "${BASE[@]}"

assert_fails "both verify endpoints fail" \
  "${BASE[@]}" CF_MOCK_ACCOUNT_VERIFY_OK=0

assert_fails "R2 list capability missing" \
  "${BASE[@]}" CF_MOCK_R2_OK=0

assert_fails "account tokens list capability missing" \
  "${BASE[@]}" CF_MOCK_TOKENS_LIST_OK=0

assert_fails "permission catalog missing" \
  "${BASE[@]}" CF_MOCK_CATALOG_OK=0

assert_fails "allow policy missing R2 write id" \
  CF_MOCK_USER_VERIFY_OK=1 \
  CF_MOCK_USER_DETAILS_OK=1 \
  CF_MOCK_DETAILS_FIXTURE=token-details-missing-r2-allow.json

assert_passes "deny policy on R2 id ignored when allow present" \
  "${BASE[@]}" CF_MOCK_DETAILS_FIXTURE=token-details-deny-r2-allow.json

echo "OK: Cloudflare preflight mutation checks passed"
